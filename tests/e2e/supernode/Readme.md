# Pastel DevNet Test Scripts

This repository contains scripts for testing various aspects of the Pastel DevNet network.

## Genesis Configuration

The DevNet uses a custom genesis file with the following key parameters:(using the custom genesis file)

- **Slashing**:
  - Signed blocks window: 10 blocks
  - Downtime jail duration: 60 seconds

- **Staking**:
  - Unbonding time: 60 seconds
  - Bond denomination: `upsl`

- **Supernode**:
  - Minimum stake requirement: 834,637,515,648 upsl
  - Default power reduction: 824,637,515,648 upsl (allows validators to operate below supernode threshold)

## Available Scripts

### 1. setup_five_supernodes.sh
- Sets up 5 supernodes on the network
- Prerequisite for other test scripts
- No additional setup required; can be run after DevNet is operational

### 2. supernode.sh
- Tests supernode lifecycle operations:
  - Registration
  - Start/Stop operations
  - Status updates
  - Deregistration
- Run after DevNet is operational

### 3. jailing.sh
- Tests validator jailing and unjailing mechanisms
- Requires:
  - Active validator setup
  - Validator number as parameter

### 4. delegation.sh
- Tests self-delegation changes and their effects on supernode status
- Requires:
  - Active validator setup
  - Validator number as parameter

## Usage

1. Start the DevNet
2. Run `setup_five_supernodes.sh` first
3. Run other test scripts as needed, providing validator numbers where required

For jailing and delegation tests:
```bash
./jailing.sh    # Uses validator 5 by default
./delegation.sh # Uses validator 2 by default
```

## Available Commands

### Query Commands
```bash
# Get module parameters
pasteld query supernode params

# Get supernode information
pasteld query supernode get-super-node [validator-address]

# List all supernodes
pasteld query supernode list-super-nodes

# Get top supernodes for a specific block
pasteld query supernode get-top-super-nodes-for-block [block-height]
```

### Transaction Commands
```bash
# Register a new supernode
pasteld tx supernode register-supernode [validator-address] [ip-address] [version] [supernode-account]

# Deregister a supernode
pasteld tx supernode deregister-supernode [validator-address]

# Start a supernode
pasteld tx supernode start-supernode [validator-address]

# Stop a supernode
pasteld tx supernode stop-supernode [validator-address] [reason]

# Update supernode information
pasteld tx supernode update-supernode [validator-address] [ip-address] [version] [supernode-account]
```

Note: All transaction commands will require additional flags such as `--from`, `--chain-id`, and `--keyring-backend` as shown in the test scripts.