---
name: Bridge Context Sync
description: Create or update context.json to reflect current code reality. Use when context.json is missing, stale, or after significant code changes.
user-invocable: false
---

# Context Synchronization

## If Creating (context.json missing)

1. Load docs/requirements.json
2. Run `git status` and `git log --oneline -20`
3. Targeted code inspection for modules relevant to first slice only
4. Create docs/context.json:

```json
{
  "schema_version": "context.v1",
  "updated": "[timestamp]",
  "project": { "name": "lumera" },
  "feature_status": [
    { "feature_id": "F01", "status": "not_started|in_progress|done|blocked", "notes": "", "evidence": [] }
  ],
  "handoff": { "stopped_at": "", "next_immediate": "", "watch_out": "" },
  "next_slice": { "slice_id": "", "goal": "", "features": [], "acceptance_tests": [] },
  "commands_to_run": { "test": "", "lint": "", "typecheck": "", "dev": "" },
  "recent_decisions": [],
  "blockers": [],
  "discrepancies": [],
  "gate_history": [],
  "eval_history": []
}
```

5. Output summary of findings. Stop.

## If Updating (context.json exists)

1. Load docs/context.json and docs/requirements.json
2. Run `git status` + `git log --oneline -10`
3. Validate feature_status for recently touched areas and next_slice
4. Update context.json to match code reality
5. Output sync report with discrepancies found
