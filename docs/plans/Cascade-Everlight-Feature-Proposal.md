# Cascade Everlight ✨ — Feature Proposal

**Doc ID:** CE-FP-002
**Status:** Draft
**Scope:** Cascade retention sustainability with no user renewals, using diversified protocol funding and a phased endowment model to compensate SuperNodes for ongoing storage.
**Non-goals (v1):** Perfect byte-accurate accounting, fully trustless measurement, per-file endowment health tracking, retrieval fee economics, cross-chain endowment payments, or unconditional "forever" marketing promises.

---

## 1. Executive Summary

Cascade Everlight transforms Lumera's storage economics from "pay once, hope SuperNodes keep your data" to **"pay once, fund storage sustainably."** SuperNodes receive **ongoing, proportional compensation** for retained Cascade data, funded by Foundation transfers, registration fee share, and Community Pool governance transfers at launch — with additional protocol-native funding streams (endowments, block reward share) added in later phases once measurement and audit rails are mature.

The system ships in four phases:

- **Phase 1 (single chain upgrade):** A new `STORAGE_FULL` SuperNode state ensures disk-constrained nodes remain productive for compute services. A new `cascade_kademlia_db_bytes` LEP-4 metric gives the chain accurate per-node Cascade storage data. Everlight pool, distribution, params, and queries are added to the existing `x/supernode` module (no separate module). Registration fee share routing funds the pool. The Foundation operationally pre-funds the pool address before the upgrade lands.
- **Phase 2:** Audit hardening via LEP-6 storage-truth enforcement, LEP-5 challenge-response proofs, and snscope cross-validation closes gaming vectors. Developed outside this project.
- **Phase 3:** A new `x/endowment` module adds per-registration endowments, staked principal, and yield-backed N-year retention guarantees.
- **Phase 4 (optional, most invasive):** `x/distribution` module surgery adds block reward share routing as an additional protocol-native funding stream.

Service-aware eligibility ensures that `STORAGE_FULL` SuperNodes remain active for compute workloads (Sense, Agents), maximizing network throughput and operator revenue.

---

## 2. Why Now

**SuperNodes are deploying, but the economics don't support them.** SuperNodes incur real monthly costs for storage — disk space, bandwidth, electricity, hardware replacement. Today they earn a one-time fee at action finalization and nothing afterward. As Cascade adoption grows, this gap between front-loaded revenue and ongoing costs will drive operators to either quietly drop data or leave the network.

**The longer we wait, the more unpaid liability accumulates.** Every Cascade upload creates a perpetual storage obligation with zero ongoing funding.

**Cosmos staking infrastructure makes this uniquely possible.** Unlike Arweave (which relies on declining storage cost assumptions alone), Lumera can use Cosmos SDK staking to generate real yield from endowments.

**LEP-4, LEP-5, and LEP-6 provide the foundation.** Self-reported metrics (LEP-4) give us the storage utilization data needed for proportional payouts. Storage verification challenges (LEP-5) provide the cryptographic proof mechanism. LEP-6 (Storage-Truth Enforcement) upgrades challenges into compound recent+old subchallenges with node suspicion scoring, reporter reliability tracking, and ticket-deterioration-driven self-healing — making storage truth the primary enforcement signal. Everlight is the economic layer that completes the picture.

---

## 3. Product Semantics (User-Facing Contract)

### 3.1 The Promise

**Phase 1 (at launch):**

> **"Pay once. Stored as long as the network sustains it."**

- Without an endowment tier: data is retained best-effort, funded by Foundation transfers, registration fee share, and Community Pool governance transfers.
- Under normal economic conditions, data is expected to persist **indefinitely** — but Phase 1 does not make bounded time guarantees.

**Phase 3+ (after endowment module):**

> **"Pay once. Stored for at least N years — with best-effort permanence beyond."**

- With an endowment tier: registration includes a tiered endowment fee that funds storage for a **guaranteed minimum period** (5, 10, or 25 years). Beyond this, best-effort retention continues while the pool is funded.
- The protocol makes bounded guarantees, not infinite ones.

### 3.2 What We Don't Call It

- Not "stored forever" publicly (internal aspiration, not a contract)
- Not "retention fees" (this model specifically avoids retention fees)
- Not "storage mining" (Lumera is not a mining-based chain)

