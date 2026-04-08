---
name: Bridge Gate Audit
description: Run quality gate checks and produce structured gate report. Use when features are in review status and need quality validation before evaluation.
---

# Quality Gate Audit

## Step 1: Identify Scope
Load features with status "review" from docs/context.json.
Load quality_gates thresholds from docs/requirements.json.

## Step 2: Run Checks

Execute using commands_to_run from context.json:
```bash
[commands_to_run.test] 2>&1 || true
[commands_to_run.lint] 2>&1 || true
[commands_to_run.typecheck] 2>&1 || true
[stack-appropriate security scan] 2>&1 || true
[build command if applicable] 2>&1 || true
```

If a command is missing, attempt stack convention and note the gap.

## Step 3: Evaluate
Per check: PASS (meets threshold) | FAIL (blocking) | WARN (non-blocking)

## Step 4: Verify Acceptance Criteria
For each in-scope feature:
1. Load acceptance_tests (ATxx)
2. Locate executable evidence
3. Mark verified or gap

## Step 5: Generate docs/gates-evals/gate-report.md

```markdown
# Gate Report
Generated: [timestamp]
Features Audited: [Fxx list]

## Summary
**OVERALL: [PASS | FAIL]**

## Test Results
- Unit: [X passed, Y failed] - Coverage: [Z%] (threshold: [T%]) - [PASS/FAIL]
- Integration: [status]

## Code Quality
- Lint Errors: [count] - [PASS/FAIL]
- Type Errors: [count] - [PASS/FAIL]

## Security
- Vulnerabilities: [high/mod/low] - [PASS/FAIL/WARN]

## Acceptance Test Evidence
| Feature | AT ID | Criterion | Evidence | Status |
|---------|-------|-----------|----------|--------|

## Blocking Issues
1. [Issue + file:line]

## Warnings
1. [Warning]

## Recommended Actions
1. [Specific fix]
```

## Step 6: Update Context
Append to gate_history in docs/context.json:
```json
{ "date": "[timestamp]", "result": "pass|fail", "features": ["Fxx"], "blocking_issues": 0, "warnings": 0, "coverage": "X%" }
```

## Step 7: Decision
- PASS → "GATE PASSED ✓ - ready for evaluation ($bridge-eval)."
- FAIL → "GATE FAILED ✗ - [N] blocking issues." + task list + "Re-run $bridge-gate after fixes."

## Step 8: Human Handoff (required)

```
HUMAN:
1. Verify these results yourself — run: [exact test/lint/typecheck commands]
2. Do NOT trust mock-only test passes — run at least one real integration test: [command]
3. Inspect docs/gates-evals/gate-report.md — do the results match what you see?
4. [If PASS] Run: $bridge-eval
5. [If FAIL] Confirm blocking issues, then feed fix instructions back
```
