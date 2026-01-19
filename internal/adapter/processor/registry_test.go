package processor

import (
	"context"
	"testing"

	"github.com/cwygoda/catcher/internal/domain"
)

type mockProcessor struct {
	name    string
	matcher func(string) bool
}

func (m *mockProcessor) Name() string                                  { return m.name }
func (m *mockProcessor) Match(url string) bool                         { return m.matcher(url) }
func (m *mockProcessor) Process(ctx context.Context, job *domain.Job) error { return nil }

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	p1 := &mockProcessor{name: "proc1", matcher: func(s string) bool { return false }}
	p2 := &mockProcessor{name: "proc2", matcher: func(s string) bool { return false }}

	r.Register(p1)
	r.Register(p2)

	procs := r.Processors()
	if len(procs) != 2 {
		t.Errorf("Processors() len = %d, want 2", len(procs))
	}
}

func TestRegistry_Match(t *testing.T) {
	r := NewRegistry()

	youtube := &mockProcessor{
		name:    "youtube",
		matcher: func(s string) bool { return s == "https://youtube.com/watch" },
	}
	generic := &mockProcessor{
		name:    "generic",
		matcher: func(s string) bool { return true },
	}

	r.Register(youtube)
	r.Register(generic)

	tests := []struct {
		url      string
		wantName string
	}{
		{"https://youtube.com/watch", "youtube"},
		{"https://other.com/video", "generic"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			p := r.Match(tt.url)
			if p == nil {
				t.Fatal("Match() returned nil")
			}
			if p.Name() != tt.wantName {
				t.Errorf("Match() name = %q, want %q", p.Name(), tt.wantName)
			}
		})
	}
}

func TestRegistry_Match_NoMatch(t *testing.T) {
	r := NewRegistry()

	specific := &mockProcessor{
		name:    "specific",
		matcher: func(s string) bool { return s == "specific-url" },
	}
	r.Register(specific)

	p := r.Match("other-url")
	if p != nil {
		t.Errorf("Match() = %v, want nil", p)
	}
}

func TestRegistry_Empty(t *testing.T) {
	r := NewRegistry()

	procs := r.Processors()
	if len(procs) != 0 {
		t.Errorf("Processors() len = %d, want 0", len(procs))
	}

	p := r.Match("any-url")
	if p != nil {
		t.Errorf("Match() = %v, want nil", p)
	}
}
