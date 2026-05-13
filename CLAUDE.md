# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Does

system-agent-rag is a Go CLI tool that monitors directories for file system changes in real-time, calls the Claude API to generate concise one-line descriptions for each file/folder, and writes those descriptions as Markdown tables to a configurable output directory.

## Build & Run

```bash
make build          # compile to bin/system-agent-rag
make run            # build + run with config.yaml
make test           # go test ./...
make tidy           # go mod tidy
make clean          # remove bin/
```

Requires `ANTHROPIC_API_KEY` environment variable (or set `llm.api_key` in config).

## Architecture & Data Flow

```
main.go → config.Load() → agent.New() → agent.Run(ctx)

Data flows through the pipeline:
  fsnotify events → watcher → debouncer (3s/30s batching)
    → scanner reads file metadata + content preview
    → summarizer calls Claude API (batched, with retry)
    → writer outputs atomic .md files (write tmp, then rename)

In-memory cache: map[watchPath]map[filePath]FileInfo
  - Incremental: only changed files get re-summarized
  - Stale descriptions preserved on LLM failure
```

**Key coupling**: `internal/agent/agent.go` is the orchestrator that wires everything together. All other packages are stateless modules except the in-memory cache held by the agent.

## Important Implementation Details

- **fsnotify does not auto-watch subdirectories** — `watcher.go` must `Add()` each one recursively, and dynamically add new directories on `Create` events.
- **Debouncer** uses `time.AfterFunc` with reset on each event. `max_wait` prevents starvation under continuous writes.
- **Summarizer** parses `PATH|DESCRIPTION` pipe-delimited output from Claude. On parse failure, old descriptions are kept.
- **Writer** uses atomic writes (`.tmp` + `os.Rename`) to prevent partial/corrupt files on crash.
- **Scanner** skips hidden files (dot prefix) and patterns from `ignore_patterns`.

## SDK Usage

Claude API: `github.com/anthropics/anthropic-sdk-go` v1.41.0. Key types:
- `anthropic.MessageNewParams` with `System []TextBlockParam`, `Messages`, `Temperature param.Opt[float64]`
- Response: `message.Content[0].Text` (ContentBlockUnion with Type == "text")
- Client: `anthropic.NewClient()` reads `ANTHROPIC_API_KEY` from env automatically

## Config

See `config.yaml.example` for the full schema. Required fields: `watch_paths`, `output_dir`. All LLM and debounce settings have defaults.
