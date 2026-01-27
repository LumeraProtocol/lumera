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

The Audit module provides deterministic, window-based reporting for supernodes. It persists audit reports and a per-window `WindowSnapshot` that serves as the minimal source-of-truth for the prober → targets mapping for that window.

## Overview

High-level behavior:
- The chain advances in deterministic reporting windows derived from block height.
- At each window start, the module persists a `WindowSnapshot` containing the prober → targets mapping (`assignments`).
- Supernodes submit one report per window containing:
  - self metrics (self-attested)
  - peer reachability observations

Notes:
- This module currently focuses on windowing/snapshotting and report persistence. Penalties and aggregation are intentionally not implemented.

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
- `peer_quorum_reports`
- `min_probe_targets_per_window`
- `max_probe_targets_per_window`
- `required_open_ports`

### 2. Window State and Window IDs

The module maintains a persisted **current window state** in KV-store and advances it deterministically as the chain height increases.

The current window state includes:
- `window_id`
- `window_start_height`
- `window_end_height`
- `window_blocks` (effective window size for the current window)

#### Window size changes (`reporting_window_blocks`)

`reporting_window_blocks` may be updated by governance. To keep historical window boundaries stable, any change is applied **at the next window boundary**:
- the current window’s start/end heights do not change mid-window
- the new window size takes effect starting at `window_end_height + 1`

Implementation note: the module persists a pending “next window size” value and consumes it when advancing into the next window.

### 3. Window Snapshots

At `window_start_height`, the module stores a `WindowSnapshot`:
- `window_id`
- `window_start_height`
- `assignments`: the per-window prober → targets mapping

Snapshots freeze the per-window prober → targets mapping at the start of the window so later logic can rely on a deterministic, persisted mapping.

### 4. Reports

Reports are stored per `(window_id, supernode_account)` and include:
- `supernode_account`
- `self_report`
- `peer_observations`

Uniqueness is guaranteed (one report per reporter per window).

## State Transitions

### Report submission

On `MsgSubmitAuditReport`:
1. Resolve reporter supernode from `supernode_account` via `x/supernode`.
2. Validate window acceptance (only the current `window_id` at the current height is accepted).
3. Ensure per-window uniqueness for the reporter.
4. Persist the report.

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

### AuditReport

Returns `AuditReport` for a window and reporter:
- gRPC: `Query/AuditReport`

### AuditReportsByReporter

Lists `AuditReport` submitted by a reporter (paginated):
- gRPC: `Query/AuditReportsByReporter`
- REST: `GET /LumeraProtocol/lumera/audit/v1/audit_reports_by_reporter/{supernode_account}`

### CurrentWindow

Returns the current reporting window boundaries:
- gRPC: `Query/CurrentWindow`
- REST: `GET /LumeraProtocol/lumera/audit/v1/current_window`

### SupernodeReports

Lists peer-observation chunks about a given supernode (by other reporters):
- gRPC: `Query/SupernodeReports`
- REST: `GET /LumeraProtocol/lumera/audit/v1/supernode_reports/{supernode_account}`

### SelfReports

Lists self-report chunks for a supernode across windows:
- gRPC: `Query/SelfReports`
- REST: `GET /LumeraProtocol/lumera/audit/v1/self_reports/{supernode_account}`

## Events

Audit-specific events are not emitted.

## Parameters

Default values (as implemented in `x/audit/v1/types/params.go`):
- `reporting_window_blocks`: `400`
- `peer_quorum_reports`: `3`
- `min_probe_targets_per_window`: `3`
- `max_probe_targets_per_window`: `5`
- `required_open_ports`: `[4444, 4445, 8002]`

## Client

- gRPC query service: `lumera.audit.v1.Query`
- gRPC msg service: `lumera.audit.v1.Msg`
- REST endpoints are defined via `google.api.http` annotations in `proto/lumera/audit/v1/query.proto`.
