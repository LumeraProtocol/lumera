# BRIDGE v2.1 — lumera

## Methodology

BRIDGE = Brainstorm → Requirements → Implementation Design → Develop → Gate → Evaluate

## Canonical Sources (priority)

1. docs/context.json — as-built truth
2. docs/requirements.json — intent (bridge.v2 schema)
3. docs/contracts/* — schemas/ADRs
4. Codebase — ultimate reality; update context if stale

## Hard Constraints

- Respect scope.in_scope / out_of_scope / non_goals. No scope creep without user instruction.
- Work in thin vertical slices. Prefer PR-sized diffs.
- Every ATxx requires executable evidence before claiming "done".
- Feature status flow: planned → in-progress → review → done | blocked.
- No full-repo scans by default. Targeted inspection only.
- Use stable IDs: Fxx, ATxx, Sxx, UFxx, Rxx.
- Unknowns → execution.open_questions. Do not invent.
- No secrets in code. No sensitive data in production logs. OWASP Top 10 awareness.
- **Every response that presents work output MUST end with a HUMAN: block.** After receiving subagent output, relay the subagent's HUMAN: block verbatim — or compose one if the subagent omitted it. Never summarize subagent results without a HUMAN: block.

## Discrepancy Protocol

- Code ≠ context.json → update context.json.
- Code ≠ requirements.json → record discrepancy in context.json, propose fix, do NOT silently rescope.

## Human Handoff Protocol

The human operator drives BRIDGE. Every significant output MUST end with a `HUMAN:` block:

```
HUMAN:
1. [Concrete verification step — what to run, what to check]
2. [Decision required, if any — with options]
3. [What to feed back next]
```

Required at: slice completion, gate results, open questions, blockers, session end.
Never declare a slice "done" without telling the human exactly how to verify it.

## Delegation Model

Use subagents for isolated, focused work. The main session acts as orchestrator.

- **bridge-architect** — design/contracts for current slice. Read-only except docs/contracts/ and docs/decisions.md.
- **bridge-coder** — implement current slice scope. Small testable increments. Tests satisfy ATxx. No unrelated refactors.
- **bridge-debugger** — reproduce first, fix root cause, add regression tests. Report: commands → results → files changed.
- **bridge-auditor** — never fixes code. Verifies ATxx evidence, checks scope, runs quality gates. Produces docs/gates-evals/gate-report.md.
- **bridge-evaluator** — only after gate passes. Generates test scenarios from user perspective. Maps to user_flows and acceptance_tests.

Pass only relevant context when delegating: relevant JSON slices + file paths, not the whole repo.

**After receiving subagent output:** Always present the subagent's HUMAN: block to the user (or compose one if the subagent omitted it). Never relay subagent results without a HUMAN: block.

## Post-Delivery Feedback Loop

After presenting slice results and the HUMAN: block, WAIT for the user's response.
Classify it before taking any action:

**ISSUES REPORTED** (default if ambiguous):
User describes bugs, missing behavior, or requests changes to CURRENT slice deliverables.
Indicators: "fix", "bug", "issue", "wrong", "missing", "doesn't work", "investigate", "implement", "however", "but", numbered problem lists, behavioral descriptions.
→ Acknowledge the reported issues explicitly. Create numbered fix tasks. Re-enter implementation for the CURRENT slice (same Sxx) by delegating to bridge-coder/bridge-debugger as needed. Do NOT delegate to bridge-auditor or bridge-evaluator. Do NOT ask about next slice. After fixes, present results with a new HUMAN: block and re-enter this loop.

**APPROVED**: Explicit approval only — "done", "approved", "PASSED", "looks good", "move on", "next slice", "continue".
→ Proceed to bridge-auditor, then bridge-evaluator, then next slice selection.

**STOP**: Explicit stop/pause request. → Run session wrap-up.

CRITICAL: Never assume approval. If the response contains ANY issue descriptions, treat as ISSUES REPORTED even if it also contains partial approval.

## Available Skills

These skills are auto-discovered. Key ones:

- **bridge-slice-plan** — plan and execute thin vertical slices
- **bridge-gate-audit** — run quality gate checks, produce gate report
- **bridge-eval-generate** — generate evaluation scenarios, E2E tests, feedback template
- **bridge-session-management** — session re-entry briefs and wrap-up procedures
- **bridge-context-sync** — create or update context.json from code reality
- **bridge-feedback-process** — triage evaluation feedback, decide iterate vs launch
