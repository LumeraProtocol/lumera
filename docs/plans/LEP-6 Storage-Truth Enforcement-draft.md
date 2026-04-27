# LEP-6: Storage-Truth Enforcement and Ticket-Driven Self-Healing

**Status:** Draft

**Type:** Protocol / Audit / Supernode

**Created:** 2026-04-02

---

## 1. Executive Summary

LEP-6 upgrades Lumera’s storage integrity model in two linked ways:

1. **storage challenges become the main storage-truth signal for audit enforcement**
2. **self-healing becomes ticket-deterioration-driven instead of proxy-watchlist-driven**

The proposal keeps the good parts of the current design:

- deterministic challenger selection
- deterministic observer selection
- observer quorum
- deterministic self-healing orchestration
- quorum-gated persistence

But it fixes the parts that are currently too weak or too indirect:

- fixed early-byte challenge shape
- very limited challenge coverage relative to dataset size
- no forced distinction between recent ingestion and old retention
- storage enforcement still leaning too much on reachability / report proxy signals
- healing triggered from indirect watchlist behavior rather than direct ticket deterioration

The core model becomes:

- only **one third of active supernodes** are challenged each epoch, selected deterministically
- each challenged supernode receives **one compound storage challenge**
- that one challenge contains:
    - **1 recent-ticket subchallenge**
    - **1 old-ticket subchallenge**
- challenge outcomes update:
    - **node suspicion score** for enforcement
    - **ticket deterioration score** for healing
- each epoch has a bounded number of deterministic singleton self-heal ops
- healing is only accepted after **independent verifier quorum**
- verified healing reduces deterioration to a **probationary baseline**, not zero

This keeps traffic bounded while materially increasing challenge value.

---

## 2. Motivation

Lumera’s current storage challenge and self-healing designs have strong primitives, but still leave meaningful gaps.

The current storage challenge design is deterministic and already includes challengers, recipients, observers, and quorum verification. But today it still selects only **2 files per challenger per epoch**, uses a fixed proof slice that starts at **byte 0** and is **1024 bytes** long, and the audit module’s active enforcement path is still based on report / observation input, specifically required-port reachability observations in epoch reports.

That creates four concrete weaknesses:

- **fixed first-KB checks can be gamed**
- **coverage is too small relative to the file universe**
- **the current design does not force recent vs old coverage**
- **storage truth is not yet the main storage-related enforcement signal**

This matters because the system must answer two different questions:

- are supernodes actually ingesting newly assigned data?
- are supernodes silently deleting old / cold data later?

The current challenge design does not test those two behaviors strongly enough.

On the healing side, the current self-healing design is already deterministic, quorum-gated, and validates reconstructed content against expected action metadata hash before persistence. But its trigger is still built from a weighted watchlist proxy derived from audit storage-challenge reports, and the design itself acknowledges that this is an indirect signal and that reconstruct / re-encode work can be expensive under load.

That means Lumera currently lacks a first-class on-chain memory that **this specific ticket is deteriorating over time**.

LEP-6 fixes that by separating two concerns cleanly:

- **node suspicion** → used for punishment / audit action
- **ticket deterioration** → used for healing

---

## 3. Goals

### Primary goals

- make storage-specific challenge evidence the main storage-related input for audit enforcement
- challenge a bounded, deterministic subset of supernodes each epoch
- force every challenged node to prove both:
    - recent storage correctness
    - old storage retention
- attach every storage challenge outcome to the parent `ticket_id`
- maintain:
    - a **node suspicion score**
    - a **reporter reliability / divergence score**
    - a **ticket deterioration score**
- schedule self-healing only from sufficiently strong ticket deterioration evidence
- allow only one deterministic healer per heal op
- require independent post-heal verification before accepting success
- keep routine evidence batched in epoch reports instead of creating tx spam

### Non-goals

- giant on-chain ticket child manifests
- per-challenge standalone txs for routine baseline evidence
- full on-chain recomputation of byte-subset answers
- redesigning the current RaptorQ repair primitive inside this LEP

---

## 4. Current State Summary

### Storage challenges today

The current system is epoch-based, selects challengers deterministically, selects **2 files per challenger per epoch**, chooses recipient / observers deterministically from the replica set, and currently challenges a fixed slice starting at byte 0 with length 1024 bytes. The active audit enforcement input is still the report / observation path based on assigned-target required-port checks.

### Self-healing today

