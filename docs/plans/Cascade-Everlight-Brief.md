# Cascade Everlight ✨

## Storage Retention Compensation for Lumera SuperNodes

---

## The Problem

Cascade is currently "pay once, store forever," while SuperNodes incur **recurring monthly storage costs**. This produces:

- **Unpriced long-term liability** — data accumulates indefinitely with no funding for retention
- **Weak operator incentives** — no ongoing compensation for the ongoing cost of keeping data available
- **Capacity rigidity** — storage constraints unnecessarily reduce non-storage throughput
- **Sustainability risk** — the network's permanence promise is economically unfunded

**Result:** SuperNodes subsidize storage at their own expense, creating long-term reliability and sustainability risk.

---

## The Goal

Users **register once**. SuperNodes get **paid continuously** for retained data. The network remains capacity-aware and sustainable.

---

## The Promise (User-Facing Framing)

**Phase 1 (at launch):**

> **"Pay once. Stored as long as the network sustains it."**

- Registration is a one-time payment. No subscriptions, no renewals.
- Data is retained on a **best-effort basis**, funded by Foundation transfers, registration fee share, and Community Pool governance transfers.
- Under normal economic conditions, data is expected to persist **indefinitely** — but Phase 1 does not make bounded time guarantees.

**Phase 3+ (after endowment module):**

> **"Pay once. Stored for at least N years — with best-effort permanence beyond."**

- Registration includes an optional tiered endowment that funds storage for a **guaranteed minimum period** (e.g., 5, 10, or 25 years).
- Beyond the guaranteed period, data persists on a best-effort basis, funded by ongoing protocol subsidies and endowment yield.

**Why this framing:** It preserves the "pay once" UX that users love, avoids subscription fatigue and data loss from missed renewals, while being legally and economically defensible. It avoids the reputational risk of promising "forever" if macro conditions shift. The Phase 1 framing is deliberately weaker — bounded guarantees require the endowment mechanism.

---

## The Proposal (4 Moves)

### Move 1 — `STORAGE_FULL` SuperNode State: Service-Aware Capacity Management

When a SuperNode's Cascade storage capacity crosses a threshold, it enters a new **`STORAGE_FULL`** state — distinct from the existing `POSTPONED` state used for general non-compliance. `STORAGE_FULL` nodes are excluded only from new storage assignments, while remaining fully eligible for compute workloads **and continuing to receive Everlight storage retention payouts** for the Cascade data they already hold. This is the core economic distinction: STORAGE_FULL nodes keep getting paid for existing data; POSTPONED nodes do not.

| Capability | `STORAGE_FULL` | `POSTPONED` |
|---|---|---|
| New Cascade storage assignments | ❌ Excluded | ❌ Excluded |
| Sense (compute) | ✅ Eligible | ❌ Excluded |
| Agents (compute) | ✅ Eligible | ❌ Excluded |
| **Everlight storage retention payouts** | **✅ Yes — paid for held data** | **❌ Not eligible** |

**What triggers `STORAGE_FULL`:** The SuperNode's `disk_usage_percent` exceeds `max_storage_usage_percent` (an existing supernode param) AND no other compliance violations exist. This reuses the existing storage metric rather than introducing a new threshold parameter.

A separate metric — `cascade_kademlia_db_bytes` — is also reported via LEP-4 and used for **Everlight payout weighting** (proportional distribution), but it does not drive the STORAGE_FULL state transition.

**Simple message:** "Disk-full nodes can still do compute work. Storage capacity is measured by disk usage percent."

---

### Move 2 — Everlight SN Pool: Dedicated Retention Funding & Payouts

Create a dedicated **Everlight SN Pool** (a named module account within `x/supernode`) that funds ongoing storage compensation. Every **Payment Period** — measured in block height — the pool distributes to eligible SuperNodes proportional to their retained Cascade data volume, as reported via LEP-4 metrics. No separate module is needed — all Everlight logic (pool, distribution, params, queries) lives within the existing `x/supernode` module.

#### Funding Sources

