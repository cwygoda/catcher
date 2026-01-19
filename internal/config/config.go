package config

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
)

// ProcessorConfig defines a URL processor from the config file.
type ProcessorConfig struct {
	Name      string   `toml:"name"`
	Pattern   string   `toml:"pattern"`
	Command   string   `toml:"command"`
	Args      []string `toml:"args"`
	TargetDir string   `toml:"target_dir"`
	Isolate   *bool    `toml:"isolate"`
}

// fileConfig represents the TOML file structure.
type fileConfig struct {
	Processors []ProcessorConfig `toml:"processor"`
}

// Config holds application configuration.
type Config struct {
	Port         int
	DBPath       string
	PollInterval time.Duration
	MaxRetries   int
	ConfigPath   string
	Processors   []ProcessorConfig
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

// DefaultConfigPath returns the default config path using XDG_CONFIG_HOME.
func DefaultConfigPath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "catcher", "config.toml")
}

// DefaultTargetDir returns the default download directory.
func DefaultTargetDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Videos")
}

// ExpandPath expands ~ to home directory.
func ExpandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

// Load parses flags, config file, and environment to build Config.
func Load() *Config {
	cfg := &Config{}

	flag.IntVar(&cfg.Port, "port", 8080, "HTTP server port")
	flag.StringVar(&cfg.DBPath, "db", DefaultDBPath(), "SQLite database path")
	flag.DurationVar(&cfg.PollInterval, "poll-interval", 5*time.Second, "Worker poll interval")
	flag.IntVar(&cfg.MaxRetries, "max-retries", 3, "Maximum retry attempts")
	flag.StringVar(&cfg.ConfigPath, "config", DefaultConfigPath(), "Config file path")
	flag.Parse()

	// Load TOML config file if exists
	configPath := ExpandPath(cfg.ConfigPath)
	if _, err := os.Stat(configPath); err == nil {
		log.Printf("loading config from %s", configPath)
		var fc fileConfig
		if _, err := toml.DecodeFile(configPath, &fc); err == nil {
			cfg.Processors = fc.Processors
			log.Printf("found %d processor(s) in config", len(cfg.Processors))
		} else {
			log.Printf("failed to parse config: %v", err)
		}
	} else {
		log.Printf("no config file at %s", configPath)
	}

	// Env overrides (runtime settings only)
	if port := os.Getenv("CATCHER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Port = p
			log.Printf("CATCHER_PORT override: %d", p)
		}
	}
	if db := os.Getenv("CATCHER_DB"); db != "" {
		cfg.DBPath = db
		log.Printf("CATCHER_DB override: %s", db)
	}

	return cfg
}
