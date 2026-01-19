package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultDBPath(t *testing.T) {
	// Test with XDG_CACHE_HOME set
	t.Run("with XDG_CACHE_HOME", func(t *testing.T) {
		original := os.Getenv("XDG_CACHE_HOME")
		defer os.Setenv("XDG_CACHE_HOME", original)

		os.Setenv("XDG_CACHE_HOME", "/custom/cache")
		path := DefaultDBPath()

		expected := "/custom/cache/catcher/jobs.db"
		if path != expected {
			t.Errorf("DefaultDBPath() = %q, want %q", path, expected)
		}
	})

	// Test without XDG_CACHE_HOME
	t.Run("without XDG_CACHE_HOME", func(t *testing.T) {
		original := os.Getenv("XDG_CACHE_HOME")
		defer os.Setenv("XDG_CACHE_HOME", original)

		os.Unsetenv("XDG_CACHE_HOME")
		path := DefaultDBPath()

		if !strings.HasSuffix(path, filepath.Join(".cache", "catcher", "jobs.db")) {
			t.Errorf("DefaultDBPath() = %q, want suffix .cache/catcher/jobs.db", path)
		}
	})
}

func TestDefaultVideoDir(t *testing.T) {
	path := DefaultVideoDir()
	if !strings.HasSuffix(path, "Videos") {
		t.Errorf("DefaultVideoDir() = %q, want suffix Videos", path)
	}
}

func TestConfig_Defaults(t *testing.T) {
	cfg := &Config{
		Port:         8080,
		PollInterval: 5_000_000_000, // 5s in nanoseconds
		MaxRetries:   3,
	}

	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
}
