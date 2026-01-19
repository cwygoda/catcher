package domain

import "time"

// JobStatus represents the processing state of a job.
type JobStatus string

const (
	StatusPending    JobStatus = "pending"
	StatusProcessing JobStatus = "processing"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
)

// Job represents a URL processing job.
type Job struct {
	ID        int64
	URL       string
	Status    JobStatus
	Attempts  int
	Error     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CanRetry returns true if the job can be retried.
func (j *Job) CanRetry(maxAttempts int) bool {
	return j.Attempts < maxAttempts && j.Status != StatusCompleted
}
