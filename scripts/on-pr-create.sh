#!/bin/bash
# cmdr: PostToolUse hook for resolving refactor tasks when a PR is created.
# Task ID is looked up from ~/.cmdr/refactors/ directory, keyed by git branch name.
# Hook data arrives via JSON on stdin.
# Installed to ~/.cmdr/hooks/on-pr-create.sh by `make install`.

# Read hook input from stdin
INPUT=$(cat)

# Check if this is a gh pr create command
echo "$INPUT" | grep -q "gh pr create" || exit 0

# Get current branch name to look up task ID
BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)
[ -z "$BRANCH" ] && exit 0

TASK_FILE="$HOME/.cmdr/refactors/$BRANCH"
[ -f "$TASK_FILE" ] || exit 0
CMDR_TASK_ID=$(cat "$TASK_FILE")
[ -z "$CMDR_TASK_ID" ] && exit 0

# Extract PR URL from the hook response
PR_URL=$(echo "$INPUT" | grep -oE "https://github.com/[^ \"]+/pull/[0-9]+")
[ -z "$PR_URL" ] && exit 0

# Notify cmdr daemon
curl -s -X POST http://127.0.0.1:7369/api/claude/tasks/resolve \
  -H 'Content-Type: application/json' \
  -d "{\"id\": $CMDR_TASK_ID, \"prUrl\": \"$PR_URL\"}" > /dev/null 2>&1

# Clean up
rm -f "$TASK_FILE"
