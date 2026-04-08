#!/bin/bash
# SessionStart hook for BRIDGE projects.
# Checks whether docs/context.json exists and reports its status
# so Claude can suggest a context sync if needed.
# Silently succeeds (exit 0) in non-BRIDGE projects.

# Never fail — hooks must not block startup
trap 'exit 0' ERR

PROJECT_DIR="${CLAUDE_PROJECT_DIR:-.}"
CONTEXT_FILE="$PROJECT_DIR/docs/context.json"
REQUIREMENTS_FILE="$PROJECT_DIR/docs/requirements.json"

# Not a BRIDGE project — exit silently
if [ ! -f "$CONTEXT_FILE" ] && [ ! -f "$REQUIREMENTS_FILE" ]; then
  exit 0
fi

# Check for context.json
if [ ! -f "$CONTEXT_FILE" ]; then
  echo "BRIDGE: docs/context.json is missing but docs/requirements.json exists."
  echo "Consider running /bridge-context-create to generate it."
  exit 0
fi

# Report context age
if command -v stat &>/dev/null; then
  MODIFIED=$(stat -c %Y "$CONTEXT_FILE" 2>/dev/null || stat -f %m "$CONTEXT_FILE" 2>/dev/null || echo "")
  if [ -n "$MODIFIED" ]; then
    NOW=$(date +%s)
    AGE_HOURS=$(( (NOW - MODIFIED) / 3600 ))
    if [ "$AGE_HOURS" -gt 48 ]; then
      echo "BRIDGE: docs/context.json was last updated ${AGE_HOURS}h ago."
      echo "Consider running /bridge-context-update to sync with current code."
    fi
  fi
fi

# Report next slice if available
if command -v jq &>/dev/null; then
  NEXT_SLICE=$(jq -r '.next_slice.goal // empty' "$CONTEXT_FILE" 2>/dev/null)
  if [ -n "$NEXT_SLICE" ]; then
    SLICE_ID=$(jq -r '.next_slice.slice_id // "?"' "$CONTEXT_FILE" 2>/dev/null)
    echo "BRIDGE: Next slice is $SLICE_ID — $NEXT_SLICE"
  fi
fi

exit 0
