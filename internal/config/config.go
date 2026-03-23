package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	DataDir        string // FACTORY_DATA_DIR — path to registry.db
	GitDir         string // FACTORY_GIT_DIR — path to bare git repos
	ClaudeBinary   string // CLAUDE_BINARY — path to claude CLI (default: claude)
	MaxFixAttempts int    // MAX_FIX_ATTEMPTS — retries per build (default: 3)
	GitHubUser     string // GITHUB_USER — for git config
}

// Load reads configuration from environment variables and validates required fields.
func Load() (*Config, error) {
	cfg := &Config{
		DataDir:        os.Getenv("FACTORY_DATA_DIR"),
		GitDir:         os.Getenv("FACTORY_GIT_DIR"),
		ClaudeBinary:   envOrDefault("CLAUDE_BINARY", "claude"),
		MaxFixAttempts: envIntOrDefault("MAX_FIX_ATTEMPTS", 3),
		GitHubUser:     os.Getenv("GITHUB_USER"),
	}

	if cfg.DataDir == "" {
		return nil, fmt.Errorf("FACTORY_DATA_DIR is required")
	}
	if cfg.GitDir == "" {
		return nil, fmt.Errorf("FACTORY_GIT_DIR is required")
	}

	return cfg, nil
}

// RegistryPath returns the full path to the SQLite registry database.
func (c *Config) RegistryPath() string {
	return filepath.Join(c.DataDir, "registry.db")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
