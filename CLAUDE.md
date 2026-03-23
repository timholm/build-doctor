# CLAUDE.md

## Project: build-doctor

Single Go binary that automatically fixes failing builds in the claude-code-factory pipeline. Reads test errors from SQLite, sends them to Claude Code CLI, applies fixes, and retries until tests pass.

## Architecture

- `main.go` — Cobra CLI entry point with `fix`, `stats`, `serve` commands
- `internal/config/` — Environment variable loading and validation
- `internal/doctor/` — Core fix loop, prompt building, git operations, stats tracking
- `internal/registry/` — SQLite wrapper for the factory build_queue table
- `internal/api/` — HTTP monitoring API server

## Build & Test

```bash
make build    # compile binary
make test     # run all tests with race detector
make clean    # remove artifacts
```

## Key Behaviors

- Uses `modernc.org/sqlite` (pure Go, no CGO) for SQLite access
- Fix loop: clone bare repo -> build prompt -> invoke claude CLI -> run make test -> commit or retry
- Max 3 fix attempts by default before marking as permanently_failed
- All functions tested with mock hooks (cloneFunc, claudeFunc, testFunc, commitFunc)

## Environment Variables

- `FACTORY_DATA_DIR` (required) — path to registry.db
- `FACTORY_GIT_DIR` (required) — path to bare git repos
- `CLAUDE_BINARY` (default: claude) — path to Claude Code CLI
- `MAX_FIX_ATTEMPTS` (default: 3)
- `GITHUB_USER` — for git commit attribution
