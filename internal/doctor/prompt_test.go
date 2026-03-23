package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPrompt(t *testing.T) {
	dir := t.TempDir()

	// Create some source files
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "internal"), 0755)
	os.WriteFile(filepath.Join(dir, "internal", "app.go"), []byte("package internal\n"), 0644)
	os.WriteFile(filepath.Join(dir, "Makefile"), []byte("test:\n\tgo test ./...\n"), 0644)

	errorLog := "--- FAIL: TestAdd (0.00s)\n    calc_test.go:10: expected 4, got 3"

	prompt, err := BuildPrompt(dir, errorLog)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	// Should contain the error log
	if !strings.Contains(prompt, "TestAdd") {
		t.Error("prompt should contain error log")
	}

	// Should contain source files
	if !strings.Contains(prompt, "main.go") {
		t.Error("prompt should contain main.go")
	}
	if !strings.Contains(prompt, "Makefile") {
		t.Error("prompt should contain Makefile")
	}

	// Should contain instructions
	if !strings.Contains(prompt, "make test") {
		t.Error("prompt should contain 'make test' instruction")
	}
}

func TestBuildPrompt_SkipsGitDir(t *testing.T) {
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("gitconfig"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	prompt, err := BuildPrompt(dir, "error")
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	if strings.Contains(prompt, "gitconfig") {
		t.Error("prompt should not contain .git directory contents")
	}
}

func TestBuildPrompt_SkipsLargeFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a file >100KB
	bigContent := make([]byte, 200*1024)
	os.WriteFile(filepath.Join(dir, "big.go"), bigContent, 0644)
	os.WriteFile(filepath.Join(dir, "small.go"), []byte("package main\n"), 0644)

	prompt, err := BuildPrompt(dir, "error")
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	if strings.Contains(prompt, "big.go") {
		t.Error("prompt should skip files >100KB")
	}
	if !strings.Contains(prompt, "small.go") {
		t.Error("prompt should include small files")
	}
}

func TestCollectSources_SkipsVendor(t *testing.T) {
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, "vendor"), 0755)
	os.WriteFile(filepath.Join(dir, "vendor", "lib.go"), []byte("package vendor\n"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	files, err := collectSources(dir)
	if err != nil {
		t.Fatalf("collectSources: %v", err)
	}

	for _, f := range files {
		if strings.Contains(f.path, "vendor") {
			t.Errorf("should skip vendor dir, got %s", f.path)
		}
	}
}
