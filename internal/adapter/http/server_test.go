package http

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cwygoda/catcher/internal/domain"
)

// mockRepo implements domain.JobRepository for testing.
type mockRepo struct {
	jobs   map[int64]*domain.Job
	nextID int64
}

func newMockRepo() *mockRepo {
	return &mockRepo{jobs: make(map[int64]*domain.Job), nextID: 1}
}

func (m *mockRepo) Create(ctx context.Context, url string) (*domain.Job, error) {
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
	job, ok := m.jobs[id]
	if !ok {
		return nil, domain.ErrJobNotFound
	}
	return job, nil
}

func (m *mockRepo) FindPending(ctx context.Context, limit int) ([]domain.Job, error) {
	return nil, nil
}
func (m *mockRepo) Claim(ctx context.Context, id int64) error                   { return nil }
func (m *mockRepo) Complete(ctx context.Context, id int64) error                { return nil }
func (m *mockRepo) Fail(ctx context.Context, id int64, reason string) error     { return nil }
func (m *mockRepo) Retry(ctx context.Context, id int64, reason string) error    { return nil }
func (m *mockRepo) RecoverStale(ctx context.Context) (int64, error)             { return 0, nil }

func setupTestServer() *Server {
	repo := newMockRepo()
	svc := domain.NewJobService(repo)
	return NewServer(svc, ":8080", "")
}

func TestServer_Webhook_Success(t *testing.T) {
	srv := setupTestServer()

	body := `{"url":"https://youtube.com/watch?v=abc123"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var resp jobResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if resp.ID == 0 {
		t.Error("response ID = 0, want non-zero")
	}
	if resp.URL != "https://youtube.com/watch?v=abc123" {
		t.Errorf("response URL = %q, want %q", resp.URL, "https://youtube.com/watch?v=abc123")
	}
	if resp.Status != "pending" {
		t.Errorf("response status = %q, want %q", resp.Status, "pending")
	}
}

func TestServer_Webhook_MissingURL(t *testing.T) {
	srv := setupTestServer()

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestServer_Webhook_InvalidURL(t *testing.T) {
	srv := setupTestServer()

	body := `{"url":"not a valid url"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestServer_Webhook_InvalidJSON(t *testing.T) {
	srv := setupTestServer()

	body := `not json`
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestServer_GetJob_Success(t *testing.T) {
	srv := setupTestServer()

	// First create a job
	body := `{"url":"https://example.com"}`
	createReq := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(body))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	srv.ServeHTTP(createRec, createReq)

	var created jobResponse
	json.NewDecoder(createRec.Body).Decode(&created)

	// Now get the job
	req := httptest.NewRequest(http.MethodGet, "/jobs/1", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp jobResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if resp.ID != created.ID {
		t.Errorf("response ID = %d, want %d", resp.ID, created.ID)
	}
}

func TestServer_GetJob_NotFound(t *testing.T) {
	srv := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/jobs/9999", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestServer_GetJob_InvalidID(t *testing.T) {
	srv := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/jobs/invalid", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestServer_Health(t *testing.T) {
	srv := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("status = %q, want %q", resp["status"], "ok")
	}
}

func TestServer_ContentType(t *testing.T) {
	srv := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

// Helper to compute signature
func computeSignature(timestamp, body, secret string) string {
	payload := fmt.Sprintf("%s\n%s\n%s", timestamp, body, secret)
	hash := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(hash[:])
}

func TestServer_Webhook_SignatureValid(t *testing.T) {
	repo := newMockRepo()
	svc := domain.NewJobService(repo)
	srv := NewServer(svc, ":8080", "test-secret")

	body := `{"url":"https://example.com"}`
	timestamp := time.Now().UTC().Format(time.RFC3339)
	signature := computeSignature(timestamp, body, "test-secret")

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signature)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

func TestServer_Webhook_MissingTimestamp(t *testing.T) {
	repo := newMockRepo()
	svc := domain.NewJobService(repo)
	srv := NewServer(svc, ":8080", "test-secret")

	body := `{"url":"https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", "anything")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestServer_Webhook_InvalidTimestampFormat(t *testing.T) {
	repo := newMockRepo()
	svc := domain.NewJobService(repo)
	srv := NewServer(svc, ":8080", "test-secret")

	body := `{"url":"https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Timestamp", "not-a-timestamp")
	req.Header.Set("X-Signature", "anything")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestServer_Webhook_TimestampTooOld(t *testing.T) {
	repo := newMockRepo()
	svc := domain.NewJobService(repo)
	srv := NewServer(svc, ":8080", "test-secret")

	body := `{"url":"https://example.com"}`
	timestamp := time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339)
	signature := computeSignature(timestamp, body, "test-secret")

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signature)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestServer_Webhook_MissingSignature(t *testing.T) {
	repo := newMockRepo()
	svc := domain.NewJobService(repo)
	srv := NewServer(svc, ":8080", "test-secret")

	body := `{"url":"https://example.com"}`
	timestamp := time.Now().UTC().Format(time.RFC3339)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Timestamp", timestamp)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestServer_Webhook_InvalidSignature(t *testing.T) {
	repo := newMockRepo()
	svc := domain.NewJobService(repo)
	srv := NewServer(svc, ":8080", "test-secret")

	body := `{"url":"https://example.com"}`
	timestamp := time.Now().UTC().Format(time.RFC3339)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", "wrong-signature")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestServer_Webhook_NoSecretConfigured(t *testing.T) {
	// When no secret is configured, verification is skipped
	srv := setupTestServer() // secret=""

	body := `{"url":"https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	// No X-Timestamp or X-Signature headers
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d (no secret = no verification)", rec.Code, http.StatusCreated)
	}
}
