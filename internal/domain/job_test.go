package domain

import (
	"testing"
	"time"
)

func TestJob_CanRetry(t *testing.T) {
	tests := []struct {
		name        string
		job         Job
		maxAttempts int
		want        bool
	}{
		{
			name:        "can retry when attempts below max",
			job:         Job{Attempts: 1, Status: StatusFailed},
			maxAttempts: 3,
			want:        true,
		},
		{
			name:        "cannot retry when attempts at max",
			job:         Job{Attempts: 3, Status: StatusFailed},
			maxAttempts: 3,
			want:        false,
		},
		{
			name:        "cannot retry when completed",
			job:         Job{Attempts: 1, Status: StatusCompleted},
			maxAttempts: 3,
			want:        false,
		},
		{
			name:        "can retry pending job",
			job:         Job{Attempts: 0, Status: StatusPending},
			maxAttempts: 3,
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.job.CanRetry(tt.maxAttempts); got != tt.want {
				t.Errorf("CanRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJobStatus_Values(t *testing.T) {
	// Verify status string values for DB storage
	if StatusPending != "pending" {
		t.Errorf("StatusPending = %q, want %q", StatusPending, "pending")
	}
	if StatusProcessing != "processing" {
		t.Errorf("StatusProcessing = %q, want %q", StatusProcessing, "processing")
	}
	if StatusCompleted != "completed" {
		t.Errorf("StatusCompleted = %q, want %q", StatusCompleted, "completed")
	}
	if StatusFailed != "failed" {
		t.Errorf("StatusFailed = %q, want %q", StatusFailed, "failed")
	}
}

func TestJob_Fields(t *testing.T) {
	now := time.Now()
	job := Job{
		ID:        1,
		URL:       "https://example.com",
		Status:    StatusPending,
		Attempts:  0,
		Error:     "",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if job.ID != 1 {
		t.Errorf("ID = %d, want 1", job.ID)
	}
	if job.URL != "https://example.com" {
		t.Errorf("URL = %q, want %q", job.URL, "https://example.com")
	}
}
