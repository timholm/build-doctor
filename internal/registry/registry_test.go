package registry

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) *Registry {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "registry.db")

	reg, err := Open(dbPath)
	if err != nil {
		// Open will fail because the db doesn't exist yet.
		// Create it manually.
		f, _ := os.Create(dbPath)
		f.Close()
		reg, err = Open(dbPath)
		if err != nil {
			t.Fatalf("opening db: %v", err)
		}
	}

	// Create the build_queue table
	_, err = reg.db.Exec(`
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

	return reg
}

func insertBuild(t *testing.T, reg *Registry, name, status, errorLog string, fixAttempts int) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := reg.db.Exec(
		"INSERT INTO build_queue (name, status, error_log, fix_attempts, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		name, status, errorLog, fixAttempts, now, now,
	)
	if err != nil {
		t.Fatalf("inserting build: %v", err)
	}
}

func TestFailedBuilds(t *testing.T) {
	reg := setupTestDB(t)
	defer reg.Close()

	insertBuild(t, reg, "repo-a", "failed", "test error A", 0)
	insertBuild(t, reg, "repo-b", "shipped", "", 0)
	insertBuild(t, reg, "repo-c", "failed", "test error C", 1)

	builds, err := reg.FailedBuilds()
	if err != nil {
		t.Fatalf("FailedBuilds: %v", err)
	}

	if len(builds) != 2 {
		t.Fatalf("expected 2 failed builds, got %d", len(builds))
	}

	names := map[string]bool{}
	for _, b := range builds {
		names[b.Name] = true
	}
	if !names["repo-a"] || !names["repo-c"] {
		t.Errorf("unexpected builds: %v", builds)
	}
}

func TestBuildByName(t *testing.T) {
	reg := setupTestDB(t)
	defer reg.Close()

	insertBuild(t, reg, "my-project", "failed", "compile error", 2)

	b, err := reg.BuildByName("my-project")
	if err != nil {
		t.Fatalf("BuildByName: %v", err)
	}
	if b.Name != "my-project" {
		t.Errorf("Name = %s", b.Name)
	}
	if b.ErrorLog != "compile error" {
		t.Errorf("ErrorLog = %s", b.ErrorLog)
	}
	if b.FixAttempts != 2 {
		t.Errorf("FixAttempts = %d", b.FixAttempts)
	}
}

func TestBuildByName_NotFound(t *testing.T) {
	reg := setupTestDB(t)
	defer reg.Close()

	_, err := reg.BuildByName("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing build")
	}
}

func TestUpdateStatus(t *testing.T) {
	reg := setupTestDB(t)
	defer reg.Close()

	insertBuild(t, reg, "repo-x", "failed", "err", 0)

	b, _ := reg.BuildByName("repo-x")
	if err := reg.UpdateStatus(b.ID, "shipped"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	b2, _ := reg.BuildByName("repo-x")
	if b2.Status != "shipped" {
		t.Errorf("status = %s, want shipped", b2.Status)
	}
}

func TestUpdateErrorLog(t *testing.T) {
	reg := setupTestDB(t)
	defer reg.Close()

	insertBuild(t, reg, "repo-y", "failed", "old error", 1)

	b, _ := reg.BuildByName("repo-y")
	if err := reg.UpdateErrorLog(b.ID, "new error"); err != nil {
		t.Fatalf("UpdateErrorLog: %v", err)
	}

	b2, _ := reg.BuildByName("repo-y")
	if b2.ErrorLog != "new error" {
		t.Errorf("ErrorLog = %s", b2.ErrorLog)
	}
	if b2.FixAttempts != 2 {
		t.Errorf("FixAttempts = %d, want 2", b2.FixAttempts)
	}
}

func TestGetStats(t *testing.T) {
	reg := setupTestDB(t)
	defer reg.Close()

	insertBuild(t, reg, "a", "failed", "", 0)
	insertBuild(t, reg, "b", "failed", "", 0)
	insertBuild(t, reg, "c", "shipped", "", 0)
	insertBuild(t, reg, "d", "building", "", 0)
	insertBuild(t, reg, "e", "pending", "", 0)

	s, err := reg.GetStats()
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}

	if s.Total != 5 {
		t.Errorf("Total = %d", s.Total)
	}
	if s.Failed != 2 {
		t.Errorf("Failed = %d", s.Failed)
	}
	if s.Shipped != 1 {
		t.Errorf("Shipped = %d", s.Shipped)
	}
	if s.Building != 1 {
		t.Errorf("Building = %d", s.Building)
	}
	if s.Pending != 1 {
		t.Errorf("Pending = %d", s.Pending)
	}
}
