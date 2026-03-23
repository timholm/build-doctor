# build-doctor

Automatic test failure fixer for [claude-code-factory](https://github.com/timholm/claude-code-factory). When a factory build fails on `make test`, build-doctor reads the error log, sends it to Claude Code CLI, applies fixes, and retries until tests pass.

Single Go binary. No runtime dependencies beyond `git` and `claude` CLI. Uses pure-Go SQLite (`modernc.org/sqlite`) -- no CGO required.

## How It Works

1. Reads failed builds from the `build_queue` table in the factory SQLite registry.
2. Clones the bare git repo to a temporary working directory.
3. Builds a prompt from the test error output and relevant source files.
4. Invokes `claude -p <prompt> --permission-mode acceptEdits --max-turns 15`.
5. Runs `make test` to verify the fix.
6. If tests pass: commits, pushes to bare repo, updates registry status to `shipped`.
7. If tests still fail: stores the new error log, increments `fix_attempts`, retries next run.
8. After max attempts (default 3): marks the build as `permanently_failed`.

## Install

```bash
go install github.com/timholm/build-doctor@latest
```

Or build from source:

```bash
git clone https://github.com/timholm/build-doctor.git
cd build-doctor
make build
```

## Usage

```bash
# Fix all failed builds in the registry
build-doctor fix

# Fix a specific repo by name
build-doctor fix --name my-project

# Show fix statistics (shipped, failed, pending counts)
build-doctor stats

# Start the HTTP monitoring API
build-doctor serve
build-doctor serve --addr :9090
```

## Configuration

All configuration via environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `FACTORY_DATA_DIR` | Yes | -- | Path to directory containing `registry.db` |
| `FACTORY_GIT_DIR` | Yes | -- | Path to directory containing bare git repos |
| `CLAUDE_BINARY` | No | `claude` | Path to Claude Code CLI binary |
| `MAX_FIX_ATTEMPTS` | No | `3` | Maximum fix attempts per build before giving up |
| `GITHUB_USER` | No | -- | Git user for commit attribution |

## HTTP API

Start the monitoring server with `build-doctor serve`. Default listen address is `:8080`.

### GET /health

Liveness check with uptime.

```json
{
  "status": "ok",
  "uptime": "2h15m30s",
  "started": "2026-03-23T10:00:00Z"
}
```

### GET /status

Current fix queue snapshot -- how many builds are failed, in-progress, and fixed.

```json
{
  "failed": 3,
  "in_progress": 1,
  "fixed": 12,
  "total": 16
}
```

### GET /history

Last 50 fix attempts ordered by most recent, with success/failure status.

```json
[
  {
    "id": 42,
    "name": "url-shortener",
    "status": "shipped",
    "fix_attempts": 2,
    "updated_at": "2026-03-23T11:30:00Z"
  },
  {
    "id": 41,
    "name": "rate-limiter",
    "status": "failed",
    "fix_attempts": 1,
    "updated_at": "2026-03-23T11:25:00Z"
  }
]
```

### GET /stats

Registry-level aggregate statistics.

```json
{
  "Total": 20,
  "Failed": 3,
  "Shipped": 15,
  "Building": 1,
  "Pending": 1
}
```

### GET /builds

Raw build records (last 50) with full detail including error logs.

## Docker

```bash
# Build the image
docker build -t build-doctor .

# Run a fix pass
docker run \
  -e FACTORY_DATA_DIR=/data \
  -e FACTORY_GIT_DIR=/git \
  -e ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY \
  -v /path/to/data:/data \
  -v /path/to/git:/git \
  build-doctor fix

# Run the monitoring API
docker run -p 8080:8080 \
  -e FACTORY_DATA_DIR=/data \
  -e FACTORY_GIT_DIR=/git \
  -v /path/to/data:/data \
  -v /path/to/git:/git \
  build-doctor serve
```

## Kubernetes Deployment

build-doctor runs as a CronJob for periodic fix passes and a Deployment for the monitoring API.

### Fix CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: build-doctor-fix
  namespace: factory
spec:
  schedule: "*/10 * * * *"
  concurrencyPolicy: Forbid
  jobTemplate:
    spec:
      backoffLimit: 0
      template:
        spec:
          restartPolicy: Never
          containers:
            - name: build-doctor
              image: build-doctor:latest
              command: ["build-doctor", "fix"]
              env:
                - name: FACTORY_DATA_DIR
                  value: /data
                - name: FACTORY_GIT_DIR
                  value: /git
                - name: ANTHROPIC_API_KEY
                  valueFrom:
                    secretKeyRef:
                      name: anthropic-credentials
                      key: api-key
                - name: MAX_FIX_ATTEMPTS
                  value: "3"
                - name: GITHUB_USER
                  value: build-doctor-bot
              volumeMounts:
                - name: factory-data
                  mountPath: /data
                - name: factory-git
                  mountPath: /git
          volumes:
            - name: factory-data
              persistentVolumeClaim:
                claimName: factory-data-pvc
            - name: factory-git
              persistentVolumeClaim:
                claimName: factory-git-pvc
```

### Monitoring API Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: build-doctor-api
  namespace: factory
spec:
  replicas: 1
  selector:
    matchLabels:
      app: build-doctor-api
  template:
    metadata:
      labels:
        app: build-doctor-api
    spec:
      containers:
        - name: api
          image: build-doctor:latest
          command: ["build-doctor", "serve", "--addr", ":8080"]
          ports:
            - containerPort: 8080
              name: http
          env:
            - name: FACTORY_DATA_DIR
              value: /data
            - name: FACTORY_GIT_DIR
              value: /git
          volumeMounts:
            - name: factory-data
              mountPath: /data
              readOnly: true
            - name: factory-git
              mountPath: /git
              readOnly: true
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 30
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 2
            periodSeconds: 10
          resources:
            requests:
              memory: 64Mi
              cpu: 50m
            limits:
              memory: 128Mi
              cpu: 200m
      volumes:
        - name: factory-data
          persistentVolumeClaim:
            claimName: factory-data-pvc
        - name: factory-git
          persistentVolumeClaim:
            claimName: factory-git-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: build-doctor-api
  namespace: factory
spec:
  selector:
    app: build-doctor-api
  ports:
    - port: 8080
      targetPort: 8080
      name: http
```

## Architecture

```
main.go                     Cobra CLI: fix, stats, serve commands
internal/
  config/config.go          Env var loading + validation
  doctor/doctor.go          Core fix loop orchestrator
  doctor/git.go             Clone bare repos, commit + push
  doctor/prompt.go          Build Claude prompts from errors + sources
  doctor/stats.go           Runtime fix statistics
  registry/registry.go      SQLite access for build_queue table
  api/api.go                HTTP monitoring server
```

## Development

```bash
make build    # compile binary
make test     # run all tests with -race
make lint     # golangci-lint
make fmt      # gofmt + goimports
make clean    # remove artifacts
```

## License

MIT
