# Audit Module Specification (audit/v1)

This document specifies the `audit/v1` on-chain contract: protobuf shapes, windowing, snapshots, report storage/query surfaces, and postpone/recovery behavior.

## Contents
1. Abstract
2. Overview
3. Reporting Windows
4. Parameters
5. Data Types (audit/v1)
6. Messages (tx)
7. Queries
8. On-Chain State
9. Postponement and Recovery
10. Out of Scope
11. Events



## 1. Abstract
The Audit module (`x/audit/v1`) provides deterministic, window-based reporting for supernodes:
- ACTIVE supernodes submit one audit report per window, containing a self report and (when they are a prober for the window) peer reachability observations for all assigned targets.
- At the start of each window, the module persists a `WindowSnapshot` that serves as the minimal source-of-truth for the prober → targets mapping for that window.

## 2. Overview
### 2.1 Roles
- **Probers**: ACTIVE supernodes at the window start height.
- **Targets**: ACTIVE + POSTPONED supernodes at the window start height.
- Reports are submitted by the registered **supernode account** (`supernode_account`) for a supernode, and the module resolves the corresponding supernode record via `x/supernode` when needed for validation.
- All audit state is scoped to a single supernode account (and to a window where applicable); there is no chain-global “single audit status”.

Assumption:
- `supernode_account` is treated as the stable identifier for audit storage and queries.

### 2.2 High-level flow (per window)
1) The module maintains a persisted “current window” state derived from block height.
2) At `window_start_height`, the module stores a `WindowSnapshot` containing the per-window prober → targets mapping (`assignments`).
3) Each reporter submits `MsgSubmitAuditReport` for a specific `window_id` during the acceptance period.
4) The module stores the report.

### Summary
- Time is divided into **reporting windows**. The module persists the current window boundaries and advances them deterministically as height increases.
- At the **first block of each window**, the chain writes a **window snapshot** that freezes, for that window:
  - the **prober → targets mapping** (`assignments`)
- Each supernode can submit **at most one report per window**. A report is signed by the supernode account and stored under `supernode_account`.
- A report contains self metrics plus peer observations. Peer observations include port states aligned by index to `required_open_ports` (position `i` refers to the `i`th configured port).
  - If the reporter is a prober for the window (i.e. appears in `WindowSnapshot.assignments`), peer observations must cover **all** assigned targets for that prober.
  - If the reporter is not a prober for the window (e.g. `POSTPONED`), peer observations must be empty (self-report only).
- A report is **accepted** only if:
  - the report targets the **current window_id** at the current height (window start through window end)
  - the reporter has not already submitted a report for that window
- When accepted, the chain stores the report as-is.
- Economic penalties and aggregation are intentionally out-of-scope for the current implementation.

## 3. Reporting Windows
Window sizing is block-based and deterministic. The module persists the current window boundaries and advances them as the chain height increases.

### 3.1 Window state
The module persists a “current window” record containing:
- `window_id`
- `window_start_height`
- `window_end_height`
- `window_blocks` (effective window size for the current window)

Initialization:
- On first run (e.g., at the first `BeginBlock` after module activation), the module writes the initial window:
  - `window_id = 0`
  - `window_start_height = H_init` (the current block height at initialization)
  - `window_blocks = reporting_window_blocks` (from params at initialization)
  - `window_end_height = window_start_height + window_blocks - 1`

Advancement:
- If `H > window_end_height`, the module advances windows until `H` is within the current window.
- Each advance increments `window_id` by 1 and sets:
  - `next_start = previous_end + 1`
  - `window_start_height = next_start`
  - `window_end_height = next_start + window_blocks - 1` (using the effective window size for that new window)

### 3.2 Report acceptance period
A report is accepted only when:
- `window_id` equals the module’s current `window_id` at the current height, and
- the current height is within `[window_start_height, window_end_height]` of that current window.

Otherwise, `MsgSubmitAuditReport` is rejected.

