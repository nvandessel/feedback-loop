# floop

[![CI](https://github.com/nvandessel/feedback-loop/actions/workflows/ci.yml/badge.svg)](https://github.com/nvandessel/feedback-loop/actions/workflows/ci.yml)
[![Go 1.25+](https://img.shields.io/badge/go-1.25%2B-blue.svg)](https://go.dev/)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

**Persistent memory for AI coding agents.**

floop captures corrections you make to AI agents, extracts reusable behaviors, and activates them in the right context — so your agents learn from mistakes and stay consistent across sessions. It uses spreading activation (inspired by how the brain retrieves memories) to surface only the behaviors relevant to what you're working on.

## Features

- **Learns from corrections** — Tell the agent what it did wrong and what to do instead; floop turns that into a durable behavior
- **Context-aware activation** — Behaviors fire based on file type, task, and semantic relevance — not a static prompt dump
- **Spreading activation** — Graph-based memory retrieval inspired by cognitive science (Collins & Loftus, ACT-R)
- **Token-optimized** — Budget-aware assembly keeps injected context within limits
- **MCP server** — Works with any AI tool that supports the Model Context Protocol
- **CLI-first** — Every operation available as a command with `--json` output for agent consumption

## Quick Start

```bash
# Install
go install github.com/nvandessel/feedback-loop/cmd/floop@latest

# Initialize in your project
cd your-project
floop init

# Teach it something
floop learn --wrong "Used fmt.Println for errors" --right "Use log.Fatal or return error"

# See what it learned
floop list

# See what's active for your current context
floop active
```

### Integrate with your AI tool

Add floop as an MCP server so your AI tool loads behaviors automatically.

**Claude Code** (`~/.claude/settings.json`):
```json
{
  "mcpServers": {
    "floop": {
      "command": "floop",
      "args": ["mcp-server"]
    }
  }
}
```

See [docs/integrations/](docs/integrations/) for setup guides for Cursor, Windsurf, Copilot, and more.

## How It Works

When you correct your AI agent, floop captures the correction and extracts a **behavior** — a reusable rule with context conditions. Behaviors are stored as nodes in a graph, connected by typed edges (similar-to, learned-from, requires, conflicts). When you start a session, floop builds a context snapshot (file types, task, project) and uses **spreading activation** to propagate energy through the graph from seed nodes, retrieving only the behaviors most relevant to your current work. This mirrors how the brain activates related memories through associative networks.

## Documentation

- [Integration guides](docs/integrations/) — Setup for Claude Code, Cursor, Windsurf, and others
- [Technical specification](docs/SPEC.md) — Full system design
- [Research & theory](docs/SCIENCE.md) — The cognitive science behind spreading activation
- [Origin story](docs/LORE.md) — How floop came to be
- [Contributing](CONTRIBUTING.md) — How to contribute
- [Changelog](CHANGELOG.md) — Release history

## Project Status

Alpha — actively developed. API may change between minor versions.

## License

[Apache License 2.0](LICENSE)
