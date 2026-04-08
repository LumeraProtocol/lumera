#!/bin/bash

# Read the tool input JSON from stdin
INPUT=$(cat)
COMMAND=$(echo "$INPUT" | jq -r '.command // ""')

# Auto-approve: cd + read-only git commands
if echo "$COMMAND" | grep -qE '^cd .+ && git (status|log|diff|branch|show|ls-files)'; then
  echo '{"decision": "approve"}'
  exit 0
fi

# Everything else: let normal permission flow handle it
exit 0