### 3.3 Window size changes (`reporting_window_blocks`)
`reporting_window_blocks` may be updated by governance. To preserve stable window boundaries, the change is applied **at the next window boundary**:
- the current window’s `start_height`/`end_height` do not change mid-window
- the new `reporting_window_blocks` value takes effect starting at `previous_end_height + 1`

## 4. Parameters
Parameters are represented by the `Params` message.

Default values:
- `reporting_window_blocks` (uint64): `400`
- `peer_quorum_reports` (uint32): `3`
- `min_probe_targets_per_window` (uint32): `3`
- `max_probe_targets_per_window` (uint32): `5`
- `required_open_ports` (repeated uint32): `[4444, 4445, 8002]`
- `min_cpu_free_percent` (uint32): `0` (disabled)
- `min_mem_free_percent` (uint32): `0` (disabled)
- `min_disk_free_percent` (uint32): `0` (disabled)
- `consecutive_windows_to_postpone` (uint32): `1`
- `keep_last_window_entries` (uint64): `200`

## 5. Data Types (audit/v1)
The module defines its reachability types under `audit/v1`.

### 5.1 PortState
```protobuf
enum PortState {
  PORT_STATE_UNKNOWN = 0;
  PORT_STATE_OPEN    = 1;
  PORT_STATE_CLOSED  = 2;
}
```

### 5.2 AuditSelfReport
Self metrics are self-attested and stored as provided by the reporter.

```protobuf
message AuditSelfReport {
  double cpu_usage_percent  = 1;
  double mem_usage_percent  = 2;
  double disk_usage_percent = 3;

  // inbound_port_states[i] refers to required_open_ports[i] for the window.
  repeated PortState inbound_port_states = 4;

  uint32 failed_actions_count = 5;
}
```

Note: the current implementation does not validate `inbound_port_states` length; it is stored as provided by the reporter.

### 5.3 AuditPeerObservation
Peer port states are index-aligned: `port_states[i]` refers to `required_open_ports[i]` for the window.

```protobuf
message AuditPeerObservation {
  string target_supernode_account = 1 [(cosmos_proto.scalar) = "cosmos.AccAddressString"];
  repeated PortState port_states = 2; // port_states[i] refers to required_open_ports[i] for the window
}
```

### 5.4 AuditReport
Reports are stored per `(window_id, supernode_account)` and are immutable once accepted.

```protobuf
message AuditReport {
  // Primary identity for audit storage and queries.
  string supernode_account = 1 [(cosmos_proto.scalar) = "cosmos.AccAddressString"];
  uint64 window_id         = 2;
  int64 report_height      = 3;

  AuditSelfReport self_report = 4 [(gogoproto.nullable) = false];
  repeated AuditPeerObservation peer_observations = 5;
}
```

### 5.8 WindowSnapshot
`WindowSnapshot` stores the minimal per-window source-of-truth for the prober → targets mapping.

```protobuf
message ProberTargets {
  string prober_supernode_account = 1 [(cosmos_proto.scalar) = "cosmos.AccAddressString"];
  repeated string target_supernode_accounts = 2 [(cosmos_proto.scalar) = "cosmos.AccAddressString"];
}

message WindowSnapshot {
  uint64 window_id           = 1;
  int64  window_start_height = 2;
  repeated ProberTargets assignments = 3;
}
```

## 6. Messages (tx)
### 6.1 Msg service
```protobuf
service Msg {
  rpc UpdateParams(MsgUpdateParams) returns (MsgUpdateParamsResponse);
  rpc SubmitAuditReport(MsgSubmitAuditReport) returns (MsgSubmitAuditReportResponse);
}
```

### 6.2 MsgUpdateParams
Parameters are updated by the module authority (governance by default).

```protobuf
message MsgUpdateParams {
  option (cosmos.msg.v1.signer) = "authority";

  string authority = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  Params params    = 2 [(gogoproto.nullable) = false];
}
```

### 6.3 MsgSubmitAuditReport
Reports are signed by `supernode_account` and applied to the supernode identified by that account.

