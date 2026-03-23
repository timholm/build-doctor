package doctor

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunStats_RecordAndRate(t *testing.T) {
	s := &RunStats{}

	s.RecordFix()
	s.RecordFix()
	s.RecordFail()

	if s.Attempted != 3 {
		t.Errorf("Attempted = %d, want 3", s.Attempted)
	}
	if s.Fixed != 2 {
		t.Errorf("Fixed = %d, want 2", s.Fixed)
	}
	if s.Failed != 1 {
		t.Errorf("Failed = %d, want 1", s.Failed)
	}

	rate := s.SuccessRate()
	if rate < 66.0 || rate > 67.0 {
		t.Errorf("SuccessRate = %.1f, want ~66.7", rate)
	}
}

func TestRunStats_ZeroAttempts(t *testing.T) {
	s := &RunStats{}
	if s.SuccessRate() != 0 {
		t.Error("expected 0% for no attempts")
	}
}

func TestRunStats_WriteTo(t *testing.T) {
	s := &RunStats{Attempted: 5, Fixed: 3, Failed: 1, Skipped: 1}

	var buf bytes.Buffer
	s.WriteTo(&buf)

	output := buf.String()
	if !strings.Contains(output, "Attempted: 5") {
		t.Error("should contain attempted count")
	}
	if !strings.Contains(output, "Fixed:     3") {
		t.Error("should contain fixed count")
	}
	if !strings.Contains(output, "Fix Rate:  60.0%") {
		t.Error("should contain fix rate")
	}
}
