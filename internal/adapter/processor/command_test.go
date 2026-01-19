package processor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cwygoda/catcher/internal/config"
	"github.com/cwygoda/catcher/internal/domain"
)

func boolPtr(b bool) *bool { return &b }

func TestNewCommandProcessor(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.ProcessorConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: config.ProcessorConfig{
				Name:    "test",
				Pattern: `^https?://example\.com/`,
				Command: "echo",
				Args:    []string{"{url}"},
			},
			wantErr: false,
		},
		{
			name: "invalid regex",
			cfg: config.ProcessorConfig{
				Name:    "bad",
				Pattern: `[invalid`,
				Command: "echo",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCommandProcessor(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCommandProcessor() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommandProcessor_Name(t *testing.T) {
	p, _ := NewCommandProcessor(config.ProcessorConfig{
		Name:    "youtube",
		Pattern: ".*",
	})

	if p.Name() != "youtube" {
		t.Errorf("Name() = %q, want %q", p.Name(), "youtube")
	}
}

func TestCommandProcessor_Match(t *testing.T) {
	p, _ := NewCommandProcessor(config.ProcessorConfig{
		Name:    "youtube",
		Pattern: `^https?://(www\.)?(youtube\.com|youtu\.be)/`,
	})

	tests := []struct {
		url  string
		want bool
	}{
		{"https://youtube.com/watch?v=abc123", true},
		{"https://www.youtube.com/watch?v=abc123", true},
		{"http://youtu.be/abc123", true},
		{"https://vimeo.com/123456", false},
		{"https://example.com/video", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := p.Match(tt.url)
			if got != tt.want {
				t.Errorf("Match(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestCommandProcessor_ProcessDirect(t *testing.T) {
	targetDir := t.TempDir()

	p, err := NewCommandProcessor(config.ProcessorConfig{
		Name:      "test",
		Pattern:   ".*",
		Command:   "touch",
		Args:      []string{"output.txt"},
		TargetDir: targetDir,
		Isolate:   boolPtr(false),
	})
	if err != nil {
		t.Fatal(err)
	}

	job := &domain.Job{ID: 1, URL: "https://example.com"}
	if err := p.Process(context.Background(), job); err != nil {
		t.Errorf("Process() error = %v", err)
	}

	// Check file was created in target dir
	if _, err := os.Stat(filepath.Join(targetDir, "output.txt")); os.IsNotExist(err) {
		t.Error("expected output.txt to exist in target dir")
	}
}

func TestCommandProcessor_ProcessIsolated(t *testing.T) {
	targetDir := t.TempDir()

	p, err := NewCommandProcessor(config.ProcessorConfig{
		Name:      "test",
		Pattern:   ".*",
		Command:   "touch",
		Args:      []string{"isolated.txt"},
		TargetDir: targetDir,
		Isolate:   boolPtr(true),
	})
	if err != nil {
		t.Fatal(err)
	}

	job := &domain.Job{ID: 1, URL: "https://example.com"}
	if err := p.Process(context.Background(), job); err != nil {
		t.Errorf("Process() error = %v", err)
	}

	// Check file was moved to target dir
	if _, err := os.Stat(filepath.Join(targetDir, "isolated.txt")); os.IsNotExist(err) {
		t.Error("expected isolated.txt to exist in target dir")
	}
}

func TestCommandProcessor_NoOverwrite(t *testing.T) {
	targetDir := t.TempDir()

	// Create existing file with content
	existingFile := filepath.Join(targetDir, "existing.txt")
	if err := os.WriteFile(existingFile, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	p, err := NewCommandProcessor(config.ProcessorConfig{
		Name:      "test",
		Pattern:   ".*",
		Command:   "sh",
		Args:      []string{"-c", "echo new > existing.txt"},
		TargetDir: targetDir,
		Isolate:   boolPtr(true),
	})
	if err != nil {
		t.Fatal(err)
	}

	job := &domain.Job{ID: 1, URL: "https://example.com"}
	if err := p.Process(context.Background(), job); err != nil {
		t.Errorf("Process() error = %v", err)
	}

	// Check original file unchanged
	content, err := os.ReadFile(existingFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "original" {
		t.Errorf("file was overwritten: got %q, want %q", string(content), "original")
	}
}

func TestCommandProcessor_URLPlaceholder(t *testing.T) {
	targetDir := t.TempDir()

	p, err := NewCommandProcessor(config.ProcessorConfig{
		Name:      "test",
		Pattern:   ".*",
		Command:   "sh",
		Args:      []string{"-c", "echo {url} > url.txt"},
		TargetDir: targetDir,
		Isolate:   boolPtr(false),
	})
	if err != nil {
		t.Fatal(err)
	}

	job := &domain.Job{ID: 1, URL: "https://example.com/video"}
	if err := p.Process(context.Background(), job); err != nil {
		t.Errorf("Process() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(targetDir, "url.txt"))
	if err != nil {
		t.Fatal(err)
	}
	// Note: echo adds newline
	if got := string(content); got != "https://example.com/video\n" {
		t.Errorf("URL placeholder not replaced: got %q", got)
	}
}

func TestCommandProcessor_DefaultIsolate(t *testing.T) {
	p, err := NewCommandProcessor(config.ProcessorConfig{
		Name:    "test",
		Pattern: ".*",
		Command: "echo",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Default should be true (isolate enabled)
	if !p.isolate {
		t.Error("expected isolate to default to true")
	}
}

func TestCommandProcessor_DefaultTargetDir(t *testing.T) {
	p, err := NewCommandProcessor(config.ProcessorConfig{
		Name:    "test",
		Pattern: ".*",
		Command: "echo",
	})
	if err != nil {
		t.Fatal(err)
	}

	expected := config.DefaultTargetDir()
	if p.TargetDir() != expected {
		t.Errorf("TargetDir() = %q, want %q", p.TargetDir(), expected)
	}
}
