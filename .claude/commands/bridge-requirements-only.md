---
description: "Phase 1: Generate requirements from existing project description (skip brainstorm)"

---

You are following the BRIDGE v2.1 methodology for solo-preneur software development with AI dev teams.

BRIDGE = Brainstorm → Requirements → Implementation Design → Develop → Gate → Evaluate

## SKIP PHASE 0 - Idea is already defined.

## TASK - CREATE REQUIREMENTS PACK (bridge.v2)

Using the project description and requirements provided below:
- Generate both requirements.json and context.json (same schemas as the bridge.v2 standard).
- Do not invent unknowns; put them in execution.open_questions.
- Keep it lean and execution-oriented: scope, constraints, acceptance tests, and slices matter most.
- Save requirements.json to docs/requirements.json
- Save context.json to docs/context.json

Use the bridge.v2 schema for requirements.json:
- schema_version: "bridge.v2"
- Stable IDs: F01/F02 for features, AT01/AT02 for acceptance tests, S01/S02 for slices
- Include: project, scope, constraints, domain_model, features (with acceptance_tests), user_flows, nfr, interfaces, quality_gates, execution (with recommended_slices, open_questions, risks)

Use the context.v1 schema for context.json:
- schema_version: "context.v1"
- Include: feature_status (all planned), handoff, next_slice, commands_to_run, empty arrays for gate_history/eval_history

### File 3: Save to docs/human-playbook.md

Generate a project-specific Human Operator Playbook. Structure:

```markdown
# Human Operator Playbook - [Project Name]
Generated from requirements.json

## Workflow Per Slice

### Before Each Slice
[Project-specific verification commands: build, test, lint — derived from constraints and quality_gates]

### After Each Slice
[How to smoke test — derived from the stack, interfaces, and what each slice produces]

## Slice Verification Guide

| Slice | Features | What YOU Test Manually | What to Read/Inspect | Decisions Needed |
|-------|----------|----------------------|---------------------|-----------------|
| S01   | F01, F02 | [concrete smoke test] | [key files to review] | [open questions for this slice] |

## Common Pitfalls
[3-5 project-specific warnings based on stack, constraints, and risks]

## Claude Code Prompt Template
```
Continue BRIDGE v2.1. Current state is in docs/context.json.
Execute next_slice [Sxx]: [goal].
Features: [Fxx list].
Exit criteria: [ATxx list].

Rules:
- Run [test/lint/typecheck commands] before declaring done
- Update docs/context.json with feature_status, evidence, gate_history
- Do NOT refactor previous slice code unless a test is failing
- If you hit an open question, STOP and ask — do not silently skip
```

## Open Questions Requiring Human Decision
[All execution.open_questions with context]
```

Tailor every section to THIS project — concrete commands, file paths, and test procedures. No placeholders.

### Specialist Recommendation

After generating requirements.json, scan the project's features, domain model, constraints, and scope against the specialist catalog at `.claude/specialists/catalog.md`.

For each specialist in the catalog:
1. Read the specialist file (e.g., `.claude/specialists/api-design.md`) and check its `triggers.keywords` and `triggers.feature_patterns` against the generated requirements
2. If keywords appear in feature titles, descriptions, domain model entities, constraint text, or scope items — the specialist is relevant
3. Determine confidence: high (3+ keyword matches or pattern+keyword), medium (1-2 keywords), low (pattern-only)

Persist matches in `execution.specialists_recommended` within the requirements.json you just generated:

```json
"specialists_recommended": [
  {
    "id": "specialist-id",
    "rationale": "Why this specialist is relevant to this project",
    "source": "bridge-requirements-only",
    "matched_signals": ["keyword1", "keyword2"],
    "applicable_features": ["F01", "F03"],
    "confidence": "high|medium|low"
  }
]
```

Only recommend specialists with at least one matched signal. Present the recommendations in the output summary.

## Required Closing (do not omit)

Your response MUST end with a HUMAN: block. Use this format:

```
HUMAN:
1. Review docs/requirements.json — are scope, features, and acceptance tests correct?
2. Review docs/context.json — does the initial state look right?
3. Check specialist recommendations — are they relevant? Override if needed
4. Decide: proceed to /bridge-start, or adjust requirements first
```

Here is the project description:

$ARGUMENTS