---

## 4. Design Goals

| Goal | Measurable Outcome |
|---|---|
| No renewals required | User pays once; retention continues without periodic payments |
| Sustainable operator economics | SNs receive ongoing compensation covering marginal storage costs |
| Diversified funding | No single funding source > 60% of pool revenue at steady state |
| Bounded on-chain complexity | Minimal additional state; predictable per-period processing (EndBlocker with `payment_period_blocks`) |
| Upgradeable trust model | MVP uses pragmatic accounting; audit hardening follows in Phase 2 |
| Honest guarantees | Define minimum retention term; avoid unconditional "forever" claims |
| Service-aware eligibility | `STORAGE_FULL` SNs excluded from storage selection only, not compute |

---

## 5. `STORAGE_FULL` SuperNode State

### 5.1 Motivation

The existing LEP-4 `POSTPONED` state is a blunt instrument — it excludes a SuperNode from all services when any compliance threshold is violated. A SuperNode that has simply filled its Cascade storage allocation is still capable of performing compute services (Sense, Agents). Conflating storage capacity exhaustion with general non-compliance is economically wasteful and reduces network throughput.

### 5.2 State Definition

```protobuf
enum SuperNodeState {
  SUPERNODE_STATE_UNSPECIFIED = 0;
  SUPERNODE_STATE_ACTIVE      = 1;
  SUPERNODE_STATE_DISABLED    = 2;
  SUPERNODE_STATE_STOPPED     = 3;
  SUPERNODE_STATE_PENALIZED   = 4;
  SUPERNODE_STATE_POSTPONED   = 5;   // General non-compliance (existing)
  SUPERNODE_STATE_STORAGE_FULL = 6;  // NEW: storage capacity exhausted; compute-eligible
}
```

### 5.3 Transition Logic

LEP-4 compliance evaluation bifurcates:

- If the **only** failing metric is disk storage capacity (`disk_usage_percent` > `max_storage_usage_percent`) → transition to `STORAGE_FULL`
- If any other compliance threshold fails (CPU, memory, version, ports) → transition to `POSTPONED` (existing behavior)
- If both storage capacity and other metrics fail → transition to `POSTPONED` (more restrictive state takes precedence)

```
[ACTIVE] ──storage full only──> [STORAGE_FULL]
[ACTIVE] ──other non-compliance──> [POSTPONED]
[STORAGE_FULL] ──storage freed──> [ACTIVE]
[STORAGE_FULL] ──other non-compliance added──> [POSTPONED]
```

### 5.4 Service & Payout Eligibility Matrix

| State | New Cascade Storage | Sense | Agents | **Everlight Storage Payouts** |
|---|---|---|---|---|
| `ACTIVE` | ✅ | ✅ | ✅ | **✅ Yes** |
| `STORAGE_FULL` | ❌ | ✅ | ✅ | **✅ Yes — paid for held data** |
| `POSTPONED` | ❌ | ❌ | ❌ | **❌ Not eligible** |
| `DISABLED` | ❌ | ❌ | ❌ | **❌ Not eligible** |

**This is the core economic motivation for STORAGE_FULL:** A `STORAGE_FULL` node is still holding Cascade data in its local Kademlia DB — it has simply reached disk capacity. It **continues receiving Everlight payouts proportional to `cascade_kademlia_db_bytes`** (the data it holds). It is excluded from new storage assignments only. A `POSTPONED` node, by contrast, loses all eligibility — including payouts — because it has non-storage compliance violations that may indicate the node is unreliable.

### 5.5 LEP-4 Metric: `cascade_kademlia_db_bytes` (Everlight Payout Weighting)

A new LEP-4 metric reports the actual Cascade data held by each SuperNode. This metric is used for **Everlight payout weighting** (proportional distribution), not for STORAGE_FULL state transitions.

**New metric key:** `cascade_kademlia_db_bytes` (integer reported as double, consistent with LEP-4 schema convention)

SuperNode processes already collect Kademlia DB size internally for the status probe endpoint. This is a reporting addition only — no new internal measurement logic required.

**Updated LEP-4 metric table (storage section):**

