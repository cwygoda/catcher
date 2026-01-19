package domain

import "context"

// JobRepository is the driven port for job persistence.
type JobRepository interface {
	Create(ctx context.Context, url string) (*Job, error)
	Get(ctx context.Context, id int64) (*Job, error)
	FindPending(ctx context.Context, limit int) ([]Job, error)
	Claim(ctx context.Context, id int64) error
	Complete(ctx context.Context, id int64) error
	Fail(ctx context.Context, id int64, reason string) error
	Retry(ctx context.Context, id int64, reason string) error
	RecoverStale(ctx context.Context) (int64, error)
}

// URLProcessor is the driven port for URL processing.
type URLProcessor interface {
	Name() string
	TargetDir() string
	Match(url string) bool
	Process(ctx context.Context, job *Job) error
}
