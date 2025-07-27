# Supernode Module

## Overview

The Supernode module manages the registration, operation, and lifecycle of supernodes in the Lumera blockchain. Supernodes are validators that meet specific stake requirements and provide additional services to the network, such as processing Sense and Cascade actions.

## Key Components

### Messages

- **RegisterSupernode**: Registers a new supernode with validator address, IP address, and account
- **DeregisterSupernode**: Removes a supernode registration
- **StartSupernode**: Activates a supernode
- **StopSupernode**: Deactivates a supernode with a reason
- **UpdateSupernode**: Updates supernode information like IP address, version, and account
- **UpdateParams**: Updates module parameters through governance

### Workflows

1. **Register Supernode**:
   - Validator operator creates a supernode registration
   - System verifies minimum stake requirements
   - Supernode is stored in the blockchain with DISABLED state
   - Supernode ID is returned to the user

2. **Start Supernode**:
   - Validator operator activates their supernode
   - System verifies eligibility (minimum stake, not jailed)
   - Supernode state changes to ACTIVE
   - Supernode begins participating in network operations

3. **Stop Supernode**:
   - Validator operator deactivates their supernode
   - Supernode state changes to STOPPED
   - Supernode stops participating in network operations

4. **Update Supernode**:
   - Validator operator updates supernode information
   - System verifies eligibility
   - Supernode information is updated

### Security Features

- **Stake Verification**: Ensure validators have sufficient stake to operate a supernode
- **Operator Verification**: Verify that only the validator operator can manage their supernode
- **Jailing Check**: Prevent jailed validators from operating supernodes
- **State Management**: Track supernode states and transitions

## Supernode States

Supernodes can be in the following states:
- **UNSPECIFIED**: Default state, not used in normal operation
- **ACTIVE**: Supernode is operational and participating in network activities
- **DISABLED**: Supernode is registered but not operational
- **STOPPED**: Supernode was active but has been temporarily stopped
- **PENALIZED**: Supernode has been penalized for violations

## Parameters

The module has several parameters that can be configured via governance:
- **minimum_stake_for_sn**: Minimum amount of tokens that must be staked to operate a supernode
- **reporting_threshold**: Threshold for reporting supernode metrics or issues
- **slashing_threshold**: Threshold for when slashing penalties are applied
- **metrics_thresholds**: Thresholds for various metrics that supernodes must maintain
- **evidence_retention_period**: How long evidence of supernode behavior is retained
- **slashing_fraction**: Fraction of stake that is slashed for violations
- **inactivity_penalty_period**: Period after which inactivity penalties are applied

## Integration with Other Modules

- **Staking Module**: For validator and delegation management
- **Slashing Module**: For penalizing misbehaving supernodes
- **Action Module**: Supernodes process Sense and Cascade actions
- **Bank Module**: For handling stake and rewards
- **Auth Module**: For account and signature verification

## Events

The module emits several events:
- **EventTypeSupernodeStarted**: When a supernode is activated
- **EventTypeSupernodeStopped**: When a supernode is deactivated
- **EventTypeSupernodeRegistered**: When a supernode is registered
- **EventTypeSupernodeDeregistered**: When a supernode is deregistered
- **EventTypeSupernodeUpdated**: When supernode information is updated
- **EventTypeSupernodePenalized**: When a supernode is penalized

## Eligibility Requirements

To operate a supernode, validators must:
1. Have sufficient stake (self-delegation or delegation from supernode account)
2. Not be jailed
3. Be the validator operator to manage the supernode
4. Maintain required metrics and performance

The minimum stake can come from:
- Self-delegation by the validator
- Delegation from a designated supernode account
- A combination of both

## Penalties

Supernodes may be penalized for:
- Inactivity
- Poor performance
- Malicious behavior
- Failing to maintain required metrics

Penalties can include:
- State change to PENALIZED
- Slashing of stake
- Temporary or permanent disqualification