# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Does

DirMap monitors directories in real-time, calls a Claude-compatible LLM API to generate concise one-line descriptions for each folder, and writes those descriptions as Markdown tables to a configurable output directory. Designed as a context tool for AI agents to quickly understand project layouts.

**Only directories are monitored** — files are intentionally ignored. The LLM explores files on its own.

## GitHub

https://github.com/CengSin/DirMap

## Build & Run

```bash
make build          # compile to bin/system-agent-rag
make run            # build + run with config.yaml
make test           # go test ./...
make tidy           # go mod tidy
make clean          # remove bin/

make docker-build   # build Docker image (29MB, alpine-based)
make docker-up      # start container (background, auto-restart)
make docker-logs    # tail container logs
make docker-down    # stop container
```

Requires `ANTHROPIC_API_KEY` env var or `llm.api_key` in config.

## Architecture

```
main.go → config.Load() → agent.New() → agent.Run(ctx)

Data flow:
  fsnotify events (or polling) → watcher → debouncer (3s/30s batching)
    → scanner reads directory metadata only
    → summarizer calls LLM API (streaming, batched, with retry)
    → cache persists to disk (*.cache.json in output_dir)
    → writer outputs atomic .md files (.tmp + rename)

Shutdown: SIGINT/SIGTERM → ctx.Cancel → debouncer.Stop → watcher.Close
```

**`internal/agent/agent.go`** is the orchestrator. All other packages are stateless modules.

## Package Map

| Package | Responsibility |
|---------|---------------|
| `agent` | Orchestrator: wires everything, manages in-memory cache, shutdown sequence |
| `cache` | Persists directory descriptions to JSON files, diffs scanned vs cached on startup |
| `config` | YAML config loading, validation, defaults |
| `model` | Shared `FileInfo` struct (Path, Name, IsDir, ModTime, Description) |
| `scanner` | Walks directories on startup, skips hidden files and ignore_patterns |
| `summarizer` | Claude API client via streaming, prompt templates, retry with backoff |
| `watcher` | fsnotify watcher (recursive dir watching) OR polling watcher (Docker on macOS) |
| `writer` | Writes Markdown table output, atomic writes |

## Key Design Decisions

- **Only directories** — scanner and watcher both filter to dirs only. Files are not sent to LLM.
- **Polling mode** — `polling.enabled: true` in config. Uses `filepath.WalkDir` on an interval instead of fsnotify. Required for Docker on macOS where inotify doesn't work reliably across bind mounts.
- **Persistent cache** — `cache.Store` saves `*.cache.json` in output_dir. On startup, `cache.Diff()` compares paths + ModTime. Unchanged dirs reuse saved descriptions (zero LLM cost). Only new/changed dirs call the API.
- **Streaming API** — `client.Messages.NewStreaming()` is required. Non-streaming calls fail with "streaming is required for operations that may take longer than 10 minutes" when batch size is large.
- **Context-aware retry** — `callWithRetry` checks `context.Canceled`/`context.DeadlineExceeded` immediately and returns without retrying.
- **Debouncer stop channel** — `flushLocked()` uses `select` with a `stop` channel to avoid goroutine leak on shutdown.
- **Remove events** — watcher sends Remove events directly without `os.Stat` (deleted paths always fail stat).

## SDK Usage

`github.com/anthropics/anthropic-sdk-go` v1.41.0:

```go
// Client with custom base URL and API key
opts := []option.RequestOption{}
if cfg.LLM.BaseURL != "" {
    opts = append(opts, option.WithBaseURL(cfg.LLM.BaseURL))
}
if cfg.LLM.APIKey != "" {
    opts = append(opts, option.WithAPIKey(cfg.LLM.APIKey))
}
client := anthropic.NewClient(opts...)

// Streaming call
stream := client.Messages.NewStreaming(ctx, params)
for stream.Next() {
    event := stream.Current()
    if event.Type == "content_block_delta" && event.Delta.Type == "text_delta" {
        text += event.Delta.Text
    }
}
```

## Config

See `config.yaml.example` for full schema. Key fields:
- `watch_paths` — directories to monitor (absolute paths)
- `output_dir` — where to write .md files and .cache.json files
- `llm.base_url` — custom API endpoint (for proxied/non-default APIs)
- `llm.api_key` — or use `ANTHROPIC_API_KEY` env var
- `polling.enabled` — set `true` for Docker on macOS
- `debounce.interval` / `debounce.max_wait` — event batching

**Config-docker.yaml** has `api_key` in plaintext — it's in `.gitignore`, never committed.

## Development Tips

- The project name on disk is `system-agent-rag`, but the GitHub repo and public name is **DirMap**.
- `config-docker.yaml` is the local Docker config (not committed). `config.yaml.example` is the template.
- `docker-compose.yml` mounts host directories as read-only into the container. The output dir is read-write.
- Cache files (`*.cache.json`) are written to the same `output_dir` as the `.md` description files.
- When modifying the prompt in `summarizer/prompt.go`, the LLM must return `PATH|DESCRIPTION` pipe-delimited format. The parser in `summarizer.go:parseDescriptions()` relies on this exact format.
- The `PollingWatcher` and `Watcher` (fsnotify) both produce the same `chan string` event channel, consumed by the same `Debouncer`. The agent selects between them in `agent.New()`.
- fsnotify on macOS Docker bind mounts is unreliable — always use polling mode in Docker.
