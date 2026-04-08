---
name: Bridge Resume
description: Resume development in a fresh session - load context and output brief. Invoke with $bridge-resume in your prompt.
---

Fresh session. Load docs/requirements.json and docs/context.json.

Follow `.agents/procedures/bridge-session-management.md` (Fresh Session Re-entry) to output the brief.

## Specialist Decision Before Further Work

Before the final HUMAN block in the re-entry brief:

1. Check `execution.specialists_recommended` in requirements.json. If it is empty or absent, skip this section silently.
2. Determine the active slice features:
   - Prefer `context.json.next_slice.features` when present
   - Else if `context.json.next_slice.slice_id` exists, resolve that slice ID in `requirements.json.execution.recommended_slices`
   - Else if `context.json.current_slice` is a slice ID, resolve that ID in `requirements.json.execution.recommended_slices`
3. Match specialist `applicable_features` against that feature list.
4. For each matching specialist, verify the file exists at `.agents/specialists/<id>.md`.
5. Insert a `SPECIALISTS SUGGESTED` block before the final HUMAN block. Do NOT load those specialists yet.
6. The final HUMAN block must ask the user to reply `load specialists`, `no specialists`, or name a subset before any further work.
7. If the user replies `load specialists` or names a subset, read those files immediately, output a `SPECIALISTS LOADED` block, and use them for all following work in this thread until the user opts out or the slice/context changes.
8. If the next user message gives task work without answering the specialist question, ask whether to load the suggested specialists first. Do not begin task work until they answer.
9. If the user replies `no specialists`, continue without them.

## HUMAN: Block (required)

Your response MUST end with a HUMAN: block. The session-management procedure includes one — include it verbatim. If specialists were suggested, the HUMAN: block must ask the user to confirm specialist loading before any further work. Never omit the HUMAN: block.

Then wait for instructions.
