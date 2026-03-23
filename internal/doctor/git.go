package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CloneBareRepo clones a bare repository into a temporary working directory.
// Returns the path to the working directory.
func CloneBareRepo(bareRepoDir string, repoName string) (string, error) {
	barePath := filepath.Join(bareRepoDir, repoName+".git")
	if _, err := os.Stat(barePath); os.IsNotExist(err) {
		return "", fmt.Errorf("bare repo not found: %s", barePath)
	}

	tmpDir, err := os.MkdirTemp("", "build-doctor-"+repoName+"-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}

	cmd := exec.Command("git", "clone", barePath, tmpDir)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("cloning %s: %w", barePath, err)
	}

	return tmpDir, nil
}

// CommitAndPush stages all changes, commits with a fix message, and pushes to origin.
func CommitAndPush(workDir string, githubUser string) error {
	commands := []struct {
		args []string
		desc string
	}{
		{[]string{"git", "add", "-A"}, "staging changes"},
		{[]string{"git", "commit", "-m", "fix: auto-fix failing tests via build-doctor"}, "committing fixes"},
		{[]string{"git", "push", "origin", "HEAD"}, "pushing to origin"},
	}

	// Set git config if we have a user
	if githubUser != "" {
		for _, cfg := range [][]string{
			{"git", "config", "user.name", githubUser},
			{"git", "config", "user.email", githubUser + "@users.noreply.github.com"},
		} {
			cmd := exec.Command(cfg[0], cfg[1:]...)
			cmd.Dir = workDir
			cmd.Run() // best-effort
		}
	}

	for _, c := range commands {
		cmd := exec.Command(c.args[0], c.args[1:]...)
		cmd.Dir = workDir
		cmd.Stderr = os.Stderr
		if out, err := cmd.Output(); err != nil {
			return fmt.Errorf("%s: %w (output: %s)", c.desc, err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

// CleanupWorkDir removes a temporary working directory.
func CleanupWorkDir(workDir string) {
	os.RemoveAll(workDir)
}
