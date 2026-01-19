package config

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Config holds application configuration.
type Config struct {
	Port         int
	DBPath       string
	PollInterval time.Duration
	MaxRetries   int
	VideoDir     string
}

// DefaultDBPath returns the default database path using XDG_CACHE_HOME.
func DefaultDBPath() string {
	cacheDir := os.Getenv("XDG_CACHE_HOME")
	if cacheDir == "" {
		home, _ := os.UserHomeDir()
		cacheDir = filepath.Join(home, ".cache")
	}
	return filepath.Join(cacheDir, "catcher", "jobs.db")
}

// DefaultVideoDir returns the default video download directory.
func DefaultVideoDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Videos")
}

// Load parses flags and environment to build Config.
func Load() *Config {
	cfg := &Config{}

	flag.IntVar(&cfg.Port, "port", 8080, "HTTP server port")
	flag.StringVar(&cfg.DBPath, "db", DefaultDBPath(), "SQLite database path")
	flag.DurationVar(&cfg.PollInterval, "poll-interval", 5*time.Second, "Worker poll interval")
	flag.IntVar(&cfg.MaxRetries, "max-retries", 3, "Maximum retry attempts")
	flag.StringVar(&cfg.VideoDir, "video-dir", DefaultVideoDir(), "Video download directory")
	flag.Parse()

	// Env overrides
	if port := os.Getenv("CATCHER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Port = p
		}
	}
	if db := os.Getenv("CATCHER_DB"); db != "" {
		cfg.DBPath = db
	}
	if videoDir := os.Getenv("CATCHER_VIDEO_DIR"); videoDir != "" {
		cfg.VideoDir = videoDir
	}

	return cfg
}
