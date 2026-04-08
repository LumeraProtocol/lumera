---
name: Bridge Feedback Process
description: Triage evaluation feedback and determine iterate vs launch. Use after the human has completed manual testing and filled the feedback form.
---

# Feedback Processing

## Step 1: Parse
- Extract issues with severity (high/medium/low)
- Identify patterns and themes
- Note positive feedback

## Step 2: Triage
- **High severity** → blocking, must fix before launch
- **Medium severity** → should fix, can defer to v1.1
- **Low severity** → add to extended features in requirements.json

## Step 3: Update Context
Update eval_history entry in docs/context.json:
```json
{ "feedback_received": "[today]", "issues_found": { "high": 0, "medium": 0, "low": 0 }, "action": "iterate|launch" }
```

## Step 4: Decision

If high severity:
```
ITERATION REQUIRED
Blocking issues:
1. [Task - Fxx]
Returning to code/debug. Re-run $bridge-gate after fixes.
```
Features → "in-progress"

If medium/low only:
```
LAUNCH CANDIDATE ✓
Optional improvements (non-blocking):
1. [Suggestion]
Recommended: Launch. Medium issues → v1.1.
```
Features → "done"

## Step 5: Human Handoff (required)

```
HUMAN:
1. [If ITERATION] Review blocking issues — do they match your testing experience?
   Feed fix instructions back, then re-run $bridge-gate after fixes
2. [If LAUNCH CANDIDATE] Final go/no-go is yours:
   - Did the app feel right during manual testing?
   - Are you comfortable deferring medium-severity issues to v1.1?
   - Any concerns not captured in the feedback?
3. Medium issues logged for v1.1 — create tracking issues if needed
```
