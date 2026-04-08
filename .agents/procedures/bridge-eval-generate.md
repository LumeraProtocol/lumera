---
name: Bridge Eval Generate
description: Generate user test scenarios, E2E tests, and feedback template. Use only after a quality gate has passed.
---

# Evaluation Pack Generation

## Precondition
Verify docs/gates-evals/gate-report.md shows PASS. If not, abort and notify.

## Step 1: Manual Test Scenarios

Create docs/gates-evals/eval-scenarios.md:

```markdown
# Evaluation Scenarios
Generated: [timestamp]
Project: lumera

## How to Use
1. Set up application (see README)
2. Execute each scenario step-by-step
3. Record results in checklists
4. Fill feedback form at bottom

---

## Scenario N: [Feature Fxx] - [Happy Path / Edge Cases / Cross-Feature]
**Goal:** [what user accomplishes]
**Preconditions:** [setup]
**Linked:** [Fxx, ATxx, UFxx]

### Steps:
1. [Action] → Expected: [result]

### Checklist:
- [ ] Step N works as expected

---

## Feedback Form

### Overall Assessment
- [ ] Ready for launch  - [ ] Minor fixes  - [ ] Major fixes

### Ratings (1-5): Usability ___ | Performance ___ | Polish ___

### Issues Found
| # | Severity | Feature | Description | Steps to Reproduce |
|---|----------|---------|-------------|-------------------|

### Suggestions
[Free form]

### Would you use this? Why/why not?
[Free form]
```

## Step 2: E2E Tests
- Create in tests/e2e/ using project's test framework
- Map to e2e_critical_paths from quality_gates
- Happy path + key edge cases per feature

## Step 3: Update Context
Append to eval_history in docs/context.json:
```json
{ "date": "[timestamp]", "scenarios_generated": 0, "e2e_tests_generated": 0, "awaiting_feedback": true }
```

## Step 4: Output
```
EVALUATION PACK GENERATED ✓
Created: docs/gates-evals/eval-scenarios.md ([X] scenarios), tests/e2e/*.spec.* ([Y] files)

HUMAN:
1. Run E2E tests yourself: [exact command]
2. Walk through each scenario in docs/gates-evals/eval-scenarios.md manually — do not skip
3. Actually use the application as a real user would for each scenario
4. Fill in the feedback form at the bottom of docs/gates-evals/eval-scenarios.md
5. Note any DX friction, performance issues, or "this feels wrong" moments
6. Paste your filled feedback into: $bridge-feedback [your feedback]

Estimated evaluation time: [X] minutes
```
