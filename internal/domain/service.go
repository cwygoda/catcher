package domain

import (
	"context"
	"errors"
	"net/url"
)

var (
	ErrInvalidURL = errors.New("invalid URL")
	ErrJobNotFound = errors.New("job not found")
)

// JobService orchestrates job operations.
type JobService struct {
	repo JobRepository
}

// NewJobService creates a new JobService.
func NewJobService(repo JobRepository) *JobService {
	return &JobService{repo: repo}
}

// Submit creates a new job for the given URL.
func (s *JobService) Submit(ctx context.Context, rawURL string) (*Job, error) {
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return nil, ErrInvalidURL
	}
	return s.repo.Create(ctx, rawURL)
}

// Get retrieves a job by ID.
func (s *JobService) Get(ctx context.Context, id int64) (*Job, error) {
	return s.repo.Get(ctx, id)
}

// GetPending retrieves pending jobs up to the limit.
func (s *JobService) GetPending(ctx context.Context, limit int) ([]Job, error) {
	return s.repo.FindPending(ctx, limit)
}

// MarkProcessing claims a job for processing.
func (s *JobService) MarkProcessing(ctx context.Context, id int64) error {
	return s.repo.Claim(ctx, id)
}

// MarkComplete marks a job as completed.
func (s *JobService) MarkComplete(ctx context.Context, id int64) error {
	return s.repo.Complete(ctx, id)
}

// MarkFailed marks a job as permanently failed.
func (s *JobService) MarkFailed(ctx context.Context, id int64, reason string) error {
	return s.repo.Fail(ctx, id, reason)
}

// MarkRetry marks a job for retry with error info.
func (s *JobService) MarkRetry(ctx context.Context, id int64, reason string) error {
	return s.repo.Retry(ctx, id, reason)
}

// RecoverStale resets stale processing jobs (crash recovery).
func (s *JobService) RecoverStale(ctx context.Context) (int64, error) {
	return s.repo.RecoverStale(ctx)
}
