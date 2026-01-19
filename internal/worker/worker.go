package worker

import (
	"context"
	"log"
	"time"

	"github.com/cwygoda/catcher/internal/adapter/processor"
	"github.com/cwygoda/catcher/internal/domain"
)

// Worker polls for pending jobs and processes them.
type Worker struct {
	svc          *domain.JobService
	registry     *processor.Registry
	pollInterval time.Duration
	maxRetries   int
}

// New creates a new worker.
func New(svc *domain.JobService, registry *processor.Registry, pollInterval time.Duration, maxRetries int) *Worker {
	return &Worker{
		svc:          svc,
		registry:     registry,
		pollInterval: pollInterval,
		maxRetries:   maxRetries,
	}
}

// Run starts the worker loop until context is cancelled.
func (w *Worker) Run(ctx context.Context) {
	log.Printf("worker started, polling every %s", w.pollInterval)
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("worker shutting down")
			return
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

func (w *Worker) poll(ctx context.Context) {
	jobs, err := w.svc.GetPending(ctx, 10)
	if err != nil {
		log.Printf("poll error: %v", err)
		return
	}

	for _, job := range jobs {
		if ctx.Err() != nil {
			return
		}
		w.processJob(ctx, &job)
	}
}

func (w *Worker) processJob(ctx context.Context, job *domain.Job) {
	proc := w.registry.Match(job.URL)
	if proc == nil {
		log.Printf("job %d: no processor for URL %s", job.ID, job.URL)
		w.svc.MarkFailed(ctx, job.ID, "no processor for URL")
		return
	}

	if err := w.svc.MarkProcessing(ctx, job.ID); err != nil {
		log.Printf("job %d: claim failed: %v", job.ID, err)
		return
	}

	log.Printf("job %d: processing with %s", job.ID, proc.Name())

	// Refresh job to get updated attempts count
	job, err := w.svc.Get(ctx, job.ID)
	if err != nil {
		log.Printf("job %d: refresh failed: %v", job.ID, err)
		return
	}

	if err := proc.Process(ctx, job); err != nil {
		log.Printf("job %d: process error: %v", job.ID, err)
		if job.CanRetry(w.maxRetries) {
			w.svc.MarkRetry(ctx, job.ID, err.Error())
		} else {
			w.svc.MarkFailed(ctx, job.ID, err.Error())
		}
		return
	}

	log.Printf("job %d: completed with %s for %s", job.ID, proc.Name(), job.URL)
	w.svc.MarkComplete(ctx, job.ID)
}
