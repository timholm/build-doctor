# AGENTS.md

## build-doctor

### Purpose
Automatic test failure fixer for the claude-code-factory pipeline. Closes the loop on failed builds by sending test errors to Claude Code CLI and retrying until tests pass.

### Agent Integration
This tool is designed to be called by autonomous build agents. It integrates with:
- **claude-code-factory** — reads from the shared SQLite registry (`build_queue` table)
- **Claude Code CLI** — invokes `claude -p` with error context to generate fixes
- **Git** — clones bare repos, commits fixes, pushes back

### Commands
- `build-doctor fix` — process all failed builds in the queue
- `build-doctor fix --name <repo>` — fix one specific repo
- `build-doctor stats` — display fix success rates from the registry
- `build-doctor serve` — HTTP monitoring API for health, stats, and build history

### Fix Loop
1. Query `build_queue WHERE status='failed'`
2. Clone bare repo from `$FACTORY_GIT_DIR/<name>.git`
3. Build prompt from error log + source files
4. Run `claude -p <prompt> --permission-mode acceptEdits --max-turns 15`
5. Run `make test`
6. Pass: commit, push, mark `shipped`
7. Fail: update error_log, increment fix_attempts, retry up to `MAX_FIX_ATTEMPTS`
8. Exhausted: mark `permanently_failed`

### Environment
- `FACTORY_DATA_DIR` — directory containing `registry.db`
- `FACTORY_GIT_DIR` — directory containing bare git repos
- `CLAUDE_BINARY` — Claude Code CLI path (default: `claude`)
- `MAX_FIX_ATTEMPTS` — max retries (default: 3)
- `GITHUB_USER` — git commit author
