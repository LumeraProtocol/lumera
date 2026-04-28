# LEP-6 Lumera Implementation Guide

This guide documents the `lumera` implementation of LEP-6 storage-truth enforcement and ticket-driven self-healing.

Priority design source: `/home/openclaw/workspace/docs/LEP6.md`

Branch: `LEP-6-consensus-gap-fixes-rebase` @ `5df4206` (rebased onto post-#118 `LEP-6-foundation`)

## Reviewer Summary

The LEP-6 work in `lumera` makes storage-truth outcomes part of the audit module's on-chain protocol:

- supernodes submit routine storage proof results inside `MsgSubmitEpochReport`
- the chain validates target assignment, reporter eligibility, proof shape, and transcript commitments
- the chain maintains node suspicion, reporter reliability, and ticket deterioration state
- reporter reliability affects the trust weight of future proof results and challenger eligibility
- epoch-end enforcement emits storage-truth bands and can postpone supernodes in active modes
- ticket deterioration schedules deterministic heal operations
- heal completion requires independent verifier majority
- recheck evidence links back to an already submitted proof transcript and adjusts scores
- genesis, query, AutoCLI, events, simulation, and tests were extended for the new state

Primary keeper entrypoints:

- `x/audit/v1/keeper/msg_submit_epoch_report.go`
- `x/audit/v1/keeper/msg_submit_epoch_report_storage_proofs.go`
- `x/audit/v1/keeper/storage_truth_scoring.go`
- `x/audit/v1/keeper/enforcement.go`
- `x/audit/v1/keeper/storage_truth_heal_ops.go`
- `x/audit/v1/keeper/msg_storage_truth.go`
- `x/audit/v1/keeper/storage_truth_fact_indexes.go`
- `x/audit/v1/keeper/storage_truth_divergence.go`

## Protocol Surface

### Protobuf Files

The LEP-6 chain surface is defined in:

- `proto/lumera/audit/v1/audit.proto`
- `proto/lumera/audit/v1/params.proto`
- `proto/lumera/audit/v1/query.proto`
- `proto/lumera/audit/v1/tx.proto`
- `proto/lumera/audit/v1/genesis.proto`

Generated Go bindings live under `x/audit/v1/types/*.pb.go`.

### Epoch Report Extension

`EpochReport` now includes:

- `supernode_account = 1`
- `epoch_id = 2`
- `report_height = 3`
- `host_report = 4`
- `storage_challenge_observations = 5`
- `storage_proof_results = 6`

`MsgSubmitEpochReport` now accepts:

- `creator = 1`
- `epoch_id = 2`
- `host_report = 3`
- `storage_challenge_observations = 4`
- `storage_proof_results = 5`

Routine LEP-6 proof evidence is submitted through `storage_proof_results`.

### StorageProofResult

`StorageProofResult` fields:

- `target_supernode_account = 1`
- `challenger_supernode_account = 2`
- `ticket_id = 3`
- `bucket_type = 4`
- `artifact_class = 5`
- `artifact_ordinal = 6`
- `artifact_key = 7`
- `result_class = 8`
- `transcript_hash = 9`
- `details = 10`
- `artifact_count = 11`
- `derivation_input_hash = 12`
- `challenger_signature = 13`
- `observer_attestation_signatures = 14`

`transcript_hash` is the primary chain commitment. LEP-6 activation also persists derivation/signature envelope fields so transcript disagreements are explicit and auditable.

### Bucket Enum Values

`StorageProofBucketType`:

- `UNSPECIFIED = 0`
- `RECENT = 1`
- `OLD = 2`
- `PROBATION = 3`
- `RECHECK = 4`

Business rules:

- `RECENT` and `OLD` are the routine compound challenge buckets.
- `PROBATION` is accepted as a valid submitted bucket.
- `RECHECK` is used for synthesized recheck outcomes.
- `RECHECK_CONFIRMED_FAIL` is valid only in `RECHECK`.
- `NO_ELIGIBLE_TICKET` is valid only in `RECENT` or `OLD`.

### Artifact Enum Values

`StorageProofArtifactClass`:

- `UNSPECIFIED = 0`
- `INDEX = 1`
- `SYMBOL = 2`

Business rules:

- Non-empty proof results require `INDEX` or `SYMBOL`.
- `NO_ELIGIBLE_TICKET` requires `UNSPECIFIED`.
- Index failures are treated as Class A faults and also satisfy strong-postpone and heal-eligibility predicates.

### Result Enum Values

`StorageProofResultClass`:

- `UNSPECIFIED = 0`
- `PASS = 1`
- `HASH_MISMATCH = 2`
- `TIMEOUT_OR_NO_RESPONSE = 3`
- `OBSERVER_QUORUM_FAIL = 4`
- `NO_ELIGIBLE_TICKET = 5`
- `INVALID_TRANSCRIPT = 6`
- `RECHECK_CONFIRMED_FAIL = 7`

Failure classes used by scoring/fact indexes:

- `HASH_MISMATCH`
- `TIMEOUT_OR_NO_RESPONSE`
- `OBSERVER_QUORUM_FAIL`
- `INVALID_TRANSCRIPT`
- `RECHECK_CONFIRMED_FAIL`

Recheck-eligible challenged result classes:

- `HASH_MISMATCH`
- `TIMEOUT_OR_NO_RESPONSE`
- `OBSERVER_QUORUM_FAIL`
- `INVALID_TRANSCRIPT`

### Enforcement Mode Enum Values

`StorageTruthEnforcementMode`:

- `UNSPECIFIED = 0`
- `SHADOW = 1`
- `SOFT = 2`
- `FULL = 3`

Mode behavior:

- `UNSPECIFIED`: storage-truth scoring, enforcement, and heal scheduling are disabled; legacy audit peer assignment is used.
- `SHADOW`: storage-truth scoring and band events run; supernode state is not changed by storage truth.
- `SOFT`: scoring and band events run; storage-truth predicates can postpone active supernodes.
- `FULL`: same enforcement behavior as `SOFT`, plus epoch reports must contain complete RECENT/OLD compound proof coverage for every assigned storage-truth target.

`DefaultParams()` sets the mode to `SHADOW`. `Params.WithDefaults()` deliberately does not promote an explicitly stored `UNSPECIFIED` mode.

## Constants And Defaults

All LEP-6 params are defined in `x/audit/v1/types/params.go` and exposed in `proto/lumera/audit/v1/params.proto`.

### Storage-Truth Challenge Shape

- `DefaultStorageTruthRecentBucketMaxBlocks = 3 * epoch_length_blocks` (default `1200`)
- `DefaultStorageTruthOldBucketMinBlocks = 30 * epoch_length_blocks` (default `12000`)
- `DefaultStorageTruthChallengeTargetDivisor = 3`
- `DefaultStorageTruthCompoundRangesPerArtifact = 4`
- `DefaultStorageTruthCompoundRangeLenBytes = 256`

Validation:

- `storage_truth_recent_bucket_max_blocks > 0`
- `storage_truth_old_bucket_min_blocks > 0`
- `storage_truth_recent_bucket_max_blocks < storage_truth_old_bucket_min_blocks`
- `storage_truth_challenge_target_divisor > 0`
- `storage_truth_compound_ranges_per_artifact > 0`
- `storage_truth_compound_range_len_bytes > 0`

### Storage-Truth Healing

- `DefaultStorageTruthMaxSelfHealOpsPerEpoch = 5`
- `DefaultStorageTruthProbationEpochs = 3`
- `DefaultStorageTruthTicketDeteriorationHealThreshold = 50`
- `DefaultStorageTruthHealDeadlineEpochs = 3`

Validation:

- `storage_truth_max_self_heal_ops_per_epoch > 0`
- `storage_truth_probation_epochs > 0`

`scheduleStorageTruthHealOpsAtEpochEnd` returns without scheduling if `StorageTruthMaxSelfHealOpsPerEpoch == 0`; validation/defaulting normally keeps it non-zero.

### Decay Factors

Decay factors are integer numerators over `1000`.

- `DefaultStorageTruthNodeSuspicionDecayPerEpoch = 920`, equivalent to `0.920` per epoch
- `DefaultStorageTruthReporterReliabilityDecayPerEpoch = 900`, equivalent to `0.900` per epoch
- `DefaultStorageTruthTicketDeteriorationDecayPerEpoch = 900`, equivalent to `0.900` per epoch

Decay formula:

```text
score = score * (factor / 1000) ^ elapsed_epochs
```

Implementation details:

- integer arithmetic is used, not floating point
- decay is capped at 50 iterations
- score is moved toward zero
- `factor <= 0` returns score unchanged
- `factor == 1000` returns score unchanged
- `factor > 1000` returns score unchanged
- param validation requires all three factors to be within `1..1000`

### Node Suspicion Thresholds

- `DefaultStorageTruthNodeSuspicionThresholdWatch = 20`
- `DefaultStorageTruthNodeSuspicionThresholdProbation = 50`
- `DefaultStorageTruthNodeSuspicionThresholdPostpone = 90`
- `DefaultStorageTruthNodeSuspicionThresholdStrongPostpone = 140`

Validation:

- `watch <= probation`
- `probation <= postpone`
- `postpone <= strong_postpone`

Band mapping:

- score `< watch`: no band
- score `>= watch`: watch
- score `>= probation`: probation
- score `>= postpone`: postpone candidate
- score `>= strong_postpone`: strong postpone

### Reporter Reliability Thresholds

Reporter reliability uses a positive-penalty model:

- `R = 0` means clean
- higher `R` means worse

Defaults:

- `DefaultStorageTruthReporterReliabilityLowTrustThreshold = 20`
- `DefaultStorageTruthReporterReliabilityDegradedThreshold = 50`
- `DefaultStorageTruthReporterReliabilityIneligibleThreshold = 90`

Validation:

- all three thresholds must be `>= 0`
- `low_trust <= degraded`
- `degraded <= ineligible`

Trust band mapping:

- `R < 20`: `NORMAL`
- `R >= 20`: `LOW_TRUST`
- `R >= 50`: `DEGRADED`
- `R >= 90`: `CHALLENGER_INELIGIBLE`

`ReporterTrustBand` numeric values:

- `UNSPECIFIED = 0`
- `NORMAL = 1`
- `LOW_TRUST = 2`
- `CHALLENGER_INELIGIBLE = 3`
- `DEGRADED = 4`

When a reporter enters `CHALLENGER_INELIGIBLE`, `ineligible_until_epoch = current_epoch + storage_truth_reporter_ineligible_duration_epochs` (default `+7`).

### Pattern, Divergence, And Recovery Windows

- `DefaultStorageTruthPatternEscalationWindow = 14`
- `DefaultStorageTruthDivergenceWindowEpochs = 14`
- `DefaultStorageTruthReporterMinReportsForDivergence = 5`
- `DefaultStorageTruthRecoveryCleanPassCount = 3`
- `DefaultStorageTruthClassAFaultWindow = 14`
- `DefaultStorageTruthClassBFaultWindow = 7`
- `DefaultStorageTruthOldClassAFaultWindow = 21`
- `DefaultStorageTruthContradictionWindowEpochs = 7`
- `DefaultStorageTruthReporterIneligibleDurationEpochs = 7`

### Store Key Prefixes

Storage-truth state and indexes use the audit KV store.

- node suspicion: `"st/ns/" + supernode_account`
- reporter reliability: `"st/rr/" + reporter_supernode_account`
- ticket deterioration: `"st/td/" + ticket_id`
- heal op: `"st/ho/" + u64be(heal_op_id)`
- heal op by ticket index: `"st/hot/" + ticket_id + 0x00 + u64be(heal_op_id)`
- heal op by status index: `"st/hos/" + u32be(status) + u64be(heal_op_id)`
- heal verification: `"st/hov/" + u64be(heal_op_id) + "/" + verifier_supernode_account`
- next heal op id: `"st/next_ho_id"`
- recheck evidence dedup: `"st/rce/" + u64be(epoch_id) + "/" + ticket_id + 0x00 + creator_account`
- proof transcript index: `"st/spt/" + transcript_hash`
- node failure fact index: `"st/nf/" + supernode_account + "/" + u64be(epoch_id) + "/" + ticket_id + 0x00 + reporter_account`
- reporter result fact index: `"st/rrs/" + reporter_account + "/" + u64be(epoch_id) + "/" + ticket_id + 0x00 + target_account`
- failed heal index: `"st/fh/" + supernode_account + "/" + u64be(epoch_id) + "/" + ticket_id`
- storage-truth postponement marker: `"ap/st/" + supernode_account`

All epoch IDs in keys are encoded as 8-byte big-endian integers so lexicographic order matches epoch order.

### Events

Storage-truth events:

- `storage_truth_score_updated`
- `storage_truth_heal_op_scheduled`
- `storage_truth_heal_op_expired`
- `storage_truth_heal_op_healer_reported`
- `storage_truth_heal_op_verified`
- `storage_truth_heal_op_failed`
- `storage_truth_recheck_evidence_submitted`
- `storage_truth_band_watch`
- `storage_truth_band_probation`
- `storage_truth_band_postpone_candidate`
- `storage_truth_enforced`
- `storage_truth_recovered`

Common attributes:

- `epoch_id`
- `reporter_supernode_account`
- `target_supernode_account`
- `ticket_id`
- `heal_op_id`
- `verifier_supernode_account`
- `healer_supernode_account`
- `verified`
- `verification_hash`
- `transcript_hash`
- `deadline_epoch_id`
- `result_class`
- `bucket_type`
- `node_suspicion_score`
- `reporter_reliability_score`
- `ticket_deterioration_score`
- `reporter_trust_band`
- `repeated_failure_count`
- `contradiction_detected`
- `contradicted_reporter`
- `storage_truth_band`
- `enforcement_mode`
- `recheck_result_class`

## Deterministic Target Assignment

Code: `x/audit/v1/keeper/audit_peer_assignment.go`

Storage-truth target assignment is used when `storage_truth_enforcement_mode != UNSPECIFIED`.

The target count is:

```text
target_count = max(1, ceil(active_supernodes / storage_truth_challenge_target_divisor))
```

With the default divisor `3`, this is one third of the active set, rounded up.

Assignment inputs:

- epoch anchor seed
- active epoch-anchor accounts
- target candidate accounts
- reporter account
- storage-truth params

Target selection:

1. Sort and deduplicate active accounts.
2. Intersect target candidates with active accounts.
3. If the intersection is empty, use the active set.
4. Compute `target_count`.
5. Rank targets by SHA-256 over `seed || 0x00 || account || 0x00 || "challenge_target"`.
6. Select the lowest-ranked targets.

Challenger pairing:

1. Iterate active challengers in sorted order.
2. For each challenger, choose the lowest-ranked unassigned selected target using SHA-256 over `seed || 0x00 || challenger || 0x00 || target || 0x00 || "pair"`.
3. Avoid self-targeting.
4. Assign at most one storage-truth target to a selected challenger.
5. If the current reporter receives a target, return exactly that target.

Reporter eligibility:

- `UNSPECIFIED` mode returns all active reporters and uses legacy assignment.
- Active reporters with decayed reliability `>= storage_truth_reporter_reliability_ineligible_threshold` are excluded while `ineligible_until_epoch == 0` or `ineligible_until_epoch >= current_epoch`.

Legacy peer-observation assignment remains in the same file and is selected only in `UNSPECIFIED` mode. It uses:

```text
k_needed = ceil(peer_quorum_reports * receivers_count / senders_count)
k_needed = clamp(k_needed, min_probe_targets_per_epoch, max_probe_targets_per_epoch)
k_needed = min(k_needed, receivers_count - 1)
```

## Epoch Report Validation

Code:

- `x/audit/v1/keeper/msg_submit_epoch_report.go`
- `x/audit/v1/keeper/msg_submit_epoch_report_storage_proofs.go`

Storage-proof validation runs inside `MsgSubmitEpochReport`.

Report-level rules:

- creator must be a registered supernode
- report epoch must match the currently accepted epoch
- epoch anchor must exist
- duplicate epoch reports are rejected
- storage-proof results are accepted only from reporters eligible for the epoch
- allowed storage-proof targets are derived from deterministic assignment

Per-result rules:

- result must not be nil
- `target_supernode_account` is required
- `challenger_supernode_account` is required
- challenger account must equal report creator
- target cannot equal reporter
- target must be assigned to reporter for that epoch
- `transcript_hash` is required
- bucket type must be one of `RECENT`, `OLD`, `PROBATION`, `RECHECK`
- result class must be one of the implemented non-unspecified result classes
- duplicate descriptors are rejected
- at most `MaxStorageProofResultsPerReport = 16` storage-proof results are accepted per report (`x/audit/v1/types/keys.go:13`; enforced at `msg_submit_epoch_report.go:126`)

The duplicate descriptor key is:

```text
(target, bucket, ticket_id, artifact_class, artifact_ordinal)
```

`artifact_key` is intentionally excluded from the dedup key: per LEP-6 §10, `artifact_key` is a deterministic function of the 5-tuple above, and including it would allow a prober to submit two contradictory results for the same logical descriptor by varying only the attacker-supplied `artifact_key` value, bypassing dedup and double-counting once scoring is active (`msg_submit_epoch_report_storage_proofs.go`).

`NO_ELIGIBLE_TICKET` shape:

- bucket must be `RECENT` or `OLD`
- `ticket_id` must be empty
- `artifact_class` must be `UNSPECIFIED`
- `artifact_ordinal` must be `0`
- `artifact_key` must be empty

All other result classes require:

- non-empty `ticket_id`
- artifact class `INDEX` or `SYMBOL`
- non-empty `artifact_key`
- `artifact_count > 0`
- `artifact_ordinal < artifact_count`
- non-empty `derivation_input_hash`
- non-empty `challenger_signature`

Canonical artifact-count anchoring:

- every non-`NO_ELIGIBLE_TICKET` proof result must reference a ticket with canonical on-chain artifact counts
- canonical counts are anchored at ticket finalization and stored per ticket as class-specific counts (`index`, `symbol`)
- submitted `artifact_count` must match the canonical class-specific count for the ticket
- submitted `artifact_ordinal` must be in range for that canonical class-specific count
- canonical counts are immutable once anchored
- `NO_ELIGIBLE_TICKET` submissions are rejected when recent transcript history already shows eligible tickets for the same target and bucket inside the bucket-consistency window

FULL-mode compound coverage:

- applies when `storage_truth_enforcement_mode == FULL`
- for every assigned target, the report must include exactly one `RECENT` result and exactly one `OLD` result
- duplicate `RECENT` entries for a target are rejected
- duplicate `OLD` entries for a target are rejected
- missing either bucket for any assigned target is rejected

## Score Model

Code: `x/audit/v1/keeper/storage_truth_scoring.go`

Storage-truth scores update only in `SHADOW`, `SOFT`, or `FULL`.

State scores are clamped at zero after applying deltas:

- node suspicion cannot go below `0`
- reporter reliability cannot go below `0`
- ticket deterioration cannot go below `0`

### Base Result Deltas

For `PASS`:

- `RECENT`: node `-3`, reporter `-4`, ticket `-2`
- `OLD`: node `-2`, reporter `-4`, ticket `-3`
- other buckets: node `-2`, reporter `-4`, ticket `-2`

For `HASH_MISMATCH` on `INDEX`:

- node `+26`
- reporter `+1`
- ticket `+12`

For `HASH_MISMATCH` on `SYMBOL` or unspecified artifact fallback:

- node `+18`
- reporter `+1`
- ticket `+5`

For `TIMEOUT_OR_NO_RESPONSE`:

- node `+7`
- reporter `-1`
- ticket `+3`

For `OBSERVER_QUORUM_FAIL`:

- node `+4` (LEP6.md §14:405)
- reporter `0`
- ticket `0`

For `NO_ELIGIBLE_TICKET`:

- node `0`
- reporter `0`
- ticket `0`

For `INVALID_TRANSCRIPT`:

- node `0`
- reporter `0`
- ticket `0`

For `RECHECK_CONFIRMED_FAIL`:

- node `+15`
- reporter `+3`
- ticket `+8`

### Reporter Trust Scaling

Before node and ticket deltas are applied, provisional failure node/ticket deltas are scaled by reporter trust:

```text
multiplier_numerator = max(50, 100 - reporter_reliability_score)
scaled_delta = delta * multiplier_numerator / 100
```

Examples:

- `R = 0` gives `100%`
- `R = 20` gives `80%`
- `R = 50` gives `50%`
- `R >= 50` remains floored at `50%`

Reporter reliability deltas are not scaled by this multiplier.

Scaling scope:

- trust scaling applies only to failure classes
- trust scaling does not apply to `RECHECK_CONFIRMED_FAIL`
- trust scaling does not apply to bucket `RECHECK`
- pass deltas are not trust-scaled

### Pattern Escalation

Pattern escalation is evaluated over `storage_truth_pattern_escalation_window`, default `14` epochs.

Distinct failed tickets for the same target:

- first distinct failed ticket: `+0` node bonus
- second distinct failed ticket: `+10` node bonus
- third or more distinct failed tickets: `+15` node bonus

Cross-bucket pattern:

- if both RECENT and OLD failures exist for a target inside the pattern window, add `+12` node bonus

Ticket holder pattern:

- same ticket fails on a different holder in a later epoch: `+10` ticket bonus
- same ticket fails again on the same holder in a later epoch: `+6` ticket bonus

Contradiction pattern:

- contradiction handling is evaluated for a later `PASS` against an earlier failure on the same `ticket_id` and target
- contradiction penalties apply only after confirmation:
  - at least one independent reporter `PASS` in the rolling `storage_truth_contradiction_window_epochs` window (default `7`; the current `PASS` is the second distinct pass), or
  - a clean recheck `PASS` in the same window
- when confirmed, current reporter receives `-4` recovery/credit
- when confirmed and prior reporter is different, prior reporter receives `+12`
- contradiction count increments for the affected reporter/ticket state

### NodeSuspicionState Fields

`NodeSuspicionState` stores:

- `supernode_account`
- `suspicion_score`
- `last_updated_epoch`
- `last_recent_fail_epoch`
- `last_old_fail_epoch`
- `distinct_ticket_fail_window`
- `window_start_epoch`
- `class_a_count_window`
- `last_class_a_epoch`
- `class_b_count_window`
- `last_class_b_epoch`
- `clean_pass_count`
- `last_clean_pass_epoch`
- `last_index_fail_epoch`

History updates:

- `PASS` increments `clean_pass_count` and sets `last_clean_pass_epoch`
- RECENT failures set `last_recent_fail_epoch`
- OLD failures set `last_old_fail_epoch`
- INDEX failures set `last_index_fail_epoch`
- Class A window counts track `HASH_MISMATCH`, `RECHECK_CONFIRMED_FAIL`, and index failures
- Class B window counts track `TIMEOUT_OR_NO_RESPONSE`
- Class A failures reset `clean_pass_count` so recovery requires a clean streak after the latest Class A

### ReporterReliabilityState Fields

`ReporterReliabilityState` stores:

- `reporter_supernode_account`
- `reliability_score`
- `last_updated_epoch`
- `trust_band`
- `contradiction_count`
- `ineligible_until_epoch`
- `window_positive_count`
- `window_negative_count`
- `window_start_epoch`

Window behavior:

- divergence window defaults to `14` epochs
- positive reporter deltas increment `window_negative_count`
- negative reporter deltas increment `window_positive_count`

### TicketDeteriorationState Fields

`TicketDeteriorationState` stores:

- `ticket_id`
- `deterioration_score`
- `last_updated_epoch`
- `active_heal_op_id`
- `probation_until_epoch`
- `last_heal_epoch`
- `last_failure_epoch`
- `recent_failure_epoch_count`
- `contradiction_count`
- `last_target_supernode_account`
- `last_reporter_supernode_account`
- `last_result_class`
- `last_result_epoch`
- `distinct_holder_failure_count`
- `last_index_failure_epoch`
- `recent_bucket_failure_epoch`
- `old_bucket_failure_epoch`

Recent failure epoch count:

- starts at `1` for the first failure
- increments when a new failure epoch occurs within `storage_truth_pattern_escalation_window`
- resets to `1` if the next failure is outside that window
- has a minimum effective window of `2` epochs

## Fact Indexes

Code: `x/audit/v1/keeper/storage_truth_fact_indexes.go`

The implementation stores auxiliary fact indexes so enforcement and divergence can evaluate rolling predicates without changing public state messages.

### Storage Proof Transcript Index

Key:

```text
"st/spt/" + transcript_hash
```

Record fields:

- `epoch_id`
- `ticket_id`
- `target_account`
- `reporter_account`
- `bucket_type`
- `result_class`
- `artifact_class`
- `recheck_eligible`

Purpose:

- links recheck evidence to an actual submitted routine proof result
- validates epoch, ticket, target, reporter independence, and recheck eligibility

### Node Failure Fact Index

Key:

```text
"st/nf/" + supernode_account + "/" + u64be(epoch_id) + "/" + ticket_id + 0x00 + reporter_account
```

Record fields:

- `epoch_id`
- `ticket_id`
- `reporter_account`
- `bucket_type`
- `result_class`
- `artifact_class`

Purpose:

- exact Class A/Class B window predicates
- exact old Class A distinct-ticket predicate
- index-failure predicate
- repeated-ticket and cross-bucket escalation support

### Reporter Result Fact Index

Key:

```text
"st/rrs/" + reporter_account + "/" + u64be(epoch_id) + "/" + ticket_id + 0x00 + target_account
```

Record fields:

- `epoch_id`
- `ticket_id`
- `target_account`
- `result_class`
- `confirmed_by_recheck`
- `overturned_by_recheck`

Purpose:

- statistical divergence scoring
- avoids penalizing reporters whose negative results are consistently confirmed by recheck

### Failed Heal Index

Key:

```text
"st/fh/" + supernode_account + "/" + u64be(epoch_id) + "/" + ticket_id
```

Purpose:

- failed heal verification can satisfy the strong-postpone predicate for the assigned healer.

## Enforcement

Code:

- `x/audit/v1/keeper/abci.go`
- `x/audit/v1/keeper/enforcement.go`
- `x/audit/v1/keeper/storage_truth_postponement_state.go`

At epoch end, audit execution runs storage-truth logic in this order:

1. `EnforceEpochEnd`
2. `ApplyReporterDivergenceAtEpochEnd`
3. `ProcessStorageTruthHealOpsAtEpochEnd`
4. pruning

Storage-truth postpone reason:

```text
audit_storage_truth_suspicion
```

Band behavior:

- all modes except `UNSPECIFIED` can emit band events
- `SHADOW` emits band events but does not postpone
- `SOFT` and `FULL` can postpone
- active supernodes already postponed by storage truth skip legacy postpone checks for that epoch
- postponed supernodes can recover through the storage-truth recovery gate

Postpone candidate predicates require the score to be in the postpone band and at least one predicate:

- one RECENT Class A fault plus any second failure in `14` epochs
- two OLD Class A faults on distinct tickets in `21` epochs
- four Class B faults in `7` epochs

Strong-postpone predicates require the score to be in the strong-postpone band and at least one predicate:

- two Class A faults on distinct tickets in `14` epochs
- any index failure in `14` epochs, or the node's `last_index_fail_epoch` is set
- failed heal verification for the node in `14` epochs

Class definitions:

- Class A: `HASH_MISMATCH`, `RECHECK_CONFIRMED_FAIL`, or any `INDEX` artifact failure
- Class B: `TIMEOUT_OR_NO_RESPONSE`

Recovery:

- decayed suspicion score must be below watch threshold
- if watch threshold is not positive, effective watch threshold is `1`
- clean pass count must be at least `storage_truth_recovery_clean_pass_count`, default `3`
- latest clean pass epoch must be after latest Class A epoch when Class A history exists
- if no node suspicion state exists, recovery is allowed

## Reporter Divergence

Code: `x/audit/v1/keeper/storage_truth_divergence.go`

Reporter divergence runs at epoch end.

Defaults:

- window: `storage_truth_divergence_window_epochs = 14`
- minimum reports: `storage_truth_reporter_min_reports_for_divergence = 5`
- penalty: `+8` reporter reliability

Algorithm:

1. For each reporter reliability state, load reporter result facts from the `st/rrs/` fact index in the divergence window (no stale window-counter fallback — D14 fix at `5df4206`).
2. Skip reporters with total reports below minimum.
3. Compute `negative_rate = negative_count / total_count` using integer cross-multiplication (`neg * medTotal <= 2 * medNeg * tot`; no float64 — D15 fix at `5df4206`).
4. Compute the median negative rate across qualifying reporters.
5. Penalize a reporter if `negative_rate > 2 * median_negative_rate`.
6. Skip the penalty if at least half of the reporter's negative results were confirmed by recheck.

The emitted score event includes:

- `divergence_penalty = 8`
- `reporter_neg_rate`
- `median_neg_rate`

## Recheck Evidence

Code:

- `proto/lumera/audit/v1/tx.proto`
- `x/audit/v1/keeper/msg_storage_truth.go`
- `x/audit/v1/keeper/storage_truth_recheck_state.go`
- `x/audit/v1/keeper/storage_truth_fact_indexes.go`

`MsgSubmitStorageRecheckEvidence` fields:

- `creator = 1`
- `epoch_id = 2`
- `challenged_supernode_account = 3`
- `ticket_id = 4`
- `challenged_result_transcript_hash = 5`
- `recheck_transcript_hash = 6`
- `recheck_result_class = 7`
- `details = 8`

Validation:

- request cannot be nil
- creator is required
- challenged supernode is required
- challenged supernode cannot equal creator
- ticket id is required
- challenged result transcript hash is required
- recheck transcript hash is required
- epoch anchor must exist for the challenged epoch
- creator must be a registered supernode
- challenged supernode must be registered
- recheck result class must be one of `PASS`, `HASH_MISMATCH`, `TIMEOUT_OR_NO_RESPONSE`, `OBSERVER_QUORUM_FAIL`, `INVALID_TRANSCRIPT`, `RECHECK_CONFIRMED_FAIL`
- challenged transcript hash must exist in the proof transcript index
- challenged transcript epoch must match request epoch
- challenged transcript ticket must match request ticket
- challenged transcript target must match challenged supernode
- creator must be independent from the challenged result reporter
- challenged result class must be recheck-eligible
- replay key `(epoch_id, ticket_id, creator)` must not already exist
- challenged transcript is linked to `recheck_transcript_hash`, and the recheck transcript hash is indexed with a back-reference to the challenged transcript hash

Replay protection key:

```text
"st/rce/" + u64be(epoch_id) + "/" + ticket_id + 0x00 + creator
```

Scoring:

- recheck creates a synthetic `StorageProofResult` in bucket `RECHECK`
- `RECHECK_CONFIRMED_FAIL` applies normal recheck-confirmed node/ticket deltas
- if recheck result is `PASS`, original reporter receives `+25`
- if recheck result is `RECHECK_CONFIRMED_FAIL`, original reporter receives `-3`
- affected reporter result facts are marked as confirmed or overturned

## Self-Healing

Code:

- `x/audit/v1/keeper/storage_truth_heal_ops.go`
- `x/audit/v1/keeper/msg_storage_truth.go`

### HealOp Status Values

`HealOpStatus`:

- `UNSPECIFIED = 0`
- `SCHEDULED = 1`
- `IN_PROGRESS = 2`
- `HEALER_REPORTED = 3`
- `VERIFIED = 4`
- `FAILED = 5`
- `EXPIRED = 6`

Final statuses:

- `VERIFIED`
- `FAILED`
- `EXPIRED`

### Scheduling

Scheduling runs at epoch end through `ProcessStorageTruthHealOpsAtEpochEnd`.

Before scheduling, non-final heal ops expire when:

```text
deadline_epoch_id != 0 && deadline_epoch_id <= current_epoch
```

Scheduling is skipped when:

- enforcement mode is `UNSPECIFIED`
- max heal ops per epoch is `0`
- there are no active scheduler accounts
- ticket is in probation
- ticket has a non-final active heal op
- ticket has any other non-final open heal op
- no independent verifier can be assigned

Candidate requirements:

- `deterioration_score >= storage_truth_ticket_deterioration_heal_threshold`, default `50`
- and one of:
  - `distinct_holder_failure_count >= 2`
  - `last_index_failure_epoch > 0`
  - `recent_failure_epoch_count >= 2`

Candidate priority:

1. higher deterioration score
2. index failure first
3. higher distinct holder failure count
4. lower last failure epoch, meaning oldest unresolved failure first
5. lexicographically smaller ticket id

The scheduler creates at most `storage_truth_max_self_heal_ops_per_epoch`, default `5`.

### Healer And Verifier Assignment

Active scheduler accounts come from:

1. epoch anchor active accounts, if present
2. otherwise current active supernodes from `x/supernode`

Participant assignment:

- deterministic index uses FNV-1a 64-bit over `ticket_id || 0x00 || decimal(epoch_id)`
- healer is `active_accounts[index % len(active_accounts)]`
- verifier count is `2`, capped to `len(active_accounts) - 1`
- verifiers are the next accounts after the healer, wrapping around
- one active account gives no verifier and the candidate is skipped

### HealOp Fields

`HealOp` stores:

- `heal_op_id`
- `ticket_id`
- `scheduled_epoch_id`
- `healer_supernode_account`
- `verifier_supernode_accounts`
- `status`
- `created_height`
- `updated_height`
- `deadline_epoch_id`
- `result_hash`
- `notes`

Scheduled heal op values:

- status: `SCHEDULED`
- created height: current block height
- updated height: current block height
- deadline: `current_epoch + storage_truth_heal_deadline_epochs`, default `+3`
- ticket state `active_heal_op_id` is set to the new heal op id
- `next_heal_op_id` increments by `1`

### ClaimHealComplete

`MsgClaimHealComplete` fields:

- `creator = 1`
- `heal_op_id = 2`
- `ticket_id = 3`
- `heal_manifest_hash = 4`
- `details = 5`

Validation:

- creator is required
- heal op id is required
- ticket id is required
- heal manifest hash is required
- heal op must exist
- ticket id must match heal op ticket id
- creator must be assigned healer
- status must be `SCHEDULED` or `IN_PROGRESS`
- verifier set cannot be empty

State transition:

- status becomes `HEALER_REPORTED`
- `updated_height` becomes current block height
- `result_hash` becomes `heal_manifest_hash`
- details are appended to notes with separator `" | "` when notes already exist

### SubmitHealVerification

`MsgSubmitHealVerification` fields:

- `creator = 1`
- `heal_op_id = 2`
- `verified = 3`
- `verification_hash = 4`
- `details = 5`

Validation:

- creator is required
- heal op id is required
- verification hash is required
- heal op must exist
- heal op status must be `HEALER_REPORTED`
- creator must be assigned verifier
- verifier can submit only once per heal op

Majority rule:

```text
majority = len(verifier_supernode_accounts) / 2 + 1
```

Finalization:

- if negative votes reach majority, status becomes `FAILED`
- if positive votes reach majority, status becomes `VERIFIED`
- otherwise votes are accumulated and no final transition occurs

Post-finalization ticket handling:

- if active heal op id matches finalized op, clear `active_heal_op_id`
- verified heal sets `D = max(8, floor(D_old * 0.25))`
- verified heal sets `last_heal_epoch = current_epoch`
- verified heal sets `probation_until_epoch = current_epoch + storage_truth_probation_epochs`
- failed heal applies `D += 15`
- failed heal extends `probation_until_epoch` to at least `current_epoch + storage_truth_probation_epochs`
- failed heal records a failed-heal fact for the healer

## Queries

Code:

- `proto/lumera/audit/v1/query.proto`
- `x/audit/v1/keeper/query_storage_truth.go`
- `x/audit/v1/module/autocli.go`

Storage-truth gRPC/REST queries:

- `NodeSuspicionState`
  - REST: `/LumeraProtocol/lumera/audit/v1/node_suspicion_state/{supernode_account}`
- `ReporterReliabilityState`
  - REST: `/LumeraProtocol/lumera/audit/v1/reporter_reliability_state/{reporter_supernode_account}`
- `TicketDeteriorationState`
  - REST: `/LumeraProtocol/lumera/audit/v1/ticket_deterioration_state/{ticket_id}`
- `HealOp`
  - REST: `/LumeraProtocol/lumera/audit/v1/heal_op/{heal_op_id}`
- `HealOpsByTicket`
  - REST: `/LumeraProtocol/lumera/audit/v1/heal_ops/by_ticket/{ticket_id}`
- `HealOpsByStatus`
  - REST: `/LumeraProtocol/lumera/audit/v1/heal_ops/by_status/{status}`

Existing assignment/report queries were also made storage-truth-aware:

- `AssignedTargets` reflects storage-truth target assignment when mode is enabled
- `EpochReport` includes `storage_proof_results`
- report listing returns the extended report structure

## Genesis

Code:

- `proto/lumera/audit/v1/genesis.proto`
- `x/audit/v1/keeper/genesis.go`
- `x/audit/v1/types/genesis.go`

Genesis now includes:

- `params = 1`
- `evidence = 2`
- `next_evidence_id = 3`
- `node_suspicion_states = 4`
- `reporter_reliability_states = 5`
- `ticket_deterioration_states = 6`
- `heal_ops = 7`
- `next_heal_op_id = 8`
- `ticket_artifact_count_states = 9`
- `postponed_supernodes = 10`

Import/export handles all storage-truth score states, heal ops, canonical artifact-count states, storage-truth postponement markers, and the next heal-op id.

### Genesis import-export contract

- `ValidateScoreStatesGenesis` hard-errors on malformed score states: negative scores or `LastUpdatedEpoch > currentEpoch` cause a hard error at chain start (`x/audit/v1/types/genesis_validate.go`; called in `keeper/genesis.go:34`). This prevents silent corruption from invalid exports.
- `StorageTruthPostponements` (field 10) round-trips through `ExportGenesis` / `InitGenesis`: markers are collected via prefix scan on export and re-applied via `setStorageTruthPostponedAtEpochID` per entry on import (`keeper/genesis.go:96-98` import, `genesis.go:159` export).
- `TicketArtifactCountStates` (field 9) are exported and imported verbatim; counts are immutable once anchored at cascade finalization and must not be mutated by genesis import.

## Params Governance

Code:

- `x/audit/v1/types/params.go`
- `x/audit/v1/keeper/msg_update_params.go`

LEP-6 params are part of audit `Params`, registered in the module param key table, and can be updated through `MsgUpdateParams` subject to the existing authority and immutable-field checks.

Storage-truth param keys:

- `StorageTruthRecentBucketMaxBlocks`
- `StorageTruthOldBucketMinBlocks`
- `StorageTruthChallengeTargetDivisor`
- `StorageTruthCompoundRangesPerArtifact`
- `StorageTruthCompoundRangeLenBytes`
- `StorageTruthMaxSelfHealOpsPerEpoch`
- `StorageTruthProbationEpochs`
- `StorageTruthNodeSuspicionDecayPerEpoch`
- `StorageTruthReporterReliabilityDecayPerEpoch`
- `StorageTruthTicketDeteriorationDecayPerEpoch`
- `StorageTruthNodeSuspicionThresholdWatch`
- `StorageTruthNodeSuspicionThresholdProbation`
- `StorageTruthNodeSuspicionThresholdPostpone`
- `StorageTruthReporterReliabilityLowTrustThreshold`
- `StorageTruthReporterReliabilityIneligibleThreshold`
- `StorageTruthTicketDeteriorationHealThreshold`
- `StorageTruthEnforcementMode`
- `StorageTruthReporterReliabilityDegradedThreshold`
- `StorageTruthPatternEscalationWindow`
- `StorageTruthDivergenceWindowEpochs`
- `StorageTruthReporterMinReportsForDivergence`
- `StorageTruthNodeSuspicionThresholdStrongPostpone`
- `StorageTruthRecoveryCleanPassCount`
- `StorageTruthClassAFaultWindow`
- `StorageTruthClassBFaultWindow`
- `StorageTruthHealDeadlineEpochs`
- `StorageTruthOldClassAFaultWindow`
- `StorageTruthContradictionWindowEpochs`
- `StorageTruthReporterIneligibleDurationEpochs`

## Release Callouts And Activation Plan

This section lists behavior-impacting callouts for production rollout and the required activation order.

### Critical Callouts

- `x/action` is already live; LEP-6 is not yet released. This is supported, but activation must be staged.
- Storage-proof validation now requires canonical per-ticket artifact counts for all non-`NO_ELIGIBLE_TICKET` results.
- Canonical counts are immutable once anchored; incorrect anchors become persistent data issues for that ticket.
- Existing finalized cascade tickets from before LEP-6 may not have anchored artifact counts in audit state.
- If LEP-6 report ingestion is active before historical backfill, proofs for those tickets can be rejected due to missing canonical counts.
- `FULL` mode introduces strict RECENT/OLD per-target proof coverage. Enabling it before reporter fleet readiness can cause report rejection and operational instability.

### Non-Breaking Guardrails

- Keep `storage_truth_enforcement_mode = UNSPECIFIED` during binary rollout to avoid behavior changes while data readiness is validated.
- Enable LEP-6 modes only after data and client readiness gates are complete.
- Treat `FULL` as the final stage only after successful `SHADOW` and `SOFT` observation windows.

### Mandatory Pre-Activation Data Plan

Before enabling LEP-6 enforcement behavior (`SHADOW`, `SOFT`, or `FULL`) on a chain with historical tickets:

- Run a one-time backfill/migration to seed `TicketArtifactCountState` for finalized cascade tickets that do not yet have canonical counts.
- Backfill source of truth is finalized cascade metadata:
  - use explicit `index_artifact_count` / `symbol_artifact_count` when present
  - for legacy finalized payloads, derive deterministic fallback from finalized symbol IDs where applicable
- Reject/flag tickets where deterministic counts cannot be derived safely; do not silently guess.
- Produce an audit report of:
  - total finalized cascade tickets
  - total already anchored
  - total newly backfilled
  - total unresolved/excluded (must be zero before activation)

### Staged Activation Sequence

1. Binary rollout: deploy LEP-6 code while mode is pinned to `UNSPECIFIED`.
2. Data migration: complete artifact-count backfill and verify unresolved count is zero.
3. Client readiness: ensure supernode/reporter version compatibility for LEP-6 proof fields and recheck/heal tx flow.
4. Shadow phase: switch to `SHADOW`, monitor score state/events and report acceptance.
5. Soft phase: switch to `SOFT` after stable shadow window and predicate sanity checks.
6. Full phase: switch to `FULL` only after sustained proof completeness and stable reporter operations.

### Go/No-Go Checks Per Stage

- No-go for `SHADOW` and above:
  - missing canonical artifact counts for any ticket likely to be challenged
  - unresolved backfill exceptions
- No-go for `SOFT`:
  - unstable reporter participation
  - unexpected spikes in rejected reports
- No-go for `FULL`:
  - incomplete RECENT/OLD proof coverage by eligible reporters
  - frequent operational fallbacks or manual intervention

## Production Hardening

Implementation-level invariants preserved at commit `5df4206`:

- **Per-report cap:** `MaxStorageProofResultsPerReport = 16` (`x/audit/v1/types/keys.go:13`) is enforced at `msg_submit_epoch_report.go:126`; reports exceeding 16 storage-proof results are rejected before any scoring occurs. This is a constant, not a governance param.
- **Dedup key excludes `ArtifactKey`:** `storageProofDescriptorKey` uses the 5-tuple `(target, bucket, ticket_id, artifact_class, artifact_ordinal)`. Excluding `artifact_key` prevents dedup bypass via attacker-supplied alternate key values (LEP-6 §10; `msg_submit_epoch_report_storage_proofs.go`).
- **Heal-op pruning lifecycle:** terminal heal-ops (`VERIFIED`, `FAILED`, `EXPIRED`) are pruned at epoch end via `pruneTerminalHealOps` in `PruneOldEpochs` (`prune.go:83`), removing status-index entries, verification sub-keys, and fact-index entries after `KeepLastEpochEntries` epochs.
- **Overflow-safe trust scaling:** `scaleInt64TowardZero` uses `math/big` arithmetic (`storage_truth_scoring.go:661`) to prevent int64 overflow when multiplying large scores by the reporter-trust numerator before dividing by 100.
- **Heal verification hash pin:** `SubmitHealVerification` requires `req.VerificationHash == healOp.ResultHash` for positive (`Verified == true`) attestations (`msg_storage_truth.go`), ensuring verifiers attest to the specific manifest the healer committed to in `ClaimHealComplete`.

## File Map

Core implementation files:

- `x/audit/v1/keeper/audit_peer_assignment.go`: deterministic reporter-target assignment and challenger eligibility
- `x/audit/v1/keeper/msg_submit_epoch_report.go`: report validation, assignment checks, scoring invocation
- `x/audit/v1/keeper/msg_submit_epoch_report_storage_proofs.go`: storage proof result shape and FULL-mode coverage validation
- `x/audit/v1/keeper/storage_truth_ticket_artifact_counts.go`: canonical ticket artifact count anchoring and immutability
- `x/audit/v1/keeper/storage_truth_scoring.go`: node/reporter/ticket score deltas, decay, trust scaling, pattern handling
- `x/audit/v1/keeper/storage_truth_fact_indexes.go`: transcript, node-failure, reporter-result, and failed-heal indexes
- `x/audit/v1/keeper/storage_truth_divergence.go`: reporter outlier detection and penalty
- `x/audit/v1/keeper/enforcement.go`: band mapping, storage-truth postponement, and recovery gates
- `x/audit/v1/keeper/storage_truth_heal_ops.go`: heal-op expiry, scheduling, priority, and participant assignment
- `x/audit/v1/keeper/msg_storage_truth.go`: recheck evidence, healer claim, verifier vote, heal finalization
- `x/audit/v1/keeper/storage_truth_state.go`: score and heal-op state accessors
- `x/audit/v1/keeper/storage_truth_recheck_state.go`: recheck replay protection
- `x/audit/v1/keeper/storage_truth_postponement_state.go`: storage-truth postponed marker
- `x/audit/v1/keeper/query_storage_truth.go`: query handlers
- `x/audit/v1/keeper/genesis.go`: genesis import/export
- `x/audit/v1/keeper/abci.go`: epoch-end wiring
- `x/action/v1/keeper/action.go`: cascade finalization hook that anchors canonical per-ticket artifact counts in audit state

Types, params, events, module integration:

- `x/audit/v1/types/params.go`
- `x/audit/v1/types/keys.go`
- `x/audit/v1/types/events.go`
- `x/audit/v1/module/autocli.go`
- `x/audit/v1/simulation/storage_truth.go`

Focused tests:

- `x/audit/v1/keeper/audit_peer_assignment_test.go`
- `x/audit/v1/keeper/msg_submit_epoch_report_test.go`
- `x/audit/v1/keeper/msg_submit_epoch_report_storage_truth_scores_test.go`
- `x/audit/v1/keeper/msg_storage_truth_test.go`
- `x/audit/v1/keeper/storage_truth_activation_test.go`
- `x/audit/v1/keeper/enforcement_predicates_test.go`
- `x/audit/v1/keeper/storage_truth_divergence_test.go`
- `x/audit/v1/keeper/storage_truth_scoring_internal_test.go`
- `x/audit/v1/keeper/storage_truth_state_test.go`
- `x/audit/v1/keeper/query_storage_truth_test.go`
- `tests/integration/audit/keeper_test.go`
- `tests/system/audit/msg_storage_truth_test.go`
- `tests/systemtests/audit_storage_truth_activation_test.go`
- `tests/systemtests/audit_storage_truth_edge_cases_test.go`

## Verification

Last verified at commit `16a838f` (`LEP-6-consensus-gap-fixes-rebase`) — full test pyramid green: unit + simulation + integration + system + e2e systemtests (25/25 PASS).

```bash
/home/openclaw/.local/go/bin/go test ./x/audit/v1/...
/home/openclaw/.local/go/bin/go test ./x/supernode/v1/...
/home/openclaw/.local/go/bin/go test ./tests/integration/audit/...
/home/openclaw/.local/go/bin/go test ./tests/system/audit/...
```

Results at `16a838f`:

- `./x/audit/v1/...` passed
- `./x/supernode/v1/...` passed
- `./tests/integration/...` passed (9/9)
- `./tests/system/...` (`-tags=system`) passed (4/4)
- `./tests/systemtests/...` (`-tags=system_test`) **passed (25/25, 0 fail, 0 skip)**

For manual/devnet validation, rebuild the binary from this branch:

```bash
/home/openclaw/.local/go/bin/go build -o build/lumerad ./cmd/lumera/
```

The local tagged systemtest framework resolves `build/lumerad`.

## Implementation Alignment

The implemented `lumera` code captures the LEP-6 business rules needed for on-chain storage-truth:

- deterministic one-third target coverage using `storage_truth_challenge_target_divisor`
- RECENT/OLD compound evidence in FULL mode
- strict proof shape validation and transcript indexing
- strict canonical per-ticket artifact-count anchoring for deterministic artifact ordinal checks
- node suspicion scoring with Class A/Class B windows and pattern escalation
- reporter reliability with positive-penalty trust bands and provisional-failure trust scaling
- challenger ineligibility from reporter reliability
- ticket deterioration with holder-diversity, index-failure, and repeated-failure predicates
- storage-truth enforcement bands and recovery gates
- recheck evidence replay protection, transcript-linked scoring, and contradiction confirmation hooks
- deterministic heal scheduling, deadline expiry, majority verification, post-heal reset, and probation
- query, genesis, events, params, AutoCLI, simulation, and tests for the new state


---

## Operator Notes

### Gas requirements for `MsgSubmitStorageRecheckEvidence`

Post CP3.5 F-B, every recheck evidence submission writes two secondary index
entries (`st/rrs-tt/...` and `st/spt-tbe/...`) in addition to the existing
state. Empirical gas usage on a 5-validator local devnet is ~204k–220k per
recheck tx, **above the Cosmos default `gas_limit=200000`**.

Validators / supernodes submitting recheck evidence MUST set:

```
--gas auto --gas-adjustment 1.3
# or explicitly
--gas 500000
```

Consequences if not set:
- Tx fails with `out of gas in location: WriteFlat` / `WritePerByte`
- Reporter loses the fee, recheck evidence is not recorded
- Postponed targets cannot be challenged again until the next epoch

This is a deliberate trade-off — the secondary indexes eliminate full
transcript-prefix scans inside `DeliverTx` (122-Copilot-3/4/5), which
previously made `SubmitEpochReport` an O(N) DoS vector. Cost is paid by
the recheck submitter, not by every block proposer.

### `StorageTruthEnforcementMode` activation

The default mode is `STORAGE_TRUTH_ENFORCEMENT_MODE_SHADOW` — scoring runs but
no postponements or heal-ops are scheduled. Flipping to `SOFT` or `FULL`
**makes the new scoring consensus-binding**. This transition must be a
governance proposal with a flag-day epoch boundary, not an ad-hoc upgrade.

Mode semantics:
- `UNSPECIFIED` — k-based peer-assignment formula; pre-LEP-6 behaviour
- `SHADOW` — one-third coverage assignment, score state evolves, no enforcement
- `SOFT` — score state evolves, postponements emitted, heal-ops scheduled
- `FULL` — `SOFT` + RECENT/OLD compound evidence required per assignment

---

## LEP-6 Round-2 Review Hardening (Zee R2 — PR #117 review 4184561676)

Behavior deltas applied on top of `LEP-6-foundation` tip `868cbc7c` to close
24 production-gate findings. Branch `LEP-6-foundation-review-fixes` (squashed
single commit `0c6f5f0`). Test pyramid green at `0c6f5f0`: unit + module
simulation + `tests/integration/` + `tests/system/` + `tests/systemtests/`
(`-tags=system_test`, 25/25 PASS). Compare:
[`LEP-6-foundation...LEP-6-foundation-review-fixes`](https://github.com/LumeraProtocol/lumera/compare/LEP-6-foundation...LEP-6-foundation-review-fixes).

### HIGH (consensus / state-correctness / money-flow)

- **NEW-C-3 — `RegistrationFeeShareBps` (2%) fee routing restored.**
  `x/action/v1/keeper/action.go:650-680`. The reward-distribution block
  removed during LEP-6 consolidation is reinstated; 2% of every cascade fee
  routes to the supernode reward pool via
  `bankKeeper.SendCoinsFromModuleToModule(actiontypes → sntypes)`. Test
  coverage in `x/action/v1/keeper/distribute_fees_test.go`.
- **NEW-C-1 — `ExportGenesis` round-trip closure for 8 epoch-scoped audit
  prefix families.** `x/audit/v1/keeper/genesis.go`. Recheck-evidence dedup
  (`st/rce/`), node-failure facts (`st/nf/`), reporter-result indexes
  (`st/rrs/`, `st/rrs-tt/`), storage-proof transcripts (`st/spt/`,
  `st/spt-tbe/`), failed-heal markers (`st/fh/`), reports / report-indexes
  (`r/`, `ri/`), healer reports (`hr/`), and storage challenges (`sc/`) now
  export and re-import deterministically. Without this, post-state-sync the
  postpone/recovery/contradiction predicates would silently return "no
  evidence" and brick storage-truth enforcement. 8 new `GenesisXxx` proto
  wrappers + `GetAllXxx` iterators + `InitGenesis` re-emission.
- **NEW-A-12 / A-17 — `WindowStartEpoch` underflow at scoring window
  resets.** `x/audit/v1/keeper/storage_truth_scoring.go:256, 332` +
  `x/audit/v1/types/genesis_validate.go`. Raw `uint64` subtraction replaced
  with `epochDelta(currentEpoch, state.WindowStartEpoch)` (uint64-safe);
  genesis validator now rejects `WindowStartEpoch > currentEpoch` for both
  `NodeSuspicionStates` and `ReporterReliabilityStates`. Spec ref §14, §15.
- **NEW-B-1 — EXPIRED heal-ops apply §20 cooldown.**
  `x/audit/v1/keeper/storage_truth_heal_ops.go:36-48`
  (`expireStorageTruthHealOpsAtEpochEnd`). Mirrors the FAILED branch:
  `D += 15` (saturated), `ProbationUntilEpoch = epochID +
  StorageTruthProbationEpochs`, write failed-heal fact. Closes the
  heal-spam loop where an assigned-but-silent healer would have the same
  ticket re-scheduled every epoch indefinitely.

### MEDIUM (semantic correctness / sibling-symmetry)

- **NEW-C-2 — FinalizeAction audit-hook artifact-count fallback.**
  `x/action/v1/keeper/action.go:245-261`. Mirrors the 122-F2 fix inline
  before calling `auditKeeper.SetStorageTruthTicketArtifactCounts`: zero
  counts fall back to `len(RqIdsIds)`. Prevents a hard-revert on legacy
  finalize when cascade metadata lacks explicit counts.
- **NEW-A-11 residue — bounded prefix scans on hot paths.**
  `x/audit/v1/keeper/storage_truth_divergence.go:131-153` and
  `x/audit/v1/keeper/storage_truth_fact_indexes.go:241-266`. Both unbounded
  scans now bracket on `[startEpoch, endEpoch+1)` via secondary indexes
  (`NodeStorageTruthFailureByEpochPrefix(account)+u64be(epoch)`,
  `ReporterStorageTruthResultByEpochPrefix(reporter)+u64be(epoch)`),
  mirroring the storage-proofs.go:318-350 pattern.
- **NEW-A-13 — `big.Int` divergence cross-multiply.**
  `x/audit/v1/keeper/storage_truth_divergence.go:65-67, 89, 92`. Replaces
  raw `uint64 * uint64` (wrap-around at scale) with `*big.Int` arithmetic
  in median ordering and outlier predicate.
- **NEW-A-14 / A-15 — trust-multiplier scope narrowed to Class A.**
  `x/audit/v1/keeper/storage_truth_scoring.go:73-81, 539-541`. Predicate
  now requires `(ResultClass == HASH_MISMATCH || ArtifactClass == INDEX)`
  in addition to the existing `RECHECK_CONFIRMED_FAIL` and
  `BUCKET_TYPE_RECHECK` exclusions. Spec §15.4.
- **NEW-A-18 — single PASS reliability delta per epoch.**
  `x/audit/v1/keeper/storage_truth_scoring.go:444, 450, 456` +
  `ApplyReporterCleanEpochRecoveryAtEpochEnd`. Per-result PASS / TIMEOUT
  reliability deltas zeroed; new epoch-end pass applies a single `-4`
  reliability delta when `(passes_in_epoch >= 5 && no_overturned_fails)`.
  Spec §15.3.
- **NEW-B-2 — decay-adjusted healer eligibility.**
  `x/audit/v1/keeper/storage_truth_heal_ops.go:105-114`. Read uses
  `decayTowardZero(ss.SuspicionScore, params.StorageTruthNodeSuspicionDecayPerEpoch,
  epochDelta(epochID, ss.LastUpdatedEpoch))` for sibling-symmetry with
  `enforcement.go:119`.
- **NEW-B-8 — verified-heal failure-pattern reset.**
  `x/audit/v1/keeper/msg_storage_truth.go:395-404`. Verified-branch now
  zeroes `DistinctHolderFailureCount`, `RecentFailureEpochCount`,
  `LastIndexFailureEpoch`, `LastFailureEpoch` so the §20 "fresh start"
  semantic is preserved post-heal.
- **F121-F12 — distinct strong-postpone reason + recovery param.**
  `x/audit/v1/keeper/keys.go` + `enforcement.go` + `params.go`. Strong
  band writes its own marker (`ap/sts/...`); recovery clean-pass count
  reads from new param `StorageTruthStrongRecoveryCleanPassCount`
  (default 5, gov-tunable). Recovery clears the strong marker explicitly.
- **F121-F10 / F119-F3 — ticket-side `ContradictionCount` confirmation
  guard.** `x/audit/v1/keeper/storage_truth_scoring.go:418-422`. A
  `contradictionConfirmed` bool now propagates from the reporter-side
  bookkeeping; the ticket-side bump only fires on `prevFailure &&
  currentPass && contradictionConfirmed`. Closes sibling-asymmetry with
  the reporter side.

### LOW (defensive / observability / spec text)

- **NEW-A-16** — median-of-even uses upper pair (was lower pair —
  removes downward bias).
- **NEW-B-3** — verifier count promoted to param
  `StorageTruthHealVerifierCount` (default 2, gov-tunable).
- **NEW-B-4** — insufficient-verifiers path emits an event
  (sibling-symmetric with `EventTypeHealOpInsufficientHealers`).
- **NEW-B-5** — link-recheck single-witness intent documented inline.
- **NEW-B-6 / B-9** — `InitGenesis` cross-validates audit
  `StorageTruthPostponements` against supernode `SuperNodeStatePostponed`.
- **NEW-B-7** — `GetNextHealOpID` panic-guards on malformed counter.
- **NEW-C-4 / A-19** — `pruneStorageProofTranscripts` logs malformed
  records instead of silently swallowing.
- **NF7** — workspace `LEP6.md:171` `pair_rank` wording corrected to the
  canonical 0x00-framed concatenation (in-repo guide already correct).
- **F119-F3 residue — cross-holder PASS bonus implemented.**
  `applyTicketDeteriorationDelta` (where `state.LastTargetSupernodeAccount`
  is available, NOT in the per-result delta switch). When PASS lands on
  a ticket whose prior failure was from a DIFFERENT holder, an additional
  `-3` ticket-deterioration delta is applied. Tests in
  `storage_truth_cross_holder_pass_test.go` (4 sub-cases: same-holder PASS
  no-bonus, cross-holder PASS bonus, no-prior-failure PASS no-bonus, FAIL
  no-bonus).

### New params introduced this round

- `StorageTruthHealVerifierCount` — default 2 (NEW-B-3).
- `StorageTruthStrongRecoveryCleanPassCount` — default 5 (F121-F12).

Both plumbed through `proto/lumera/audit/v1/params.proto` and
`x/audit/v1/types/params.go` (default + Validate).

### Why round-1 missed these (process retrospective)

Three production-gate sweeps were not executed before declaring the
earlier audit GREEN:

1. **Out-of-scope diff sweep** vs `master` would have caught NEW-C-3
   (silent fee-routing deletion).
2. **`Set*` ↔ `ExportGenesis` symmetry sweep** (every `Set*` write should
   round-trip through `ExportGenesis`) would have caught NEW-C-1 (8
   missing prefix families).
3. **PARTIAL-fix write-path enumeration** (when a fix is marked partial,
   enumerate every write site that mirrors the predicate before claiming
   resolution) would have caught NEW-C-2, F121-F12, F121-F10
   sibling-symmetry misses.

These three sweeps are now mandatory pre-master items (see checklist
below) and codified as Skill Pitfall #31 in the
`lumera-lep6-pr-comment-triage` skill.

---

## Pre-Release Checklist

This section is the canonical aggregator of every operational, follow-up,
and process item that must complete **before LEP-6 enforcement can flip
from `SHADOW` to `SOFT`/`FULL` on a live network**. Items below are NOT
blocked by the merging PR itself; they are gates for the activation
governance proposal.

Source-of-truth references: `ACTIVE_WORK.md` (in-flight tracking),
`.lep6-review-pending-doc-updates/` (per-review queues),
`docs/leps/LEP-6-implementation-guide.md` (this file — design-of-record).

### A. Migrations & data backfills

- [ ] **`KeepLastEpochEntries` v1→v2 migration applied (122-F4).**
  Handler at `x/audit/v1/module/migrations.go` registered at
  `module.go:100` (`RegisterMigration(types.ModuleName, 1, NewMigrateV1ToV2)`).
  On upgrade, bumps `KeepLastEpochEntries` to
  `max(KeepLastEpochEntries, OldClassAFaultWindow)` (default
  `OldClassAFaultWindow=21` if zero). `ConsensusVersion=2` at
  `types/genesis.go:5`. Verify the upgrade handler is wired into
  `app/upgrades/<vN>/upgrades.go` for the activation release and that
  post-upgrade state shows `KeepLastEpochEntries >= 21`.
- [ ] **TicketArtifactCountState backfill** for finalized cascade tickets
  pre-dating LEP-6 anchoring (see "Mandatory Pre-Activation Data Plan"
  above). Required: zero unresolved exceptions before flipping to
  `SHADOW` or above. Backfill source: finalized cascade metadata
  (`index_artifact_count` / `symbol_artifact_count` when present;
  deterministic fallback from finalized symbol IDs otherwise — never
  silently guess).
- [ ] **Audit report published**: total finalized cascade tickets, total
  already anchored, total newly backfilled, total
  unresolved/excluded (must be zero). Attach to the activation
  governance proposal body.
- [ ] **New params seeded with defaults** at the upgrade boundary
  (`StorageTruthHealVerifierCount=2`,
  `StorageTruthStrongRecoveryCleanPassCount=5`). Confirm proto schema
  defaults populate on `InitGenesis` of an upgraded chain.

### B. Implementation follow-ups

- [ ] **Class-A state-counter cleanup** — remove the residual zero-events
  fallback in Postpone/StrongPostpone bands (CP3.5 audit F2, LOW).
  Auditor confirmed unreachable post-activation with indexed data; safe
  to defer but should be cleaned up before mainnet rollout.
- [ ] **Full-app simulation harness for LEP-6 paths** — repo currently
  lacks `TestFullAppSimulation` exercising decay-then-add ordering,
  recheck flow, heal-op lifecycle, and postponement+recovery loops over
  N=1000+ random blocks. Module-level simulation (`x/audit/v1/simulation`)
  is green; full-app harness is the missing tier.

### C. Review-process sweeps (mandatory before each release-gate PR merge)

These three sweeps were missed in round 1 and produced the 24 R2
findings. They are now mandatory pre-master gates per Skill Pitfall #31.

- [ ] **Out-of-scope diff sweep** — `git diff <release-base>..HEAD --stat`
  filtered to files outside the announced scope; flag any deletion or
  rewrite touching unrelated modules (especially money-flow paths in
  `x/action`, `x/bank`, `x/distribution`, `x/supernode`).
- [ ] **`Set*` ↔ `ExportGenesis` symmetry sweep** — for every keeper
  `Set*` write, confirm a matching `GetAll*` iterator and an
  `ExportGenesis` emission round-trip. Run on the audit + supernode
  modules at minimum.
- [ ] **PARTIAL-fix write-path enumeration** — for every finding marked
  PARTIAL in any prior review, enumerate every write/predicate site
  that mirrors the relevant rule before claiming resolution; check
  reporter-side ↔ ticket-side ↔ node-side symmetry where applicable.

### D. Cross-repo integration

- [ ] **Supernode-side recheck-builder integration** — recheck evidence
  flow has only been driven by hand-crafted CLI submissions in tests so
  far. Production correctness depends on the off-chain runtime building
  these txs with field shapes matching what `validateRecheckEvidence`
  expects. Track in the supernode repo.
- [ ] **End-to-end devnet validation runbook** — once supernode integrates
  the recheck-builder, validate the full flow on a local 5-validator
  devnet:
  1. Build `lumerad` from the activation tag → start devnet
     (`make devnet-up`).
  2. Build `supernode` runtime from matching branch with new tx shape.
  3. Trigger a real cascade upload via `sn-api-server`.
  4. Force a fail (corrupt one node's storage); let challenge fire;
     observe storage proof results land on-chain.
  5. Trigger recheck path; observe `MsgSubmitStorageRecheckEvidence`
     build correctly with all fields populated; confirm chain accepts
     it (note: `--gas auto --gas-adjustment 1.3` — see Operator Notes).
  6. Watch `NodeSuspicionState` / `ReporterReliabilityState` /
     `TicketDeteriorationState` evolve via queries.
  7. Cycle to recovery (clean passes); verify postponement→active
     transition and (for strong postpone) the new
     `StorageTruthStrongRecoveryCleanPassCount` gate.
  8. Verify R2 deltas land as designed: a cross-holder PASS produces
     `D -= 3` extra; per-epoch PASS reward is single `-4` (not
     per-result); EXPIRED heal-op advances probation and bumps `D`.

### E. Test pyramid re-validation at activation tag

- [ ] `./x/...` unit suite green at activation tag.
- [ ] `./x/audit/v1/simulation/...` + `./x/audit/v1/module/...`
  module-simulation green.
- [ ] `./tests/integration/...` green.
- [ ] `./tests/system/...` (`-tags=system`) green.
- [ ] `./tests/systemtests/...` (`-tags=system_test`) green
  (25/25 last verified at `0c6f5f0`).
- [ ] Determinism scan clean — `grep -rE 'float|math\.Pow|time\.Now|rand\.|sort\.Float|FormatFloat'`
  on `x/audit/v1/keeper` returns zero hits in consensus paths; no
  `range map[]` in scoring/divergence/enforcement consensus paths.

### F. Governance / operations

- [ ] **Mainnet activation governance proposal** — flipping
  `StorageTruthEnforcementMode` from `SHADOW` to `SOFT` or `FULL` is a
  consensus-binding change. Draft a governance proposal with a flag-day
  epoch boundary; coordinate with all validators on the activation
  epoch. Cannot be reversed without another governance proposal.
- [ ] **Validator/supernode operator advisory** — publish the
  gas-requirements note (above) in operator docs. Include in the
  SOFT/FULL activation proposal body so all participants see it before
  voting.
- [ ] **Operator changelog** — publish the R2 behavior deltas (new params,
  per-epoch PASS reward semantics, EXPIRED heal-op cooldown, strong-band
  recovery threshold, cross-holder PASS bonus) in the release notes so
  operators understand observable score-evolution changes.

### G. Documentation queue close-out

- [ ] `.lep6-review-pending-doc-updates/CP1_TRIAGE.md` and
  `CP2_SPEC_ALIGNMENT.md` items resolved or explicitly deferred with
  rationale.
- [ ] `.lep6-review-pending-doc-updates/r2/` items reflected in this
  guide (this section) and in `workspace/docs/LEP6.md` where the spec
  text needed correction (NF7 done; sweep for any further drift).
- [ ] `docs/agent-context/02_lumera.md` updated with R2 behavior deltas
  and new params for cross-session continuity.
