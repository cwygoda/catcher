package sqlite

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"time"

	"github.com/cwygoda/catcher/internal/domain"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS jobs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    url        TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'pending',
    attempts   INTEGER NOT NULL DEFAULT 0,
    error      TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
`

// Repository implements domain.JobRepository using SQLite.
type Repository struct {
	db *sql.DB
}

// New creates a new SQLite repository, initializing the schema if needed.
func New(dbPath string) (*Repository, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Initialize schema
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return &Repository{db: db}, nil
}

// Close closes the database connection.
func (r *Repository) Close() error {
	return r.db.Close()
}

// Create inserts a new job.
func (r *Repository) Create(ctx context.Context, url string) (*domain.Job, error) {
	now := time.Now()
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO jobs (url, status, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		url, domain.StatusPending, now, now,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &domain.Job{
		ID:        id,
		URL:       url,
		Status:    domain.StatusPending,
		Attempts:  0,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Get retrieves a job by ID.
func (r *Repository) Get(ctx context.Context, id int64) (*domain.Job, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, url, status, attempts, COALESCE(error, ''), created_at, updated_at
		 FROM jobs WHERE id = ?`, id,
	)
	return scanJob(row)
}

// FindPending returns pending jobs up to limit.
func (r *Repository) FindPending(ctx context.Context, limit int) ([]domain.Job, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, url, status, attempts, COALESCE(error, ''), created_at, updated_at
		 FROM jobs WHERE status = ? ORDER BY created_at ASC LIMIT ?`,
		domain.StatusPending, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []domain.Job
	for rows.Next() {
		var job domain.Job
		var status string
		if err := rows.Scan(&job.ID, &job.URL, &status, &job.Attempts, &job.Error, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, err
		}
		job.Status = domain.JobStatus(status)
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

// Claim atomically claims a pending job for processing.
func (r *Repository) Claim(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE jobs SET status = ?, attempts = attempts + 1, updated_at = ?
		 WHERE id = ? AND status = ?`,
		domain.StatusProcessing, time.Now(), id, domain.StatusPending,
	)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrJobNotFound
	}
	return nil
}

// Complete marks a job as completed.
func (r *Repository) Complete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE jobs SET status = ?, updated_at = ? WHERE id = ?`,
		domain.StatusCompleted, time.Now(), id,
	)
	return err
}

// Fail marks a job as permanently failed.
func (r *Repository) Fail(ctx context.Context, id int64, reason string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE jobs SET status = ?, error = ?, updated_at = ? WHERE id = ?`,
		domain.StatusFailed, reason, time.Now(), id,
	)
	return err
}

// Retry marks a job for retry (back to pending with error info).
func (r *Repository) Retry(ctx context.Context, id int64, reason string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE jobs SET status = ?, error = ?, updated_at = ? WHERE id = ?`,
		domain.StatusPending, reason, time.Now(), id,
	)
	return err
}

// RecoverStale resets all processing jobs back to pending (for crash recovery).
func (r *Repository) RecoverStale(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`UPDATE jobs SET status = ?, error = 'recovered after crash', updated_at = ?
		 WHERE status = ?`,
		domain.StatusPending, time.Now(), domain.StatusProcessing,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanJob(row scanner) (*domain.Job, error) {
	var job domain.Job
	var status string
	err := row.Scan(&job.ID, &job.URL, &status, &job.Attempts, &job.Error, &job.CreatedAt, &job.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrJobNotFound
	}
	if err != nil {
		return nil, err
	}
	job.Status = domain.JobStatus(status)
	return &job, nil
}
