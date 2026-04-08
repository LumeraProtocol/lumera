---
name: Bridge Design
description: Integrate a design document, PRD, or version spec into an existing BRIDGE project. Invoke with $bridge-design in your prompt.
---

You are following the BRIDGE v2.1 methodology. This is an EXISTING project with an established requirements.json and context.json.

The user is providing a design document — this could be a PRD, feature spec, architectural plan, version upgrade spec, or any structured description of new/changed functionality. Your job is to decompose it and integrate it into the existing BRIDGE artifacts without losing existing progress.

## TASK — INTEGRATE DESIGN

### Step 1: Load Existing State

1. Load docs/requirements.json — note ALL existing IDs (highest Fxx, ATxx, Sxx, UFxx, Rxx)
2. Load docs/context.json — note feature_status, completed work, active slices
3. Load docs/decisions.md — note existing architectural decisions
4. Run `git log --oneline -20` to understand recent activity
5. Targeted code inspection of areas the design will affect

### Step 2: Analyze the Design Document

Parse the provided design and classify each element:

```
DESIGN ANALYSIS

Document: [title/source]
Type: [PRD | feature spec | version spec | architectural plan | API spec | other]

ELEMENTS FOUND:
[N] new features (no overlap with existing)
[N] extensions to existing features (Fxx affected)
[N] modifications to existing behavior (breaking/non-breaking)
[N] deprecations or removals
[N] new architectural decisions
[N] new constraints or NFRs
[N] new integrations or interfaces
```

For each element, classify as:
- **NEW** — no overlap with existing features, gets new Fxx IDs
- **EXTEND** — adds capability to existing Fxx, gets new ATxx under existing feature
- **MODIFY** — changes existing behavior, needs careful migration
- **DEPRECATE** — marks existing features for removal
- **CONSTRAINT** — new technical constraint or NFR affecting existing work

### Step 3: Conflict Detection

Before making changes, identify conflicts:

```
CONFLICT REPORT

BREAKING CHANGES:
- [Fxx: what changes, what existing tests are affected]

DEPENDENCY CONFLICTS:
- [New feature depends on Fxx which is currently blocked/incomplete]

SCOPE CONFLICTS:
- [Design element X contradicts existing scope.out_of_scope or non_goals]

ASSUMPTION CHANGES:
- [Design assumes X, but existing scope.assumptions states Y]
```

### Step 4: Human Confirmation Gate

If the design involves MODIFY or DEPRECATE elements, or if conflicts were found in Step 3:
Present the Design Analysis and Conflict Report, then STOP and WAIT for the human to confirm before proceeding to Step 5.

If the design is purely additive (only NEW/EXTEND elements, no conflicts):
Proceed directly to Step 5, but still include the analysis in your output.

### Step 5: Update docs/requirements.json

Apply changes by category:

**For NEW elements:**
- Append new features continuing from highest existing Fxx
- New acceptance tests continuing from highest ATxx
- New slices continuing from highest Sxx
- New user flows, risks, interfaces as needed

**For EXTEND elements:**
- Add new acceptance tests to existing features (continue ATxx numbering)
- Update feature descriptions to reflect expanded scope
- Add new slices for the extension work

**For MODIFY elements:**
- Update affected feature descriptions
- Add new acceptance tests that verify the CHANGED behavior
- Add regression acceptance tests: "existing behavior Y still works after modification"
- Record the modification rationale in the feature's non_goals or a new decision

**For DEPRECATE elements:**
- Add `"deprecated": true, "deprecated_reason": "[reason]", "deprecated_by": "F[NEW]"` to affected features
- Add acceptance tests verifying deprecated features are properly handled (graceful degradation, migration paths)
- Do NOT remove existing features — mark them only

**For CONSTRAINT elements:**
- Update constraints, nfr, or quality_gates sections
- If a new constraint conflicts with existing implementation, add to execution.open_questions

**General rules:**
- NEVER overwrite or remove existing features, acceptance tests, or slices
- NEVER reuse existing IDs
- Preserve all existing progress and evidence
- If the design conflicts with existing scope, add to execution.open_questions — do not resolve silently
- Update scope.in_scope if project scope has expanded

### Step 6: Update docs/context.json

- Add new features to feature_status as "planned"
- For EXTEND/MODIFY features: keep existing status, add note about pending design integration
- Set next_slice to first new slice (or keep current if mid-slice)
- Add recent_decision: "[TODAY]: Integrated design '[title]' — [N] new features, [N] extensions, [N] modifications"
- Add any blockers or open questions surfaced during integration

### Step 7: Update docs/decisions.md

Record architectural decisions from the design document:

```
YYYY-MM-DD: [Decision from design] - [Rationale] (Source: [design document title])
```

If the design overrides existing decisions, record both:

```
YYYY-MM-DD: SUPERSEDES [previous decision date] - [New decision] - [Why the change] (Source: [design document title])
```

### Step 8: Update docs/human-playbook.md

- Append new slices to the Slice Verification Guide table
- Add new common pitfalls specific to the integrated design
- If MODIFY elements exist, add a "Migration/Regression Checklist" section:
  - What existing behavior to verify hasn't broken
  - What integration tests to run
  - What data migration steps are needed (if any)

### Step 9: Recommended Slice Ordering

Propose an ordered slice plan that accounts for dependencies:

```
RECOMMENDED SLICE ORDER

Phase 1 — Foundation (safe, no breaking changes):
  S[xx]: [goal] — [features] — [why first]

Phase 2 — Extensions (builds on existing):
  S[xx]: [goal] — [features] — [depends on]

Phase 3 — Modifications (breaking changes, needs migration):
  S[xx]: [goal] — [features] — [what breaks, how to migrate]

Phase 4 — Cleanup (deprecation, removal):
  S[xx]: [goal] — [features] — [what's removed]
```

### Step 10: Output Integration Report

```
DESIGN INTEGRATION COMPLETE

Source: [design document title/description]

Integrated:
- [N] new features: F[xx]-F[yy]
- [N] extended features: [Fxx list with what was added]
- [N] modified features: [Fxx list with what changed]
- [N] deprecated features: [Fxx list]
- [N] new acceptance tests: AT[xx]-AT[yy]
- [N] new slices: S[xx]-S[yy]
- [N] architectural decisions recorded
- [N] new risks / [N] new open questions

Conflicts found: [N] (see conflict report above)
Breaking changes: [yes/no — list affected features]

Files modified:
- docs/requirements.json — [summary of changes]
- docs/context.json — [N] features added, next_slice set to S[xx]
- docs/decisions.md — [N] decisions recorded
- docs/human-playbook.md — [N] slice entries, [N] pitfalls added
```

## Required Closing (do not omit)

Your response MUST end with a HUMAN: block. Use this format:

```
HUMAN:
1. Review the integration report — are new features and IDs correct?
2. Check for conflicts — do any breaking changes need discussion?
3. Review recommended slice ordering — does it make sense?
4. Decide: proceed to $bridge-start, or adjust the integration first