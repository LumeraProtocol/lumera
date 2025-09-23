# Action Module

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

The Action module enables secure processing of Sense and Cascade actions in the Lumera blockchain. It provides a framework for requesting, finalizing, and approving actions with cryptographic verification, while managing fee distribution to participating supernodes based on their contributions.

## Overview

The Action module serves as a core component of the Lumera blockchain by:
- Managing the lifecycle of Sense and Cascade actions
- Validating action metadata and signatures
- Coordinating supernode participation in action processing
- Enforcing security through cryptographic verification
- Distributing fees to participating supernodes
- Handling action expiration and state transitions

Actions go through a well-defined lifecycle, starting with creation, proceeding through processing by supernodes, and potentially ending with approval by the creator. The module ensures that only authorized supernodes can finalize actions and that fees are distributed fairly.

## Genesis State Implementation

The Genesis State defines the initial state of the action module. Below is a detailed breakdown of its components and implementation details.

### Genesis State Structure

```protobuf
message GenesisState {
    Params params = 1;
    repeated Action actions = 2;
    uint64 action_count = 3;
}
```

## Components

### 1. Actions

The core data structure of the module is the Action:

```protobuf
message Action {
  string creator = 1;
  string actionID = 2;
  ActionType actionType = 3;
  bytes metadata = 4;
  string price = 5;
  int64 expirationTime = 6;
  ActionState state = 7;
  int64 blockHeight = 8;
  repeated string superNodes = 9;
}
```

Key fields:
- `creator`: The address that created the action
- `actionID`: Unique identifier for the action
- `actionType`: Type of action (SENSE or CASCADE)
- `metadata`: Action-specific data in binary format
- `price`: Fee paid for the action
- `expirationTime`: When the action expires
- `state`: Current state of the action
- `blockHeight`: Block height when the action was created
- `superNodes`: List of supernodes that have processed the action

### 2. Action Types

The module supports two main action types:

```protobuf
enum ActionType {
  ACTION_TYPE_UNSPECIFIED = 0;
  ACTION_TYPE_SENSE = 1;
  ACTION_TYPE_CASCADE = 2;
}
```

- **SENSE**: Actions for sensing data and creating fingerprints
- **CASCADE**: Actions for storing data in the network

### 3. Action States

Actions can be in the following states:

```go
enum ActionState {
  ACTION_STATE_UNSPECIFIED = 0;
  ACTION_STATE_PENDING = 1;
  ACTION_STATE_PROCESSING = 2;
  ACTION_STATE_DONE = 3;
  ACTION_STATE_APPROVED = 4;
  ACTION_STATE_REJECTED = 5;
  ACTION_STATE_FAILED = 6;
  ACTION_STATE_EXPIRED = 7;
}
```

- **PENDING**: Initial state after creation
- **PROCESSING**: Intermediate state during finalization (Sense actions)
- **DONE**: Action successfully finalized
- **APPROVED**: Action approved by creator after finalization
- **REJECTED**: Action rejected during processing
- **FAILED**: Action failed during finalization
- **EXPIRED**: Action expired before being finalized

### 4. Metadata

Metadata is specific to each action type:

#### Sense Metadata

```go
message SenseMetadata {
  // RequestAction required fields
  string data_hash = 1;
  uint64 dd_and_fingerprints_ic = 2;
  
  // RequestAction optional fields
  string collection_id = 3;
  string group_id = 4;
  
  // Added by Keeper
  uint64 dd_and_fingerprints_max = 5;
  
  // FinalizeAction fields
  repeated string dd_and_fingerprints_ids = 6;
  string signatures = 7;
}
```

- `data_hash`: Hash of the data being sensed
- `dd_and_fingerprints_ic`: Counter for fingerprint tracking
- `collection_id`: Optional collection identifier
- `group_id`: Optional group identifier
- `dd_and_fingerprints_max`: Maximum number of fingerprints (set by keeper)
- `dd_and_fingerprints_ids`: List of fingerprint IDs after processing
- `signatures`: Signatures from supernodes

#### Cascade Metadata

```go
message CascadeMetadata {
  // RequestAction required fields
  string data_hash = 1;
  string file_name = 2;
  uint64 rq_ids_ic = 3;
  
  // Added by Keeper
  uint64 rq_ids_max = 4;
  
  // FinalizeAction fields
  repeated string rq_ids_ids = 5;
  // RequestAction required field
  string signatures = 6;
}
```

- `data_hash`: Hash of the data being stored
- `file_name`: Name of the file being stored
- `rq_ids_ic`: Counter for RQ IDs
- `rq_ids_max`: Maximum number of RQ IDs (set by keeper)
- `rq_ids_ids`: List of RQ IDs after processing
- `signatures`: Signatures from creator and supernodes

## State Transitions

### Action Processing

