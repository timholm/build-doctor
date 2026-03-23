package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BuildPrompt constructs the prompt sent to Claude Code CLI to fix a failing build.
// It includes the test error output and relevant source files from the working directory.
func BuildPrompt(workDir string, errorLog string) (string, error) {
	var b strings.Builder

	b.WriteString("You are fixing a Go project that has failing tests.\n\n")
	b.WriteString("## Test Errors\n\n```\n")
	b.WriteString(errorLog)
	b.WriteString("\n```\n\n")

	sources, err := collectSources(workDir)
	if err != nil {
		return "", fmt.Errorf("collecting sources: %w", err)
	}

	if len(sources) > 0 {
		b.WriteString("## Source Files\n\n")
		for _, sf := range sources {
			b.WriteString(fmt.Sprintf("### %s\n```go\n%s\n```\n\n", sf.path, sf.content))
		}
	}

	b.WriteString("## Instructions\n\n")
	b.WriteString("1. Read and understand the test errors above.\n")
	b.WriteString("2. Fix the source code so that `make test` passes.\n")
	b.WriteString("3. Do NOT change the test expectations unless the tests themselves are clearly wrong.\n")
	b.WriteString("4. Keep fixes minimal — only change what is needed to make tests pass.\n")
	b.WriteString("5. After making changes, run `make test` to verify.\n")

	return b.String(), nil
}

type sourceFile struct {
	path    string
	content string
}

func collectSources(root string) ([]sourceFile, error) {
	var files []sourceFile
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		ext := filepath.Ext(path)

		// Include Go files, Makefile, and config files
		include := ext == ".go" || ext == ".mod" || ext == ".sum" ||
			info.Name() == "Makefile" || ext == ".yaml" || ext == ".yml"
		if !include {
			return nil
		}

		// Skip very large files (>100KB)
		if info.Size() > 100*1024 {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		files = append(files, sourceFile{path: rel, content: string(data)})
		return nil
	})
	return files, err
}
