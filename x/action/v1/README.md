# Action Module

## Overview

The Action module handles the processing of Sense and Cascade actions in the Lumera blockchain. It manages the creation, finalization, and approval of actions, along with fee distribution to participating supernodes.

## Key Components

### Messages

- **RequestAction**: Initiates a new action (Sense or Cascade)
- **FinalizeAction**: Completes an action with processing results from supernodes
- **ApproveAction**: Optionally approves a completed action with the creator's signature

### Workflows

1. **Request Action**:
   - User creates an action with metadata
   - Action is stored in the blockchain with PENDING state
   - An action ID is returned to the user

2. **Finalize Action**:
   - Supernodes process the action data
   - For Cascade actions, a single supernode finalizes the action
   - For Sense actions, three supernodes must agree on the results
   - Action state changes to DONE when finalized successfully

3. **Approve Action (Optional)**:
   - Creator can add their signature to an action in DONE state
   - Action state changes to APPROVED

### Security Features

- **Signature Verification**: Verify signatures from creators and supernodes
- **ID Format Verification**: Ensure Kademlia IDs match the expected format
- **Supernode Authorization**: Validate supernodes are authorized to finalize actions

### Fee Distribution

- Fees are distributed equally among participating supernodes
- Distribution occurs when actions reach DONE or APPROVED state

## Action States

Actions can be in the following states:
- **PENDING**: Initial state after creation
- **PROCESSING**: Intermediate state during finalization (Sense actions)
- **DONE**: Action successfully finalized
- **APPROVED**: Action approved by creator after finalization
- **FAILED**: Action failed during finalization
- **EXPIRED**: Action expired before being finalized

## Metadata

Metadata is specific to each action type:

### Sense Metadata

- **DataHash**: Hash of the data being sensed
- **DdAndFingerprintsIc**: Counter for fingerprint tracking
- **DdAndFingerprintsIds**: List of fingerprint IDs after processing
- **Signatures**: Signatures from supernodes

### Cascade Metadata

- **DataHash**: Hash of the data being stored
- **FileName**: Name of the file being stored
- **RqIdsIc**: Counter for RQ IDs
- **RqIdsIds**: List of RQ IDs after processing
- **RqIdsOti**: List of OTI values
- **Signatures**: Signatures from creator and supernodes

## Configuration

The module has several parameters that can be configured via governance:
- Expiration duration for actions
- Maximum number of Kademlia IDs for redundancy
- Fee distribution settings

## Integration with Other Modules

- **Bank Module**: For fee distribution
- **Staking Module**: For supernode validation
- **Auth Module**: For account and signature verification