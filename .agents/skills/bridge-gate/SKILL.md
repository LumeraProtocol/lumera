---
name: Bridge Gate
description: Run quality gate audit on features in review status. Invoke with $bridge-gate in your prompt.
---

Run quality gate. Switch to audit mode to audit all features currently in "review" status. Follow `.agents/procedures/bridge-gate-audit.md` to produce docs/gates-evals/gate-report.md.

Your response MUST end with a HUMAN: block. The gate-audit procedure specifies one — include it verbatim. Never present gate results without a HUMAN: block.
