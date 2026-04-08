---
description: "Phase 0: Scope a feature, fix, or extension for an existing project"

---

You are following the BRIDGE v2.1 methodology. This is an EXISTING project — not greenfield.

BRIDGE = Brainstorm → Requirements → Implementation Design → Develop → Gate → Evaluate

## TASK — PHASE 0: SCOPE (Existing Project)

The user wants to add a feature, fix a bug, or extend functionality in an existing codebase.

### Step 1: Understand Current State

1. Load docs/requirements.json and docs/context.json if they exist
2. Run `git log --oneline -20` to understand recent activity
3. Inspect project structure: build files, src/ layout, test structure
4. Targeted code inspection of areas likely affected by the requested change
5. Note: existing tech stack, patterns in use, test conventions, relevant dependencies

### Step 2: Scope the Change

Output format:

```
### Phase 0 — Scope Results

#### Change Summary
[1-2 sentences: what changes and why]

#### Type
[feature | fix | refactor | extension | integration]

#### Impact Analysis
- **Files likely affected:** [list with brief reason]
- **Files that MUST NOT change:** [boundaries]
- **Dependencies added/removed:** [if any]
- **Risk areas:** [what could break]

#### Existing Patterns to Follow
[How the codebase currently handles similar concerns — naming, error handling, testing, module structure. The implementation MUST follow these conventions.]

#### Approach
[2-5 bullets: high-level implementation strategy]

#### Acceptance Criteria (draft)
1. [Given/When/Then — what "done" looks like]
2. [Edge cases to handle]
3. [What should NOT change in behavior]

#### Open Questions
[Anything the human needs to decide before proceeding]

#### Estimated Scope
[S/M/L — number of slices likely needed, which existing features are touched]
```

### Step 3: Human Handoff

```
HUMAN:
1. Review the impact analysis — are the file boundaries correct?
2. Review the approach — does it match how you'd solve this?
3. Decide any open questions listed above
4. If this looks right, run: /bridge-feature [paste this scope output or "proceed"]
5. If scope needs adjustment, tell me what to change
```

Now scan the project and scope this change:

$ARGUMENTS
