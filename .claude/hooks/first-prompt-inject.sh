#!/bin/bash
# Fallback: inject behaviors on first prompt if SessionStart didn't fire
# This ensures new conversations also get behavior injection

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | jq -r '.session_id // "unknown"')
MARKER="/tmp/floop-session-$SESSION_ID"

# Only run once per session
if [ -f "$MARKER" ]; then
    exit 0
fi
touch "$MARKER"

FLOOP_CMD="${CLAUDE_PROJECT_DIR}/floop"

# Check if floop binary exists and is executable
if [ -x "$FLOOP_CMD" ]; then
    # Generate prompt with behaviors, budget 2000 tokens
    "$FLOOP_CMD" prompt --format markdown --token-budget 2000 2>/dev/null
fi

exit 0
