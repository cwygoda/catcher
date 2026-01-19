package sqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cwygoda/catcher/internal/domain"
)

func setupTestRepo(t *testing.T) (*Repository, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	repo, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	cleanup := func() {
		repo.Close()
		os.Remove(dbPath)
	}
	return repo, cleanup
}

func TestRepository_Create(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()
	url := "https://example.com/video"

	job, err := repo.Create(ctx, url)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if job.ID == 0 {
		t.Error("Create() job.ID = 0, want non-zero")
	}
	if job.URL != url {
		t.Errorf("Create() job.URL = %q, want %q", job.URL, url)
	}
	if job.Status != domain.StatusPending {
		t.Errorf("Create() job.Status = %q, want %q", job.Status, domain.StatusPending)
	}
	if job.Attempts != 0 {
		t.Errorf("Create() job.Attempts = %d, want 0", job.Attempts)
	}
}

func TestRepository_Get(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create a job
	created, _ := repo.Create(ctx, "https://example.com")

	// Get existing
	job, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if job.ID != created.ID {
		t.Errorf("Get() job.ID = %d, want %d", job.ID, created.ID)
	}

	// Get non-existent
	_, err = repo.Get(ctx, 9999)
	if !errors.Is(err, domain.ErrJobNotFound) {
		t.Errorf("Get() error = %v, want %v", err, domain.ErrJobNotFound)
	}
}

func TestRepository_FindPending(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple jobs
	repo.Create(ctx, "https://example.com/1")
	repo.Create(ctx, "https://example.com/2")
	repo.Create(ctx, "https://example.com/3")

	// Find with limit
	jobs, err := repo.FindPending(ctx, 2)
	if err != nil {
		t.Fatalf("FindPending() error = %v", err)
	}
	if len(jobs) != 2 {
		t.Errorf("FindPending() returned %d jobs, want 2", len(jobs))
	}

	// Verify all are pending
	for _, job := range jobs {
		if job.Status != domain.StatusPending {
			t.Errorf("FindPending() job.Status = %q, want %q", job.Status, domain.StatusPending)
		}
	}
}

func TestRepository_Claim(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	job, _ := repo.Create(ctx, "https://example.com")

	// Claim the job
	err := repo.Claim(ctx, job.ID)
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}

	// Verify status changed
	claimed, _ := repo.Get(ctx, job.ID)
	if claimed.Status != domain.StatusProcessing {
		t.Errorf("Claim() status = %q, want %q", claimed.Status, domain.StatusProcessing)
	}
	if claimed.Attempts != 1 {
		t.Errorf("Claim() attempts = %d, want 1", claimed.Attempts)
	}

	// Try to claim again (should fail - not pending)
	err = repo.Claim(ctx, job.ID)
	if !errors.Is(err, domain.ErrJobNotFound) {
		t.Errorf("Claim() second attempt error = %v, want %v", err, domain.ErrJobNotFound)
	}
}

func TestRepository_Complete(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	job, _ := repo.Create(ctx, "https://example.com")
	repo.Claim(ctx, job.ID)

	err := repo.Complete(ctx, job.ID)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	completed, _ := repo.Get(ctx, job.ID)
	if completed.Status != domain.StatusCompleted {
		t.Errorf("Complete() status = %q, want %q", completed.Status, domain.StatusCompleted)
	}
}

func TestRepository_Fail(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	job, _ := repo.Create(ctx, "https://example.com")
	repo.Claim(ctx, job.ID)

	err := repo.Fail(ctx, job.ID, "download error")
	if err != nil {
		t.Fatalf("Fail() error = %v", err)
	}

	failed, _ := repo.Get(ctx, job.ID)
	if failed.Status != domain.StatusFailed {
		t.Errorf("Fail() status = %q, want %q", failed.Status, domain.StatusFailed)
	}
	if failed.Error != "download error" {
		t.Errorf("Fail() error = %q, want %q", failed.Error, "download error")
	}
}

func TestRepository_Retry(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	job, _ := repo.Create(ctx, "https://example.com")
	repo.Claim(ctx, job.ID)

	err := repo.Retry(ctx, job.ID, "temporary error")
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}

	retried, _ := repo.Get(ctx, job.ID)
	if retried.Status != domain.StatusPending {
		t.Errorf("Retry() status = %q, want %q", retried.Status, domain.StatusPending)
	}
	if retried.Error != "temporary error" {
		t.Errorf("Retry() error = %q, want %q", retried.Error, "temporary error")
	}

	// Can be claimed again after retry
	err = repo.Claim(ctx, job.ID)
	if err != nil {
		t.Errorf("Claim() after retry error = %v", err)
	}

	reclaimed, _ := repo.Get(ctx, job.ID)
	if reclaimed.Attempts != 2 {
		t.Errorf("Claim() after retry attempts = %d, want 2", reclaimed.Attempts)
	}
}

func TestNew_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "subdir", "nested", "test.db")

	repo, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer repo.Close()

	// Verify directory was created
	if _, err := os.Stat(filepath.Dir(dbPath)); os.IsNotExist(err) {
		t.Error("New() did not create parent directory")
	}
}

func TestRepository_RecoverStale(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create jobs in different states
	job1, _ := repo.Create(ctx, "https://example.com/1")
	job2, _ := repo.Create(ctx, "https://example.com/2")
	job3, _ := repo.Create(ctx, "https://example.com/3")

	// job1: processing (stale)
	repo.Claim(ctx, job1.ID)
	// job2: processing (stale)
	repo.Claim(ctx, job2.ID)
	// job3: pending (not stale)

	// Recover stale jobs
	count, err := repo.RecoverStale(ctx)
	if err != nil {
		t.Fatalf("RecoverStale() error = %v", err)
	}
	if count != 2 {
		t.Errorf("RecoverStale() count = %d, want 2", count)
	}

	// Verify all jobs are now pending
	j1, _ := repo.Get(ctx, job1.ID)
	j2, _ := repo.Get(ctx, job2.ID)
	j3, _ := repo.Get(ctx, job3.ID)

	if j1.Status != domain.StatusPending {
		t.Errorf("job1 status = %q, want %q", j1.Status, domain.StatusPending)
	}
	if j2.Status != domain.StatusPending {
		t.Errorf("job2 status = %q, want %q", j2.Status, domain.StatusPending)
	}
	if j3.Status != domain.StatusPending {
		t.Errorf("job3 status = %q, want %q", j3.Status, domain.StatusPending)
	}

	// Verify error message was set
	if j1.Error != "recovered after crash" {
		t.Errorf("job1 error = %q, want %q", j1.Error, "recovered after crash")
	}
}
