---
description: "Run quality gate audit on features in review status"
---

Run quality gate. Use the bridge-auditor subagent to audit all features currently in "review" or "testing" status. The auditor will use the bridge-gate-audit skill and produce docs/gates-evals/gate-report.md.

After the subagent completes, present its HUMAN: block to the user verbatim. If the subagent omitted a HUMAN: block, compose one yourself with verification steps and next actions. Never summarize subagent results without ending your response with a HUMAN: block.
