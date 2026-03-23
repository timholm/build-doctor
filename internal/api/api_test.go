package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/timholm/build-doctor/internal/registry"

	_ "modernc.org/sqlite"
)

func setupTestAPI(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "registry.db")

	f, _ := os.Create(dbPath)
	f.Close()

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

	now := time.Now().UTC().Format(time.RFC3339)
	db.Exec("INSERT INTO build_queue (name, status, error_log, fix_attempts, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)", "repo-a", "failed", "test error", 1, now, now)
	db.Exec("INSERT INTO build_queue (name, status, error_log, fix_attempts, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)", "repo-b", "shipped", "", 0, now, now)
	db.Close()

	reg, err := registry.Open(dbPath)
	if err != nil {
		t.Fatalf("opening registry: %v", err)
	}
	t.Cleanup(func() { reg.Close() })

	return New(reg)
}

func TestHealthEndpoint(t *testing.T) {
	srv := setupTestAPI(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("status = %s", resp["status"])
	}
}

func TestStatsEndpoint(t *testing.T) {
	srv := setupTestAPI(t)

	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp registry.Stats
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 2 {
		t.Errorf("Total = %d, want 2", resp.Total)
	}
}

func TestStatusEndpoint(t *testing.T) {
	srv := setupTestAPI(t)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp registry.StatusSummary
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 2 {
		t.Errorf("Total = %d, want 2", resp.Total)
	}
	if resp.Failed != 1 {
		t.Errorf("Failed = %d, want 1", resp.Failed)
	}
	if resp.Fixed != 1 {
		t.Errorf("Fixed = %d, want 1", resp.Fixed)
	}
}

func TestBuildsEndpoint(t *testing.T) {
	srv := setupTestAPI(t)

	req := httptest.NewRequest(http.MethodGet, "/builds", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var builds []registry.Build
	json.NewDecoder(w.Body).Decode(&builds)
	if len(builds) != 2 {
		t.Errorf("got %d builds, want 2", len(builds))
	}
}

func TestHistoryEndpoint(t *testing.T) {
	srv := setupTestAPI(t)

	req := httptest.NewRequest(http.MethodGet, "/history", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var entries []HistoryEntry
	json.NewDecoder(w.Body).Decode(&entries)
	if len(entries) != 2 {
		t.Errorf("got %d entries, want 2", len(entries))
	}

	// Verify structure
	for _, e := range entries {
		if e.Name == "" {
			t.Error("expected non-empty name")
		}
		if e.Status == "" {
			t.Error("expected non-empty status")
		}
		if e.UpdatedAt == "" {
			t.Error("expected non-empty updated_at")
		}
	}
}

func TestHistoryEndpoint_Empty(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "empty.db")
	f, _ := os.Create(dbPath)
	f.Close()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	db.Exec(`CREATE TABLE IF NOT EXISTS build_queue (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		error_log TEXT,
		fix_attempts INTEGER DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	db.Close()

	reg, err := registry.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()

	srv := New(reg)
	req := httptest.NewRequest(http.MethodGet, "/history", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var entries []HistoryEntry
	json.NewDecoder(w.Body).Decode(&entries)
	if len(entries) != 0 {
		t.Errorf("got %d entries, want 0", len(entries))
	}
}

func TestHealthEndpoint_HasUptime(t *testing.T) {
	srv := setupTestAPI(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if _, ok := resp["uptime"]; !ok {
		t.Error("expected uptime field in health response")
	}
	if _, ok := resp["started"]; !ok {
		t.Error("expected started field in health response")
	}
}

func TestContentTypeJSON(t *testing.T) {
	srv := setupTestAPI(t)

	for _, path := range []string{"/health", "/status", "/history", "/stats", "/builds"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		ct := w.Header().Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("%s: Content-Type = %s, want application/json", path, ct)
		}
	}
}
