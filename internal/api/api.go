package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/timholm/build-doctor/internal/registry"
)

// Server provides an HTTP API for monitoring build-doctor state.
type Server struct {
	reg       *registry.Registry
	mux       *http.ServeMux
	startedAt time.Time
}

// New creates a monitoring API server backed by the given registry.
func New(reg *registry.Registry) *Server {
	s := &Server{
		reg:       reg,
		mux:       http.NewServeMux(),
		startedAt: time.Now().UTC(),
	}
	s.routes()
	return s
}

// ListenAndServe starts the HTTP server on the given address with production timeouts.
func (s *Server) ListenAndServe(addr string) error {
	log.Printf("build-doctor API listening on %s", addr)
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return srv.ListenAndServe()
}

// ServeHTTP implements http.Handler so Server can be used directly in tests.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Handler returns the underlying http.Handler (useful for tests).
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/stats", s.handleStats)
	s.mux.HandleFunc("/status", s.handleStatus)
	s.mux.HandleFunc("/history", s.handleHistory)
	s.mux.HandleFunc("/builds", s.handleBuilds)
}

// GET /health -- liveness check with uptime
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"uptime":  time.Since(s.startedAt).String(),
		"started": s.startedAt.Format(time.RFC3339),
	})
}

// GET /stats -- aggregate build counts by status
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.reg.GetStats()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// GET /status -- queue-centric view: failed, in-progress, fixed
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	summary, err := s.reg.GetStatusSummary()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// HistoryEntry is a single fix attempt returned by /history.
type HistoryEntry struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	FixAttempts int    `json:"fix_attempts"`
	UpdatedAt   string `json:"updated_at"`
}

// GET /history -- last 50 fix attempts with success/failure
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	builds, err := s.reg.RecentBuilds(50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	entries := make([]HistoryEntry, len(builds))
	for i, b := range builds {
		entries[i] = HistoryEntry{
			ID:          b.ID,
			Name:        b.Name,
			Status:      b.Status,
			FixAttempts: b.FixAttempts,
			UpdatedAt:   b.UpdatedAt.Format(time.RFC3339),
		}
	}
	writeJSON(w, http.StatusOK, entries)
}

// GET /builds -- raw build records (last 50)
func (s *Server) handleBuilds(w http.ResponseWriter, r *http.Request) {
	builds, err := s.reg.RecentBuilds(50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, builds)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
