# DirMap

A real-time directory structure monitor that uses LLM to generate concise descriptions for each folder. Designed as a context tool for AI agents to quickly understand project layouts.

## How It Works

```
File system events → Debounce → LLM Analysis → Markdown output
```

1. Monitors specified directories for changes (create/delete/rename)
2. Sends directory metadata to Claude-compatible API
3. Generates one-line descriptions for each folder
4. Outputs a Markdown table to the configured directory

## Quick Start

### Run locally

```bash
# Set API key
export ANTHROPIC_API_KEY=sk-xxx

# Build and run
make build
./bin/system-agent-rag -config config.yaml
```

### Run with Docker

```bash
# Build image
make docker-build

# Start service (runs in background, auto-restarts)
make docker-up

# View logs
make docker-logs

# Stop service
make docker-down
```

> **Note for macOS users**: Docker on macOS doesn't reliably propagate file system events (inotify). Enable polling mode in `config-docker.yaml`:
> ```yaml
> polling:
>   enabled: true
>   interval: 10s
> ```

## Configuration

Copy `config.yaml.example` to `config.yaml` and edit:

```yaml
watch_paths:
  - /path/to/watch

output_dir: /path/to/output

llm:
  base_url: ""                    # custom API endpoint (optional)
  api_key: ""                     # or use ANTHROPIC_API_KEY env var
  model: "claude-haiku-4-5"
  max_tokens: 4096
  temperature: 0.3
  max_batch_size: 10

debounce:
  interval: 3s                    # batch events within this window
  max_wait: 30s                   # force flush after this duration

initial_scan: true                # scan all dirs on startup
ignore_patterns:
  - ".git"
  - "node_modules"
```

## Output Example

```markdown
# Directory Descriptions: /project
Generated: 2026-05-13 12:00:00

| Path | Modified | Description |
|------|----------|-------------|
| ./ | 2026-05-12 | Root workspace for Go projects |
| src/ | 2026-05-10 | Main application source code |
| internal/ | 2026-05-11 | Private packages not exposed to external consumers |
```

## Architecture

```
main.go → config.Load() → agent.New() → agent.Run(ctx)

Data flow:
  fsnotify events → watcher → debouncer (3s/30s batching)
    → scanner reads directory metadata
    → summarizer calls LLM API (batched, streaming, with retry)
    → writer outputs atomic .md files

In-memory cache: map[watchPath]map[dirPath]FileInfo
  - Incremental: only changed directories get re-summarized
  - Stale descriptions preserved on LLM failure
```

## License

MIT
