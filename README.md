# Chronicle

Privacy-first local browsing history capture, search, and recall — designed to work with [fabric](https://github.com/danielmiessler/fabric).

## What is Chronicle?

Chronicle is a local-only observability + recall layer that continuously captures your browsing activity, retains it for a configurable rolling window (default: 30 days), and makes it searchable. Pipe results into fabric patterns for summarization, code extraction, insight mining, and more.

**No cloud. No tracking. Your data stays on your machine.**

## Quick Start

```bash
# Install
go install github.com/runnerr0/chronicle/cmd/chronicle@latest

# Add a page manually
chronicle --add --url "https://example.com/article" --title "Great Article"

# Search your history
chronicle --search -q "vector database" --since 7d

# Pipe into fabric patterns
chronicle --search -q "local llm" --since 14d | fabric chronicle_summarize

# View a specific item
chronicle --open --id CHR-abc123 | fabric extract_code

# Check status
chronicle --status
```

## Features

- **Passive capture** via browser extension (Chrome, Firefox)
- **Rolling retention** with configurable TTL (default: 30 days)
- **Keyword search** over titles, URLs, and page content
- **Semantic search** via local embeddings (Ollama + LanceDB)
- **Fabric integration** — pipe results directly into any fabric pattern
- **Privacy-first** — local-only, incognito excluded, domain denylist

## Architecture

Chronicle is a standalone companion to fabric (not a fork). It consists of:

1. **CLI** (`chronicle`) — Search, retrieve, manage your browsing history
2. **Daemon** — Local HTTP service that receives events from the browser extension
3. **Browser Extension** — Captures URL/title (optionally content) as you browse
4. **Pattern Pack** — Fabric-compatible patterns optimized for browsing context

## Development

```bash
# Build
make build

# Test
make test

# Lint
make lint
```

## License

MIT
