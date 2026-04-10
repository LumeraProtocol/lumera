# Cascade Everlight Phase 1 — Implementation Plan

**Status:** Planning
**Scope:** lumera repo (chain) — Phase 1 only
**Related repos:** supernode, sdk-go, sdk-js (tracked here, implemented separately)
**Source docs:** Cascade-Everlight-Brief.md, Cascade-Everlight-Feature-Proposal.md

---

## Overview

Phase 1 delivers the core operational Everlight system in a single chain upgrade:
- `STORAGE_FULL` SuperNode state (service-aware capacity management)
- `cascade_kademlia_db_bytes` LEP-4 metric
- Everlight pool, distribution, params, and queries within `x/supernode` (no separate module)
- Registration fee share routing to Everlight pool

Phase 1 funding: Foundation direct transfers (pre-upgrade), registration fee share (2%), Community Pool governance transfers. Block reward routing via `x/distribution` is deferred to optional Phase 4.

---

## Multi-Repo Coordination

The lumera repo is the knowledge provider. Other repos consume chain state.

| Repo | Everlight Work | Dependency |
|---|---|---|
| **lumera** (this) | Extend x/supernode with Everlight pool + distribution, proto schemas, state transitions, fee routing, upgrade handler | None — goes first |
| **supernode** | Report `cascade_kademlia_db_bytes` in LEP-4 metrics (already collected internally) | lumera proto + SDK types |
| **sdk-go** | Updated proto bindings, Everlight query client helpers | lumera proto |
| **sdk-js** | Updated proto bindings, Everlight query helpers | lumera proto |

---

## Slices (lumera repo)

### S10 — Everlight Proto Schemas + Codegen

**Features:** F10 (SuperNode proto extensions), F11 (Everlight proto schemas within supernode)
**Goal:** All proto definitions in place; codegen passes; Go types compile.

Changes:
- `proto/lumera/supernode/v1/supernode_state.proto` — add `SUPERNODE_STATE_STORAGE_FULL = 6`
- `proto/lumera/supernode/v1/params.proto` — add `RewardDistribution` sub-message (field 19). `STORAGE_FULL` uses existing `max_storage_usage_percent`.
- `proto/lumera/supernode/v1/metrics.proto` — add `cascade_kademlia_db_bytes` (field 15)
- `proto/lumera/supernode/v1/query.proto` — add Everlight pool state, eligibility, payout history queries
- `proto/lumera/supernode/v1/genesis.proto` — extend with Everlight pool state

### S11 — STORAGE_FULL State + Compliance Bifurcation

**Features:** F12 (STORAGE_FULL state), F13 (Compliance bifurcation)
**Goal:** SNs with only disk-capacity violations enter STORAGE_FULL, not POSTPONED. STORAGE_FULL nodes are excluded from new Cascade storage assignments but **continue receiving Everlight payouts** for held data. POSTPONED nodes lose all eligibility including payouts.

Changes:
- `x/supernode/v1/keeper/metrics_validation.go` — split `evaluateCompliance` to identify disk-usage-only vs other violations
- `x/supernode/v1/keeper/metrics_state.go` — add `markStorageFull()`, `recoverFromStorageFull()`
- `x/supernode/v1/keeper/msg_server_report_supernode_metrics.go` — handle STORAGE_FULL transitions
- `x/supernode/v1/keeper/abci.go` — handle STORAGE_FULL in staleness handler
- Action SN selection — exclude STORAGE_FULL from Cascade, allow Sense/Agents

### S12 — Everlight Pool + Keeper Extensions

**Features:** F14 (Everlight pool and queries within x/supernode)
**Goal:** Pool account registered, Everlight params added to supernode, genesis import/export extended, query endpoints for pool state and eligibility.

Changes:
- `x/supernode/v1/keeper/everlight_pool.go` — pool balance, distribution height tracking
- `x/supernode/v1/keeper/everlight_queries.go` — pool state, SN eligibility queries
- `x/supernode/v1/types/` — Everlight pool state types
- `app/app_config.go` — register Everlight pool account (named account within supernode)

### S13 — Periodic Distribution Logic

**Features:** F15 (Block-height periodic distribution)
**Goal:** Supernode EndBlocker distributes pool balance to eligible SNs every `payment_period_blocks`.

