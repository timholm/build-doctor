package config

import (
	"os"
	"testing"
)

func TestLoad_RequiresDataDir(t *testing.T) {
	os.Unsetenv("FACTORY_DATA_DIR")
	os.Unsetenv("FACTORY_GIT_DIR")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when FACTORY_DATA_DIR is not set")
	}
}

func TestLoad_RequiresGitDir(t *testing.T) {
	t.Setenv("FACTORY_DATA_DIR", "/tmp/data")
	t.Setenv("FACTORY_GIT_DIR", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when FACTORY_GIT_DIR is not set")
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("FACTORY_DATA_DIR", "/tmp/data")
	t.Setenv("FACTORY_GIT_DIR", "/tmp/git")
	t.Setenv("CLAUDE_BINARY", "")
	t.Setenv("MAX_FIX_ATTEMPTS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ClaudeBinary != "claude" {
		t.Errorf("expected ClaudeBinary=claude, got %s", cfg.ClaudeBinary)
	}
	if cfg.MaxFixAttempts != 3 {
		t.Errorf("expected MaxFixAttempts=3, got %d", cfg.MaxFixAttempts)
	}
}

func TestLoad_CustomValues(t *testing.T) {
	t.Setenv("FACTORY_DATA_DIR", "/opt/factory/data")
	t.Setenv("FACTORY_GIT_DIR", "/opt/factory/git")
	t.Setenv("CLAUDE_BINARY", "/usr/local/bin/claude")
	t.Setenv("MAX_FIX_ATTEMPTS", "5")
	t.Setenv("GITHUB_USER", "testuser")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DataDir != "/opt/factory/data" {
		t.Errorf("DataDir = %s", cfg.DataDir)
	}
	if cfg.GitDir != "/opt/factory/git" {
		t.Errorf("GitDir = %s", cfg.GitDir)
	}
	if cfg.ClaudeBinary != "/usr/local/bin/claude" {
		t.Errorf("ClaudeBinary = %s", cfg.ClaudeBinary)
	}
	if cfg.MaxFixAttempts != 5 {
		t.Errorf("MaxFixAttempts = %d", cfg.MaxFixAttempts)
	}
	if cfg.GitHubUser != "testuser" {
		t.Errorf("GitHubUser = %s", cfg.GitHubUser)
	}
}

func TestRegistryPath(t *testing.T) {
	cfg := &Config{DataDir: "/tmp/data"}
	if cfg.RegistryPath() != "/tmp/data/registry.db" {
		t.Errorf("RegistryPath = %s", cfg.RegistryPath())
	}
}

func TestEnvIntOrDefault_Invalid(t *testing.T) {
	t.Setenv("MAX_FIX_ATTEMPTS", "not-a-number")
	t.Setenv("FACTORY_DATA_DIR", "/tmp/data")
	t.Setenv("FACTORY_GIT_DIR", "/tmp/git")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxFixAttempts != 3 {
		t.Errorf("expected fallback 3, got %d", cfg.MaxFixAttempts)
	}
}