| # | Source | Phase | Chain Change? | Scales With | Notes |
|---|---|---|---|---|---|
| 1 | **Foundation staking rewards** | Pre-Phase 1 | No (operational) | Foundation stake | Bridges funding gap before protocol sources are live; Foundation sends directly to module account via standard `MsgSend` |
| 2 | **2% of action registration fees** (Cascade, Sense, Agents) | Phase 1 | Yes (parameter/governance) | Service demand | Entire Community Pool share of registration fees redirected to Everlight Pool |
| 3 | **Community Pool governance transfers** | Any time | No (governance vote) | Governance decision | Always available; any token holder can propose a transfer via `MsgCommunityPoolSpend` |
| 4 | **Endowment staking yield** (Phase 3) | Phase 3 | Yes (new module) | Cascade usage | Scales directly with stored data volume |
| 5 | **~1% of validator block rewards** | Phase 4 (optional) | Yes (`x/distribution`) | Network activity | Taken from Community Pool's existing allocation, not from validator or SuperNode earnings. Deferred due to consensus risk of `x/distribution` modification. |

**Simple message:** "Retention is funded by protocol incentives + registration flow + endowment yield. Users pay once."

---

### Move 3 — One-Time Retention Endowment at Registration (Phase 3 Only)

At registration, the user pays:

- **Registration Fee** — covers ingestion + initial replication (existing)
- **Everlight Endowment Fee** — optional tiered add-on that funds long-term retention

**What happens to the endowment principal (P):**

- P is **staked** (protocol-managed delegation spread across validators)
- Only the **staking rewards (yield)** flow into the Everlight SN Pool
- **Principal is preserved** as a long-lived buffer (policy choice, governance-tunable)

**Endowment is not required for MVP.** The pool funded by Foundation transfers, registration fee share, and Community Pool governance transfers provides a viable starting baseline. Endowments are added in Phase 3, and block reward routing optionally in Phase 4, once payout measurement and audit rails are stable.

---

### Move 4 — Audit Hardening (LEP-6 — Final Operational Phase)

Once the basic payout rails are stable, graduated audit mechanisms ensure payouts reflect actual Cascade data retention. **LEP-6 (Storage-Truth Enforcement)** is the primary vehicle — see `docs/plans/LEP-6 Storage-Truth Enforcement-draft.md`:

- **LEP-6 compound storage challenges** replace the current fixed-byte checks with deterministic multi-range byte-subset hashing, recent + old ticket subchallenges, node suspicion scoring, reporter reliability scoring, and ticket-deterioration-driven self-healing
- **Challenge-response proofs** (as specified in LEP-5) gate full payout eligibility
- **snscope cross-validation** — the existing snscope monitoring service acts as an independent oracle; persistent discrepancies between self-reported metrics and snscope observations trigger governance alerts and potential slashing
- **Anti-gaming enforcement:** Score-band-driven postpone/probation, Sybil detection, false-reporting penalties

---

## How Payments Work (Business View)

Every payment period (driven by block-height interval):

1. The Everlight SN Pool balance is calculated from all incoming sources
2. EndBlocker checks if `payment_period_blocks` have elapsed since last distribution
3. Pool distributes to eligible SuperNodes **proportional to `cascade_kademlia_db_bytes`** reported via LEP-4
4. SNs must meet minimum storage thresholds to qualify

**MVP Measurement:** Self-reported Cascade Kademlia DB size with guardrails (growth caps, smoothing window, new-SN ramp-up period).

**Hardened Measurement:** LEP-5 challenge-response proofs gate full payouts. snscope data provides cross-validation.

**Key property:** Payouts are funded by Foundation transfers, registration fee share, Community Pool transfers, and (later) endowment staking yield and block reward share. Users are never billed again after registration.

---

## Data Lifecycle (What Happens to Old Data?)

### 1. Normal Operation (Expected Steady State)
Foundation transfers + registration fee share + Community Pool transfers (and later endowment yield + block reward share) exceed aggregate storage costs. Data persists indefinitely. Hardware costs historically decline; additional funding streams added in later phases strengthen sustainability.

