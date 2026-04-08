---
name: Bridge Slice Plan
description: Plan and execute a thin vertical slice from requirements. Use when starting a new slice, planning implementation tasks, or coordinating the architect→coder→debugger delegation flow.
---

# Slice Planning & Execution

## Planning

1. Select next slice from execution.recommended_slices (or propose smallest viable)
2. Output slice plan:

```
SLICE: [Sxx] - [goal]
FEATURES: [Fxx list]
EXIT CRITERIA: [ATxx list]

TASKS:
1. [task_id] architect: [design/contract work]
   → Inputs: [files/JSON sections]
   → Output: [deliverable]

2. [task_id] code: [implementation]
   → Inputs: [architect output + relevant files]
   → Tests: [ATxx → test description]

3. [task_id] debug: [verification]
   → Run: [commands]
   → Expect: [results]

DEPENDENCIES: [task order]
```

## Execution Loop

### Specialist Propagation

Before switching modes, check if specialists were loaded for this slice (from $bridge-start or the SPECIALISTS LOADED block). For each loaded specialist:
- Read the specialist file from `.agents/specialists/<id>.md`
- Apply the guidance sections matching your current mode/role
- Only use guidance for roles listed in the specialist's `applies_to` field

### Mode Delegation

1. Switch to architect mode (if slice needs design) → relevant JSON + file paths; apply specialist guidance for architect role
2. Switch to code mode → implement with tests satisfying ATxx; apply specialist guidance for coder role
3. Switch to debug mode → run tests, fix failures; apply specialist guidance for debugger role (if any)
4. Verify: ATxx → evidence mapping
5. Update docs/context.json: feature_status, handoff, next_slice

## Completion & Feedback Loop

- Slice is PRESENTED (not "done") when all ATxx have passing evidence
- Output the HUMAN: block above
- Then STOP and WAIT for the user's response

### Classify User Response

**ISSUES REPORTED** (default if ambiguous):
User describes bugs, missing behavior, incorrect behavior, or requests changes.
Indicators: "fix", "bug", "issue", "wrong", "missing", "doesn't work", "investigate", "implement", "however", "but", numbered problem lists.
→ Create numbered fix tasks from feedback. Re-enter the Execution Loop above for the SAME slice (same Sxx). Do NOT update feature status to "review" or "done". Do NOT trigger gate or evaluate. After fixes, output a new HUMAN: block and re-enter this loop.

**APPROVED**:
Explicit approval only: "done", "approved", "PASSED", "looks good", "move on", "next slice", "continue".
→ Update feature status to "review" (triggers gate) or "done" (if trivial). Update docs/context.json: feature_status, handoff, next_slice.

**STOP**:
Explicit stop/pause request.
→ Hand off to session wrap-up.

CRITICAL: Never assume approval. Never update feature status until explicitly approved. If the response contains ANY issue descriptions, treat as ISSUES REPORTED regardless of other content.

## Human Handoff (required)

After completing each slice, output:

```
HUMAN:
1. Verify: [exact test/lint/build commands to run yourself]
2. Smoke test: [specific manual test — what to run, what to check, what "working" looks like]
3. Read: [2-3 key files to inspect and what to look for]
4. Decide: [any open questions hit during this slice]
5. Next: [exact prompt to feed back for next slice, or $bridge-gate if ready]
```

Consult docs/human-playbook.md for project-specific verification per slice.