The current self-healing system is deterministic and safety-gated with request / verify / commit, observer quorum, and hash validation against expected action metadata. However, its trigger is still based on a watchlist proxy derived from audit storage-challenge reports, and that trigger is explicitly indirect.

---

## 5. Core Proposal

LEP-6 introduces six linked changes:

1. **ticket-scoped compound storage challenges**
2. **one-third deterministic target coverage per epoch**
3. **recent + old ticket subchallenges in every compound challenge**
4. **node suspicion scoring for audit enforcement**
5. **reporter reliability / divergence scoring**
6. **ticket deterioration scoring for self-healing**

---

## 6. Target Selection Model

## 6.1 Which supernodes get challenged each epoch

Instead of challenging every node every epoch, LEP-6 challenges a deterministic subset:

`challenge_target_count = max(1, ceil(active_supernodes / 3))`

This keeps network traffic bounded.

## 6.2 Deterministic target set

For each active supernode `sn`, compute:

`target_rank(sn) = H(epoch_seed || sn || "challenge_target")`

Sort ascending and select the first `challenge_target_count`.

This creates the challenged target set for the epoch.

## 6.3 Challenger and observer selection

Keep the **current challenger selection model** and **current observer-selection model**. That means LEP-6 does **not** redesign the challenger / observer topology already present in the current storage challenge runtime.

The change is in **what each challenge contains**, not in replacing the existing challenger / observer framework.

## 6.4 Pairing challengers to targets

Use deterministic pairing between the current epoch’s challenger set and the selected target set.

For each challenger `c`, assign the unassigned target `t != c` minimizing:

`pair_rank(c, t) = H(epoch_seed || c || t || "pair")`

Process challengers in deterministic order and assign the best remaining target.

This creates exactly one target per challenger and one challenger per target for the epoch.

---

## 7. Compound Challenge Structure

Each selected target receives **one compound challenge** containing two subchallenges:

- **1 recent-ticket subchallenge**
- **1 old-ticket subchallenge**

This gives more value per challenge while keeping traffic bounded.

The same challenger and same observer set handle both subchallenges for that target.

---

## 8. Exact Recent / Old Bucket Definitions

Bucket definitions must be block-based, not time-based.

Let:

- `current_height` = current block height
- `ticket_anchor_height` = canonical ticket completion / approval / finalization height
- `epoch_block_span` = number of blocks in one epoch

### Recent bucket

A ticket is **recent** if:

`current_height - ticket_anchor_height <= 3 * epoch_block_span`

### Old bucket

A ticket is **old** if:

`current_height - ticket_anchor_height >= 30 * epoch_block_span`

### Middle bucket

Anything in between is not part of mandatory baseline coverage, but remains eligible for:

- rechecks
- probation sampling
- score-driven extra challenges

These thresholds should be configurable chain params:

- `recent_bucket_max_blocks`
- `old_bucket_min_blocks`

The formulas above are the recommended defaults.

---

## 9. Deterministic Ticket Selection Rule

For a challenged target node and a bucket:

1. build the eligible ticket set that the node is expected to hold
2. exclude tickets with an active heal op
3. include probationary tickets as eligible
4. compute:

`ticket_rank(ticket) = H(epoch_seed || target_node || bucket || ticket_id)`

1. choose the ticket with the lowest rank

This gives:

- exactly one recent ticket
- exactly one old ticket

for each challenged target node.

If there is no eligible ticket in one bucket:

- record `NO_ELIGIBLE_TICKET`
- do not penalize the node for that bucket
- still execute the other bucket subchallenge if available

---

## 10. Deterministic Artifact Selection Rule

For each selected ticket subchallenge:

### Step 1: choose artifact class

Compute:

`class_roll = H(epoch_seed || target_node || ticket_id || "artifact_class") mod 10`

Default rule:

- `0–1` → `INDEX`
- `2–9` → `SYMBOL`

So baseline distribution is:

- **20% index**
- **80% symbol**

If the chosen class does not exist, fall back deterministically to the other class.

### Step 2: choose artifact ordinal

Compute:

`artifact_ordinal = H(epoch_seed || target_node || ticket_id || artifact_class || "artifact_ordinal") mod artifact_count`

Where `artifact_count` is derived from canonical ticket / index metadata off-chain.

### Step 3: resolve concrete key

The challenger, recipient, and observers resolve the concrete storage key from:

- `ticket_id`
- `artifact_class`
- `artifact_ordinal`

The transcript must include all three plus the resolved key.

