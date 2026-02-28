# OpenAI Codex Integration

## Overview

Codex can use floop through MCP, but it does not currently expose Claude-style
lifecycle hooks (such as `SessionStart`, `PreToolUse`, `UserPromptSubmit`).

That means Codex integrations need two layers:

1. MCP for bidirectional floop tool access
2. Instruction orchestration (`AGENTS.md` + optional `SKILL.md`) to emulate hook cadence

This guide shows both.

## Quick Install (Codex)

Run from your project root:

```bash
# 1) Ensure floop is installed
which floop

# 2) Configure MCP in Codex (~/.codex/config.toml)
cat >> ~/.codex/config.toml <<'EOF'
[mcp_servers.floop]
command = "floop"
args = ["mcp-server"]
cwd = "/path/to/your/project"
EOF

# 3) Install AGENTS baseline rules
cp docs/integrations/templates/codex-agents.md.template AGENTS.md

# 4) Install floop skill via Codex skill discovery path
mkdir -p ~/.agents/skills/floop
ln -sfn "$(pwd)/docs/integrations/templates/floop.SKILL.md.template" ~/.agents/skills/floop/SKILL.md
```

Then restart Codex (fully quit and relaunch).

If you already have an `AGENTS.md`, merge the template content instead of
overwriting the file.

## 1) Configure MCP

Add floop as an MCP server in your Codex config.

Example `~/.codex/config.toml`:

```toml
[mcp_servers.floop]
command = "floop"
args = ["mcp-server"]
# Optional but recommended for project-local behavior stores
cwd = "/path/to/your/project"
```

If you prefer per-project setup, use Codex's local config equivalent and keep
`cwd` set to your project root.

## 2) Verify MCP Is Working

Use this checklist in a Codex session:

1. Confirm Codex shows `floop_*` tools in its MCP tool list.
2. Ask Codex to call `floop_active` for your current file/task.
3. Confirm a structured response is returned (with `active` behaviors and `count`).
4. Ask Codex to call `floop_list` and verify the store is reachable.

If tools are missing:

- Confirm `floop` is in `PATH` (`which floop`).
- Confirm `floop mcp-server` runs manually in your project.
- Confirm Codex loaded the expected config file and restart Codex.

## 3) Add Codex Orchestration Rules (No Hooks)

Because Codex has no lifecycle hooks, add explicit runtime rules to your
project `AGENTS.md`.

Use this template:

- [codex-agents.md.template](./templates/codex-agents.md.template)

These rules force the cadence hooks normally provide:

- At task start -> call `floop_active`
- On context change (file/task/tooling mode) -> refresh `floop_active`
- On correction -> call `floop_learn` immediately
- After following or overriding behavior -> call `floop_feedback`

## 4) Add Optional Codex Skill

For stronger consistency, add a dedicated Codex skill:

- [floop.SKILL.md.template](./templates/floop.SKILL.md.template)

Recommended install location:

`~/.agents/skills/floop/SKILL.md`

Use project-local skills if your team prefers repository-scoped behavior.

## 5) CLI Fallback (If MCP Is Unavailable)

If MCP is not available in your Codex environment, keep read/write behavior via
CLI commands:

```bash
# Load context
floop active --file <path> --task <task> --json

# Learn correction
floop learn --right "what to do instead" --wrong "what happened" --file <path>

# Review behaviors
floop list --json
```

Static prompt fallback:

```bash
floop prompt > AGENTS.md
```

Note: static prompt mode is read-only for the agent. New corrections must be
captured manually via `floop learn`.

## 6) Dogfood Verification Flow

Run this sequence to validate end-to-end behavior:

1. Start a new Codex task and trigger `floop_active`.
2. Deliberately correct Codex and ensure it calls `floop_learn`.
3. Start a second related task and verify the new behavior appears in `floop_active`.
4. Confirm `floop_feedback` gets emitted when behavior is followed/overridden.

If step 2 or 4 is missed, tighten your `AGENTS.md` trigger language or install
the Codex skill template.

## Skill Maintenance

Because the recommended install uses a symlink, updates are immediate when the
template file changes in your repo.

### Verify Skill Install

```bash
ls -la ~/.agents/skills/floop/SKILL.md
```

You should see a symlink pointing to:
`docs/integrations/templates/floop.SKILL.md.template`.

### Update

```bash
cd /path/to/your/project
git pull
```

### Uninstall

```bash
rm -f ~/.agents/skills/floop/SKILL.md
```