When processing MsgRequestAction:
1. Validate prerequisites (price, expiration time)
2. Parse and validate action type
3. Process metadata with appropriate handler
4. Create new action with PENDING state
5. Register action and generate action ID
6. Transfer fees from creator to module account
7. Emit event

When processing MsgFinalizeAction:
1. Validate action exists and can be finalized
2. Verify supernode authorization
3. Process metadata with appropriate handler
4. Update action state based on handler result
5. Add supernode to the list
6. If action is now DONE, distribute fees
7. Emit event

When processing MsgApproveAction:
1. Validate action exists and is in DONE state
2. Verify creator matches action's creator
3. Update action state to APPROVED
4. Emit event

### Expiration Handling

For each block:
1. Check PENDING and PROCESSING actions
2. Mark expired actions as EXPIRED
3. Emit events for expired actions

## Messages

### MsgRequestAction

Initiates a new action (Sense or Cascade):

```protobuf
message MsgRequestAction {
  string creator = 1;
  string actionType = 2;
  string metadata = 3;
  string price = 4;
  string expirationTime = 5;
}
```

Required fields:
- `creator`: Address of the action creator
- `actionType`: Type of action ("SENSE" or "CASCADE")
- `metadata`: JSON string containing action-specific metadata
- `price`: Fee for the action
- `expirationTime`: Optional expiration time (unix timestamp)

Validation:
- Valid creator address
- Valid action type
- Valid metadata format for the action type
- Price meets minimum requirements
- Expiration time is in the future

### MsgFinalizeAction

Completes an action with processing results from supernodes:

```protobuf
message MsgFinalizeAction {
  string creator = 1; // must be supernode address
  string actionId = 2;
  string actionType = 3;
  string metadata = 4;
}
```

Required fields:
- `creator`: Address of the supernode finalizing the action
- `actionId`: ID of the action to finalize
- `actionType`: Type of action ("SENSE" or "CASCADE")
- `metadata`: JSON string containing finalization data