| Key | Description | Value Type | Used For |
|---|---|---|---|
| `disk.total_gb` | Total disk space in GB | positive double | compliance |
| `disk.free_gb` | Available disk space in GB | positive double | compliance |
| `disk.usage_percent` | Overall disk usage % | 0–100 | compliance + **STORAGE_FULL trigger** |
| `cascade.kademlia_db_bytes` | **NEW** — Bytes held in Cascade Kademlia store | integer as double | **Everlight payout weighting** |

**STORAGE_FULL uses existing param:** `max_storage_usage_percent` (already in supernode Params). No new threshold parameter needed — `cascade_kademlia_db_max_bytes` was removed from the proto.

---

## 6. Everlight within `x/supernode`

### 6.1 Overview

All Everlight logic lives within the existing `x/supernode` module — no separate module. This simplifies wiring, avoids cross-keeper dependencies (distribution already reads supernode metrics), and keeps the module count lean. A separate `x/endowment` module is planned for Phase 3 when the scope justifies it.

The `x/supernode` module is extended with:
- A named **Everlight pool account** (sub-account of the supernode module)
- An EndBlocker extension that checks if `payment_period_blocks` have elapsed for periodic distribution
- Everlight governance parameters added to supernode Params
- Query endpoints for pool state, payout history, and SN eligibility

**snscope's role:** The existing snscope monitoring service is a valuable validation layer and will be formally integrated in Phase 2 as a cross-validation oracle. It does not change the Phase 1 architecture.

### 6.2 Pool Account

```
// Named account within x/supernode module
PoolAccountName = "everlight"
// Registered as a module account with receive + distribute only; no Minter/Burner
```

The pool account accepts `MsgSend` transfers from any address, including the Foundation wallet. This is how Foundation pre-funding works prior to the upgrade and as ongoing supplemental funding. The pool account has no Minter, Burner, or governance voting permissions.

### 6.3 On-Chain State

#### EverlightPoolState (Global — stored in supernode KVStore)

```go
type EverlightPoolState struct {
    Balance                  sdk.Coin  // Current undistributed pool balance
    LastDistributionHeight   int64     // Block height of last distribution
    LastDistributionPayout   sdk.Coin  // Amount distributed in last period
    TotalPayoutsAllTime      sdk.Coin  // Cumulative distributions
}
```

Phase 3 fields (`TotalEndowmentStaked`, `PendingEndowmentRewards`) will be added by the `x/endowment` module when it ships.

### 6.4 Governance Parameters (added to supernode Params)

The Everlight parameters are added to the existing `x/supernode` Params proto as a nested `Distribution` sub-message at field 19:

```protobuf
message RewardDistribution {
  uint64 payment_period_blocks          = 1; // Block-height interval between distributions
  uint64 registration_fee_share_bps     = 2; // Share of action registration fees (default: 200 = 2%)
  uint64 min_cascade_bytes_for_payment  = 3; // Minimum Cascade data to qualify for payouts
  uint64 new_sn_ramp_up_periods         = 4; // New SN ramp-up period in payment periods
  uint64 measurement_smoothing_periods  = 5; // Rolling average window for weight calculation
  uint64 usage_growth_cap_bps_per_period = 6; // Max rate of reported cascade bytes increase per period
}

// In existing supernode Params message:
RewardDistribution reward_distribution = 19;
```

The nested message keeps Everlight params cleanly separated within supernode Params. Endowment parameters (Phase 3) will be owned by the `x/endowment` module.

---

## 7. Periodic Distribution (Block-Height EndBlocker)

### 7.1 Distribution Period

Distribution is driven by a block-height interval configured via the governance parameter `payment_period_blocks`. The EndBlocker checks on every block whether the interval has elapsed since the last distribution. This avoids a dependency on the `x/epochs` module which is not yet available.

**Future migration:** When `x/epochs` becomes available, distribution can optionally migrate to an `AfterEpochEnd` hook, replacing the per-block height check with an epoch-driven callback. This is tracked as a Phase 4 optional item.

### 7.2 EndBlocker Distribution Logic

The distribution logic is part of the supernode keeper's existing EndBlocker:

