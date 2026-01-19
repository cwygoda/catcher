package http

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cwygoda/catcher/internal/domain"
)

// Server is the HTTP adapter for the webhook service.
type Server struct {
	svc    *domain.JobService
	mux    *http.ServeMux
	server *http.Server
	secret string
}

// NewServer creates a new HTTP server.
func NewServer(svc *domain.JobService, addr string, secret string) *Server {
	s := &Server{
		svc:    svc,
		mux:    http.NewServeMux(),
		secret: secret,
	}
	s.routes()
	s.server = &http.Server{
		Addr:    addr,
		Handler: s.mux,
	}
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /webhook", s.handleWebhook)
	s.mux.HandleFunc("GET /jobs/{id}", s.handleGetJob)
	s.mux.HandleFunc("GET /health", s.handleHealth)
}

// webhookRequest is the request body for POST /webhook.
type webhookRequest struct {
	URL string `json:"url"`
}

// jobResponse is the JSON response for job endpoints.
type jobResponse struct {
	ID        int64  `json:"id"`
	URL       string `json:"url"`
	Status    string `json:"status"`
	Attempts  int    `json:"attempts"`
	Error     string `json:"error,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// errorResponse is the JSON error response.
type errorResponse struct {
	Error string `json:"error"`
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	// Read body for verification and parsing
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	// Verify signature if secret is configured
	if s.secret != "" {
		if err := s.verifySignature(r, body); err != nil {
			log.Printf("webhook verification failed: %v", err)
			s.writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
	}

	var req webhookRequest
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.URL == "" {
		s.writeError(w, http.StatusBadRequest, "url is required")
		return
	}

	job, err := s.svc.Submit(r.Context(), req.URL)
	if err != nil {
		if err == domain.ErrInvalidURL {
			s.writeError(w, http.StatusBadRequest, "invalid URL")
			return
		}
		log.Printf("submit error: %v", err)
		s.writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	s.writeJSON(w, http.StatusCreated, jobToResponse(job))
}

const maxTimestampSkew = 5 * time.Minute

func (s *Server) verifySignature(r *http.Request, body []byte) error {
	// Check X-Timestamp header
	timestamp := r.Header.Get("X-Timestamp")
	if timestamp == "" {
		return fmt.Errorf("missing X-Timestamp header")
	}

	ts, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return fmt.Errorf("invalid X-Timestamp: must be ISO8601/RFC3339 format")
	}

	skew := time.Since(ts)
	if skew < 0 {
		skew = -skew
	}
	if skew > maxTimestampSkew {
		return fmt.Errorf("X-Timestamp too far from current time (skew: %v, max: %v)", skew.Truncate(time.Second), maxTimestampSkew)
	}

	// Check X-Signature header
	signature := r.Header.Get("X-Signature")
	if signature == "" {
		return fmt.Errorf("missing X-Signature header")
	}

	// Calculate expected signature: SHA256("${timestamp}\n${body}\n${secret}")
	payload := fmt.Sprintf("%s\n%s\n%s", timestamp, string(body), s.secret)
	hash := sha256.Sum256([]byte(payload))
	expected := hex.EncodeToString(hash[:])

	if signature != expected {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	job, err := s.svc.Get(r.Context(), id)
	if err != nil {
		if err == domain.ErrJobNotFound {
			s.writeError(w, http.StatusNotFound, "job not found")
			return
		}
		log.Printf("get job error: %v", err)
		s.writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	s.writeJSON(w, http.StatusOK, jobToResponse(job))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (s *Server) writeError(w http.ResponseWriter, status int, msg string) {
	s.writeJSON(w, status, errorResponse{Error: msg})
}

func jobToResponse(job *domain.Job) jobResponse {
	return jobResponse{
		ID:        job.ID,
		URL:       job.URL,
		Status:    string(job.Status),
		Attempts:  job.Attempts,
		Error:     job.Error,
		CreatedAt: job.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: job.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// ServeHTTP implements http.Handler for testing.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Addr returns the server address.
func (s *Server) Addr() string {
	return s.server.Addr
}

// Port extracts the port from the address.
func (s *Server) Port() int {
	addr := s.server.Addr
	if idx := strings.LastIndex(addr, ":"); idx >= 0 {
		port, _ := strconv.Atoi(addr[idx+1:])
		return port
	}
	return 0
}
