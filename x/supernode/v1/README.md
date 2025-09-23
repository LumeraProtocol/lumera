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
    Params Params // Module parameters
}
```

Key implementation aspects:
- Parameters are initialized from genesis

## Components

### 1. SuperNode

```go
type SuperNode struct {
    ValidatorAddress      string                      // Validator operator address
    States                []SuperNodeStateRecord      // State history records
    Evidence              []Evidence                  // Evidence of behavior/violations
    PrevIpAddresses       []IPAddressHistory         // History of IP addresses
    Note                  string                      // Optional operator note (free-form)
    Metrics               MetricsAggregate           // Performance metrics
    SupernodeAccount      string                      // Associated account for delegations
    PrevSupernodeAccounts []SupernodeAccountHistory  // History of supernode accounts
    P2PPort               string                      // P2P network port
}
```

Key implementation details:
- Each supernode is linked to exactly one validator
- State changes are recorded with timestamps and block heights
- Evidence collection helps maintain accountability
- Note is an optional, free-form field for operator comments or release notes
- Metrics aggregation provides performance insights

### 2. SuperNodeState

```go
enum SuperNodeState {
    SUPERNODE_STATE_UNSPECIFIED = 0; // Default, unused state
    SUPERNODE_STATE_ACTIVE = 1;      // Operational and processing actions
    SUPERNODE_STATE_DISABLED = 2;    // Terminal state - requires re-registration
    SUPERNODE_STATE_STOPPED = 3;     // Temporarily deactivated
    SUPERNODE_STATE_PENALIZED = 4;   // Penalized for violations
}
```

State transitions follow specific rules:
- New supernodes start in ACTIVE state upon registration
- DISABLED is a terminal state set only by deregistration (permanent removal)
- STOPPED is a temporary state; can restart with StartSupernode (only from STOPPED)
- Re-registration of a DISABLED supernode changes state to ACTIVE (other fields unchanged); use UpdateSupernode for field changes
- ACTIVE supernodes can be STOPPED by operator or hooks
- Hooks only transition between ACTIVE and STOPPED; they never set DISABLED and never re-activate from DISABLED
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
   - Supernode is created in ACTIVE state

2. **Re-registration** (for disabled supernodes):
   - Only changes state from DISABLED to ACTIVE
   - Does not update IP address, account, or other fields
   - Use UpdateSupernode message to update fields

3. **Deactivation (STOPPED)**:
   - Operator submits stop request or hooks trigger on ineligibility/unbonding
   - State changes to STOPPED
   - Supernode leaves the active set; can be restarted with StartSupernode

4. **Deregistration (DISABLED)**:
   - Operator submits deregistration request
   - State changes to DISABLED (terminal state)
   - Requires re-registration to become ACTIVE again; hooks will not re-activate a DISABLED supernode

5. **Penalization**:
   - Evidence of violations triggers automatic or governance review
   - If threshold is reached, state changes to PENALIZED
   - Possible slashing of stake occurs

## Messages

### MsgRegisterSupernode

Registers a new supernode:

```protobuf
message MsgRegisterSupernode {
    string creator           = 1; // Signer; must be validator operator
    string validator_address = 2; // Validator operator address
    string ip_address        = 3; // IP address for supernode operations
    string supernode_account = 4; // Optional supernode account
    string p2p_port          = 5; // P2P communication port
}
```

Required fields:
- `validator_address`: Validator operator address (bech32)
- `ip_address`: Valid IPv4 or IPv6 address

Validation:
- Sender must be validator operator
- Validator must meet minimum stake requirement
- Validator must not be jailed
- IP address must be valid

### MsgDeregisterSupernode

Permanently disables a supernode (terminal state):

```protobuf
message MsgDeregisterSupernode {
    string creator           = 1; // Signer; must be validator operator
    string validator_address = 2; // Validator operator address
}
```

Validation:
- Sender must be validator operator
- Supernode must exist
- Sets state to DISABLED (requires re-registration to reactivate)

### MsgStartSupernode

Activates a registered supernode (no field updates):

```protobuf
message MsgStartSupernode {
    string creator           = 1; // Signer; must be validator operator
    string validator_address = 2; // Validator operator address
}
```

Validation:
- Sender must be validator operator
- Supernode must be in STOPPED state (DISABLED requires re-registration)

### MsgStopSupernode

Temporarily stops an active supernode:

```protobuf
message MsgStopSupernode {
    string creator           = 1; // Signer; must be validator operator
    string validator_address = 2; // Validator operator address
    string reason            = 3; // Reason for stopping
}
```

Validation:
- Sender must be validator operator
- Supernode must be in ACTIVE state
- Sets state to STOPPED (can be restarted with StartSupernode)

### MsgUpdateSupernode

Updates supernode information (idempotent, append-only history for selected fields):

```protobuf
message MsgUpdateSupernode {
    string creator           = 1;  // Signer; must be validator operator
    string validator_address = 2;  // Validator operator address
    string ip_address        = 3;  // Optional new IP address
    string note              = 4;  // Optional operator note 
    string supernode_account = 5;  // Optional new supernode account
    string p2p_port          = 6;  // Optional new P2P port
}
```

Validation and effects:
- Sender must be validator operator
- Supernode must exist
- If `ip_address` provided: appended to `PrevIpAddresses` with current height (dedup consecutive)
- If `supernode_account` provided and valid bech32: appended to `PrevSupernodeAccounts` with height; emits old/new account attributes
- If `note` provided: replaces `Note` (no history kept)
- If `p2p_port` provided: replaces `P2PPort`

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
 - re_registered: "true" if this is a re-registration
 - old_state: Previous state when re-registering (e.g. "disabled")
 - p2p_port: P2P port value
```

