package domain

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockRepo implements JobRepository for testing.
type mockRepo struct {
	jobs      map[int64]*Job
	nextID    int64
	createErr error
	getErr    error
	findErr   error
	claimErr  error
}

func newMockRepo() *mockRepo {
	return &mockRepo{jobs: make(map[int64]*Job), nextID: 1}
}

func (m *mockRepo) Create(ctx context.Context, url string) (*Job, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	job := &Job{
		ID:        m.nextID,
		URL:       url,
		Status:    StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.jobs[m.nextID] = job
	m.nextID++
	return job, nil
}

func (m *mockRepo) Get(ctx context.Context, id int64) (*Job, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	job, ok := m.jobs[id]
	if !ok {
		return nil, ErrJobNotFound
	}
	return job, nil
}

func (m *mockRepo) FindPending(ctx context.Context, limit int) ([]Job, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	var result []Job
	for _, job := range m.jobs {
		if job.Status == StatusPending {
			result = append(result, *job)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *mockRepo) Claim(ctx context.Context, id int64) error {
	if m.claimErr != nil {
		return m.claimErr
	}
	job, ok := m.jobs[id]
	if !ok {
		return ErrJobNotFound
	}
	job.Status = StatusProcessing
	job.Attempts++
	job.UpdatedAt = time.Now()
	return nil
}

func (m *mockRepo) Complete(ctx context.Context, id int64) error {
	job, ok := m.jobs[id]
	if !ok {
		return ErrJobNotFound
	}
	job.Status = StatusCompleted
	job.UpdatedAt = time.Now()
	return nil
}

func (m *mockRepo) Fail(ctx context.Context, id int64, reason string) error {
	job, ok := m.jobs[id]
	if !ok {
		return ErrJobNotFound
	}
	job.Status = StatusFailed
	job.Error = reason
	job.UpdatedAt = time.Now()
	return nil
}

func (m *mockRepo) Retry(ctx context.Context, id int64, reason string) error {
	job, ok := m.jobs[id]
	if !ok {
		return ErrJobNotFound
	}
	job.Status = StatusPending
	job.Error = reason
	job.UpdatedAt = time.Now()
	return nil
}

func (m *mockRepo) RecoverStale(ctx context.Context) (int64, error) {
	var count int64
	for _, job := range m.jobs {
		if job.Status == StatusProcessing {
			job.Status = StatusPending
			count++
		}
	}
	return count, nil
}

func TestJobService_Submit(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr error
	}{
		{
			name:    "valid URL",
			url:     "https://example.com/video",
			wantErr: nil,
		},
		{
			name:    "invalid URL",
			url:     "not a url",
			wantErr: ErrInvalidURL,
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: ErrInvalidURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepo()
			svc := NewJobService(repo)

			job, err := svc.Submit(context.Background(), tt.url)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Submit() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && job == nil {
				t.Error("Submit() returned nil job for valid URL")
			}
			if tt.wantErr == nil && job.URL != tt.url {
				t.Errorf("Submit() job.URL = %q, want %q", job.URL, tt.url)
			}
		})
	}
}

func TestJobService_Get(t *testing.T) {
	repo := newMockRepo()
	svc := NewJobService(repo)
	ctx := context.Background()

	// Create a job first
	created, _ := svc.Submit(ctx, "https://example.com")

	// Get existing job
	job, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if job.ID != created.ID {
		t.Errorf("Get() job.ID = %d, want %d", job.ID, created.ID)
	}

	// Get non-existent job
	_, err = svc.Get(ctx, 999)
	if !errors.Is(err, ErrJobNotFound) {
		t.Errorf("Get() error = %v, want %v", err, ErrJobNotFound)
	}
}

func TestJobService_GetPending(t *testing.T) {
	repo := newMockRepo()
	svc := NewJobService(repo)
	ctx := context.Background()

	// Create multiple jobs
	svc.Submit(ctx, "https://example.com/1")
	svc.Submit(ctx, "https://example.com/2")
	svc.Submit(ctx, "https://example.com/3")

	// Get with limit
	jobs, err := svc.GetPending(ctx, 2)
	if err != nil {
		t.Fatalf("GetPending() error = %v", err)
	}
	if len(jobs) > 2 {
		t.Errorf("GetPending() returned %d jobs, want <= 2", len(jobs))
	}
}

func TestJobService_MarkProcessing(t *testing.T) {
	repo := newMockRepo()
	svc := NewJobService(repo)
	ctx := context.Background()

	job, _ := svc.Submit(ctx, "https://example.com")

	err := svc.MarkProcessing(ctx, job.ID)
	if err != nil {
		t.Fatalf("MarkProcessing() error = %v", err)
	}

	updated, _ := svc.Get(ctx, job.ID)
	if updated.Status != StatusProcessing {
		t.Errorf("Status = %q, want %q", updated.Status, StatusProcessing)
	}
}

func TestJobService_MarkComplete(t *testing.T) {
	repo := newMockRepo()
	svc := NewJobService(repo)
	ctx := context.Background()

	job, _ := svc.Submit(ctx, "https://example.com")
	svc.MarkProcessing(ctx, job.ID)

	err := svc.MarkComplete(ctx, job.ID)
	if err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}

	updated, _ := svc.Get(ctx, job.ID)
	if updated.Status != StatusCompleted {
		t.Errorf("Status = %q, want %q", updated.Status, StatusCompleted)
	}
}

func TestJobService_MarkFailed(t *testing.T) {
	repo := newMockRepo()
	svc := NewJobService(repo)
	ctx := context.Background()

	job, _ := svc.Submit(ctx, "https://example.com")
	svc.MarkProcessing(ctx, job.ID)

	err := svc.MarkFailed(ctx, job.ID, "download failed")
	if err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}

	updated, _ := svc.Get(ctx, job.ID)
	if updated.Status != StatusFailed {
		t.Errorf("Status = %q, want %q", updated.Status, StatusFailed)
	}
	if updated.Error != "download failed" {
		t.Errorf("Error = %q, want %q", updated.Error, "download failed")
	}
}

func TestJobService_MarkRetry(t *testing.T) {
	repo := newMockRepo()
	svc := NewJobService(repo)
	ctx := context.Background()

	job, _ := svc.Submit(ctx, "https://example.com")
	svc.MarkProcessing(ctx, job.ID)

	err := svc.MarkRetry(ctx, job.ID, "temporary error")
	if err != nil {
		t.Fatalf("MarkRetry() error = %v", err)
	}

	updated, _ := svc.Get(ctx, job.ID)
	if updated.Status != StatusPending {
		t.Errorf("Status = %q, want %q", updated.Status, StatusPending)
	}
}
