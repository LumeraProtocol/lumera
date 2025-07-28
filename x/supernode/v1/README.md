# Supernode Module

## Contents
1. [Abstract](#abstract)
2. [Overview](#overview)
3. [Genesis State Implementation](#genesis-state-implementation)
4. [Components](#components)
5. [State Transitions](#state-transitions)
6. [Messages](#messages)
7. [Events](#events)
8. [Parameters](#parameters)
9. [Client](#client)

## Abstract

The Supernode module enables validators with sufficient stake to provide advanced services to the Lumera network. Supernodes are responsible for processing Sense and Cascade actions, maintaining data integrity, and enhancing network security through specialized operations.

## Overview

The Supernode module manages the lifecycle of supernodes by:
- Registering validators as supernode operators
- Verifying stake requirements and validator eligibility
- Tracking supernode states and transitions
- Facilitating action processing by supernodes
- Enforcing quality of service through metrics and evidence collection
- Penalizing misbehaving supernodes through state changes and slashing

## Genesis State Implementation

The Genesis State defines the initial state of the Supernode module. Below is a detailed breakdown of its components and implementation details.

### Genesis State Structure

```go
type GenesisState struct {
    Params               Params       // Module parameters
    Supernodes           []SuperNode  // List of initial supernodes
    Evidence             []Evidence   // Initial evidence records
    CurrentVersions      []string     // List of compatible software versions
    PerformanceThresholds MetricsThresholds // Performance thresholds for monitoring
}
```

Key implementation aspects:
- Parameters are initialized with safe defaults
- No supernodes are active at genesis
- Compatible versions list can be pre-populated
- Evidence storage is initially empty
- Performance thresholds are set to reasonable defaults

## Components

### 1. SuperNode

```go
type SuperNode struct {
    ValidatorAddress string                 // Validator operator address
    States           []SuperNodeStateRecord // State history records
    Evidence         []Evidence             // Evidence of behavior/violations
    PrevIpAddresses  []IPAddressHistory    // History of IP addresses
    Version          string                 // Software version
    Metrics          MetricsAggregate      // Performance metrics
    SupernodeAccount string                 // Associated account for delegations
    P2pPort          string                 // P2P network port
}
```

Key implementation details:
- Each supernode is linked to exactly one validator
- State changes are recorded with timestamps and block heights
- Evidence collection helps maintain accountability
- Version tracking ensures compatible software is running
- Metrics aggregation provides performance insights

### 2. SuperNodeState

```go
enum SuperNodeState {
    SUPERNODE_STATE_UNSPECIFIED = 0; // Default, unused state
    SUPERNODE_STATE_ACTIVE = 1;      // Operational and processing actions
    SUPERNODE_STATE_DISABLED = 2;    // Registered but not operational
    SUPERNODE_STATE_STOPPED = 3;     // Temporarily deactivated
    SUPERNODE_STATE_PENALIZED = 4;   // Penalized for violations
}
```

State transitions follow specific rules:
- New supernodes start in DISABLED state
- Only DISABLED supernodes can transition to ACTIVE
- ACTIVE supernodes can be STOPPED by operator or PENALIZED by the system
- STOPPED supernodes can return to ACTIVE if eligibility requirements are met
- PENALIZED supernodes may require governance intervention to return to service

### 3. Evidence

```go
type Evidence struct {
    ReporterAddress  string // Address that reported the issue
    ValidatorAddress string // Address of the validator being reported
    ActionId         string // Related action ID if applicable
    EvidenceType     string // Type of evidence
    Description      string // Description of the issue
    Severity         uint64 // Severity level
    Height           int32  // Block height when reported
}
```

Evidence is used to:
- Track performance issues
- Document violations
- Support slashing decisions
- Provide transparency in supernode operations

## State Transitions

### Registration Workflow

1. **Registration**:
   - Validator operator submits a registration transaction
   - System validates eligibility requirements
   - Supernode is created in DISABLED state

2. **Activation**:
   - Operator submits activation request
   - System verifies current eligibility
   - State changes to ACTIVE
   - Supernode joins the active set

3. **Deactivation**:
   - Operator submits deactivation request
   - State changes to STOPPED
   - Supernode leaves the active set

4. **Penalization**:
   - Evidence of violations triggers automatic or governance review
   - If threshold is reached, state changes to PENALIZED
   - Possible slashing of stake occurs

## Messages

### MsgRegisterSupernode

Registers a new supernode:

```protobuf
message MsgRegisterSupernode {
    string validator_address = 1; // Validator operator address
    string ip_address = 2;       // IP address for supernode operations
    string account = 3;          // Optional supernode account
    string p2p_port = 4;         // P2P communication port
}
```

Required fields:
- `validator_address`: Validator operator address (bech32)
- `ip_address`: Valid IPv4 or IPv6 address

Validation:
- Sender must be validator operator
- Validator must meet minimum stake requirement
- Validator must not be jailed
- IP address must be valid and not already registered

### MsgDeregisterSupernode

Removes a supernode registration:

```protobuf
message MsgDeregisterSupernode {
    string validator_address = 1; // Validator operator address
}
```

Validation:
- Sender must be validator operator
- Supernode must exist

### MsgStartSupernode

Activates a registered supernode:

```protobuf
message MsgStartSupernode {
    string validator_address = 1; // Validator operator address
    string version = 2;          // Software version
}
```

Validation:
- Sender must be validator operator
- Supernode must be in DISABLED or STOPPED state
- Validator must meet minimum stake requirement
- Validator must not be jailed
- Version must be compatible

### MsgStopSupernode

Deactivates an active supernode:

```protobuf
message MsgStopSupernode {
    string validator_address = 1; // Validator operator address
    string reason = 2;           // Reason for stopping
}
```

Validation:
- Sender must be validator operator
- Supernode must be in ACTIVE state

### MsgUpdateSupernode

Updates supernode information:

```protobuf
message MsgUpdateSupernode {
    string validator_address = 1; // Validator operator address
    string ip_address = 2;       // New IP address
    string version = 3;          // New software version
    string account = 4;          // New supernode account
    string p2p_port = 5;         // New P2P port
}
```

Validation:
- Sender must be validator operator
- Supernode must exist
- IP address must be valid if provided
- Version must be compatible if provided

### MsgUpdateParams

Updates module parameters through governance:

```protobuf
message MsgUpdateParams {
    string authority = 1; // Must be governance module
    Params params = 2;    // New parameters
}
```

Validation:
- Sender must be governance module
- Parameters must be valid

## Events

### EventTypeSupernodeRegistered

Emitted when a supernode is registered:
```
Attributes:
- validator_address: Validator operator address
- ip_address: Assigned IP address
- supernode_account: Associated account if any
- height: Block height of registration
```

### EventTypeSupernodeDeregistered

Emitted when a supernode is deregistered:
```
Attributes:
- validator_address: Validator operator address
- height: Block height of deregistration
```

### EventTypeSupernodeStarted

Emitted when a supernode is activated:
```
Attributes:
- validator_address: Validator operator address
- version: Software version
- height: Block height of activation
```

### EventTypeSupernodeStopped

Emitted when a supernode is deactivated:
```
Attributes:
- validator_address: Validator operator address
- reason: Reason for stopping
- height: Block height of deactivation
```

### EventTypeSupernodeUpdated

Emitted when supernode information is updated:
```
Attributes:
- validator_address: Validator operator address
- fields_updated: List of updated fields
- height: Block height of update
```

### EventTypeSupernodePenalized

Emitted when a supernode is penalized:
```
Attributes:
- validator_address: Validator operator address
- reason: Reason for penalty
- severity: Penalty severity
- evidence_id: Related evidence if any
- height: Block height of penalty
```

## Parameters

```protobuf
message Params {
    uint64 minimum_stake_for_sn = 1;       // Minimum stake required
    uint64 reporting_threshold = 2;         // Threshold for reporting
    uint64 slashing_threshold = 3;          // Threshold for slashing
    MetricsThresholds metrics_thresholds = 4; // Performance thresholds
    uint64 evidence_retention_period = 5;   // Evidence retention in blocks
    string slashing_fraction = 6;           // Fraction of stake to slash
    uint64 inactivity_penalty_period = 7;   // Blocks before inactivity penalty
}
```

Default values:
- `minimum_stake_for_sn`: 50000ulume
- `reporting_threshold`: 10
- `slashing_threshold`: 100
- `evidence_retention_period`: 100800 (~ 7 days)
- `slashing_fraction`: "0.01"
- `inactivity_penalty_period`: 600 (~ 1 hour)

## Client

### CLI

Query commands:
```bash
# Query module parameters
lumerad query supernode params

# Get supernode for validator
lumerad query supernode supernode [validator-addr]

# List all supernodes with pagination
lumerad query supernode supernodes

# List active supernodes
lumerad query supernode active-supernodes

# Get evidence for a supernode
lumerad query supernode evidence [validator-addr]
```

Transaction commands:
```bash
# Register a supernode
lumerad tx supernode register \
  --ip-address=[ip] \
  --account=[account] \
  --p2p-port=[port] \
  --from=[validator-key] \
  --chain-id=[chain-id]

# Deregister a supernode
lumerad tx supernode deregister \
  --from=[validator-key] \
  --chain-id=[chain-id]

# Start a supernode
lumerad tx supernode start \
  --version=[version] \
  --from=[validator-key] \
  --chain-id=[chain-id]

# Stop a supernode
lumerad tx supernode stop \
  --reason=[reason] \
  --from=[validator-key] \
  --chain-id=[chain-id]

# Update a supernode
lumerad tx supernode update \
  --ip-address=[ip] \
  --version=[version] \
  --account=[account] \
  --p2p-port=[port] \
  --from=[validator-key] \
  --chain-id=[chain-id]

# Submit parameter change proposal
lumerad tx gov submit-proposal [proposal-file] \
  --from=[key] \
  --chain-id=[chain-id]
```

### gRPC

The module exposes the following gRPC services:

```protobuf
service Query {
  // Parameters queries the parameters of the module
  rpc Params(QueryParamsRequest) returns (QueryParamsResponse) {
    option (google.api.http).get = "/lumera/supernode/v1/params";
  }

  // Supernode queries a specific supernode by validator address
  rpc Supernode(QuerySupernodeRequest) returns (QuerySupernodeResponse) {
    option (google.api.http).get = "/lumera/supernode/v1/supernode/{validator_address}";
  }

  // Supernodes queries all supernodes with pagination
  rpc Supernodes(QuerySupernodesRequest) returns (QuerySupernodesResponse) {
    option (google.api.http).get = "/lumera/supernode/v1/supernodes";
  }

  // ActiveSupernodes queries supernodes in ACTIVE state
  rpc ActiveSupernodes(QueryActiveSupernodesRequest) returns (QueryActiveSupernodesResponse) {
    option (google.api.http).get = "/lumera/supernode/v1/active_supernodes";
  }

  // Evidence queries evidence for a specific supernode
  rpc Evidence(QueryEvidenceRequest) returns (QueryEvidenceResponse) {
    option (google.api.http).get = "/lumera/supernode/v1/evidence/{validator_address}";
  }
}

service Msg {
  // RegisterSupernode registers a new supernode
  rpc RegisterSupernode(MsgRegisterSupernode) returns (MsgRegisterSupernodeResponse);

  // DeregisterSupernode removes a supernode registration
  rpc DeregisterSupernode(MsgDeregisterSupernode) returns (MsgDeregisterSupernodeResponse);

  // StartSupernode activates a supernode
  rpc StartSupernode(MsgStartSupernode) returns (MsgStartSupernodeResponse);

  // StopSupernode deactivates a supernode
  rpc StopSupernode(MsgStopSupernode) returns (MsgStopSupernodeResponse);

  // UpdateSupernode updates supernode information
  rpc UpdateSupernode(MsgUpdateSupernode) returns (MsgUpdateSupernodeResponse);

  // UpdateParams updates the module parameters
  rpc UpdateParams(MsgUpdateParams) returns (MsgUpdateParamsResponse);
}
```

### REST

Endpoints are mounted on the REST API:

```
GET /lumera/supernode/v1/params               # Query parameters
GET /lumera/supernode/v1/supernode/{address}  # Get supernode
GET /lumera/supernode/v1/supernodes           # List supernodes
GET /lumera/supernode/v1/active_supernodes    # List active supernodes
GET /lumera/supernode/v1/evidence/{address}   # Get supernode evidence
```