### EventTypeSupernodeDeRegistered

Emitted when a supernode is deregistered:
```
Attributes:
- validator_address: Validator operator address
- old_state: Previous state before disabling
- height: Block height of deregistration
```

### EventTypeSupernodeStarted

Emitted when a supernode is activated:
```
Attributes:
- validator_address: Validator operator address
- reason: Optional reason for starting, examples:
  - tx_start (operator invoked start tx)
  - validator_bonded_eligible (hook: validator bonded and meets requirements)
  - delegation_modified_eligible (hook: stake meets requirements after delegation change)
- old_state: Previous state before activation (e.g. "stopped")
- height: Block height of activation
```

### EventTypeSupernodeStopped

Emitted when a supernode is deactivated:
```
Attributes:
- validator_address: Validator operator address
- reason: Reason for stopping, examples:
  - operator-provided string (from stop tx)
  - validator_bonded_not_eligible (hook: bonded but not eligible)
  - validator_begin_unbonding (hook: began unbonding)
  - delegation_modified_not_eligible (hook: stake below minimum after delegation change)
  - validator_removed (hook: validator removed)
- old_state: Previous state before deactivation (e.g. "active")
- height: Block height of deactivation
```

### EventTypeSupernodeUpdated

Emitted when supernode information is updated:
```
Attributes:
- validator_address: Validator operator address
- fields_updated: List of updated fields
- height: Block height of update
 - old_account: Previous supernode account (when supernode_account changes)
 - new_account: New supernode account (when supernode_account changes)
 - old_p2p_port: Previous P2P port (when p2p_port changes)
 - p2p_port: New P2P port (when p2p_port changes)
 - old_ip_address: Previous IP (when ip_address changes)
- ip_address: New IP (when ip_address changes)
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

Note: This event is documented as planned; emission is not implemented yet in the current codebase.

## Parameters

```protobuf
message Params {
    cosmos.base.v1beta1.Coin minimum_stake_for_sn = 1; // Minimum stake required
    uint64                    reporting_threshold   = 2; // Threshold for reporting
    uint64                    slashing_threshold    = 3; // Threshold for slashing
    string                    metrics_thresholds    = 4; // Performance thresholds (encoded)
    string                    evidence_retention_period = 5; // Retention window
    string                    slashing_fraction     = 6; // Fraction of stake to slash
    string                    inactivity_penalty_period = 7; // Penalty window
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
  --note=[text] \
  --supernode-account=[account] \
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
