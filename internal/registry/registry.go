package registry

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Build represents a row in the build_queue table.
type Build struct {
	ID           int
	Name         string
	Status       string
	ErrorLog     string
	FixAttempts  int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Registry wraps SQLite access for the factory build_queue.
type Registry struct {
	db *sql.DB
}

// Open connects to the SQLite database at the given path.
func Open(path string) (*Registry, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database %s: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database %s: %w", path, err)
	}
	return &Registry{db: db}, nil
}

// Close closes the underlying database connection.
func (r *Registry) Close() error {
	return r.db.Close()
}

// DB returns the underlying sql.DB for direct access in tests.
func (r *Registry) DB() *sql.DB {
	return r.db
}

// FailedBuilds returns all builds with status 'failed'.
func (r *Registry) FailedBuilds() ([]Build, error) {
	return r.queryBuilds("SELECT id, name, status, COALESCE(error_log, ''), COALESCE(fix_attempts, 0), created_at, updated_at FROM build_queue WHERE status = 'failed'")
}

// BuildByName returns a single build by name.
func (r *Registry) BuildByName(name string) (*Build, error) {
	builds, err := r.queryBuilds("SELECT id, name, status, COALESCE(error_log, ''), COALESCE(fix_attempts, 0), created_at, updated_at FROM build_queue WHERE name = ?", name)
	if err != nil {
		return nil, err
	}
	if len(builds) == 0 {
		return nil, fmt.Errorf("build %q not found", name)
	}
	return &builds[0], nil
}

// UpdateStatus sets the status of a build.
func (r *Registry) UpdateStatus(id int, status string) error {
	_, err := r.db.Exec("UPDATE build_queue SET status = ?, updated_at = ? WHERE id = ?", status, time.Now().UTC().Format(time.RFC3339), id)
	return err
}

// UpdateErrorLog sets the error log and increments fix_attempts.
func (r *Registry) UpdateErrorLog(id int, errorLog string) error {
	_, err := r.db.Exec("UPDATE build_queue SET error_log = ?, fix_attempts = fix_attempts + 1, updated_at = ? WHERE id = ?", errorLog, time.Now().UTC().Format(time.RFC3339), id)
	return err
}

// IncrementFixAttempts bumps the fix_attempts counter for a build.
func (r *Registry) IncrementFixAttempts(id int) error {
	_, err := r.db.Exec("UPDATE build_queue SET fix_attempts = fix_attempts + 1, updated_at = ? WHERE id = ?", time.Now().UTC().Format(time.RFC3339), id)
	return err
}

// Stats returns counts by status.
type Stats struct {
	Total    int
	Failed   int
	Shipped  int
	Building int
	Pending  int
}

// GetStats returns aggregate build statistics.
func (r *Registry) GetStats() (*Stats, error) {
	s := &Stats{}
	rows, err := r.db.Query("SELECT status, COUNT(*) FROM build_queue GROUP BY status")
	if err != nil {
		return nil, fmt.Errorf("querying stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scanning stats row: %w", err)
		}
		s.Total += count
		switch status {
		case "failed":
			s.Failed = count
		case "shipped":
			s.Shipped = count
		case "building":
			s.Building = count
		case "pending":
			s.Pending = count
		}
	}
	return s, rows.Err()
}

// StatusSummary provides a snapshot of the fix queue for the /status endpoint.
type StatusSummary struct {
	Failed     int `json:"failed"`
	InProgress int `json:"in_progress"`
	Fixed      int `json:"fixed"`
	Total      int `json:"total"`
}

// GetStatusSummary returns a queue-centric view: how many failed, in-progress, fixed.
func (r *Registry) GetStatusSummary() (*StatusSummary, error) {
	s := &StatusSummary{}
	rows, err := r.db.Query("SELECT status, COUNT(*) FROM build_queue GROUP BY status")
	if err != nil {
		return nil, fmt.Errorf("querying status summary: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scanning status row: %w", err)
		}
		s.Total += count
		switch status {
		case "failed", "permanently_failed":
			s.Failed += count
		case "building":
			s.InProgress += count
		case "shipped":
			s.Fixed += count
		}
	}
	return s, rows.Err()
}

// RecentBuilds returns the last N builds ordered by most recently updated.
func (r *Registry) RecentBuilds(limit int) ([]Build, error) {
	return r.queryBuilds(
		"SELECT id, name, status, COALESCE(error_log, ''), COALESCE(fix_attempts, 0), created_at, updated_at FROM build_queue ORDER BY updated_at DESC LIMIT ?",
		limit,
	)
}

func (r *Registry) queryBuilds(query string, args ...interface{}) ([]Build, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying builds: %w", err)
	}
	defer rows.Close()

	var builds []Build
	for rows.Next() {
		var b Build
		var createdAt, updatedAt string
		if err := rows.Scan(&b.ID, &b.Name, &b.Status, &b.ErrorLog, &b.FixAttempts, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scanning build row: %w", err)
		}
		b.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		b.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		builds = append(builds, b)
	}
	return builds, rows.Err()
}
