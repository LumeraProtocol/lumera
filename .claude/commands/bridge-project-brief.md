---
description: "Generate or refresh docs/project-brief.md — a portable project explanation for external agents and discussions"
---

You are generating a project brief for external consumption. This document explains the project to outside AI agents, multi-agent councils, quorum-style discussions, and anyone onboarding to this codebase for the first time.

## Inputs

Read these in order:

1. `docs/context.json` — current state, feature status, handoff, gate history
2. `docs/requirements.json` — scope, features, acceptance tests, constraints
3. `docs/contracts/*` — schemas and architectural decisions (if present)
4. `docs/decisions.md` — architectural decision log (if present)

## Architecture Scan

After reading the docs, scan for architecture signals:

1. Package manifests in the repo root (package.json, Cargo.toml, go.mod, pyproject.toml, etc.)
2. Top-level directory structure (1 level deep only)
3. README.md and any CLAUDE.md / AGENTS.md
4. CI/CD config files (.github/workflows/, .gitlab-ci.yml, etc.)
5. Docker/container config (Dockerfile, docker-compose.yml)
6. Entrypoints (main files, CLI definitions, server startup files)

Do NOT read source code files unless a specific architectural claim needs validation.

If the directory scan reveals directories that are not referenced in `docs/requirements.json`, `docs/context.json`, or `README.md`, ignore them unless they materially affect an external reader's understanding of the documented product surface. If they do matter, mention them briefly as present in the repo but not part of the documented product surface.

## Source Interpretation

When sources differ:

- `docs/context.json` wins for current state, handoff, and gate/eval history
- `docs/requirements.json` wins for intended scope, feature inventory, user flows, and acceptance tests
- repository inspection validates architecture only; it must not become a substitute for stale BRIDGE artifacts
- do not overstate pack symmetry: the supported packs share BRIDGE methodology and command surface, but not the same internal implementation model

## Preconditions

This command is for canonical-state summarization, not reconciliation.

Before generating the brief, verify that:

- `docs/requirements.json` and `docs/context.json` exist and are current
- the latest gate/eval artifacts already reflect the current intended project state
- there is no known need to run `bridge-context-update`, `bridge-gate`, or `bridge-eval` first

If the canonical BRIDGE artifacts are stale, contradictory, or obviously behind the intended current state:

- do NOT try to reconcile them inside the brief
- tell the human to refresh the BRIDGE artifacts first
- stop instead of generating a misleading brief

## Output

Write the complete document to `docs/project-brief.md`. If the file already exists, overwrite it entirely — this is a derived artifact, not a place for durable manual notes.

Use this exact section structure:

```markdown
# Project Brief — [project name]

Last updated: [today's date]
Version: [from requirements.json project.version or inferred]
Status: [active development | maintenance | planning | blocked]

## For External Agents

When using this brief as context:
- Treat docs/requirements.json as the source of truth for scope and acceptance tests
- Treat docs/context.json as the source of truth for current state and handoff
- This brief is a summary — if it conflicts with requirements.json or context.json, the JSON files win
- Respect scope.out_of_scope and scope.non_goals when proposing features
- Use stable IDs (Fxx, ATxx, Sxx, UFxx, Rxx) when referring to project elements
- Do not assume features marked "planned" are committed — they are candidates

Recommended context packet for external discussions:
1. This file (docs/project-brief.md)
2. docs/requirements.json
3. docs/context.json
4. Optionally: a specific design note, issue, or feature proposal

## What This Project Does

[2-3 sentences: what it does, who it's for, what problem it solves, key differentiator]

## Architecture Overview

[What the system is and isn't. Core components. Key patterns. Install/packaging model if applicable. Do not claim all packs share the same internal implementation model.]

## Repository Map

[Top-level directory structure with purpose of each major area. Release artifacts if applicable. Mention potentially stale directories only if they materially affect external understanding.]

## Key Workflows

[Primary user journeys — keep to 3-5 workflows, one line each. Distinguish: greenfield (brainstorm -> requirements -> start), existing BRIDGE project (scope -> feature -> start), and non-BRIDGE onboarding (description/scope -> requirements-only -> context create/update -> start).]

## Current State

[Feature status table from requirements + context. Active or next slice from context if present. Latest gate/eval state. Recent activity recorded in canonical docs only.]

## Constraints and Non-Goals

[Technical constraints and explicit non-goals from requirements.json scope]

## Open Questions and Future Direction

[Planned features not yet started, open questions, active proposals]

## Origin and Evolution

[Brief project history — 2-3 sentences. Truncate this section first if over word limit.]
```

## Output Constraints

- **Word limit:** Keep the total document under 2000 words. If the project is complex, truncate from the bottom up — Origin/Evolution first, then Open Questions. Architecture and Current State are the most valuable sections.
- **Tone:** Factual, compact, oriented toward a reader who has never seen this project.
- **No invention:** If you cannot confirm a claim from the repository, label it: `inferred`, `unknown`, or `not yet documented`. Never invent architecture or capabilities.
- **Summarize, don't duplicate:** Link to canonical files. Do not copy full requirements or full context into the brief.
- **Canonical-only mode:** Do not describe uncommitted worktree drift, temporary implementation state, or chat-only intentions. If canonical BRIDGE artifacts are stale, stop and ask for them to be refreshed first.

## Required Closing (do not omit)

Your response MUST end with a HUMAN: block. Use this format:

```
HUMAN:
1. Review docs/project-brief.md — does it accurately represent the project?
2. If anything important is missing, record it in a separate decision note or proposal — do not rely on manual edits inside project-brief.md (it gets overwritten on refresh)
3. Decide: is this ready to feed to external agents, or does it need a rerun?
```

$ARGUMENTS
