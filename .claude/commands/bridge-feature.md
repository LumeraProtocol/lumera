---
description: "Generate incremental requirements for a feature/fix/extension in an existing project"

---

You are following the BRIDGE v2.1 methodology. This is an EXISTING project — you are adding to it, not starting from scratch.

## TASK — INCREMENTAL REQUIREMENTS

Based on the scope output from /bridge-scope (or the description provided below), generate incremental additions to the project's BRIDGE artifacts.

### Step 1: Load Existing State

1. Load docs/requirements.json — note the HIGHEST existing IDs (e.g., if F12 exists, new features start at F13)
2. Load docs/context.json — note current feature_status, active slices, and commands_to_run
3. If neither exists, treat this as a fresh setup and create both from scratch (fall back to /bridge-requirements-only behavior)

### Step 2: Generate New Features

For each new feature/fix, create entries that CONTINUE existing numbering:

```json
{
  "id": "F[NEXT]",
  "title": "",
  "priority": "must|should|could",
  "description": "",
  "acceptance_tests": [
    { "id": "AT[NEXT]", "given": "", "when": "", "then": "" }
  ],
  "dependencies": ["F[existing if any]"],
  "non_goals": [""]
}
```

Rules:
- Continue from highest existing Fxx/ATxx IDs. Do NOT reuse or overwrite existing IDs.
- Reference existing features in dependencies where applicable.
- Keep acceptance tests concrete and testable — no vague criteria.
- Include negative tests: "given X, when Y, then Z should NOT change" for risk areas.
- If this is a bug fix, the acceptance test should reproduce the bug as a regression test.

### Step 3: Generate Slices

Create new slices continuing from highest existing Sxx:

```json
{
  "slice_id": "S[NEXT]",
  "goal": "",
  "features": ["F[NEXT]"],
  "exit_criteria": ["AT[NEXT]"]
}
```

Rules:
- Keep slices thin — prefer multiple small slices over one large one.
- First slice should be the smallest provable change (kill criterion for feasibility if needed).
- If the feature touches existing code, first slice should include regression tests for existing behavior.

### Step 4: Update docs/requirements.json

APPEND to the existing file — do NOT overwrite existing features, slices, or other sections:
- Add new features to the `features` array
- Add new slices to `execution.recommended_slices`
- Add new user flows to `user_flows` if applicable
- Add any new open questions to `execution.open_questions`
- Add any new risks to `execution.risks`
- Update `scope.in_scope` if the project scope has expanded
- Do NOT modify existing features, acceptance tests, or slices unless explicitly asked

### Step 5: Update docs/context.json

- Add new features to `feature_status` with status "planned"
- Set `next_slice` to the first new slice
- Update `handoff.next_immediate` to describe what's next
- Add a recent_decision entry: "[TODAY]: Added [feature/fix description] — [N] features, [N] slices"
- Do NOT change status of existing features

### Step 6: Update docs/human-playbook.md

Append to the Slice Verification Guide table:

| Slice | Features | What YOU Test Manually | What to Read/Inspect | Decisions Needed |
|-------|----------|----------------------|---------------------|-----------------|
| S[NEXT] | F[NEXT] | [concrete smoke test] | [key files] | [open questions] |

Add any new common pitfalls specific to this change.

Update the Open Questions section if new questions were added.

### Step 7: Specialist Recommendation (Incremental)

After updating requirements.json, scan the NEW features you just added against the specialist catalog at `.claude/specialists/catalog.md`.

For each specialist in the catalog:
1. Read the specialist file and check its `triggers.keywords` and `triggers.feature_patterns` against the NEW feature titles, descriptions, and any new domain concepts
2. If keywords match — the specialist is relevant to the new features

Check `execution.specialists_recommended` in the existing requirements.json:
- If a specialist is already recommended: merge `applicable_features` (add the new Fxx IDs), keep the higher confidence level, union `matched_signals`, update `rationale` to reflect expanded scope
- If a specialist is new: append a new entry with `"source": "bridge-feature"`

```json
{
  "id": "specialist-id",
  "rationale": "Why this specialist is relevant",
  "source": "bridge-feature",
  "matched_signals": ["keyword1"],
  "applicable_features": ["F13", "F14"],
  "confidence": "high|medium|low"
}
```

Only recommend specialists with at least one matched signal against the NEW features. Include any new or updated specialist recommendations in the output summary.

### Step 8: Output Summary

```
INCREMENTAL REQUIREMENTS GENERATED

Added to existing project:
- [N] new features: [Fxx-Fyy] — [brief titles]
- [N] new acceptance tests: [ATxx-ATyy]
- [N] new slices: [Sxx-Syy]
- [N] new risks / [N] new open questions

Existing artifacts preserved:
- [N] existing features unchanged
- [N] existing slices unchanged

Files modified:
- docs/requirements.json — [N] features, [N] slices appended
- docs/context.json — feature_status updated, next_slice set to [Sxx]
- docs/human-playbook.md — [N] slice verification entries added

HUMAN:
1. Review the new features in requirements.json — are the acceptance tests concrete enough?
2. Review the slices — is the ordering and granularity right?
3. Check dependencies on existing features — are they correct?
4. Decide any open questions listed
5. If everything looks right: /bridge-start
6. If this is a critical fix, you can also: /bridge-start S[first new slice]
```

Now load the existing project state and generate incremental requirements for:

$ARGUMENTS
