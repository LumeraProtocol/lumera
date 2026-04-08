# BRIDGE v2.1 — lumera

## Methodology

BRIDGE = Brainstorm → Requirements → Implementation Design → Develop → Gate → Evaluate

A structured methodology for solo-preneur software development with AI coding agents.

## Canonical Sources (priority)

1. `docs/context.json` — as-built truth
2. `docs/requirements.json` — intent (bridge.v2 schema)
3. `docs/contracts/*` — schemas/ADRs
4. Codebase — ultimate reality; update context if stale

## Hard Constraints

- Respect `scope.in_scope` / `out_of_scope` / `non_goals`. No scope creep without user instruction.
- Work in thin vertical slices. Prefer PR-sized diffs.
- Every ATxx requires executable evidence before claiming "done".
- Feature status flow: `planned → in-progress → review → done | blocked`.
- No full-repo scans by default. Targeted inspection only.
- Use stable IDs: Fxx, ATxx, Sxx, UFxx, Rxx.
- Unknowns → `execution.open_questions`. Do not invent.
- No secrets in code. No sensitive data in production logs. OWASP Top 10 awareness.
- **Every response that presents work output MUST end with a HUMAN: block.** After following a procedure, ensure your response includes the procedure's HUMAN: block. If the procedure omitted one, compose your own with verification steps and next actions.

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

## Role Modes

Since Codex is single-agent, switch between these modes mentally based on the current task. Each mode has different rules:

### Orchestrator (default)
- Operate per-slice (Sxx). Select from `execution.recommended_slices` or propose smallest next.
- Delegate work by switching to appropriate mode below.
- After each slice: update `docs/context.json` (feature_status, handoff, next_slice).
- After each slice: output a HUMAN: block with verification steps.
- Consult `docs/human-playbook.md` for project-specific verification.

**Post-Delivery Feedback Loop:** After presenting slice results and the HUMAN: block, WAIT for the user's response and classify it:
- **ISSUES REPORTED** (default if ambiguous): User describes bugs, missing behavior, or requests changes. Indicators: "fix", "bug", "issue", "wrong", "missing", "investigate", "however", "but", problem lists. → Acknowledge issues, create fix tasks, re-implement CURRENT slice (same Sxx). Do NOT switch to Audit/Evaluate mode. Do NOT ask about next slice. After fixes, present new HUMAN: block and re-enter this loop.
- **APPROVED**: Explicit approval only ("done", "PASSED", "looks good", "move on", "next slice"). → Proceed to Audit mode, then Evaluate, then next slice.
- **STOP**: Explicit stop/pause. → Wrap up session.
- **CRITICAL**: Never assume approval. Any issue descriptions = ISSUES REPORTED.

### Architect Mode
- Produce only what current slice needs. No speculative design.
- Contracts → `docs/contracts/`. Decisions → `docs/decisions.md`.
- Minimal, explicit interfaces. Brief tradeoff notes.

### Code Mode
- Implement only current slice scope.
- Small, testable increments. Tests must satisfy ATxx.
- No unrelated refactors. No TODO placeholders. No debug prints in committed code.
- Follow project conventions from constraints in `docs/requirements.json`.

### Debug Mode
- Reproduce first, then fix root cause.
- Add regression tests. Ensure quality_gates pass after fix.
- Report: commands run → results → files changed.

### Audit Mode
- NEVER fix code. Only report findings.
- Verify ATxx evidence exists for every in-scope feature.
- Check scope boundaries. Flag violations.
- Use `commands_to_run` from `docs/context.json`.
- Write only to: `docs/gates-evals/gate-report.md`, `docs/gates-evals/gate-report-S*.md`, `docs/context.json`.

### Evaluate Mode
- Only run after gate passes (verify `docs/gates-evals/gate-report.md`).
- Generate from user perspective. Map to user_flows and acceptance_tests.
- Use project's configured test framework for E2E.
- Write only to: `docs/gates-evals/eval-scenarios.md`, `docs/context.json`, `tests/e2e/*`.

## Available Skills

User-invokable skill files live in `.agents/skills/*/SKILL.md`.
Internal procedures live in `.agents/procedures/*.md` and are referenced by workflow skills.

Invoke skills with `$skill-name` in your prompt. Key skills:

**Workflow commands** (invoke directly):
- `$bridge-brainstorm` — Phase 0: brainstorm new project
- `$bridge-scope` — Phase 0: scope feature/fix for existing project
- `$bridge-requirements` — Phase 1: generate requirements from brainstorm
- `$bridge-requirements-only` — Phase 1: requirements from description (skip brainstorm)
- `$bridge-feature` — Phase 1: incremental requirements for existing project
- `$bridge-design` — Integrate a design document, PRD, or version spec
- `$bridge-start` — Start implementation
- `$bridge-resume` — Resume in fresh session
- `$bridge-end` — End session
- `$bridge-gate` — Run quality gate
- `$bridge-eval` — Generate evaluation pack
- `$bridge-feedback` — Process evaluation feedback
- `$bridge-context-create` — Create context.json
- `$bridge-context-update` — Sync context.json
- `$bridge-advisor` — Strategic advisor: viability, positioning, launch readiness

**Internal procedures** (not invoked directly with `$`):
- `.agents/procedures/bridge-slice-plan.md` — Plan and execute thin vertical slices
- `.agents/procedures/bridge-gate-audit.md` — Quality gate check procedures
- `.agents/procedures/bridge-eval-generate.md` — Evaluation pack generation procedures
- `.agents/procedures/bridge-session-management.md` — Session re-entry and wrap-up procedures
- `.agents/procedures/bridge-context-sync.md` — Context synchronization procedures
- `.agents/procedures/bridge-feedback-process.md` — Feedback triage procedures
