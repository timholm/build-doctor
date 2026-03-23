package doctor

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/timholm/build-doctor/internal/config"
	"github.com/timholm/build-doctor/internal/registry"

	_ "modernc.org/sqlite"
)

func setupTestRegistry(t *testing.T) (*registry.Registry, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "registry.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS build_queue (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			error_log TEXT,
			fix_attempts INTEGER DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("creating table: %v", err)
	}
	db.Close()

	reg, err := registry.Open(dbPath)
	if err != nil {
		t.Fatalf("opening registry: %v", err)
	}

	return reg, dir
}

func insertTestBuild(t *testing.T, reg *registry.Registry, name, status, errorLog string, fixAttempts int) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := reg.DB().Exec(
		"INSERT INTO build_queue (name, status, error_log, fix_attempts, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		name, status, errorLog, fixAttempts, now, now,
	)
	if err != nil {
		t.Fatalf("inserting build: %v", err)
	}
}

func TestFixBuild_TestsPassFirstTry(t *testing.T) {
	reg, dataDir := setupTestRegistry(t)
	defer reg.Close()

	gitDir := t.TempDir()
	insertTestBuild(t, reg, "good-repo", "failed", "undefined: foo", 0)

	cfg := &config.Config{
		DataDir:        dataDir,
		GitDir:         gitDir,
		ClaudeBinary:   "claude",
		MaxFixAttempts: 3,
	}

	doc := New(cfg, reg)

	// Mock: clone returns a temp dir
	doc.cloneFunc = func(bareDir, name string) (string, error) {
		d := t.TempDir()
		// Create a minimal go project so prompt builder works
		os.WriteFile(filepath.Join(d, "main.go"), []byte("package main\n"), 0644)
		return d, nil
	}

	// Mock: claude succeeds
	doc.claudeFunc = func(workDir, prompt string) error {
		return nil
	}

	// Mock: tests pass
	doc.testFunc = func(workDir string) (string, error) {
		return "PASS", nil
	}

	// Mock: commit succeeds
	doc.commitFunc = func(workDir, user string) error {
		return nil
	}

	err := doc.FixOne("good-repo")
	if err != nil {
		t.Fatalf("FixOne: %v", err)
	}

	// Verify status updated to shipped
	b, _ := reg.BuildByName("good-repo")
	if b.Status != "shipped" {
		t.Errorf("status = %s, want shipped", b.Status)
	}

	if doc.stats.Fixed != 1 {
		t.Errorf("stats.Fixed = %d, want 1", doc.stats.Fixed)
	}
}

func TestFixBuild_TestsStillFail(t *testing.T) {
	reg, dataDir := setupTestRegistry(t)
	defer reg.Close()

	gitDir := t.TempDir()
	insertTestBuild(t, reg, "bad-repo", "failed", "panic: nil pointer", 0)

	cfg := &config.Config{
		DataDir:        dataDir,
		GitDir:         gitDir,
		ClaudeBinary:   "claude",
		MaxFixAttempts: 3,
	}

	doc := New(cfg, reg)

	doc.cloneFunc = func(bareDir, name string) (string, error) {
		d := t.TempDir()
		os.WriteFile(filepath.Join(d, "main.go"), []byte("package main\n"), 0644)
		return d, nil
	}

	doc.claudeFunc = func(workDir, prompt string) error {
		return nil
	}

	doc.testFunc = func(workDir string) (string, error) {
		return "FAIL: TestSomething", fmt.Errorf("exit status 1")
	}

	doc.commitFunc = func(workDir, user string) error {
		return nil
	}

	err := doc.FixOne("bad-repo")
	if err != nil {
		t.Fatalf("FixOne: %v", err)
	}

	// Verify error_log updated and fix_attempts incremented
	b, _ := reg.BuildByName("bad-repo")
	if b.Status != "failed" {
		t.Errorf("status = %s, want failed", b.Status)
	}
	if b.FixAttempts != 1 {
		t.Errorf("fix_attempts = %d, want 1", b.FixAttempts)
	}

	if doc.stats.Failed != 1 {
		t.Errorf("stats.Failed = %d, want 1", doc.stats.Failed)
	}
}

func TestFixBuild_MaxAttemptsReached(t *testing.T) {
	reg, dataDir := setupTestRegistry(t)
	defer reg.Close()

	gitDir := t.TempDir()
	insertTestBuild(t, reg, "hopeless-repo", "failed", "everything broken", 3)

	cfg := &config.Config{
		DataDir:        dataDir,
		GitDir:         gitDir,
		ClaudeBinary:   "claude",
		MaxFixAttempts: 3,
	}

	doc := New(cfg, reg)

	err := doc.FixOne("hopeless-repo")
	if err != nil {
		t.Fatalf("FixOne: %v", err)
	}

	b, _ := reg.BuildByName("hopeless-repo")
	if b.Status != "permanently_failed" {
		t.Errorf("status = %s, want permanently_failed", b.Status)
	}

	if doc.stats.Skipped != 1 {
		t.Errorf("stats.Skipped = %d, want 1", doc.stats.Skipped)
	}
}

func TestFixAll_NoFailedBuilds(t *testing.T) {
	reg, dataDir := setupTestRegistry(t)
	defer reg.Close()

	insertTestBuild(t, reg, "ok-repo", "shipped", "", 0)

	cfg := &config.Config{
		DataDir:        dataDir,
		GitDir:         t.TempDir(),
		ClaudeBinary:   "claude",
		MaxFixAttempts: 3,
	}

	doc := New(cfg, reg)
	err := doc.FixAll()
	if err != nil {
		t.Fatalf("FixAll: %v", err)
	}
}

func TestPrintStats(t *testing.T) {
	reg, dataDir := setupTestRegistry(t)
	defer reg.Close()

	insertTestBuild(t, reg, "a", "shipped", "", 0)
	insertTestBuild(t, reg, "b", "failed", "", 0)

	cfg := &config.Config{
		DataDir:        dataDir,
		GitDir:         t.TempDir(),
		ClaudeBinary:   "claude",
		MaxFixAttempts: 3,
	}

	doc := New(cfg, reg)

	var buf bytes.Buffer
	if err := doc.PrintStats(&buf); err != nil {
		t.Fatalf("PrintStats: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("Shipped")) {
		t.Errorf("expected 'Shipped' in output, got: %s", output)
	}
}
