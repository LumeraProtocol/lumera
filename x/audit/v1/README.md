# Audit Module (v1)

## Contents
1. [Abstract](#abstract)
2. [Overview](#overview)
3. [Genesis State Implementation](#genesis-state-implementation)
4. [Components](#components)
5. [State Transitions](#state-transitions)
6. [Messages](#messages)
7. [Queries](#queries)
8. [Events](#events)
9. [Parameters](#parameters)
10. [Client](#client)

## Abstract

The Audit module provides deterministic, window-based reporting and peer reachability observations for supernodes. It aggregates peer evidence per window and enforces minimum participation by postponing ACTIVE supernodes that fail to submit required reports after a grace period.

## Overview

High-level behavior:
- The chain advances in deterministic reporting windows derived from block height.
- At each window start, the module snapshots the ACTIVE set (senders) and the ACTIVE+POSTPONED set (receivers), plus a window seed.
- ACTIVE supernodes submit one report per window:
  - self metrics (self-attested)
  - peer reachability observations for deterministically assigned targets
- Peer observations are aggregated per `(window_id, target_validator_address, port_index)` using quorum + unanimity semantics.
- After the grace period, any ACTIVE sender in the window snapshot that did not submit a report is transitioned to POSTPONED.

Note: this implementation currently focuses on windowing, deterministic assignment, evidence aggregation, and missing-report postponement. Resource threshold enforcement and POSTPONED recovery via peer consensus are not yet implemented.

## Genesis State Implementation

The Audit module genesis state only initializes module parameters:

```protobuf
message GenesisState {
  Params params = 1;
}
```

## Components

### 1. Params

Module parameters are defined in `proto/lumera/audit/v1/params.proto` and persisted under a module store key.

Key fields:
- `reporting_window_blocks`
- `missing_report_grace_blocks`
- `peer_quorum_reports`
- `min_probe_targets_per_window`
- `max_probe_targets_per_window`
- `required_open_ports`

### 2. Window Origin and Window IDs

On first use, the module stores a `window_origin_height` and uses it to derive:
- `window_id`
- `window_start_height`
- `window_end_height`

### 3. Window Snapshots

At `window_start_height`, the module stores a `WindowSnapshot`:
- `seed_bytes` (from the block header hash)
- ordered `senders` (ACTIVE validator addresses)
- ordered `receivers` (ACTIVE + POSTPONED validator addresses)
- `k_window` (targets assigned per sender for the window)

Snapshots make target assignment deterministic for the entire window, even if membership changes mid-window.

### 4. Reports

Reports are stored per `(window_id, reporter_validator_address)` and include:
- `supernode_account`
- `self_report`
- `peer_observations`

Uniqueness is enforced (one report per reporter per window).

### 5. Evidence Aggregates

Evidence is aggregated per `(window_id, target_validator_address, port_index)` as:
- `count` of distinct probers contributing OPEN/CLOSED
- `first_state` (OPEN/CLOSED)
- `conflict` flag (true if any prober disagrees with first_state)

Consensus state derivation:
- if `count < peer_quorum_reports` => `UNKNOWN`
- if `conflict == true` => `UNKNOWN`
- else => `first_state`

### 6. Audit Status

`AuditStatus` provides a compact, queryable view per validator:
- last reported window + height
- compliance flag and bounded reasons
- last-derived `required_ports_state` (ordered like `required_open_ports`)

## State Transitions

### Report submission

On `MsgSubmitAuditReport`:
1. Resolve reporter validator address from `supernode_account` via `x/supernode`.
2. Validate window acceptance (from `window_start_height` until `window_end_height + grace`).
3. Enforce per-window uniqueness for the reporter.
4. If reporter is ACTIVE, validate peer observation targets match deterministic assignment from the window snapshot.
5. Persist the report and update evidence aggregates and `AuditStatus`.

### Missing report enforcement

After `missing_report_grace_blocks` past `window_end_height`, for the enforced window:
- For each sender in the window snapshot without a report, transition the corresponding supernode to POSTPONED and record `"missing_report"` in its `AuditStatus`.

## Messages

### MsgSubmitAuditReport

Signed by `supernode_account`:

```protobuf
message MsgSubmitAuditReport {
  string supernode_account = 1;
  uint64 window_id         = 2;
  AuditSelfReport self_report = 3;
  repeated AuditPeerObservation peer_observations = 4;
}
```

### MsgUpdateParams

Governance-authority-gated parameter update:

```protobuf
message MsgUpdateParams {
  string authority = 1;
  Params params    = 2;
}
```

## Queries

### Params

Returns current module params:
- gRPC: `Query/Params`
- REST: `GET /LumeraProtocol/lumera/audit/v1/params`

### AuditStatus

Returns `AuditStatus` for a validator:
- gRPC: `Query/AuditStatus`
- REST: `GET /LumeraProtocol/lumera/audit/v1/audit_status/{validator_address}`

## Events

The module currently emits the `x/supernode` postponement event when a sender misses a report window. Audit-specific events are not yet emitted.

## Parameters

Default values (as implemented in `x/audit/v1/types/params.go`):
- `reporting_window_blocks`: `400`
- `missing_report_grace_blocks`: `100`
- `peer_quorum_reports`: `3`
- `min_probe_targets_per_window`: `3`
- `max_probe_targets_per_window`: `5`
- `required_open_ports`: `[4444, 4445, 8002]`

## Client

- gRPC query service: `lumera.audit.v1.Query`
- gRPC msg service: `lumera.audit.v1.Msg`
- REST endpoints are defined via `google.api.http` annotations in `proto/lumera/audit/v1/query.proto`.