### 2. Guaranteed Period (Phase 3+)
Data within its guaranteed retention period (N years from registration) is **unconditionally retained** when an endowment tier is chosen. The protocol treats this as a hard commitment.

### 3. Beyond Guaranteed Period — Best-Effort Retention
After the guaranteed period (or for data without an endowment tier), data continues to be retained as long as the Everlight Pool can fund it. If pool funding becomes insufficient:

1. **Grace period** — governance-defined buffer (e.g., 6–12 months) before any action
2. **Notification** — data flagged as "at risk" with opportunity for re-endowment or community sponsorship
3. **Governance cleanup** — only via explicit governance proposal can data be marked for deletion

### 4. Emergency Scenario (Unlikely)
If funding falls below aggregate storage costs, governance can adjust parameters, transfer from Community Pool, or as a last resort mark unfunded data as expired.

**Simple message:** "Data lives as long as the economics hold — and the economics are designed to hold."

---

## Governance Knobs (What Can Be Tuned)

### Distribution Parameters (nested under `Params.reward_distribution`)
- `payment_period_blocks` — block-height interval between distributions
- `registration_fee_share_bps` — share of action registration fees to pool
- `min_cascade_bytes_for_payment` — minimum stored Cascade data to qualify
- `measurement_smoothing_periods` — rolling average window to prevent gaming
- `new_sn_ramp_up_periods` — gradual payout ramp for new nodes
- `usage_growth_cap_bps_per_period` — maximum rate of self-reported usage increase

### Eligibility Parameters
- `max_storage_usage_percent` — disk usage threshold for `STORAGE_FULL` state (existing supernode param)

### Endowment Parameters (Phase 3)
- Tiered pricing table — endowment cost per retention tier (5y / 10y / 25y)
- `everlight_delegation_spread` — how endowment is distributed across validators
- `guaranteed_retention_years_per_tier` — N-year guarantees by tier
- `risk_buffer_bps` — yield retained to absorb slashing events

---

## Delivery Roadmap (4 Phases)

### Phase 1 — Core Infrastructure: State + Everlight Pool + Registration Fee Routing
**Goal:** Ship the core operational system — SN state management, the Everlight pool (within `x/supernode`), block-height-driven periodic distribution, and registration fee routing — in a single chain upgrade.
**Requires:** Chain upgrade + governance proposal.

- Add **`STORAGE_FULL`** SuperNode state; update LEP-4 compliance logic to route storage-only violations to `STORAGE_FULL` (not `POSTPONED`)
- Add **`cascade_kademlia_db_bytes`** metric to LEP-4 self-reporting schema (SNs already collect this internally)
- Update Cascade action selection to exclude `STORAGE_FULL` nodes; Sense/Agents selection unaffected
- Extend **`x/supernode`** with Everlight pool account, block-height-driven periodic distribution (EndBlocker with `payment_period_blocks`), Everlight governance parameters, and query endpoints — no separate module needed
- Route **2% of registration fees** to Everlight pool via action module changes
- **Pre-upgrade (operational):** Foundation sends staking rewards to the Everlight pool account address via standard `MsgSend`; governance begins Community Pool transfers if desired

**Outcome:** SNs receive ongoing retention compensation immediately. Disk-full nodes remain productive. Pool funded by Foundation transfers, registration fee share, and Community Pool governance transfers.

---

### Phase 2 — Audit Hardening & Anti-Gaming (LEP-6)
**Goal:** Ensure payouts reflect actual data retention; close gaming vectors.
**Requires:** Chain upgrade. Developed outside this project — see `docs/plans/LEP-6 Storage-Truth Enforcement-draft.md`.

- **LEP-6 Storage-Truth Enforcement:** Compound storage challenges (recent + old ticket subchallenges) with deterministic one-third-of-nodes-per-epoch target selection, node suspicion scoring, reporter reliability/divergence scoring, and ticket deterioration scoring for self-healing
- LEP-5 challenge-response proofs gate full payout eligibility (partial payouts for unaudited SNs)
- snscope formally integrated as cross-validation oracle; persistent misreporting triggers governance alerts
- Anti-gaming enforcement: Sybil detection, false-reporting penalties, score-band-driven postpone/probation
- Storage challenge evidence becomes the main storage-related truth signal for audit enforcement

