---
name: Bridge Requirements
description: "Phase 1: Generate requirements.json and context.json from brainstorm output. Invoke with $bridge-requirements in your prompt."
---

## TASK - PHASE 1: GENERATE REQUIREMENTS PACK (bridge.v2)

Based on the Phase 0 brainstorm output above (or provided below), generate TWO JSON files to be saved in the project.

Rules:
- Output JSON only per file. No prose between them.
- Every feature has stable IDs (F01, F02…) with acceptance tests (AT01, AT02…).
- Include scope (in_scope / out_of_scope / non_goals / assumptions).
- Include constraints, quality_gates, and execution.recommended_slices.
- If something is unknown, put it in execution.open_questions - do not invent.

### File 1: Save to docs/requirements.json

```json
{
  "schema_version": "bridge.v2",
  "project": {
    "name": "",
    "one_liner": "",
    "mission": "",
    "target_users": [""],
    "success_metrics": [""]
  },
  "scope": {
    "in_scope": [""],
    "out_of_scope": [""],
    "non_goals": [""],
    "assumptions": [""]
  },
  "constraints": {
    "languages": [""],
    "platforms": [""],
    "must_use": [""],
    "must_not_use": [""],
    "compliance_security_notes": [""]
  },
  "domain_model": {
    "core_entities": [
      { "name": "", "description": "", "key_fields": [""] }
    ],
    "state_machine_notes": [""]
  },
  "features": [
    {
      "id": "F01",
      "title": "",
      "priority": "must|should|could",
      "description": "",
      "acceptance_tests": [
        { "id": "AT01", "given": "", "when": "", "then": "" }
      ],
      "dependencies": [],
      "non_goals": [""]
    }
  ],
  "user_flows": [
    {
      "id": "UF01",
      "title": "",
      "preconditions": [""],
      "steps": [""],
      "postconditions": [""],
      "linked_features": ["F01"]
    }
  ],
  "nfr": {
    "performance_budgets": [""],
    "reliability": [""],
    "security": [""],
    "observability": [""],
    "scalability": [""]
  },
  "interfaces": {
    "apis": [
      { "name": "", "type": "http|grpc|cli", "notes": "", "schema_ref": "" }
    ],
    "external_services": [
      { "name": "", "purpose": "", "contracts_or_sdks": [""] }
    ]
  },
  "quality_gates": {
    "ci_required": true,
    "tests": {
      "unit": true,
      "integration": true,
      "e2e": "optional|required",
      "coverage_target": "80%"
    },
    "linters_typechecks": [""],
    "security_checks": [""],
    "performance_budgets": {
      "bundle_size": "",
      "api_response": "",
      "page_load": ""
    }
  },
  "execution": {
    "recommended_slices": [
      { "slice_id": "S01", "goal": "", "features": ["F01"], "exit_criteria": ["AT01"] }
    ],
    "open_questions": [""],
    "risks": [
      { "id": "R01", "risk": "", "impact": "low|med|high", "mitigation": "" }
    ]
  }
}
```

### File 2: Save to docs/context.json

```json
{
  "schema_version": "context.v1",
  "updated": "[TODAY'S DATE]",
  "project": { "name": "" },
  "feature_status": [
    { "feature_id": "F01", "status": "planned", "notes": "", "evidence": [] }
  ],
  "handoff": {
    "stopped_at": "Project initialization",
    "next_immediate": "Set up project scaffolding",
    "watch_out": ""
  },
  "next_slice": { "slice_id": "S01", "goal": "", "features": ["F01"], "acceptance_tests": ["AT01"] },
  "commands_to_run": { "test": "", "lint": "", "typecheck": "", "dev": "" },
  "recent_decisions": [],
  "blockers": [],
  "discrepancies": [],
  "gate_history": [],
  "eval_history": []
}
```

Now produce both JSON files using the Phase 0 brainstorm output. Save them to the docs/ directory.

### File 3: Save to docs/human-playbook.md

Generate a project-specific Human Operator Playbook based on the requirements and slices you just produced. Structure:

```markdown
# Human Operator Playbook - [Project Name]
Generated from requirements.json

## Workflow Per Slice

### Before Each Slice
[Project-specific verification commands: build, test, lint — derived from constraints and quality_gates]

### After Each Slice
[How to smoke test — derived from the stack, interfaces, and what each slice produces]

## Slice Verification Guide

For each slice in execution.recommended_slices, generate a row:

| Slice | Features | What YOU Test Manually | What to Read/Inspect | Decisions Needed |
|-------|----------|----------------------|---------------------|-----------------|
| S01   | F01, F02 | [concrete smoke test] | [key files to review] | [open questions relevant to this slice] |

## Common Pitfalls
[3-5 project-specific warnings based on the stack, constraints, and risks — e.g. "agent generates tests that only test mocks", "agent refactors previous slices", etc.]

## Codex Prompt Template
The exact prompt to feed Codex for each subsequent slice:
```
Continue BRIDGE v2.1. Current state is in docs/context.json.
Execute next_slice [Sxx]: [goal].
Features: [Fxx list].
Exit criteria: [ATxx list].

Rules:
- Run [test/lint/typecheck commands from quality_gates] before declaring done
- Update docs/context.json with feature_status, evidence, gate_history
- Do NOT refactor previous slice code unless a test is failing
- If you hit an open question, STOP and ask — do not silently skip
```

## Big Picture Cadence
[Visual showing the verify → feed → verify loop with estimated time per cycle]

## Open Questions Requiring Human Decision
[List all execution.open_questions with context for decision-making]
```

Tailor every section to THIS project's stack, constraints, features, and slices. Do not leave placeholders — fill in concrete commands, file paths, and test procedures based on the requirements you just generated.

### Step 4: Specialist Recommendation

After generating requirements.json, scan the project's features, domain model, constraints, and scope against the specialist catalog at `.agents/specialists/catalog.md`.

For each specialist in the catalog:
1. Read the specialist file (e.g., `.agents/specialists/api-design.md`) and check its `triggers.keywords` and `triggers.feature_patterns` against the generated requirements
2. If keywords appear in feature titles, descriptions, domain model entities, constraint text, or scope items — the specialist is relevant
3. Determine confidence: high (3+ keyword matches or pattern+keyword), medium (1-2 keywords), low (pattern-only)

Persist matches in `execution.specialists_recommended` within the requirements.json you just generated:

```json
"specialists_recommended": [
  {
    "id": "specialist-id",
    "rationale": "Why this specialist is relevant to this project",
    "source": "bridge-requirements",
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
4. Decide: proceed to $bridge-start, or adjust requirements first
```

The user will provide arguments inline with the skill invocation.