Observers reject if the resolved key is inconsistent.

---

## 11. Deterministic Byte-Subset Challenge Rule

Replace the current fixed first-KB proof with deterministic random multi-range sampling.

For each challenged artifact:

- `k = 4` ranges
- `range_len = 256 bytes`

For each range `i`:

`offset_i = H(epoch_seed || target_node || ticket_id || artifact_class || artifact_ordinal || i) mod (artifact_size - range_len)`

Challenge input:

`challenge_bytes = concat(range_0, range_1, range_2, range_3)`

Challenge result:

`challenge_hash = blake3(challenge_bytes)`

This makes prefix-only optimization much less useful while keeping verification deterministic.

---

## 12. Evidence Submission Model

## 12.1 Routine evidence stays inside epoch health reports

Baseline storage challenge evidence should remain part of the existing epoch health report submission path.

That is better than creating a separate tx for every routine challenge because it:

- avoids tx spam
- keeps evidence aligned to epoch boundaries
- requires less migration from the current system

## 12.2 Health report schema upgrade

Add structured `storage_proof_results[]` entries to the epoch health report.

Each entry should carry:

- `ticket_id`
- `bucket` (`recent`, `old`, `probation`, `recheck`)
- `target_node`
- `artifact_class`
- `artifact_ordinal`
- resolved key
- derivation seed / range inputs
- result class
- challenger signature
- observer attestations or transcript hash

---

## 13. New Messages and Why They Are Needed

### `MsgSubmitStorageRecheckEvidence`

Used for asynchronous rechecks on disputed or severe failures.

**Why needed:** rechecks should not wait for the next routine epoch report if they are required to confirm or overturn a serious storage event.

### `MsgClaimHealComplete`

Used by the assigned healer to move a heal op from `IN_PROGRESS` to `HEALER_REPORTED`.

**Why needed:** healer completion must be explicit, but it must not finalize the heal.

### `MsgSubmitHealVerification`

Used by assigned verifiers to submit pass / fail evidence on a heal op.

**Why needed:** healing must not be accepted on healer self-report alone.

### Messages not needed

- no standalone `MsgSubmitStorageChallengeEvidence` for routine baseline challenges
- no user-submitted `MsgScheduleHealOp`; scheduling should happen deterministically from chain state at epoch transition

---

## 14. Node Suspicion Score

The node suspicion score is used for **audit enforcement**.

Let:

`N(node)` = node suspicion score

Lazy decay on update:

`N_new = floor(N_old * 0.92^(epochs_since_last_update)) + delta`

Clamp at `>= 0`.

### Event deltas

- symbol `HASH_MISMATCH`: `+18`
- index `HASH_MISMATCH`: `+26`
- `TIMEOUT_OR_NO_RESPONSE`: `+7`
- unresolved `OBSERVER_QUORUM_FAIL`: `+4`
- `RECHECK_CONFIRMED_FAIL`: `+15`
- recent `PASS`: `3`
- old `PASS`: `2`

### Pattern escalation

- second distinct failed ticket in last 14 epochs: `+10`
- third or more distinct failed tickets in last 14 epochs: `+15`
- at least one recent fail and one old fail in last 14 epochs: `+12`

---

## 15. Reporter Reliability and Divergence Score

This score captures the case you called out:

**a supernode keeps reporting bad outcomes about other supernodes while the rest of the network keeps reporting good outcomes about those same targets.**

Let:

`R(reporter)` = reporter reliability / divergence penalty

Lazy decay on update:

`R_new = floor(R_old * 0.90^(epochs_since_last_update)) + delta`

Clamp at `>= 0`.

## 15.1 Direct contradiction events

### Reported fail overturned by explicit recheck

If reporter submits a fail and an independent recheck confirms pass:

- `+25`

### Reporter submits bad outcome, but same target gets 2 independent clean passes

If within the next `W = 7` epochs the same target / same ticket receives at least 2 clean passes from distinct reporters or one clean pass plus a clean recheck:

- `+12`

This is the exact “I keep reporting bad, others keep reporting good” pattern.

## 15.2 Statistical divergence events

At epoch end, compute for each reporter with at least `min_reports = 5` storage results in the rolling `14`-epoch window:

- `neg_rate_reporter = bad_reports / total_reports`
- `neg_rate_network = median negative rate across reporters with sufficient volume`

If:

`neg_rate_reporter > 2.0 * neg_rate_network`

and the reporter’s negative results are **not** being consistently confirmed by rechecks, then:

- `+8`

This catches chronic outlier reporters even before every single report is individually overturned.

## 15.3 Positive recovery

- clean reproducible reporting epoch with no overturned fails and at least 5 results: `4`
- confirmed correct severe fail: `3`

## 15.4 How `R(reporter)` affects the system

### Trust multiplier

Provisional suspicion additions from that reporter are scaled by:

`trust_multiplier = max(0.5, 1 - R / 100)`

This applies only until recheck confirms the failure.

### Selection effects

- `R < 20` → normal
- `20–49` → low-trust, mandatory recheck on severe reported fails
- `50–89` → degraded, all fails treated as provisional until recheck
- `90+` → temporarily ineligible as challenger for `K = 7` epochs unless score drops

This is how the score is maintained and actually used in the grand scheme.

---

## 16. Ticket Deterioration Score

The ticket deterioration score is used for **healing**, not punishment.

Let:

`D(ticket)` = ticket deterioration score

Lazy decay on update:

`D_new = floor(D_old * 0.90^(epochs_since_last_update)) + delta`

Clamp at `>= 0`.

### Event deltas

- symbol fail on ticket: `+5`
- index fail on ticket: `+12`
- timeout on ticket: `+3`
- recheck-confirmed fail on ticket: `+8`
- same ticket fails on different holder within 14 epochs: `+10`
- same ticket fails again on same holder in different epoch: `+6`
- clean pass on ticket: `2`
- clean pass by different holder on ticket: `3`
- failed heal verification: `+15`

### Deterioration bands

- `0–9` → normal
- `10–19` → watch
- `20–34` → degraded
- `35–49` → at risk
- `50+` → heal candidate

A ticket only becomes heal-eligible if score threshold is met **and** one of the following holds:

- at least 2 distinct holders failed it recently
- an index artifact failed
- the same ticket failed repeatedly across epochs

---

## 17. Audit Penalty Matrix

The audit module should separate storage-specific actions from generic liveness health.

### Severity classes

### Class A — confirmed storage fault

- `RECHECK_CONFIRMED_FAIL`
- repeated `HASH_MISMATCH` on same ticket
- confirmed index artifact mismatch

### Class B — liveness-storage fault

- `TIMEOUT_OR_NO_RESPONSE`

### Class C — ambiguous fault

- `OBSERVER_QUORUM_FAIL`
- `INVALID_TRANSCRIPT`

Class C should trigger recheck, not direct punishment.

## Node score bands and actions

| Node score | Audit action | Challenge rate |
| --- | --- | --- |
| `0–19` | none | baseline |
| `20–49` | storage watch | elevated |
| `50–89` | storage probation | higher |
| `90–139` | postpone candidate | high |
| `140+` | strong postpone / severe storage action | max |

## Exact triggers

### Storage watch

- `N(node) >= 20`

### Storage probation

- `N(node) >= 50`
- or 1 Class A fault in last 14 epochs

### Postpone candidate

- `N(node) >= 90`
- and one of:
    - 1 recent Class A fault plus any second failure in 14 epochs
    - 2 old Class A faults on distinct tickets in 21 epochs
    - 4 Class B faults in 7 epochs

### Strong postpone / severe storage action

- `N(node) >= 140`
- and one of:
    - 2 Class A faults on distinct tickets in 14 epochs
    - any confirmed index artifact failure
    - failed heal verification by assigned healer

Recovery should require both:

- score decay below band threshold
- required number of clean passes with no new Class A failure

---

## 18. Self-Healing Scheduling Model

Each epoch can schedule up to:

`max_self_heal_ops_per_epoch = N`

Each heal op contains:

- `heal_op_id`
- `ticket_id`
- `assigned_healer`
- `assigned_verifiers`
- `scheduled_epoch`
- `deadline_epoch`
- `status`

### Ticket selection

Choose up to `N` tickets from the heal-eligible set deterministically, prioritizing:

- higher deterioration
- index failure presence
- distinct-holder failure diversity
- oldest unresolved deterioration

### Healer selection

Choose exactly one deterministic healer.

### Verifier selection

Choose exactly one deterministic verifier set.

---

## 19. Post-Heal Verification Scope

This should be done in **two layers**.

### Immediate post-heal verification

Immediate verification must challenge the **healer-served path only**.

Reason:
If verifiers check “broader availability” first, they can pass the heal even when the assigned healer did not actually perform the recovery work.

So immediate verifier checks must answer:

- did the assigned healer actually restore the ticket?

### Probation period

Broader holder availability should be tested during probation through normal storage challenges over subsequent epochs.

### Final rule

- immediate post-heal verification → healer-served path only
- probation → broader holder availability

---

## 20. Post-Heal Score Handling

On verified heal:

`D(ticket) = max(8, floor(D_old * 0.25))`

Also set:

`probation_until_epoch = current_epoch + K`

Recommended:
- `K = 3 to 5 epochs`

On failed heal verification:

- `D(ticket) += 15`
- healer reliability / execution penalty applies
- ticket enters cooldown before rescheduling

---

## 21. Chain State Additions

### Per-node

- `node_suspicion_score`
- `last_node_score_update_epoch`
- `reporter_reliability_score`
- `last_reporter_score_update_epoch`

### Per-ticket

- `ticket_deterioration_score`
- `last_ticket_score_update_epoch`
- `active_heal_op_id | null`
- `last_heal_epoch`
- `probation_until_epoch`

### Per-heal-op

- `heal_op_id`
- `ticket_id`
- `assigned_healer`
- `assigned_verifiers`
- `scheduled_epoch`
- `deadline_epoch`
- `status`

No giant on-chain manifest is needed.

---

## 22. Implementation Workstreams

### Workstream A — Chain / audit

- add node suspicion score
- add reporter reliability / divergence score
- add ticket deterioration score
- add heal-op state machine
- upgrade epoch health report schema with structured storage challenge evidence
- add recheck and heal-verification messages
- implement penalty matrix

### Workstream B — Storage challenge runtime

- keep current challenger / observer framework
- implement deterministic target subset selection
- implement compound recent + old challenge format
- implement block-based bucket rules
- implement deterministic artifact selection
- implement deterministic multi-range byte-subset hashing
- attach `ticket_id` to every result

### Workstream C — Self-healing runtime

- replace watchlist proxy as primary trigger with ticket deterioration
- support deterministic singleton healer
- support deterministic verifier set
- support healer-complete then verifier-confirm flow
- support probation

### Workstream D — Observability

Track:

- challenged target count per epoch
- recent / old challenge coverage
- pass / fail / timeout / quorum-fail split
- reporter contradiction rate
- reporter divergence score changes
- node suspicion changes
- ticket deterioration changes
- heal scheduled / verified / failed / expired
- probation pass rate

---

## 23. Two-Phase Rollout

## Phase 1 — Shadow + soft activation

Ship:

- deterministic one-third target selection
- compound recent + old challenge format
- block-based bucket rules
- ticket-scoped challenge evidence in health reports
- node suspicion score
- reporter reliability / divergence score
- ticket deterioration score
- deterministic heal scheduling
- post-heal verifier path

But only use the new system for:

- shadow scoring
- challenge-rate escalation
- ticket healing decisions
- audit visibility

Not yet for hard storage-specific postpone decisions.

## Phase 2 — Full storage-first enforcement

Activate:

- storage-specific audit penalty matrix
- score-band-driven postpone / probation decisions
- full reliance on ticket deterioration for heal scheduling
- full reporter-score effects on provisional trust and challenger eligibility

At the end of Phase 2, storage challenge evidence becomes the main storage-related truth signal for enforcement.

---

## 24. Why This Design Is Better

This design fixes the biggest structural gaps in the current system:

- current storage challenges are too predictable and too sparse for the dataset size
- current storage enforcement still leans too much on reachability observations
- current healing trigger is indirect and not ticket-native

And it does so without blowing up traffic or chain state:

- only one third of nodes are challenged each epoch
- each challenged node gets one higher-value compound challenge
- routine evidence stays batched in epoch reports
- no giant ticket manifest lands on chain

---

## 25. One-Paragraph Summary

LEP-6 upgrades Lumera from a system that mostly remembers reachability and proxy watchlist behavior into one that remembers **storage truth**, **reporter quality**, and **ticket deterioration** directly. Each epoch, one third of active supernodes are selected deterministically and each receives one compound storage challenge containing one recent-ticket and one old-ticket subchallenge. Challenge results remain batched in epoch health reports and update a node suspicion score, a reporter reliability / divergence score, and a ticket deterioration score on chain. Tickets that deteriorate beyond threshold are scheduled for deterministic singleton healing, and healing is only accepted after independent verifiers confirm the healer-served path, followed by probation-based broader checks. This yields a storage integrity model that is leaner, smarter, and much harder to game.