```go
func (k Keeper) everlightDistribute(ctx sdk.Context) {
    params := k.GetParams(ctx)
    lastDistHeight := k.GetEverlightLastDistributionHeight(ctx)
    if ctx.BlockHeight()-lastDistHeight < int64(params.EverlightPaymentPeriodBlocks) {
        return
    }

    poolBalance := k.GetEverlightPoolBalance(ctx)
    if poolBalance.IsZero() {
        k.SetEverlightLastDistributionHeight(ctx, ctx.BlockHeight())
        return
    }

    // Read smoothed cascade_kademlia_db_bytes from LEP-4 metrics (same keeper — no cross-module call)
    // Eligible: ACTIVE or STORAGE_FULL, meets min_cascade_bytes_for_payment
    eligibleSNs := k.GetEligibleStorageSNsWithSmoothedMetrics(ctx)
    totalBytes := sdk.ZeroInt()
    for _, sn := range eligibleSNs {
        totalBytes = totalBytes.Add(sn.SmoothedCascadeBytes)
    }
    if totalBytes.IsZero() {
        k.SetEverlightLastDistributionHeight(ctx, ctx.BlockHeight())
        return
    }

    // Proportional distribution
    for _, sn := range eligibleSNs {
        share := poolBalance.Amount.Mul(sn.SmoothedCascadeBytes).Quo(totalBytes)
        k.DistributeToSN(ctx, sn.OperatorAddress, sdk.NewCoin(poolBalance.Denom, share))
    }

    k.SetEverlightLastDistributionHeight(ctx, ctx.BlockHeight())
    k.EmitDistributionEvent(ctx, ctx.BlockHeight(), poolBalance, int64(len(eligibleSNs)))
}
```

**Trade-off:** The EndBlocker runs a cheap height comparison on every block. Actual distribution work (SN iteration, bank sends) only occurs once per `payment_period_blocks`. This is acceptable for the expected SN count. Since distribution lives in the supernode keeper, it reads metrics directly — no cross-keeper dependency.

---

## 8. Funding Sources & Fee Routing

### 8.1 Foundation Direct Transfers (Operational — No Chain Change)

The Foundation claims its own staking rewards and sends them to the Everlight pool account using standard `MsgSend`. This works from day one — before any chain upgrade — because the named account address is deterministic.

```
foundation_wallet  --MsgSend-->  cosmos1<everlight_pool_account>
```

The pool distributes whatever balance it holds at each distribution period, regardless of source. No funding-source tracking is required in the distributor.

Community Pool governance transfers (`MsgCommunityPoolSpend`) target the same address and are always available as a supplemental source.

### 8.2 Block Reward Share (Phase 4 — Optional)

Modify `x/distribution` `AllocateTokens` to divert a portion of the Community Pool allocation to the Everlight Pool. This is the most invasive change in the Everlight roadmap and is deferred to Phase 4 as an optional funding enhancement.

**Current block reward split:**
| Recipient | Share |
|---|---|
| Validators / Delegators | ~98% |
| Community Pool | ~2% |

**Proposed (Phase 4):**
| Recipient | Share | Change |
|---|---|---|
| Validators / Delegators | ~98% | Unchanged |
| Community Pool | ~1% | Halved |
| Everlight Pool | ~1% | **NEW — from Community Pool's share** |

The exact basis points are governance parameters (`validator_reward_share_bps`, default 0 = disabled until Phase 4). Validator and SuperNode earnings are untouched.

**Why deferred:** Modifying `x/distribution` `AllocateTokens` carries consensus risk (R11) and requires extensive testing. Phase 1 funding via Foundation transfers, registration fee share, and Community Pool governance transfers is sufficient to bootstrap the system. Block reward routing is additive, not essential.

### 8.3 Registration Fee Share (Phase 1)

Modify `x/action` fee distribution to redirect the Community Pool share of registration fees to the Everlight Pool:

**Current Cascade fee split:**
| Recipient | Share |
|---|---|
| Finalizing SuperNodes | 98% |
| Community Pool | 2% |

**Proposed (Phase 1):**
| Recipient | Share | Change |
|---|---|---|
| Finalizing SuperNodes | 98% | Unchanged |
| Community Pool | 0% | Community Pool share fully redirected |
| Everlight Pool | 2% | **NEW** |

Applies to all action types (Cascade, Sense, Agents) — every service contributes to Cascade storage sustainability.

### 8.4 Funding Source Summary

