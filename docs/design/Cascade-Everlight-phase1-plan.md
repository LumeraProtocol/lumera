# Cascade Everlight Phase 1 - Implementation Plan

**Status:** Planning
**Scope:** lumera repo (chain) - Phase 1 only
**Related repos:** supernode, sdk-go, sdk-js (tracked here, implemented separately)
**Source docs:** Cascade-Everlight-Brief.md, Cascade-Everlight-Feature-Proposal.md

---

## Overview

Phase 1 delivers the core operational Everlight system in a single chain upgrade:
- `STORAGE_FULL` SuperNode state (service-aware capacity management)
- `cascade_kademlia_db_bytes` LEP-4 metric
- Everlight pool, distribution, params, and queries within `x/supernode` (no separate module)
- Registration fee share routing to Everlight pool

Phase 1 funding: Foundation direct transfers (pre-upgrade), registration fee share (2%), Community Pool governance transfers.

---

## Multi-Repo Coordination

The lumera repo is the knowledge provider. Other repos consume chain state.

| Repo | Everlight Work | Dependency |
|---|---|---|
| **lumera** (this) | Extend x/supernode with Everlight pool + distribution, proto schemas, state transitions, fee routing, upgrade handler | None - goes first |
| **supernode** | Report `cascade_kademlia_db_bytes` in LEP-4 metrics (already collected internally) | lumera proto + SDK types |
| **sdk-go** | Updated proto bindings, Everlight query client helpers | lumera proto |
| **sdk-js** | Updated proto bindings, Everlight query helpers | lumera proto |

---

## Slices (lumera repo)

### S10 - Everlight Proto Schemas + Codegen

**Features:** F10 (SuperNode proto extensions), F11 (Everlight proto schemas within supernode)
**Goal:** All proto definitions in place; codegen passes; Go types compile.

Changes:
- `proto/lumera/supernode/v1/supernode_state.proto` - add `SUPERNODE_STATE_STORAGE_FULL = 6`
- `proto/lumera/supernode/v1/params.proto` - add `RewardDistribution` sub-message (field 19). `STORAGE_FULL` uses existing `max_storage_usage_percent`.
- `proto/lumera/supernode/v1/metrics.proto` - add `cascade_kademlia_db_bytes` (field 15)
- `proto/lumera/supernode/v1/query.proto` - add Everlight pool state, eligibility, payout history queries
- `proto/lumera/supernode/v1/genesis.proto` - extend with Everlight pool state

### S11 - STORAGE_FULL State + Compliance Bifurcation

**Features:** F12 (STORAGE_FULL state), F13 (Compliance bifurcation)
**Goal:** SNs with only disk-capacity violations enter STORAGE_FULL, not POSTPONED. STORAGE_FULL nodes are excluded from new Cascade storage assignments but **continue receiving Everlight payouts** for held data. POSTPONED nodes lose all eligibility including payouts.

Changes:
- `x/supernode/v1/keeper/metrics_validation.go` - split `evaluateCompliance` to identify disk-usage-only vs other violations
- `x/supernode/v1/keeper/metrics_state.go` - add `markStorageFull()`, `recoverFromStorageFull()`
- `x/supernode/v1/keeper/msg_server_report_supernode_metrics.go` - handle STORAGE_FULL transitions
- `x/supernode/v1/keeper/abci.go` - handle STORAGE_FULL in staleness handler
- Action SN selection - exclude STORAGE_FULL from Cascade, allow Sense/Agents

### S12 - Everlight Pool + Keeper Extensions

**Features:** F14 (Everlight pool and queries within x/supernode)
**Goal:** Pool account registered, Everlight params added to supernode, genesis import/export extended, query endpoints for pool state, eligibility, and payout history.

Changes:
- `x/supernode/v1/keeper/everlight_pool.go` - pool balance, distribution height tracking
- `x/supernode/v1/keeper/everlight_queries.go` - pool state, SN eligibility queries
- `x/supernode/v1/types/` - Everlight pool state types
- `app/app_config.go` - use existing supernode module account as Everlight pool in Phase 1 (no separate named account)

### S13 - Periodic Distribution Logic

**Features:** F15 (Block-height periodic distribution)
**Goal:** Supernode EndBlocker distributes pool balance to eligible SNs every `payment_period_blocks`.

Changes:
- `x/supernode/v1/keeper/everlight_distribution.go` - proportional distribution by cascade_kademlia_db_bytes
- `x/supernode/v1/keeper/everlight_eligibility.go` - eligible SN calculation (ACTIVE or STORAGE_FULL, min bytes, freshness)
- `x/supernode/v1/keeper/everlight_anti_gaming.go` - growth cap, smoothing window, new-SN ramp-up
- `x/supernode/v1/keeper/abci.go` - extend EndBlocker with block-height distribution check

### S14 - Registration Fee Routing

**Features:** F16 (Registration fee share)
**Goal:** Registration fee share flows to Everlight pool.

Changes:
- `x/action/v1/keeper/action.go` (`DistributeFees`) - route configured bps to supernode module account (Everlight pool in Phase 1)
- `x/action/v1/types/expected_keepers.go` - add interface for Everlight-aware bank ops

### S15 - Upgrade Handler + Integration Tests

