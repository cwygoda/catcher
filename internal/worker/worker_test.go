package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cwygoda/catcher/internal/adapter/processor"
	"github.com/cwygoda/catcher/internal/domain"
)

// mockRepo implements domain.JobRepository for testing.
type mockRepo struct {
	mu     sync.Mutex
	jobs   map[int64]*domain.Job
	nextID int64
}

func newMockRepo() *mockRepo {
	return &mockRepo{jobs: make(map[int64]*domain.Job), nextID: 1}
}

func (m *mockRepo) Create(ctx context.Context, url string) (*domain.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := &domain.Job{
		ID:        m.nextID,
		URL:       url,
		Status:    domain.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.jobs[m.nextID] = job
	m.nextID++
	return job, nil
}

func (m *mockRepo) Get(ctx context.Context, id int64) (*domain.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return nil, domain.ErrJobNotFound
	}
	// Return a copy
	copy := *job
	return &copy, nil
}

func (m *mockRepo) FindPending(ctx context.Context, limit int) ([]domain.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []domain.Job
	for _, job := range m.jobs {
		if job.Status == domain.StatusPending {
			result = append(result, *job)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *mockRepo) Claim(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok || job.Status != domain.StatusPending {
		return domain.ErrJobNotFound
	}
	job.Status = domain.StatusProcessing
	job.Attempts++
	job.UpdatedAt = time.Now()
	return nil
}

func (m *mockRepo) Complete(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return domain.ErrJobNotFound
	}
	job.Status = domain.StatusCompleted
	job.UpdatedAt = time.Now()
	return nil
}

func (m *mockRepo) Fail(ctx context.Context, id int64, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return domain.ErrJobNotFound
	}
	job.Status = domain.StatusFailed
	job.Error = reason
	job.UpdatedAt = time.Now()
	return nil
}

func (m *mockRepo) Retry(ctx context.Context, id int64, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return domain.ErrJobNotFound
	}
	job.Status = domain.StatusPending
	job.Error = reason
	job.UpdatedAt = time.Now()
	return nil
}

func (m *mockRepo) RecoverStale(ctx context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var count int64
	for _, job := range m.jobs {
		if job.Status == domain.StatusProcessing {
			job.Status = domain.StatusPending
			count++
		}
	}
	return count, nil
}

func (m *mockRepo) getJob(id int64) *domain.Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.jobs[id]
}

// mockProcessor implements domain.URLProcessor for testing.
type mockProcessor struct {
	name       string
	matchFunc  func(string) bool
	processErr error
	processed  []int64
	mu         sync.Mutex
}

func (p *mockProcessor) Name() string { return p.name }
func (p *mockProcessor) Match(url string) bool {
	if p.matchFunc != nil {
		return p.matchFunc(url)
	}
	return true
}
func (p *mockProcessor) Process(ctx context.Context, job *domain.Job) error {
	p.mu.Lock()
	p.processed = append(p.processed, job.ID)
	p.mu.Unlock()
	return p.processErr
}

func TestWorker_ProcessJob_Success(t *testing.T) {
	repo := newMockRepo()
	svc := domain.NewJobService(repo)
	registry := processor.NewRegistry()

	proc := &mockProcessor{name: "test"}
	registry.Register(proc)

	w := New(svc, registry, 100*time.Millisecond, 3)

	// Create a job
	job, _ := repo.Create(context.Background(), "https://example.com")

	// Process it directly
	ctx := context.Background()
	w.processJob(ctx, job)

	// Verify completed
	updated := repo.getJob(job.ID)
	if updated.Status != domain.StatusCompleted {
		t.Errorf("status = %q, want %q", updated.Status, domain.StatusCompleted)
	}
}

func TestWorker_ProcessJob_NoProcessor(t *testing.T) {
	repo := newMockRepo()
	svc := domain.NewJobService(repo)
	registry := processor.NewRegistry() // Empty registry

	w := New(svc, registry, 100*time.Millisecond, 3)

	job, _ := repo.Create(context.Background(), "https://example.com")

	ctx := context.Background()
	w.processJob(ctx, job)

	updated := repo.getJob(job.ID)
	if updated.Status != domain.StatusFailed {
		t.Errorf("status = %q, want %q", updated.Status, domain.StatusFailed)
	}
	if updated.Error != "no processor for URL" {
		t.Errorf("error = %q, want %q", updated.Error, "no processor for URL")
	}
}

func TestWorker_ProcessJob_Retry(t *testing.T) {
	repo := newMockRepo()
	svc := domain.NewJobService(repo)
	registry := processor.NewRegistry()

	proc := &mockProcessor{name: "test", processErr: errors.New("temporary error")}
	registry.Register(proc)

	w := New(svc, registry, 100*time.Millisecond, 3)

	job, _ := repo.Create(context.Background(), "https://example.com")

	ctx := context.Background()
	w.processJob(ctx, job)

	// Should be pending (for retry) since attempts < maxRetries
	updated := repo.getJob(job.ID)
	if updated.Status != domain.StatusPending {
		t.Errorf("status = %q, want %q (retry)", updated.Status, domain.StatusPending)
	}
	if updated.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", updated.Attempts)
	}
}

func TestWorker_ProcessJob_MaxRetriesExceeded(t *testing.T) {
	repo := newMockRepo()
	svc := domain.NewJobService(repo)
	registry := processor.NewRegistry()

	proc := &mockProcessor{name: "test", processErr: errors.New("permanent error")}
	registry.Register(proc)

	w := New(svc, registry, 100*time.Millisecond, 3)

	job, _ := repo.Create(context.Background(), "https://example.com")

	ctx := context.Background()

	// Process 3 times to exceed max retries
	for i := 0; i < 3; i++ {
		// Get fresh job state
		current := repo.getJob(job.ID)
		w.processJob(ctx, current)
	}

	updated := repo.getJob(job.ID)
	if updated.Status != domain.StatusFailed {
		t.Errorf("status = %q, want %q", updated.Status, domain.StatusFailed)
	}
}

func TestWorker_Run_Cancellation(t *testing.T) {
	repo := newMockRepo()
	svc := domain.NewJobService(repo)
	registry := processor.NewRegistry()

	w := New(svc, registry, 50*time.Millisecond, 3)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Cancel and verify it stops
	cancel()

	select {
	case <-done:
		// Good, worker stopped
	case <-time.After(time.Second):
		t.Error("worker did not stop after context cancellation")
	}
}

func TestWorker_Poll_ProcessesJobs(t *testing.T) {
	repo := newMockRepo()
	svc := domain.NewJobService(repo)
	registry := processor.NewRegistry()

	proc := &mockProcessor{name: "test"}
	registry.Register(proc)

	w := New(svc, registry, 100*time.Millisecond, 3)

	// Create jobs
	repo.Create(context.Background(), "https://example.com/1")
	repo.Create(context.Background(), "https://example.com/2")

	ctx := context.Background()
	w.poll(ctx)

	proc.mu.Lock()
	processedCount := len(proc.processed)
	proc.mu.Unlock()

	if processedCount != 2 {
		t.Errorf("processed %d jobs, want 2", processedCount)
	}
}
