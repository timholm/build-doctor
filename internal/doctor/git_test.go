package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initBareRepo creates a bare git repo with one commit, returns the bare repo path.
func initBareRepo(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()

	// Create a normal repo first, add a commit, then clone --bare
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("creating src dir: %v", err)
	}

	cmds := [][]string{
		{"git", "init", srcDir},
		{"git", "-C", srcDir, "config", "user.name", "test"},
		{"git", "-C", srcDir, "config", "user.email", "test@test.com"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("running %v: %v\n%s", args, err, out)
		}
	}

	// Write a file and commit
	if err := os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatalf("writing main.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "Makefile"), []byte("test:\n\tgo test ./...\n"), 0644); err != nil {
		t.Fatalf("writing Makefile: %v", err)
	}

	commitCmds := [][]string{
		{"git", "-C", srcDir, "add", "-A"},
		{"git", "-C", srcDir, "commit", "-m", "initial commit"},
	}
	for _, args := range commitCmds {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("running %v: %v\n%s", args, err, out)
		}
	}

	// Clone to bare repo at the expected path
	bareDir := filepath.Join(dir, "bare")
	if err := os.MkdirAll(bareDir, 0755); err != nil {
		t.Fatalf("creating bare dir: %v", err)
	}

	barePath := filepath.Join(bareDir, name+".git")
	cmd := exec.Command("git", "clone", "--bare", srcDir, barePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cloning bare: %v\n%s", err, out)
	}

	return bareDir
}

func TestCloneBareRepo(t *testing.T) {
	bareDir := initBareRepo(t, "test-project")

	workDir, err := CloneBareRepo(bareDir, "test-project")
	if err != nil {
		t.Fatalf("CloneBareRepo: %v", err)
	}
	defer CleanupWorkDir(workDir)

	// Verify main.go exists in the cloned repo
	mainPath := filepath.Join(workDir, "main.go")
	if _, err := os.Stat(mainPath); os.IsNotExist(err) {
		t.Error("main.go should exist in cloned working directory")
	}

	// Verify it's a git repo
	gitDir := filepath.Join(workDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Error("cloned directory should contain .git")
	}
}

func TestCloneBareRepo_NotFound(t *testing.T) {
	dir := t.TempDir()

	_, err := CloneBareRepo(dir, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent bare repo")
	}
	if !strings.Contains(err.Error(), "bare repo not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCommitAndPush(t *testing.T) {
	bareDir := initBareRepo(t, "push-test")

	// Clone the bare repo to a working directory
	workDir, err := CloneBareRepo(bareDir, "push-test")
	if err != nil {
		t.Fatalf("CloneBareRepo: %v", err)
	}
	defer CleanupWorkDir(workDir)

	// Make a change in the working directory
	fixedContent := "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"fixed\") }\n"
	if err := os.WriteFile(filepath.Join(workDir, "main.go"), []byte(fixedContent), 0644); err != nil {
		t.Fatalf("writing fix: %v", err)
	}

	// CommitAndPush should succeed
	if err := CommitAndPush(workDir, "test-bot"); err != nil {
		t.Fatalf("CommitAndPush: %v", err)
	}

	// Verify the commit exists by checking git log
	cmd := exec.Command("git", "-C", workDir, "log", "--oneline", "-1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(string(out), "auto-fix") {
		t.Errorf("commit message should contain 'auto-fix', got: %s", string(out))
	}

	// Verify the push landed in the bare repo by cloning again and checking content
	verifyDir := t.TempDir()
	barePath := filepath.Join(bareDir, "push-test.git")
	cmd = exec.Command("git", "clone", barePath, verifyDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("verifying clone: %v\n%s", err, out)
	}

	data, err := os.ReadFile(filepath.Join(verifyDir, "main.go"))
	if err != nil {
		t.Fatalf("reading verified main.go: %v", err)
	}
	if !strings.Contains(string(data), "fixed") {
		t.Error("pushed changes should appear in bare repo clone")
	}
}

func TestCommitAndPush_WithGitHubUser(t *testing.T) {
	bareDir := initBareRepo(t, "user-test")

	workDir, err := CloneBareRepo(bareDir, "user-test")
	if err != nil {
		t.Fatalf("CloneBareRepo: %v", err)
	}
	defer CleanupWorkDir(workDir)

	// Make a change
	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("writing README: %v", err)
	}

	if err := CommitAndPush(workDir, "mybot"); err != nil {
		t.Fatalf("CommitAndPush: %v", err)
	}

	// Verify git config was set
	cmd := exec.Command("git", "-C", workDir, "config", "user.name")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git config user.name: %v", err)
	}
	if strings.TrimSpace(string(out)) != "mybot" {
		t.Errorf("user.name = %q, want mybot", strings.TrimSpace(string(out)))
	}

	cmd = exec.Command("git", "-C", workDir, "config", "user.email")
	out, err = cmd.Output()
	if err != nil {
		t.Fatalf("git config user.email: %v", err)
	}
	if strings.TrimSpace(string(out)) != "mybot@users.noreply.github.com" {
		t.Errorf("user.email = %q", strings.TrimSpace(string(out)))
	}
}

func TestCommitAndPush_NoGitHubUser(t *testing.T) {
	bareDir := initBareRepo(t, "nouser-test")

	workDir, err := CloneBareRepo(bareDir, "nouser-test")
	if err != nil {
		t.Fatalf("CloneBareRepo: %v", err)
	}
	defer CleanupWorkDir(workDir)

	// Set a default git identity so the commit works even without GITHUB_USER
	exec.Command("git", "-C", workDir, "config", "user.name", "fallback").Run()
	exec.Command("git", "-C", workDir, "config", "user.email", "fallback@test.com").Run()

	// Make a change
	if err := os.WriteFile(filepath.Join(workDir, "new.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	// Empty github user should still work (skips config setting)
	if err := CommitAndPush(workDir, ""); err != nil {
		t.Fatalf("CommitAndPush with empty user: %v", err)
	}
}

func TestCleanupWorkDir(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("data"), 0644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	// Create a subdirectory to clean
	subDir := filepath.Join(dir, "cleanup-target")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "file.txt"), []byte("x"), 0644)

	CleanupWorkDir(subDir)

	if _, err := os.Stat(subDir); !os.IsNotExist(err) {
		t.Error("CleanupWorkDir should remove the directory")
	}

	// Parent dir should still exist
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("parent directory should still exist")
	}
}

func TestCloneBareRepo_ReturnsUniqueTempDirs(t *testing.T) {
	bareDir := initBareRepo(t, "unique-test")

	dir1, err := CloneBareRepo(bareDir, "unique-test")
	if err != nil {
		t.Fatalf("first clone: %v", err)
	}
	defer CleanupWorkDir(dir1)

	dir2, err := CloneBareRepo(bareDir, "unique-test")
	if err != nil {
		t.Fatalf("second clone: %v", err)
	}
	defer CleanupWorkDir(dir2)

	if dir1 == dir2 {
		t.Error("two clones should return different temp directories")
	}
}
