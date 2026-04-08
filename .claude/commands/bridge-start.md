---
description: "Start BRIDGE implementation - plan and execute slices from requirements"
---

Load docs/requirements.json and docs/context.json.

## Specialist Loading

Before planning the slice, if the user explicitly says `no specialists` for this start invocation, skip this section. Otherwise check `execution.specialists_recommended` in requirements.json. If it exists and is non-empty:

1. Determine the active slice features:
   - Prefer `context.json.next_slice.features` when present
   - Else if `context.json.next_slice.slice_id` exists, resolve that slice ID in `requirements.json.execution.recommended_slices`
   - Else if `context.json.current_slice` is a slice ID, resolve that ID in `requirements.json.execution.recommended_slices`
2. For each matching specialist, read the specialist file from `.claude/specialists/<id>.md`
3. Output a SPECIALISTS LOADED block:

```
SPECIALISTS LOADED for [Sxx]:
- [specialist-name] (confidence: high) — [rationale]. Applies to: [roles]
- [specialist-name] (confidence: medium) — [rationale]. Applies to: [roles]

To skip specialist guidance: tell me "no specialists" or edit execution.specialists_recommended in requirements.json.
```

4. When delegating to subagents, pass the relevant specialist file paths to each role listed in the specialist's `applies_to` field:
   ```
   Read the following specialist guidance files before starting:
   - .claude/specialists/<id>.md (applies to: <roles>)
   Apply the "Guidance for <your-role>" section from each file.
   ```

If `execution.specialists_recommended` is empty or absent, skip this step silently.

## Slice Execution

Plan and execute the next slice using the bridge-slice-plan skill. Delegate to subagents: bridge-architect (if design needed), bridge-coder (implementation), bridge-debugger (if tests fail).

Start with the first recommended slice or the slice indicated in context.json next_slice.

## Required Closing (do not omit)

After completing the slice, your response MUST end with a HUMAN: block. If a subagent produced a HUMAN: block, present it to the user verbatim. If no subagent produced one, compose your own using this format:

```
HUMAN:
1. Verify: [exact test/build commands to run]
2. Smoke test: [what to check manually and what "working" looks like]
3. Read: [2-3 key files to inspect]
4. Decide: [any open questions from this slice]
5. Next: [what to feed back — approve, report issues, or /bridge-gate]
```

Never present slice results without a HUMAN: block.

$ARGUMENTS