```protobuf
message MsgSubmitAuditReport {
  option (cosmos.msg.v1.signer) = "supernode_account";

  string supernode_account = 1 [(cosmos_proto.scalar) = "cosmos.AccAddressString"];
  uint64 window_id         = 2;

  AuditSelfReport self_report = 3 [(gogoproto.nullable) = false];
  repeated AuditPeerObservation peer_observations = 4;
}
```

### 6.4 Message validation rules
On `MsgSubmitAuditReport`:
1) Reject if current height is outside the acceptance period for `window_id` (section 3.2).
2) Resolve reporter supernode from `supernode_account` via `x/supernode`; reject if not found.
3) If the reporter is a prober for the window (per `WindowSnapshot.assignments`), require peer observations for **all** assigned targets.
4) Reject duplicates: at most one report per `(window_id, supernode_account)`.

## 7. Queries
```protobuf
service Query {
  rpc Params(QueryParamsRequest) returns (QueryParamsResponse);
  rpc CurrentWindow(QueryCurrentWindowRequest) returns (QueryCurrentWindowResponse);

  rpc WindowSnapshot(QueryWindowSnapshotRequest) returns (QueryWindowSnapshotResponse);
  rpc AssignedTargets(QueryAssignedTargetsRequest) returns (QueryAssignedTargetsResponse);

  rpc AuditReport(QueryAuditReportRequest) returns (QueryAuditReportResponse);
  rpc AuditReportsByReporter(QueryAuditReportsByReporterRequest) returns (QueryAuditReportsByReporterResponse);

  rpc SupernodeReports(QuerySupernodeReportsRequest) returns (QuerySupernodeReportsResponse);
  rpc SelfReports(QuerySelfReportsRequest) returns (QuerySelfReportsResponse);
}

message QueryParamsRequest {}
message QueryParamsResponse { Params params = 1 [(gogoproto.nullable) = false]; }

message QueryCurrentWindowRequest {}
message QueryCurrentWindowResponse {
  uint64 window_id           = 1;
  int64  window_start_height = 2;
  int64  window_end_height   = 3;
}

message QueryWindowSnapshotRequest { uint64 window_id = 1; }
message QueryWindowSnapshotResponse { WindowSnapshot snapshot = 1; }

message QueryAssignedTargetsRequest {
  string supernode_account = 1 [(cosmos_proto.scalar) = "cosmos.AccAddressString"];
  uint64 window_id = 2;
  bool filter_by_window_id = 3;
}
message QueryAssignedTargetsResponse {
  uint64 window_id = 1;
  int64 window_start_height = 2;
  repeated uint32 required_open_ports = 3;
  repeated string target_supernode_accounts = 4 [(cosmos_proto.scalar) = "cosmos.AccAddressString"];
}

message QueryAuditReportRequest {
  uint64 window_id = 1;
  string supernode_account = 2 [(cosmos_proto.scalar) = "cosmos.AccAddressString"];
}
message QueryAuditReportResponse { AuditReport report = 1 [(gogoproto.nullable) = false]; }

message QueryAuditReportsByReporterRequest {
  string supernode_account = 1 [(cosmos_proto.scalar) = "cosmos.AccAddressString"];
  cosmos.base.query.v1beta1.PageRequest pagination = 2;
}
message QueryAuditReportsByReporterResponse {
  repeated AuditReport reports = 1;
  cosmos.base.query.v1beta1.PageResponse pagination = 2;
}

message QuerySupernodeReportsRequest {
  string supernode_account = 1 [(cosmos_proto.scalar) = "cosmos.AccAddressString"];
  uint64 window_id = 2;
  cosmos.base.query.v1beta1.PageRequest pagination = 3;
  bool filter_by_window_id = 4;
}

message SupernodeReport {
  string reporter_supernode_account = 1 [(cosmos_proto.scalar) = "cosmos.AccAddressString"];
  uint64 window_id = 2;
  int64 report_height = 3;
  repeated PortState port_states = 4;
}

message QuerySupernodeReportsResponse {
  repeated SupernodeReport reports = 1;
  cosmos.base.query.v1beta1.PageResponse pagination = 2;
}

message QuerySelfReportsRequest {
  string supernode_account = 1 [(cosmos_proto.scalar) = "cosmos.AccAddressString"];
  uint64 window_id = 2;
  cosmos.base.query.v1beta1.PageRequest pagination = 3;
  bool filter_by_window_id = 4;
}

message SelfReport {
  uint64 window_id = 1;
  int64 report_height = 2;
  AuditSelfReport self_report = 3 [(gogoproto.nullable) = false];
}

message QuerySelfReportsResponse {
  repeated SelfReport reports = 1;
  cosmos.base.query.v1beta1.PageResponse pagination = 2;
}
```