**Features:** F18 (Chain upgrade)
**Goal:** Clean upgrade using the branch's target upgrade version. Everlight params initialized within supernode. Pool account registered.

Changes:
- `app/upgrades/v1_11_x/` - upgrade handler (initialize Everlight params in supernode, initialize pool state in supernode module account model)
- `app/upgrades/upgrades.go` - register v1.11.x
- Integration tests for full flow

---

## Open Questions

| ID | Question | Impact |
|---|---|---|
| OQ10 | Upgrade version - **resolved in current branch:** v1.11.x | S15 |
| OQ12 | Fee routing - **resolved:** full 2% Community Pool share of registration fees redirected to Everlight pool (`registration_fee_share_bps` = 200). | S14 |
| OQ13 | Cascade SN selection - **resolved:** STORAGE_FULL nodes excluded from Cascade selection. Verified via AT31. | S11 |
| OQ14 | Module account permissions - **resolved:** Phase 1 uses existing supernode module account permissions; no separate constrained Everlight account. | S12 |

---

## Acceptance Tests (Everlight-specific)

| ID | Description |
|---|---|
| AT30 | SN with only `disk_usage_percent > max_storage_usage_percent` violation → STORAGE_FULL (not POSTPONED) |
| AT31 | STORAGE_FULL SN excluded from Cascade selection, included in Sense/Agents |
| AT32 | STORAGE_FULL SN recovers to ACTIVE when disk usage drops below threshold |
| AT33 | STORAGE_FULL + other violation → POSTPONED (more restrictive wins) |
| AT34 | supernode module account (Everlight pool in Phase 1) accepts MsgSend transfers |
| AT35 | Pool distributes proportionally by cascade_kademlia_db_bytes at period boundary |
| AT36 | SNs below min_cascade_bytes_for_payment excluded from distribution |
| AT37 | New SN receives ramped-up (partial) payout weight |
| AT38 | Usage growth cap limits reported cascade bytes increase per period |
| AT39 | Registration fee share flows to Everlight pool on action finalization |
| AT41 | All Everlight params governable via MsgUpdateParams |
| AT42 | Upgrade handler initializes Everlight params within supernode for module-account-based pool model |
| AT43 | Existing SN states and actions unaffected by upgrade |
| AT44 | Pool with zero balance → no distribution, no panic |
| AT45 | No eligible SNs → no distribution, no panic |

---

## Risks

| ID | Risk | Mitigation |
|---|---|---|
| R10 | Metric gaming - SNs inflate cascade_kademlia_db_bytes | Growth cap + smoothing window + new-SN ramp-up from day one. Phase 2 (LEP-6) adds compound storage challenges and node suspicion scoring. |
| R12 | Proto field conflicts with in-flight changes | Coordinate: SN params field 19, SN metrics field 15, SN state enum value 6 |
| R13 | Pool account security | Phase 1 uses existing supernode module account as pool; permission hardening of a dedicated account is explicitly out of scope for this phase. |
| R14 | Multi-repo coordination delays | lumera ships first; other repos only need updated proto bindings |

---

## Future Phases (out of scope for Phase 1)

- **Phase 2 (LEP-6):** Storage-truth enforcement (compound challenges, node suspicion scoring, ticket deterioration), LEP-5 challenge-response proofs gate payouts, snscope cross-validation. Developed outside this project.
- **Phase 3:** x/endowment module, per-registration endowments, N-year guarantees

---

## Supernode-side integration knowledge (authoritative implementation notes)

This section is maintained as the durable integration contract for supernode team implementation.

### 1) Source of payout bytes (critical)
- Chain payout weighting uses **audit epoch reports**, not legacy supernode health reports.
- Value path in reports: `HostReport.CascadeKademliaDbBytes`.
- Supernode must submit this field in `MsgSubmitEpochReport` host report each epoch.
- Keeper path: `x/supernode/v1/keeper/audit_metrics.go#getLatestCascadeBytesFromAudit`.

### 2) Legacy health reports behavior
- Legacy `x/supernode` health reports (`MsgReportSupernodeMetrics`) are still used for supernode compliance state transitions (`ACTIVE/POSTPONED/STORAGE_FULL`).
- They are **not** the payout-byte source for Everlight distribution.

### 3) Query default behavior for STORAGE_FULL
- `GetTopSuperNodesForBlock` default behavior (no explicit `state` filter): excludes both `POSTPONED` and `STORAGE_FULL`.
- To include `STORAGE_FULL`, caller must explicitly request state (e.g. `SUPERNODE_STATE_STORAGE_FULL`).
- This default matters for all downstream callers that rely on this query without explicit state semantics.

### 4) What exact metric name/value to send
- Metric name: `cascade_kademlia_db_bytes`.
- Wire location for payout source: `audit.v1 HostReport.cascade_kademlia_db_bytes` in epoch report.
- Unit: raw bytes (numeric double in proto representation; semantically bytes).

### 5) Payout-eligibility queries
- `PoolState`: pool balance, last distribution height, total distributed, eligible SN count.
- `SNEligibility`: per-validator payout eligibility + current byte/smoothed weight view.
- `PayoutHistory`: per-validator historical payout rows.
