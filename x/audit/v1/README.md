# Audit Module (v1)

## Contents
1. [Abstract](#abstract)
2. [Overview](#overview)
3. [Epochs and Anchoring](#epochs-and-anchoring)
4. [Reports](#reports)
5. [Epoch-End Enforcement](#epoch-end-enforcement)
6. [Evidence](#evidence)
7. [Pruning and State Layout](#pruning-and-state-layout)
8. [Messages](#messages)
9. [Queries](#queries)
10. [Parameters](#parameters)
11. [Genesis](#genesis)
12. [Client](#client)

## Abstract

The audit module provides **epoch-based** reporting and enforcement for supernodes. Each epoch persists an on-chain `EpochAnchor` that freezes:

- a deterministic epoch seed, and
- the eligible supernode sets at epoch start (ACTIVE set and target set).

Supernodes submit one report per epoch (`MsgSubmitAuditReport`). At epoch end, the chain evaluates the collected reports (plus selected evidence) to postpone or recover supernodes.

## Overview

High-level flow:

1. **Derive epoch boundaries from height** using `epoch_zero_height` and `epoch_length_blocks`.
2. **At epoch start height** (`BeginBlocker`), persist `EpochAnchor(epoch_id)` for the epoch.
3. **During the epoch**, supernodes submit `MsgSubmitAuditReport` (one per supernode per epoch).
4. **At epoch end height** (`EndBlocker`), evaluate postponement/recovery rules and prune old epoch-scoped state.

## Epochs and Anchoring

Epoch boundaries are computed deterministically from params:

- `epoch_id`: integer epoch index
- `epoch_start_height`: inclusive
- `epoch_end_height`: inclusive

At `epoch_start_height`, `BeginBlocker` creates an `EpochAnchor` if it does not already exist. The anchor stores:

- `seed`: 32-byte deterministic seed derived at epoch start
- `active_supernode_accounts`: sorted ACTIVE supernode accounts at epoch start
- `target_supernode_accounts`: sorted (ACTIVE + POSTPONED) supernode accounts at epoch start
- commitment fields (`params_commitment`, `active_set_commitment`, `targets_set_commitment`)

Note: commitment fields are stored on-chain but are not currently validated/used by the module logic.

## Reports

Reports are persisted per `(epoch_id, supernode_account)` and include:

- `self_report`: self-attested host metrics and counters
- `peer_observations`: port-state observations about other supernodes

Report submission rules:

- only the **current** `epoch_id` (as derived at the current block height) is accepted
- only one report per `(epoch_id, supernode_account)` is accepted
- if `required_open_ports` is non-empty, each peer observation must include exactly that many `port_states`
- `self_report.inbound_port_states` must be either empty (unknown/unreported) or exactly `len(required_open_ports)` (same ordering)

### Deterministic peer-observation gating

Peer observation requirements are enforced at `MsgSubmitAuditReport` time:

- If the reporter is **ACTIVE at epoch start** (i.e. is present in `EpochAnchor.active_supernode_accounts`), the chain deterministically computes the reporter’s assigned targets and requires exactly one observation per target (no extras, no duplicates).
- If the reporter is **not** ACTIVE at epoch start, `peer_observations` must be empty.

Assignments are derived from:

- anchored ACTIVE/target sets,
- anchored epoch seed, and
- current params that drive the per-reporter target count (`peer_quorum_reports`, `min_probe_targets_per_epoch`, `max_probe_targets_per_epoch`).

## Epoch-End Enforcement

Enforcement occurs only at epoch end, and may transition supernodes via the supernode keeper.

### Postponement (`ACTIVE -> POSTPONED`)

At epoch end, a supernode can be postponed for:

- **Action-finalization evidence thresholds** (per-epoch counts meeting consecutive-epoch windows),
- **Missing reports** for `consecutive_epochs_to_postpone` consecutive epochs,
- **Self host minimum failures** (CPU/mem/disk free% thresholds),
- **Peer port thresholds**: a required port is treated as CLOSED if peer observations meet `peer_port_postpone_threshold_percent`, and this happens for `consecutive_epochs_to_postpone` consecutive epochs.

### Recovery (`POSTPONED -> ACTIVE`)

At epoch end, a supernode can recover:

- If postponed due to action-finalization evidence: by the action-finalization recovery window and total-bad-evidence constraint.
- Otherwise: if it has a compliant self report and at least one peer observation in the epoch where all required ports are `OPEN`.

Detailed behavior is implemented in `x/audit/v1/keeper/enforcement.go`.

## Evidence

Evidence records are append-only on-chain records used by enforcement logic.

Supported evidence types include:

- action module evidence (submitted by the action module account via keeper integration; reserved types are rejected by `MsgSubmitEvidence`)
- storage challenge failure evidence (submitted by challengers; optionally restricted to deterministic challengers for an epoch)

Evidence metadata is provided as JSON in `MsgSubmitEvidence` and stored on-chain as protobuf-binary bytes derived from the JSON. Evidence submission enforces size bounds (and honors `sc_evidence_max_bytes` for SC evidence).

For storage challenge failure evidence, challenger authorization (when enabled) is derived deterministically from the epoch anchor seed and anchored ACTIVE set.

## Pruning and State Layout

At epoch end, `PruneOldEpochs` prunes epoch-scoped state to keep only the last `keep_last_epoch_entries` epochs (inclusive).

State is stored under human-readable prefixes with binary epoch IDs (`u64be(epoch_id)`) so lexicographic ordering matches numeric ordering. Key layouts are defined in `x/audit/v1/types/keys.go`, including:

- epoch anchors: `ea/<u64be(epoch_id)>`
- reports: `r/<u64be(epoch_id)><reporter>`
- indices for reporter/self/target views
- evidence records and evidence indices
- evidence epoch counts used by enforcement

Note: evidence records are not currently pruned by `PruneOldEpochs`.

## Messages

### `MsgSubmitAuditReport`

Signed by `supernode_account`:

```protobuf
message MsgSubmitAuditReport {
  string supernode_account = 1;
  uint64 epoch_id          = 2;
  AuditSelfReport self_report = 3;
  repeated AuditPeerObservation peer_observations = 4;
}
```

### `MsgSubmitEvidence`

Signed by `creator`:

```protobuf
message MsgSubmitEvidence {
  string creator = 1;
  string subject_address = 2;
  EvidenceType evidence_type = 3;
  string action_id = 4;
  string metadata = 5; // JSON
}
```

### `MsgUpdateParams`

Governance-authority-gated parameter update:

- `epoch_zero_height` and `epoch_length_blocks` are immutable after genesis (changing epoch math would break deterministic epoch mapping).

## Queries

- `Query/Params`
- `Query/CurrentEpoch`
- `Query/EpochAnchor(epoch_id)`
- `Query/CurrentEpochAnchor`
- `Query/AssignedTargets(supernode_account)` (optional `epoch_id` filter)
- `Query/AuditReport(epoch_id, supernode_account)`
- `Query/AuditReportsByReporter(supernode_account)` (paginated; optional `epoch_id` filter)
- `Query/SupernodeReports(supernode_account)` (paginated; optional `epoch_id` filter)
- `Query/SelfReports(supernode_account)` (paginated; optional `epoch_id` filter)
- Evidence:
  - `Query/EvidenceById`
  - `Query/EvidenceBySubject` (paginated)
  - `Query/EvidenceByAction` (paginated)

## Parameters

Params are initialized from genesis and may later be updated by governance via `MsgUpdateParams` (with epoch-cadence fields immutable). Defaults (see `x/audit/v1/types/params.go`):

- Epoch cadence:
  - `epoch_length_blocks`: `400` (immutable after genesis)
  - `epoch_zero_height`: `1` (immutable after genesis)
- Report/assignment gating:
  - `peer_quorum_reports`: `3`
  - `min_probe_targets_per_epoch`: `3`
  - `max_probe_targets_per_epoch`: `5`
  - `required_open_ports`: `[4444, 4445, 8002]`
- Enforcement:
  - `min_cpu_free_percent`: `0` (disabled)
  - `min_mem_free_percent`: `0` (disabled)
  - `min_disk_free_percent`: `0` (disabled)
  - `consecutive_epochs_to_postpone`: `1`
  - `peer_port_postpone_threshold_percent`: `100`
  - `keep_last_epoch_entries`: `200`
- Action-finalization evidence:
  - `action_finalization_*` thresholds and windows
- Storage challenge:
  - `sc_*` settings, including `sc_evidence_max_bytes` (max SC evidence metadata bytes)

## Genesis

Genesis initializes:

- params
- optional evidence records
- `next_evidence_id`

Epoch boundaries are purely param-derived; there is no mutable “current epoch” state.

## Client

- gRPC query service: `lumera.audit.v1.Query`
- gRPC msg service: `lumera.audit.v1.Msg`
- REST endpoints are defined via `google.api.http` annotations in `proto/lumera/audit/v1/query.proto`.
