#!/bin/bash
# Detect corrections in user prompts and auto-capture
# This hook runs on UserPromptSubmit events

INPUT=$(cat)
PROMPT=$(echo "$INPUT" | jq -r '.prompt // empty')

# Skip if no prompt
[ -z "$PROMPT" ] && exit 0

FLOOP_CMD="${CLAUDE_PROJECT_DIR}/floop"

# Check if floop binary exists and is executable
[ -x "$FLOOP_CMD" ] || exit 0

# Use floop's detection (calls MightBeCorrection + LLM extraction)
# Run in background with timeout to avoid blocking the prompt
timeout 5s $FLOOP_CMD detect-correction --prompt "$PROMPT" --json 2>/dev/null &

exit 0
