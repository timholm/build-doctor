package doctor

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/timholm/build-doctor/internal/api"
	"github.com/timholm/build-doctor/internal/config"
	"github.com/timholm/build-doctor/internal/registry"
)

// Doctor orchestrates the fix loop: read errors, invoke Claude, retry tests.
type Doctor struct {
	cfg   *config.Config
	reg   *registry.Registry
	stats *RunStats

	// Hooks for testing — when nil, real implementations are used.
	cloneFunc  func(bareDir, name string) (string, error)
	claudeFunc func(workDir, prompt string) error
	testFunc   func(workDir string) (string, error)
	commitFunc func(workDir, user string) error
}

// New creates a Doctor with the given config and registry.
func New(cfg *config.Config, reg *registry.Registry) *Doctor {
	return &Doctor{
		cfg:   cfg,
		reg:   reg,
		stats: &RunStats{},
	}
}

// FixAll processes all builds with status 'failed'.
func (d *Doctor) FixAll() error {
	builds, err := d.reg.FailedBuilds()
	if err != nil {
		return fmt.Errorf("querying failed builds: %w", err)
	}

	if len(builds) == 0 {
		log.Println("No failed builds found")
		return nil
	}

	log.Printf("Found %d failed build(s)", len(builds))

	for _, b := range builds {
		if err := d.fixBuild(&b); err != nil {
			log.Printf("ERROR fixing %s: %v", b.Name, err)
		}
	}

	d.stats.WriteTo(os.Stdout)
	return nil
}

// FixOne processes a single build by name.
func (d *Doctor) FixOne(name string) error {
	b, err := d.reg.BuildByName(name)
	if err != nil {
		return err
	}
	if err := d.fixBuild(b); err != nil {
		return err
	}
	d.stats.WriteTo(os.Stdout)
	return nil
}

func (d *Doctor) fixBuild(b *registry.Build) error {
	log.Printf("Fixing %s (attempt %d/%d, status: %s)", b.Name, b.FixAttempts+1, d.cfg.MaxFixAttempts, b.Status)

	if b.FixAttempts >= d.cfg.MaxFixAttempts {
		log.Printf("Skipping %s: max attempts (%d) reached", b.Name, d.cfg.MaxFixAttempts)
		d.stats.RecordSkip()
		if err := d.reg.UpdateStatus(b.ID, "permanently_failed"); err != nil {
			return fmt.Errorf("updating status: %w", err)
		}
		return nil
	}

	// Clone bare repo
	cloneFn := d.cloneFunc
	if cloneFn == nil {
		cloneFn = func(bareDir, name string) (string, error) {
			return CloneBareRepo(bareDir, name)
		}
	}

	workDir, err := cloneFn(d.cfg.GitDir, b.Name)
	if err != nil {
		return fmt.Errorf("cloning repo: %w", err)
	}
	defer CleanupWorkDir(workDir)

	// Build prompt from error log
	prompt, err := BuildPrompt(workDir, b.ErrorLog)
	if err != nil {
		return fmt.Errorf("building prompt: %w", err)
	}

	// Invoke Claude Code CLI
	claudeFn := d.claudeFunc
	if claudeFn == nil {
		claudeFn = d.invokeClaudeCLI
	}

	log.Printf("Sending errors for %s to Claude...", b.Name)
	if err := claudeFn(workDir, prompt); err != nil {
		log.Printf("Claude CLI error for %s: %v", b.Name, err)
	}

	// Run make test
	testFn := d.testFunc
	if testFn == nil {
		testFn = runMakeTest
	}

	testOutput, testErr := testFn(workDir)

	if testErr == nil {
		// Tests pass — commit, push, mark shipped
		log.Printf("Tests PASS for %s", b.Name)

		commitFn := d.commitFunc
		if commitFn == nil {
			commitFn = CommitAndPush
		}

		if err := commitFn(workDir, d.cfg.GitHubUser); err != nil {
			return fmt.Errorf("committing fixes: %w", err)
		}
		if err := d.reg.UpdateStatus(b.ID, "shipped"); err != nil {
			return fmt.Errorf("updating status to shipped: %w", err)
		}
		d.stats.RecordFix()
		return nil
	}

	// Tests still fail
	log.Printf("Tests FAIL for %s: %s", b.Name, truncate(testOutput, 200))
	if err := d.reg.UpdateErrorLog(b.ID, testOutput); err != nil {
		return fmt.Errorf("updating error log: %w", err)
	}
	d.stats.RecordFail()
	return nil
}

func (d *Doctor) invokeClaudeCLI(workDir string, prompt string) error {
	cmd := exec.Command(d.cfg.ClaudeBinary,
		"-p", prompt,
		"--permission-mode", "acceptEdits",
		"--max-turns", "15",
	)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runMakeTest(workDir string) (string, error) {
	cmd := exec.Command("make", "test")
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// PrintStats writes registry-level stats and run stats to the writer.
func (d *Doctor) PrintStats(w io.Writer) error {
	s, err := d.reg.GetStats()
	if err != nil {
		return fmt.Errorf("getting stats: %w", err)
	}

	fmt.Fprintf(w, "Registry Stats\n")
	fmt.Fprintf(w, "──────────────\n")
	fmt.Fprintf(w, "Total:    %d\n", s.Total)
	fmt.Fprintf(w, "Shipped:  %d\n", s.Shipped)
	fmt.Fprintf(w, "Failed:   %d\n", s.Failed)
	fmt.Fprintf(w, "Building: %d\n", s.Building)
	fmt.Fprintf(w, "Pending:  %d\n", s.Pending)
	if s.Total > 0 {
		fmt.Fprintf(w, "Ship Rate: %.1f%%\n", float64(s.Shipped)/float64(s.Total)*100)
	}
	return nil
}

// Serve starts the HTTP monitoring API server.
func (d *Doctor) Serve(addr string) error {
	srv := api.New(d.reg)
	return srv.ListenAndServe(addr)
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