## 8. On-Chain State
This section describes the minimum state persisted by the module:
- Current window state (window_id, start/end, effective window size).
- Pending next window size (optional; applied at the next boundary when present).
- `WindowSnapshot` stored per `window_id`.
- `AuditReport` stored per `(window_id, supernode_account)`.

State growth considerations:
- State must remain bounded. The module MAY prune per-window state (`WindowSnapshot`, `AuditReport`) for any `window_id` once the acceptance period for that window has ended (section 3.2).
- Current implementation note: pruning is implemented at window end and keeps the last `keep_last_window_entries` windows of window-scoped state.
  - Operational note: `keep_last_window_entries` should be >= `consecutive_windows_to_postpone` so missing-report and peer-port rules always have enough history available.

## 9. Postponement and Recovery

This section defines the rules used to set a supernode to `POSTPONED` (and recover to `ACTIVE`) based on audit reports.

Important note:
- Peer-observation assignment/gating is enforced **when `MsgSubmitAuditReport` is accepted**. Enforcement later assumes only gated observations are stored.

### 9.1 Self-report based postponement (host requirements)

Inputs:
- `AuditSelfReport.cpu_usage_percent`, `mem_usage_percent`, `disk_usage_percent`

Params:
- `min_cpu_free_percent`
- `min_mem_free_percent`
- `min_disk_free_percent`

Compute free%:
- `cpu_free_percent = 100 - cpu_usage_percent`
- `mem_free_percent = 100 - mem_usage_percent`
- `disk_free_percent = 100 - disk_usage_percent`

Unknown special case:
- If a usage% is `0`, treat that metric as unknown and do not postpone based on it.

Rule:
- If any known free% is below its minimum, set the supernode to `POSTPONED`.

Non-rules:
- Ignore `failed_actions_count`.
- Ignore `inbound_port_states` (self inbound traffic is not reachability).

### 9.2 Peer-report based postponement (ports)

Inputs:
- Peer observations for required ports (index-aligned to `required_open_ports`).

Param:
- `consecutive_windows_to_postpone` (default `1`)

Per window `W` and required port index `i`, “port i is closed for target T in window W” is true only if:
- there is at least **1** peer reporter about `T` in `W`, and
- **all** those peer reporters report `PORT_STATE_CLOSED` for port index `i` for `T`.

Rule:
- If any required port index `i` is closed for `consecutive_windows_to_postpone` consecutive windows, set `T` to `POSTPONED`.

### 9.3 Missing report based postponement

Param:
- `consecutive_windows_to_postpone` (default `1`)

Rule:
- If a supernode in state `ACTIVE` fails to submit any `MsgSubmitAuditReport` for `consecutive_windows_to_postpone` consecutive windows, set it to `POSTPONED`.

### 9.4 Recovery (POSTPONED -> ACTIVE)

Rule:
- A `POSTPONED` supernode becomes `ACTIVE` after, in a single window:
  - one compliant self report (meets host requirements; treating 0 values as unknown), and
  - at least **1** peer report about the target in that window where all required ports are `PORT_STATE_OPEN`.

## 10. Out of Scope
This specification does not define economic penalties (e.g. slashing, jailing, rewards) for audit reports in its current scope.

## 11. Events
The current implementation does not emit audit-specific events.
