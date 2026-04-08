---
name: Bridge End
description: End development session - update handoff and save state. Invoke with $bridge-end in your prompt.
---

Follow `.agents/procedures/bridge-session-management.md` (Session Wrap-up). Update docs/context.json with current state and handoff, log decisions to docs/decisions.md, and output session summary.

End your response with a HUMAN: block listing what was saved, any uncommitted changes, and what to do next session. Never omit the HUMAN: block.
