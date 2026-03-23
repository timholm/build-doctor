# build-doctor

Automatic test failure fixer for [claude-code-factory](https://github.com/timholm/claude-code-factory). When a factory build fails on `make test`, build-doctor reads the error log, sends it to Claude Code CLI, applies fixes, and retries until tests pass.

## How it works

1. Reads failed builds from the `build_queue` table in the factory SQLite registry
2. Clones the bare git repo to a temp directory
3. Builds a prompt from the test errors + source code
4. Invokes `claude -p <prompt> --permission-mode acceptEdits --max-turns 15`
5. Runs `make test` to verify the fix
6. If tests pass: commits, pushes to bare repo, updates registry to `shipped`
7. If still failing after max attempts: marks as `permanently_failed`

## Install

```bash
go install github.com/timholm/build-doctor@latest
```

Or build from source:

```bash
make build
```

## Usage

```bash
# Fix all failed builds
build-doctor fix

# Fix a specific repo
build-doctor fix --name my-project

# Show fix statistics
build-doctor stats

# Start HTTP monitoring API
build-doctor serve --addr :8080
```

## Configuration

All configuration via environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `FACTORY_DATA_DIR` | Yes | - | Path to directory containing `registry.db` |
| `FACTORY_GIT_DIR` | Yes | - | Path to directory containing bare git repos |
| `CLAUDE_BINARY` | No | `claude` | Path to Claude Code CLI binary |
| `MAX_FIX_ATTEMPTS` | No | `3` | Maximum fix attempts per build |
| `GITHUB_USER` | No | - | Git user for commit attribution |

## HTTP API

When running `build-doctor serve`:

- `GET /health` — health check
- `GET /stats` — registry-level statistics
- `GET /status` — queue-centric status summary
- `GET /builds` — recent builds (last 50)

## Docker

```bash
docker build -t build-doctor .
docker run -e FACTORY_DATA_DIR=/data -e FACTORY_GIT_DIR=/git \
  -v /path/to/data:/data -v /path/to/git:/git \
  build-doctor fix
```
