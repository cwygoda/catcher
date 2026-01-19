# Catcher

Webhook-based URL processor. Receives URLs via HTTP, queues them in SQLite, processes via background worker with regex-based handler dispatch.

## Usage

```bash
# Start server
go run ./cmd/catcher

# Submit URL
curl -X POST localhost:8080/webhook -d '{"url":"https://youtube.com/watch?v=dQw4w9WgXcQ"}'

# Check job status
curl localhost:8080/jobs/1
```

## Configuration

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--port` | `CATCHER_PORT` | 8080 | HTTP server port |
| `--db` | `CATCHER_DB` | `$XDG_CACHE_HOME/catcher/jobs.db` | SQLite database path |
| `--poll-interval` | - | 5s | Worker poll interval |
| `--max-retries` | - | 3 | Max retry attempts |
| `--config` | - | `$XDG_CONFIG_HOME/catcher/config.toml` | Config file path |
| - | `CATCHER_SECRET` | - | Webhook signing secret (see below) |

### Webhook Verification

When `secret` is configured (via config file or `CATCHER_SECRET` env), all `/webhook` requests require signed headers:

| Header | Description |
|--------|-------------|
| `X-Timestamp` | ISO8601/RFC3339 timestamp (e.g., `2024-01-15T10:30:00Z`) |
| `X-Signature` | SHA256 hex signature |

**Signature calculation:**
```
SHA256("${X-Timestamp}\n${RequestBody}\n${secret}")
```

**Validation rules:**
- Timestamp must be within 5 minutes of server time (replay protection)
- Signature must match exactly

**Config file:**
```toml
secret = "your-secret-here"

[[processor]]
# ...
```

**Example request:**
```bash
SECRET="your-secret"
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
BODY='{"url":"https://youtube.com/watch?v=abc123"}'
SIGNATURE=$(printf "%s\n%s\n%s" "$TIMESTAMP" "$BODY" "$SECRET" | sha256sum | cut -d' ' -f1)

curl -X POST localhost:8080/webhook \
  -H "Content-Type: application/json" \
  -H "X-Timestamp: $TIMESTAMP" \
  -H "X-Signature: $SIGNATURE" \
  -d "$BODY"
```

When no secret is configured, verification is disabled.

## API

### POST /webhook
Submit URL for processing.

```json
{"url": "https://youtube.com/watch?v=..."}
```

Returns:
```json
{"id": 1, "url": "...", "status": "pending", "attempts": 0, "created_at": "...", "updated_at": "..."}
```

### GET /jobs/:id
Get job status.

### GET /health
Health check.

## Processors

Processors are defined in `config.toml`:

```toml
[[processor]]
name = "youtube"
pattern = "youtube\\.com|youtu\\.be"
command = "yt-dlp"
args = ["-o", "%(title)s.%(ext)s", "{url}"]
target_dir = "~/Videos"
isolate = true  # default: run in temp dir, move files on success
```

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `name` | yes | - | Processor name (for logging) |
| `pattern` | yes | - | Regex to match URLs |
| `command` | yes | - | Command to execute |
| `args` | yes | - | Arguments (`{url}` replaced with job URL) |
| `target_dir` | no | `~/Videos` | Final destination for files |
| `isolate` | no | `true` | Run in temp dir, move on success |

URLs are matched by regex. First matching processor handles the job.

## Architecture

Hexagonal architecture with clear separation:

```
cmd/catcher/          # Entry point, wiring
internal/
  domain/             # Job entity, ports (interfaces), service
  adapter/
    http/             # HTTP adapter (driving)
    sqlite/           # SQLite adapter (driven)
    processor/        # URL processors (driven)
  worker/             # Background job processor
  config/             # Configuration
```

## Features

- **Crash recovery** - Stale processing jobs reset to pending on startup
- **Atomic downloads** - Downloads to temp dir, moves to final on success
- **Retry logic** - Failed jobs retry up to max-retries
- **Graceful shutdown** - Waits for in-flight requests

## Logging

Catcher logs key events to stdout:

```
loading config from /home/user/.config/catcher/config.toml
found 2 processor(s) in config
registered processor: youtube (pattern: youtube\.com, target: ~/Videos)
job 1: processing with youtube -> /home/user/Videos
job 1: running isolated in /tmp/catcher-job-1-abc123
job 1: found 2 file(s): [video.mp4 thumbnail.jpg]
job 1: moved 2 file(s) to /home/user/Videos
job 1: completed with youtube for https://...
```

## Requirements

- Go 1.21+
- Commands referenced in processor configs (e.g., `yt-dlp`, `gallery-dl`)

## Deployment (macOS LaunchDaemon)

Run catcher as a system service that starts at boot.

### Install

```bash
make build
sudo ./deploy/install.sh
```

The installer prompts for username (defaults to current user). Edit `/etc/catcher/config.toml` to configure processors.

### Paths

| Path | Purpose |
|------|---------|
| `/Library/LaunchDaemons/com.cwygoda.catcher.plist` | Service definition |
| `/usr/local/bin/catcher` | Binary |
| `/etc/catcher/config.toml` | Config |
| `/var/lib/catcher/jobs.db` | Database |
| `/var/log/catcher/catcher.log` | Stdout |
| `/var/log/catcher/catcher.err` | Stderr |

### Management

```bash
# Status
sudo launchctl print system/com.cwygoda.catcher

# Restart
sudo launchctl kickstart -k system/com.cwygoda.catcher

# Logs
tail -f /var/log/catcher/catcher.log

# Stop (without uninstall)
sudo launchctl bootout system/com.cwygoda.catcher
```

### Uninstall

```bash
sudo ./deploy/uninstall.sh          # keeps config/data
sudo ./deploy/uninstall.sh --purge  # removes everything
```

### Config Notes

Daemon context doesn't expand `~`. Use absolute paths:

```toml
command = "/opt/homebrew/bin/yt-dlp"
target_dir = "/Users/cwygoda/Videos"
```

## License

MIT
