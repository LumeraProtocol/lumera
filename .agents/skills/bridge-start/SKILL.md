---
name: Bridge Start
description: Start BRIDGE implementation - plan and execute slices from requirements. Invoke with $bridge-start in your prompt.
---

Load docs/requirements.json and docs/context.json.

## Specialist Loading

Before planning the slice, if the user explicitly says `no specialists` for this start invocation, skip this section. Otherwise check `execution.specialists_recommended` in requirements.json. If it exists and is non-empty:

1. Determine the active slice features:
   - Prefer `context.json.next_slice.features` when present
   - Else if `context.json.next_slice.slice_id` exists, resolve that slice ID in `requirements.json.execution.recommended_slices`
   - Else if `context.json.current_slice` is a slice ID, resolve that ID in `requirements.json.execution.recommended_slices`
2. For each matching specialist, read the specialist file from `.agents/specialists/<id>.md`
3. Output a SPECIALISTS LOADED block:

```
SPECIALISTS LOADED for [Sxx]:
- [specialist-name] (confidence: high) — [rationale]. Applies to: [roles]
- [specialist-name] (confidence: medium) — [rationale]. Applies to: [roles]

To skip specialist guidance: tell me "no specialists" or edit execution.specialists_recommended in requirements.json.
```

4. When working on the slice, read and apply the guidance sections matching your current role from each loaded specialist file

If `execution.specialists_recommended` is empty or absent, skip this step silently.

## Slice Execution

Plan and execute the next slice using `.agents/procedures/bridge-slice-plan.md`. Switch to architect mode (if design needed), code mode (implementation), debug mode (if tests fail).

Start with the first recommended slice or the slice indicated in context.json next_slice.

## HUMAN: Block (required)

After completing the slice, you MUST end your response with a HUMAN: block as specified in the slice-plan procedure. Never present slice results without a HUMAN: block.

The user will provide arguments inline with the skill invocation.
