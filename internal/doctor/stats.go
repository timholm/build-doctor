package doctor

import (
	"fmt"
	"io"
	"sync"
)

// RunStats tracks fix outcomes during a doctor run.
type RunStats struct {
	mu        sync.Mutex
	Attempted int
	Fixed     int
	Failed    int
	Skipped   int
}

// RecordFix records a successful fix.
func (s *RunStats) RecordFix() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Attempted++
	s.Fixed++
}

// RecordFail records a failed fix attempt.
func (s *RunStats) RecordFail() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Attempted++
	s.Failed++
}

// RecordSkip records a skipped build (e.g. max attempts exceeded).
func (s *RunStats) RecordSkip() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Skipped++
}

// SuccessRate returns the percentage of attempted builds that were fixed.
func (s *RunStats) SuccessRate() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Attempted == 0 {
		return 0
	}
	return float64(s.Fixed) / float64(s.Attempted) * 100
}

// WriteTo writes a human-readable stats summary to w.
func (s *RunStats) WriteTo(w io.Writer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintf(w, "Build Doctor Stats\n")
	fmt.Fprintf(w, "──────────────────\n")
	fmt.Fprintf(w, "Attempted: %d\n", s.Attempted)
	fmt.Fprintf(w, "Fixed:     %d\n", s.Fixed)
	fmt.Fprintf(w, "Failed:    %d\n", s.Failed)
	fmt.Fprintf(w, "Skipped:   %d\n", s.Skipped)
	if s.Attempted > 0 {
		fmt.Fprintf(w, "Fix Rate:  %.1f%%\n", float64(s.Fixed)/float64(s.Attempted)*100)
	}
}
