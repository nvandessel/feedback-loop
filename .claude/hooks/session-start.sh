#!/bin/bash
# Inject behaviors at session start
# This hook runs when a Claude Code session begins

FLOOP_CMD="${CLAUDE_PROJECT_DIR}/floop"

# Check if floop binary exists and is executable
if [ -x "$FLOOP_CMD" ]; then
    # Generate prompt with behaviors, budget 2000 tokens
    $FLOOP_CMD prompt --format markdown --token-budget 2000 2>/dev/null
fi

exit 0