Validation:
- Valid supernode address
- Action exists and is in PENDING or PROCESSING state
- Supernode is authorized (in top-10 for action's block height)
- Valid metadata format for the action type

### MsgApproveAction

Optionally approves a completed action with the creator's signature:

```protobuf
message MsgApproveAction {
  string creator = 1;
  string actionId = 2;
}
```

Required fields:
- `creator`: Address of the action creator
- `actionId`: ID of the action to approve

Validation:
- Valid creator address
- Action exists and is in DONE state
- Creator matches action's original creator

### MsgUpdateParams

Updates module parameters through governance:

```protobuf
message MsgUpdateParams {
  string authority = 1;
  Params params = 2;
}
```

Requirements:
- Authority must be gov module
- Valid parameter values
- Passes governance process

## Events

### ActionRegistered

Emitted when an action is created:
```
EventTypeActionRegistered = "action_registered"
Attributes:
- action_id: Action ID
- creator: Creator address
- action_type: Type of action
- fee: Action fee
```

### ActionFinalized

Emitted when an action is finalized:
```
EventTypeActionFinalized = "action_finalized"
Attributes:
- action_id: Action ID
- creator: Creator address
- action_type: Type of action
- super_nodes: Comma-separated list of supernodes
```

### ActionApproved

Emitted when an action is approved:
```
EventTypeActionApproved = "action_approved"
Attributes:
- action_id: Action ID
- creator: Creator address
- action_type: Type of action
```

### ActionFailed

Emitted when an action fails:
```
EventTypeActionFailed = "action_failed"
Attributes:
- action_id: Action ID
- creator: Creator address
- action_type: Type of action
- error: Error message
- super_nodes: Comma-separated list of supernodes
```

### ActionExpired

Emitted when an action expires:
```
EventTypeActionExpired = "action_expired"
Attributes:
- action_id: Action ID
- creator: Creator address
- action_type: Type of action
```

## Parameters

```protobuf
message Params {
  // Fees
  cosmos.base.v1beta1.Coin base_action_fee = 1;
  cosmos.base.v1beta1.Coin fee_per_kbyte = 2;

  // Limits
  uint64 max_actions_per_block = 3;
  uint64 min_super_nodes = 4;
  uint64 max_dd_and_fingerprints = 5;
  uint64 max_raptor_q_symbols = 6;
  
  // Time Constraints
  google.protobuf.Duration expiration_duration = 7;
  google.protobuf.Duration min_processing_time = 8;
  google.protobuf.Duration max_processing_time = 9;
  
  // Reward Distribution
  string super_node_fee_share = 10;
  string foundation_fee_share = 11;
}
```

Key parameters:
- `base_action_fee`: Base fee for actions
- `fee_per_kbyte`: Fee per kilobyte of data
- `max_actions_per_block`: Maximum number of actions per block
- `min_super_nodes`: Minimum number of supernodes required
- `max_dd_and_fingerprints`: Maximum number of DD and fingerprints
- `max_raptor_q_symbols`: Maximum number of Raptor Q symbols
- `expiration_duration`: Duration after which actions expire
- `min_processing_time`: Minimum time for processing
- `max_processing_time`: Maximum time for processing
- `super_node_fee_share`: Share of fees for supernodes
- `foundation_fee_share`: Share of fees for the foundation

Parameter update governance proposal:
```json
{
    "messages": [{
        "@type": "/lumera.action.MsgUpdateParams",
        "authority": "lumera1...",
        "params": {
            "base_action_fee": {"denom": "ulume", "amount": "1000000"},
            "fee_per_kbyte": {"denom": "ulume", "amount": "100000"},
            "max_actions_per_block": "100",
            "min_super_nodes": "3",
            "max_dd_and_fingerprints": "100",
            "max_raptor_q_symbols": "100",
            "expiration_duration": "86400s",
            "min_processing_time": "60s",
            "max_processing_time": "3600s",
            "super_node_fee_share": "0.9",
            "foundation_fee_share": "0.1"
        }
    }],
    "metadata": "ipfs://CID",
    "deposit": "10000ulume",
    "title": "Update Action Parameters",
    "summary": "Update action module parameters"
}
```

## Client

### CLI

Query commands:
```bash
# Query module parameters
lumerad query action params

# Get action by ID
lumerad query action action [id]

# List all actions
lumerad query action actions

# List actions by state
lumerad query action actions-by-state [state]

# List actions by creator
lumerad query action actions-by-creator [address]
```

Transaction commands:
```bash
# Request a Sense action
lumerad tx action request-action \
  --action-type="SENSE" \
  --metadata='{"data_hash":"...","dd_and_fingerprints_ic":10}' \
  --price="1000000ulume" \
  --expiration-time="1627776000" \
  --from [key] \
  --chain-id [chain-id]

# Request a Cascade action
lumerad tx action request-action \
  --action-type="CASCADE" \
  --metadata='{"data_hash":"...","file_name":"file.txt","rq_ids_ic":10,"signatures":"..."}' \
  --price="1000000ulume" \
  --expiration-time="1627776000" \
  --from [key] \
  --chain-id [chain-id]

# Finalize an action
lumerad tx action finalize-action \
  --action-id="1" \
  --action-type="SENSE" \
  --metadata='{"dd_and_fingerprints_ids":["id1","id2"],"signatures":"..."}' \
  --from [key] \
  --chain-id [chain-id]

# Approve an action
lumerad tx action approve-action \
  --action-id="1" \
  --from [key] \
  --chain-id [chain-id]

# Submit parameter change proposal
lumerad tx gov submit-proposal [proposal-file] \
  --from [key] \
  --chain-id [chain-id]
```

### gRPC

The module exposes the following gRPC services:

```protobuf
service Query {
    // Parameters queries the parameters of the module.
    rpc Params(QueryParamsRequest) returns (QueryParamsResponse) {
        option (google.api.http).get = "/lumera/action/params";
    }

    // Action queries an action by ID.
    rpc Action(QueryActionRequest) returns (QueryActionResponse) {
        option (google.api.http).get = "/lumera/action/actions/{id}";
    }
    
    // Actions queries all actions.
    rpc Actions(QueryActionsRequest) returns (QueryActionsResponse) {
        option (google.api.http).get = "/lumera/action/actions";
    }
    
    // ActionsByState queries actions by state.
    rpc ActionsByState(QueryActionsByStateRequest) returns (QueryActionsByStateResponse) {
        option (google.api.http).get = "/lumera/action/actions/state/{state}";
    }
    
    // ActionsByCreator queries actions by creator.
    rpc ActionsByCreator(QueryActionsByCreatorRequest) returns (QueryActionsByCreatorResponse) {
        option (google.api.http).get = "/lumera/action/actions/creator/{address}";
    }
}

service Msg {
    // UpdateParams updates the module parameters.
    rpc UpdateParams(MsgUpdateParams) returns (MsgUpdateParamsResponse);
    
    // RequestAction submits a request for a new action.
    rpc RequestAction(MsgRequestAction) returns (MsgRequestActionResponse);
    
    // FinalizeAction finalizes an action with processing results.
    rpc FinalizeAction(MsgFinalizeAction) returns (MsgFinalizeActionResponse);
    
    // ApproveAction approves a finalized action.
    rpc ApproveAction(MsgApproveAction) returns (MsgApproveActionResponse);
}
```

### REST

Endpoints are mounted on the REST API:

```
GET /lumera/action/params                   # Query parameters
GET /lumera/action/actions/{id}             # Get action by ID
GET /lumera/action/actions                  # List all actions
GET /lumera/action/actions/state/{state}    # List actions by state
GET /lumera/action/actions/creator/{address} # List actions by creator
```