Changes:
- `x/supernode/v1/keeper/everlight_distribution.go` — proportional distribution by cascade_kademlia_db_bytes
- `x/supernode/v1/keeper/everlight_eligibility.go` — eligible SN calculation (ACTIVE or STORAGE_FULL, min bytes, freshness)
- `x/supernode/v1/keeper/everlight_anti_gaming.go` — growth cap, smoothing window, new-SN ramp-up
- `x/supernode/v1/keeper/abci.go` — extend EndBlocker with block-height distribution check

### S14 — Registration Fee Routing

**Features:** F16 (Registration fee share)
**Goal:** Registration fee share flows to Everlight pool.

Changes:
- `x/action/v1/keeper/action.go` (`DistributeFees`) — route configured bps to Everlight pool account
- `x/action/v1/types/expected_keepers.go` — add interface for Everlight-aware bank ops

Note: F17 (Block reward share via `x/distribution`) is out of scope for Phase 1 — see Phase 4 in the roadmap.

### S15 — Upgrade Handler + Integration Tests

**Features:** F18 (Chain upgrade)
**Goal:** Clean upgrade using the branch's target upgrade version. Everlight params initialized within supernode. Pool account registered.

Changes:
- `app/upgrades/v1_15_0/` — upgrade handler (initialize Everlight params in supernode, register pool account)
- `app/upgrades/upgrades.go` — register v1.15.0
- Integration tests for full flow

---

## Open Questions

| ID | Question | Impact |
|---|---|---|
| OQ10 | Upgrade version — **resolved in current branch:** v1.15.0 | S15 |
| OQ12 | Fee routing — **resolved:** full 2% Community Pool share of registration fees redirected to Everlight pool (`registration_fee_share_bps` = 200). | S14 |
| OQ13 | Cascade SN selection — **resolved:** STORAGE_FULL nodes excluded from Cascade selection. Verified via AT31. | S11 |
| OQ14 | Module account permissions — **resolved:** receive+distribute only, no Minter/Burner. | S12 |

---

## Acceptance Tests (Everlight-specific)

| ID | Description |
|---|---|
| AT30 | SN with only `disk_usage_percent > max_storage_usage_percent` violation → STORAGE_FULL (not POSTPONED) |
| AT31 | STORAGE_FULL SN excluded from Cascade selection, included in Sense/Agents |
| AT32 | STORAGE_FULL SN recovers to ACTIVE when disk usage drops below threshold |
| AT33 | STORAGE_FULL + other violation → POSTPONED (more restrictive wins) |
| AT34 | Everlight pool account accepts MsgSend transfers |
| AT35 | Pool distributes proportionally by cascade_kademlia_db_bytes at period boundary |
| AT36 | SNs below min_cascade_bytes_for_payment excluded from distribution |
| AT37 | New SN receives ramped-up (partial) payout weight |
| AT38 | Usage growth cap limits reported cascade bytes increase per period |
| AT39 | Registration fee share flows to Everlight pool on action finalization |
| AT41 | All Everlight params governable via MsgUpdateParams |
| AT42 | Upgrade handler initializes Everlight params within supernode and registers pool account |
| AT43 | Existing SN states and actions unaffected by upgrade |
| AT44 | Pool with zero balance → no distribution, no panic |
| AT45 | No eligible SNs → no distribution, no panic |

---

## Risks

| ID | Risk | Mitigation |
|---|---|---|
| R10 | Metric gaming — SNs inflate cascade_kademlia_db_bytes | Growth cap + smoothing window + new-SN ramp-up from day one. Phase 2 (LEP-6) adds compound storage challenges and node suspicion scoring. |
| R12 | Proto field conflicts with in-flight changes | Coordinate: SN params field 19, SN metrics field 15, SN state enum value 6 |
| R13 | Pool account security | Everlight pool account: receive + distribute only, no Minter/Burner, no voting rights. |
| R14 | Multi-repo coordination delays | lumera ships first; other repos only need updated proto bindings |

---

## Future Phases (out of scope for Phase 1)

- **Phase 2 (LEP-6):** Storage-truth enforcement (compound challenges, node suspicion scoring, ticket deterioration), LEP-5 challenge-response proofs gate payouts, snscope cross-validation. Developed outside this project.
- **Phase 3:** x/endowment module, per-registration endowments, N-year guarantees
- **Phase 4 (optional):** x/distribution surgery for block reward share routing (~1% of Community Pool allocation to Everlight pool), optional x/epochs migration