| # | Source | Phase | Chain Change? | Scales With |
|---|---|---|---|---|
| 1 | Foundation direct transfers | Pre-Phase 1 + ongoing | No | Foundation decision |
| 2 | Community Pool governance transfers | Any time | No (governance vote only) | Governance appetite |
| 3 | 2% registration fee share | Phase 1 | Yes (`x/action`) | Service demand |
| 4 | Endowment staking yield | Phase 3 | Yes (new module) | Cascade data volume |
| 5 | ~1% block reward share | Phase 4 (optional) | Yes (`x/distribution`) | Network activity |

---

## 9. SN Payout Model

### 9.1 Weight Metric

Payouts are weighted by **`cascade.kademlia_db_bytes`** — the actual Cascade data held in the node's Kademlia store, as self-reported via LEP-4. This is more meaningful than raw disk usage (which includes chain state and OS overhead) and more directly proportional to actual retention costs.

### 9.2 Anti-Gaming Guardrails (Active from Phase 1)

| Mechanism | Parameter | Purpose |
|---|---|---|
| Growth cap | `usage_growth_cap_bps_per_period` | Limits how fast a node's reported share can increase per distribution period |
| Smoothing window | `measurement_smoothing_periods` | Rolling average prevents point-in-time manipulation |
| New-SN ramp-up | `new_sn_ramp_up_periods` | New nodes receive partial weight until established |
| Minimum threshold | `min_cascade_bytes_for_payment` | Excludes dust/spam nodes from distribution |

### 9.3 Eligibility Checklist

For each distribution period, an SN is eligible for payouts if:
- State is `ACTIVE` or `STORAGE_FULL`
- `cascade.kademlia_db_bytes` ≥ `min_cascade_bytes_for_payment`
- LEP-4 report is within `metrics_freshness_max_blocks` (not stale)
- (Phase 2+) Challenge pass rate meets minimum threshold

---

## 10. Delivery Roadmap

### Phase 1 — Core Infrastructure (Single Chain Upgrade)

**Summary:** `STORAGE_FULL` SN state + `cascade_kademlia_db_bytes` metric + Everlight pool and distribution within `x/supernode` + registration fee share routing.

**Pre-upgrade (operational, no chain change):**
- Foundation begins routing staking rewards to the Everlight pool account address via `MsgSend`
- Governance may initiate Community Pool transfer proposals at any time

**Chain upgrade deliverables:**
- `STORAGE_FULL` state added to SuperNode module protobuf; LEP-4 compliance logic updated so `disk_usage_percent > max_storage_usage_percent` with no other violations routes to `STORAGE_FULL`
- `cascade.kademlia_db_bytes` metric key added to LEP-4 schema; SuperNode software updated to report it
- Cascade action selection updated to exclude `STORAGE_FULL` nodes; Sense/Agents selection unchanged
- `x/supernode` extended with: Everlight pool account (receive + distribute only), EndBlocker distribution every `payment_period_blocks`, Everlight params, pool state and eligibility query endpoints — no separate module
- `x/action` modified: `2%` registration fee share to Everlight pool

**Outcome:** SNs receive compensation from first distribution period. Disk-full nodes remain productive. Pool funded by Foundation transfers, registration fee share, and Community Pool governance transfers.

---

### Phase 2 — Audit Hardening & Anti-Gaming (LEP-6)

**Summary:** Close gaming vectors via LEP-6 storage-truth enforcement; integrate snscope as cross-validation oracle. **Developed outside this project** — see `docs/plans/LEP-6 Storage-Truth Enforcement-draft.md` for full design.

**LEP-6 introduces:**
- **Compound storage challenges:** Each challenged node receives one recent-ticket + one old-ticket subchallenge per epoch, with deterministic multi-range byte-subset hashing (replacing fixed first-KB checks)
- **One-third deterministic target selection:** Only a bounded subset of nodes challenged each epoch, keeping traffic manageable
- **Node suspicion scoring:** Storage-specific evidence feeds audit enforcement (not reachability proxies)
- **Reporter reliability/divergence scoring:** Detects and deprioritizes unreliable challengers
- **Ticket deterioration scoring:** Direct on-chain memory of per-ticket storage health, replacing indirect watchlist proxy for self-healing triggers
- **Deterministic self-healing:** Singleton healer assignment with independent verifier quorum and probation period

