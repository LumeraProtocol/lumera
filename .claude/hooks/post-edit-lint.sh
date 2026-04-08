#!/bin/bash
# Post-edit lint hook for BRIDGE projects.
# Reads the lint command from docs/context.json if available,
# otherwise falls back to common project conventions.
# Silently succeeds (exit 0) in non-BRIDGE projects.

# Never fail — hooks must not block edits
trap 'exit 0' ERR

PROJECT_DIR="${CLAUDE_PROJECT_DIR:-.}"
CONTEXT_FILE="$PROJECT_DIR/docs/context.json"

# Try to read lint command from context.json
if [ -f "$CONTEXT_FILE" ] && command -v jq &>/dev/null; then
  LINT_CMD=$(jq -r '.commands_to_run.lint // empty' "$CONTEXT_FILE" 2>/dev/null)
  if [ -n "$LINT_CMD" ]; then
    eval "$LINT_CMD" 2>&1 || true
    exit 0
  fi
fi

# Fallback: detect project type and run appropriate linter
if [ -f "$PROJECT_DIR/package.json" ]; then
  if grep -q '"lint"' "$PROJECT_DIR/package.json" 2>/dev/null; then
    npm run lint --prefix "$PROJECT_DIR" 2>&1 || true
  fi
elif [ -f "$PROJECT_DIR/Cargo.toml" ]; then
  cargo clippy --manifest-path "$PROJECT_DIR/Cargo.toml" 2>&1 || true
elif [ -f "$PROJECT_DIR/pyproject.toml" ] || [ -f "$PROJECT_DIR/setup.py" ]; then
  if command -v ruff &>/dev/null; then
    ruff check "$PROJECT_DIR" 2>&1 || true
  fi
fi

exit 0
