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
| `--video-dir` | `CATCHER_VIDEO_DIR` | `~/Videos` | Download directory |

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

Currently supported:
- **YouTube** - Downloads via `yt-dlp` (must be installed)

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

## Requirements

- Go 1.21+
- `yt-dlp` (for YouTube downloads)

## License

MIT