**Outcome:** Battle-hardened, manipulation-resistant payout system. Storage truth is measured directly, not inferred from reachability proxies.

---

### Phase 3 — Endowment Module
**Goal:** Add per-registration endowments for usage-proportional, self-sustaining long-term funding.
**Requires:** Chain upgrade.

- `x/endowment` module: tiered one-time fee at registration, principal staking via `x/staking`, yield routing to Everlight Pool
- On-chain endowment records per data item: tier, minimum retention end date, status
- Principal preservation policy encoded in parameters (governance-tunable)
- Per-endowment health monitoring; governance cleanup for matured/unfunded records

**Outcome:** "Pay once, stored for N years+" is backed by a dedicated endowment.

---

### Phase 4 (Optional) — Block Reward Routing + Distribution Surgery
**Goal:** Add block reward share as an additional protocol-native funding stream; replace governance-parameter-driven splits with proper module-level changes.
**Requires:** Chain upgrade. Most invasive phase — modifies `x/distribution` AllocateTokens.

- `x/distribution` modifications: permanent, automatic validator reward split routing ~1% of block rewards to Everlight Pool (taken from Community Pool's existing allocation, not from validator/SN earnings)
- Optional: migrate from block-height epoch to `x/epochs` module if available
- Optional: replace Phase 1's registration fee routing with a unified module-level fee split

**Outcome:** Fully diversified, protocol-native funding. Self-sustaining economics without reliance on Foundation transfers.

---

## Why This Wins (Business Outcomes)

| Outcome | How Everlight Delivers |
|---|---|
| **Best-in-class UX** | "Pay once" from Phase 1; "stored for N years+" from Phase 3 — no renewals, no subscription fatigue |
| **Sustainable operator economics** | SNs receive ongoing compensation aligned with ongoing costs |
| **Diversified funding** | Multiple revenue sources reduce single-point-of-failure risk (up to 5 across all phases) |
| **Network security bonus** | Endowment delegation increases total staked LUME |
| **Higher throughput** | Storage scarcity doesn't remove compute capacity (`STORAGE_FULL` state) |
| **Governance flexibility** | Every economic parameter is tunable without hard forks |
| **Competitive positioning** | "Arweave-grade retention + Cosmos interoperability + compute services" |
| **Low-risk rollout** | Foundation bridge starts payouts before upgrade; Community Pool always available; invasive `x/distribution` changes deferred to optional Phase 4 |

---

## Trade-offs Acknowledged

| Dimension | This Proposal | Pure Lease Model Alternative |
|---|---|---|
| User experience | Pay once, done | Must renew or lose data |
| Revenue predictability | Multiple streams, yield-dependent | Recurring, usage-based |
| Data expiration signal | Best-effort at Phase 1; bounded guarantee from Phase 3 | Clear (lease expires) |
| Implementation complexity | Higher (4 phases, Phase 4 optional) | Lower (fee + timer) |
| Macro risk exposure | Diversified across up to 5 streams (added incrementally) | None (user pays market rate) |
| Marketing story | "Pay once, stored sustainably" (Phase 1); "stored for N years+" (Phase 3) | "Affordable storage, pay as you go" |

---

## The Ask

Approve **Cascade Everlight** as the retention compensation model:

1. **Immediately (operational):** Foundation routes staking rewards to Everlight pool account address; governance considers Community Pool transfer proposals
2. **Phase 1 (chain upgrade):** `STORAGE_FULL` state, `cascade_kademlia_db_bytes` metric, Everlight pool + distribution logic within `x/supernode`, registration fee share routing
3. **Phase 2 (chain upgrade):** LEP-6 storage-truth enforcement, LEP-5 challenge-response proofs, snscope cross-validation, score-based anti-gaming (developed outside this project)
4. **Phase 3 (chain upgrade):** `x/endowment` module for "N-year guarantee" tier system
5. **Phase 4 (optional chain upgrade):** `x/distribution` surgery for block reward share routing to Everlight Pool
