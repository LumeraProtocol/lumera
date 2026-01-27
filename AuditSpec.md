# Audit Module Specification (audit/v1)

This document specifies the `audit/v1` on-chain contract: protobuf shapes, windowing, snapshots, and report storage/query surfaces.

## Contents
1. Abstract
2. Overview
3. Reporting Windows
4. Parameters
5. Data Types (audit/v1)
6. Messages (tx)
7. Queries
8. On-Chain State
11. Out of Scope
12. Events



## 1. Abstract
The Audit module (`x/audit/v1`) provides deterministic, window-based reporting for supernodes:
- ACTIVE supernodes submit one audit report per window, containing a self report and optional peer reachability observations.
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
1) The chain derives `window_id` from block height and a stored `origin_height`.
2) At `window_start_height`, the module stores a `WindowSnapshot` containing the per-window prober → targets mapping (`assignments`).
3) Each reporter submits `MsgSubmitAuditReport` for a specific `window_id` during the acceptance period.
4) The module stores the report.

### Summary
- Time is divided into fixed-size **reporting windows** (a set number of blocks). Window IDs/boundaries are derived deterministically from block height and a one-time **origin height** stored on first use.
- At the **first block of each window**, the chain writes a **window snapshot** that freezes, for that window:
  - the **prober → targets mapping** (`assignments`)
- Each supernode can submit **at most one report per window**. A report is signed by the supernode account and stored under `supernode_account`.
- A report contains self metrics plus optional peer observations. Peer observations include port states aligned by index to `required_open_ports` (position `i` refers to the `i`th configured port).
- A report is **accepted** only if:
  - the chain is within the window’s acceptance period (window start through window end)
  - the reporter has not already submitted a report for that window
- When accepted, the chain stores the report as-is.
- Penalties and aggregation are intentionally out-of-scope for the current implementation.

## 3. Reporting Windows
Window sizing is block-based and deterministic. The module stores an `origin_height` once and uses it to derive window boundaries.

### 3.1 Window origin
`origin_height` is stored in module state on first use and remains fixed for the lifetime of the chain.

### 3.2 Window derivation
Let:
- `origin = origin_height`
- `W = reporting_window_blocks`
- `H = current block height`

Then:
- `window_id = floor((H - origin) / W)` for `H >= origin` (else `0`)
- `window_start_height(window_id) = origin + window_id * W`
- `window_end_height(window_id) = window_start_height(window_id) + W - 1`

### 3.3 Report acceptance period
A report for `window_id` is accepted only when the current height is within:
- `[window_start_height(window_id), window_end_height(window_id)]`

Outside this range, `MsgSubmitAuditReport` is rejected.

## 4. Parameters
Parameters are represented by the `Params` message.

Default values:
- `reporting_window_blocks` (uint64): `400`
- `peer_quorum_reports` (uint32): `3`
- `min_probe_targets_per_window` (uint32): `3`
- `max_probe_targets_per_window` (uint32): `5`
- `required_open_ports` (repeated uint32): `[4444, 4445, 8002]`

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
1) Reject if current height is outside the acceptance period for `window_id` (section 3.3).
2) Resolve reporter supernode from `supernode_account` via `x/supernode`; reject if not found.
3) Reject duplicates: at most one report per `(window_id, supernode_account)`.

## 7. Queries
```protobuf
service Query {
  rpc Params(QueryParamsRequest) returns (QueryParamsResponse);
  rpc CurrentWindow(QueryCurrentWindowRequest) returns (QueryCurrentWindowResponse);

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

message QueryAuditReportRequest {
  uint64 window_id = 1;
  string supernode_account = 2 [(cosmos_proto.scalar) = "cosmos.AccAddressString"];
}
message QueryAuditReportResponse { AuditReport report = 1 [(gogoproto.nullable) = false]; }

message QueryAuditReportsByReporterRequest {
  string supernode_account = 1 [(cosmos_proto.scalar) = "cosmos.AccAddressString"];
  // pagination omitted in this spec; implementations may add pagination.
}
message QueryAuditReportsByReporterResponse { repeated AuditReport reports = 1; }

message QuerySupernodeReportsRequest {
  string supernode_account = 1 [(cosmos_proto.scalar) = "cosmos.AccAddressString"];
  // pagination omitted in this spec; implementations may add pagination.
}

message SupernodeReport {
  string reporter_supernode_account = 1 [(cosmos_proto.scalar) = "cosmos.AccAddressString"];
  uint64 window_id = 2;
  int64 report_height = 3;
  repeated PortState port_states = 4;
}

message QuerySupernodeReportsResponse { repeated SupernodeReport reports = 1; }

message QuerySelfReportsRequest {
  string supernode_account = 1 [(cosmos_proto.scalar) = "cosmos.AccAddressString"];
  // pagination omitted in this spec; implementations may add pagination.
}

message SelfReport {
  uint64 window_id = 1;
  int64 report_height = 2;
  AuditSelfReport self_report = 3 [(gogoproto.nullable) = false];
}

message QuerySelfReportsResponse { repeated SelfReport reports = 1; }
```

## 8. On-Chain State
This section describes the minimum state persisted by the module:
- `origin_height` (int64) stored once.
- `WindowSnapshot` stored per `window_id`.
- `AuditReport` stored per `(window_id, supernode_account)`.

State growth considerations:
- State must remain bounded. The module MAY prune per-window state (`WindowSnapshot`, `AuditReport`) for any `window_id` once the acceptance period for that window has ended (section 3.3).
- Current implementation note: pruning is not yet implemented; per-window state accumulates over time.

## 11. Out of Scope
This specification does not define penalties or participation requirements for audit reports in its current scope.

## 12. Events
The current implementation does not emit audit-specific events.