**Additional Phase 2 deliverables:**
- LEP-5 challenge-response proofs integrated with Everlight eligibility: unchallenged or failed SNs receive partial payouts; fully audited SNs receive full payouts
- snscope formally integrated: persistent discrepancies between self-reported `cascade.kademlia_db_bytes` and snscope observations trigger on-chain governance alerts
- Score-band-driven postpone/probation decisions for storage-specific audit enforcement

**Outcome:** Payout system resistant to manipulation. Storage truth measured directly, not inferred from reachability. Full payouts require demonstrated data retention.

---

### Phase 3 — `x/endowment` Module

**Summary:** Per-registration endowments with staked principal and yield-backed retention guarantees.

**Deliverables:**
- **`x/endowment` module:**
  - `MsgRequestAction` (Cascade) extended with optional `everlight_tier` and `endowment_payment` fields
  - On-chain `EverlightEndowment` records keyed by `action_id`: tier, minimum retention end, status, cumulative yield
  - Module account delegates endowment principal across whitelisted validators via `x/staking`; yield withdrawn each distribution period and routed to Everlight Pool (minus `risk_buffer_bps`)
  - Principal preserved in module account; governance-tunable preservation policy
- Per-endowment health monitoring; governance cleanup for matured/unfunded records
- Data without endowment tier continues on best-effort basis (protocol-funded)

**Outcome:** "Pay once, stored for N years+" backed by a dedicated endowment. Foundation bridge scales down to purely supplemental.

---

### Phase 4 (Optional) — `x/distribution` Surgery + Block Reward Routing

**Summary:** Permanent automated block reward split as an additional protocol-native funding stream. Most invasive change — modifies `x/distribution` `AllocateTokens`.

**Deliverables:**
- **`x/distribution` surgery:** Add a module-level `CommunityPoolTax` bifurcation that routes `validator_reward_share_bps` (default ~1%) of the Community Pool allocation to the Everlight Pool. More robust than governance-parameter-driven splits; no parameter drift risk.
- Optional: migrate distribution trigger from block-height EndBlocker to `x/epochs` `AfterEpochEnd` hook if the module is available.

**Outcome:** Fully diversified protocol-native funding. Block rewards provide a network-activity-proportional funding stream independent of Cascade registration volume.

---

## 11. Risk List & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| **Metric Gaming (Sybil Storage)** — SNs inflate `cascade.kademlia_db_bytes` in Phase 1 to capture excess payouts | High | High | Growth caps + smoothing window + new-SN ramp-up from day one. Phase 2 (LEP-6) adds compound storage challenges with node suspicion scoring, reporter reliability scoring, and ticket deterioration tracking. snscope cross-validation adds external validation layer. |
| **Endowment Principal Erosion** — slashing of delegated validators reduces endowment principal | Low | Critical | Governance validator whitelist; per-validator delegation cap; `risk_buffer_bps` yield retention to recapitalize against minor slash events. |
| **Macro-Economic Decoupling** — LUME price crash or hardware cost spike makes payouts insufficient | Medium | High | Diversified funding (5 streams); compute revenue (Sense/Agents) supplements storage costs via `STORAGE_FULL` nodes; governance levers to adjust parameters. |
| **Block Processing Overhead** — distributing to many SNs per distribution period is too slow | Low | High | EndBlocker height check is cheap (runs every block); actual distribution only runs once per `payment_period_blocks`. Minimum threshold excludes dust nodes. Batching fallback if SN count exceeds limit. |
| **Governance Capture** — large endowment pool gives pool account outsized voting power | Low | High | Disable voting rights on Everlight pool account (standard Cosmos SDK capability). |
| **Marketing vs. Reality Gap** — users interpret "best-effort permanence" as "forever" | Medium | Medium | Explicit UI framing: "Guaranteed Term: X Years." No "Permaweb" branding. Legally distinct tier records on-chain. |

### Overall Risk Posture

The current "do nothing" state represents an existential risk: without Everlight, SNs have zero incentive to store data post-finalization. By implementing Cascade Everlight, the protocol trades a **guaranteed failure mode** (unfunded liabilities) for **manageable economic risks**.

The primary exposure during the Phase 1–2 window is metric gaming. The growth caps and smoothing window are the first line of defense; Phase 2 LEP-5 integration closes the vector cryptographically. The `x/distribution` modification risk (R11) is deferred to Phase 4, reducing Phase 1 consensus risk.
