# Pastel Network Devnet Setup

## 1. Overview

This tool automates the creation of blockchain validator networks by generating Docker configurations and validator scripts from JSON input files. It provides:

- Configuration-driven network generation
- Dynamic validator scaling
- Automated peer discovery and network setup
- Customizable chain parameters
- Docker-based deployment system

The system takes `config.json` and `validators.json` as inputs to generate Docker Compose files and validator scripts, enabling rapid deployment of distributed validator networks of any size.

## 2. Directory Structure

![Alt text](./imgs/dir-structure.png "Image title")

## 3. Configuration Files

### 3.1 config.json

The `config.json` file defines the global configuration for the validator network. This includes chain parameters, Docker settings, filesystem paths, and daemon configurations. All validators share these settings to ensure network consistency.

```json
{
    "chain": {
        "id": "chain-id",          // Network identifier
        "denom": {
            "bond": "token",       // Staking token denomination
            "mint": "token",       // Minting token denomination
            "minimum_gas_price": "0token" // Minimum transaction fee
        }
    },
    "docker": {
        "network_name": "network", // Docker network name
        "container_prefix": "prefix", // Container naming prefix
        "volume_prefix": "prefix"  // Volume naming prefix
    },
    "paths": {
        "base": {
            "host": "~",          // Host machine path
            "container": "/root"   // Container path
        },
        "directories": {
            "daemon": ".chain"     // Chain data directory
        }
    },
    "daemon": {
        "binary": "chaind",       // Chain daemon binary
        "keyring_backend": "test" // Keyring storage type
    }
}
```

### 3.2 validators.json
```json
[
    {
        // Primary validator - Uses default Tendermint ports
        "name": "validator1",
        "moniker": "validator1",
        "key_name": "key1",
        "port": 26656,      // Default P2P port
        "rpc_port": 26657,  // Default RPC port
        "rest_port": 1317,  // Default REST port
        "grpc_port": 9090,  // Default gRPC port
        "initial_distribution": {
            "account_balance": "1000token",
            "validator_stake": "900token"
        }
    },
    {
        // Secondary validators - Use incremented ports
        "name": "validator2",
        "moniker": "validator2", 
        "key_name": "key2",
        "port": 26666,      // P2P: 26656 + 10
        "rpc_port": 26667,  // RPC: 26657 + 10
        "rest_port": 1327,  // REST: 1317 + 10
        "grpc_port": 9091,  // gRPC: 9090 + 1
        "initial_distribution": {
            "account_balance": "1000token",
            "validator_stake": "900token"
        }
    }
]
```

#### 3.2.1 Validator Configuration Details

- **Primary Validator (First Entry)**
  - Uses standard Tendermint ports (26656, 26657, 1317, 9090)
  - Acts as the genesis validator for network initialization
  - Manages initial account creation and token distribution

- **Secondary Validators**
  - Use incremented ports to prevent conflicts
  - P2P ports increment by 10 (26666, 26676, etc.)
  - RPC ports increment by 10 (26667, 26677, etc.)
  - REST ports increment by 10 (1327, 1337, etc.)
  - gRPC ports increment by 1 (9091, 9092, etc.)

- **Common Configuration**
  - `name`: Unique identifier for the validator
  - `moniker`: Display name on the network
  - `key_name`: Keyring key identifier
  - `initial_distribution`: Token allocation and staking amounts

Each validator can have unique initial token distributions and stakes, though they typically match for network stability.

## 4. Core Components

### 4.1 Config Package (`config.go`)
Core configuration management system that handles:
- Loading and parsing of config files
- Data models for chain and validator settings
- Configuration validation

```go
// Key structures
type ChainConfig struct {...}    // Global chain configuration
type Validator struct {...}      // Individual validator config
func LoadConfigs() {...}         // Configuration loader
```

### 4.2 Generators Package
Set of generators that create deployment configurations and scripts.

#### 4.2.1 Docker Compose Generator (`docker-compose.go`)
Creates Docker network configuration:
- Service definitions for each validator
- Network and volume mappings
- Port configurations
- Container dependencies

#### 4.2.2 Validator Script Generators
Produces initialization and startup scripts:

1. Primary Validator (`primary-validator.go`)
   ```bash
   # Key responsibilities:
   - Chain initialization
   - Genesis configuration
   - Account creation
   - Network bootstrapping
   ```

2. Secondary Validator (`secondary-validator.go`)
   ```bash
   # Key responsibilities:
   - Validator initialization
   - Genesis synchronization
   - Peer discovery
   - Network joining
   ```

Each generator uses the config package structures to ensure consistent network setup across all components.

# 5. Generated Scripts Guide

## 5.1 Primary Validator Script
Key initialization sequence from `primary-validator.go`

### Chain Initialization
```bash
# 1. Initialize chain with configuration
mkdir -p /root/.pastel/config
pasteld init validator1 --chain-id pastel-devnet --overwrite

# 2. Update denominations in genesis
cat /root/.pastel/config/genesis.json | jq '.app_state.staking.params.bond_denom = "upsl"' > tmp_genesis.json
mv tmp_genesis.json /root/.pastel/config/genesis.json

# Same process for:
# - mint_denom
# - crisis constant fee
# - gov min deposit
# - gov expedited min deposit
```

### Account & Genesis Setup
```bash
# 3. Create validator accounts
for VALIDATOR in "alice" "bob" "carol" "dave" "eve"; do
    echo "Creating key for $VALIDATOR..."
    pasteld keys add $VALIDATOR --keyring-backend test
    
    ADDR=$(pasteld keys show $VALIDATOR -a --keyring-backend test)
    pasteld genesis add-genesis-account $ADDR 1000000000000000upsl
done
```

### File Sharing & Network Setup
```bash
# 4. Share necessary files
mkdir -p /shared
cp -r /root/.pastel/keyring-test /shared/keyring-test
cp /root/.pastel/config/genesis.json /shared/genesis.json
mkdir -p /shared/gentx
echo "true" > /shared/genesis_accounts_ready

# 5. Create primary gentx
pasteld genesis gentx alice 900000000000000upsl \
    --chain-id pastel-devnet \
    --keyring-backend test

# 6. Wait for other validators
while [[ $(ls /shared/gentx/* 2>/dev/null | wc -l) -lt 4 ]]; do
    echo "Found $(ls /shared/gentx/* 2>/dev/null | wc -l) of 4 gentx files..."
    sleep 2
done

# 7. Finalize genesis
mkdir -p /root/.pastel/config/gentx
cp /shared/gentx/*.json /root/.pastel/config/gentx/
pasteld genesis collect-gentxs
cp /root/.pastel/config/genesis.json /shared/final_genesis.json
echo "true" > /shared/setup_complete
```

### Peer Configuration
```bash
# 8. Share node ID and IP
nodeid=$(pasteld tendermint show-node-id)
echo $nodeid > /shared/validator1_nodeid
ip=$(hostname -i)
echo $ip > /shared/validator1_ip

# 9. Wait for other validators
while [[ ! -f /shared/validator2_nodeid || ! -f /shared/validator2_ip ]]; do
    sleep 1
done

# 10. Configure persistent peers
NODE_ID=$(cat /shared/validator2_nodeid)
NODE_IP=$(cat /shared/validator2_ip)
PEERS="${NODE_ID}@${NODE_IP}:26656"
sed -i "s/^persistent_peers *=.*/persistent_peers = \"$PEERS\"/" /root/.pastel/config/config.toml
```

## 5.2 Secondary Validator Script

### Initial Setup
```bash
# 1. Validate input parameters
if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]; then
    echo "Usage: $0 <key-name> <stake-amount> <moniker>"
    exit 1
fi

KEY_NAME=$1
STAKE_AMOUNT=$2
MONIKER=$3
```

### Chain & Genesis Setup
```bash
# 2. Wait for primary initialization
while [ ! -f /shared/genesis_accounts_ready ]; do
    sleep 1
done

# 3. Copy keyring
cp -r /shared/keyring-test /root/.pastel/keyring-test

# 4. Initialize chain
pasteld init $MONIKER --chain-id pastel-devnet --overwrite

# 5. Copy initial genesis
cp /shared/genesis.json /root/.pastel/config/genesis.json

# 6. Create and share gentx
pasteld genesis gentx $KEY_NAME $STAKE_AMOUNT \
    --chain-id pastel-devnet \
    --keyring-backend test

mkdir -p /shared/gentx
cp /root/.pastel/config/gentx/* /shared/gentx/${MONIKER}_gentx.json
```

### Network Integration
```bash
# 7. Wait for final genesis
while [ ! -f /shared/final_genesis.json ]; do
    sleep 1
done
cp /shared/final_genesis.json /root/.pastel/config/genesis.json

# 8. Share node information
nodeid=$(pasteld tendermint show-node-id)
echo $nodeid > /shared/${MONIKER}_nodeid
ip=$(hostname -i)
echo $ip > /shared/${MONIKER}_ip

# 9. Configure peers (excluding self)
for v in validator1 validator2 validator3 validator4 validator5; do
    if [ "$v" != "${MONIKER}" ]; then
        NODE_ID=$(cat /shared/${v}_nodeid)
        NODE_IP=$(cat /shared/${v}_ip)
        PEERS="${PEERS}${NODE_ID}@${NODE_IP}:26656,"
    fi
done

# 10. Update peer configuration
sed -i "s/^persistent_peers *=.*/persistent_peers = \"$PEERS\"/" /root/.pastel/config/config.toml

# 11. Wait for network setup
while [ ! -f /shared/setup_complete ]; do
    sleep 1
done
```

### Start Chain
```bash
# 12. Start validator node
pasteld start --minimum-gas-prices 0upsl
```

## 6. Docker Components

### 6.1 Dockerfile Structure
From [dockerfile](dockerfile):
```dockerfile
FROM debian:bookworm-slim

# System dependencies
RUN apt-get update && apt-get install -y \
    curl \
    jq \
    bash \
    sed \
    ca-certificates

# Copy chain binary and WASM library
COPY pasteld /usr/local/bin/pasteld            # Chain binary executable
COPY libwasmvm.x86_64.so /usr/lib/            # Required for WASM contract support
RUN chmod +x /usr/local/bin/pasteld && ldconfig

# Copy validator scripts
COPY primary-validator.sh /root/scripts/
COPY secondary-validator.sh /root/scripts/
RUN chmod +x /root/scripts/*.sh

# Expose ports
EXPOSE 26656 26657 1317 9090  # P2P, RPC, REST, gRPC respectively

WORKDIR /root
```

### 6.2 Docker Compose
Generated from `docker-compose.go`
```yaml
services:
  validator1:
    build: .
    container_name: pastel-validator1
    ports:
      - "26656:26656"  # P2P
      - "26657:26657"  # RPC
      - "1317:1317"    # REST API
      - "9090:9090"    # gRPC
    volumes:
      - ~/validator1-data:/root/.pastel  # Chain data directory
      - ~/shared:/shared                 # Shared directory for validator coordination
    environment:
      MONIKER: validator1
    command: bash /root/scripts/primary-validator.sh

  validator2:
    # ... similar config with incremented ports
    depends_on:
      - validator1
    command: bash /root/scripts/secondary-validator.sh bob 900000000000000upsl validator2
```

## 7. Usage Guide

### 7.1 Build and Setup
```bash
# Full build process
make build    
# 1. Downloads WasmVM v2.1.2 library
# 2. Builds chain binary with Ignite
# 3. Extracts binary from release archive
# 4. Copies files to devnet/:
#    - pasteld binary
#    - libwasmvm.x86_64.so
# 5. Generates network configuration
# 6. Builds Docker images

# Clean old data
make clean   # Removes ~/shared and all ~/validator*-data directories
```

### 7.2 Network Operations
```bash
# Start network with console output
make up

# Start network in background
make up-detach

# Clean start (removes old data and regenerates configs)
make up-clean

# Stop network and cleanup containers
make down
```

### 7.3 Network Files Location
```bash
# Final Genesis File
~/validator1-data/config/genesis.json   # On host machine
/root/.pastel/config/genesis.json       # Inside containers

# Node IDs and IPs
~/shared/validator*_nodeid              # Node IDs 
~/shared/validator*_ip                  # Container IPs
```

### 7.4 Joining New Node to Network

#### 1. Get Validator Info
```bash
# Get node ID
VALIDATOR1_ID=$(cat ~/shared/validator1_nodeid)
# e.g., 4cb8e2eb7bb90fd026e02f693927230fe3fb9c89

# Get container IP
VALIDATOR1_IP=$(cat ~/shared/validator1_ip) 
# e.g., 172.20.0.2
```

#### 2. Initialize New Node
```bash
# Clean previous data (if needed)
rm -rf ~/.pasteld

# Initialize node with same chain-id
pasteld init my-local-node --chain-id pastel-devnet
```

#### 3. Copy Genesis
```bash
# Copy from validator1's data directory /shared
cp ~/validator1-data/config/genesis.json ~/.pasteld/config/
```

#### 4. Start Node
```bash
# Start with container IP peer connection
pasteld start --minimum-gas-prices 0upsl \
    --p2p.persistent_peers "${VALIDATOR1_ID}@${VALIDATOR1_IP}:26656" \
    --p2p.laddr tcp://0.0.0.0:26626 \
    --rpc.laddr tcp://127.0.0.1:26627
```

*Note: You can use any validator as a peer by using their respective node ID and IP from the shared directory.*

### 7.5 Verify Connection
```bash
# Check peer connections
pasteld net-info

# Check sync status 
pasteld status | jq .SyncInfo
```

### 7.6 CLI Sessions

#### Access Container Shell
```bash
# Access primary validator
docker exec -it pastel-validator1 bash

# Access any validator (n = 1-5)
docker exec -it pastel-validator{n} bash
```

#### Direct CLI Commands
```bash
# Primary validator commands
docker exec -it pastel-validator1 pasteld keys list --keyring-backend test
docker exec -it pastel-validator1 pasteld query bank balances <address>

# Secondary validator commands (n = 2-5)
docker exec -it pastel-validator{n} pasteld status
```

#### Interactive CLI Sessions
From inside container after `docker exec -it pastel-validator1 bash`:
```bash
# Query commands
pasteld query bank balances <address>
pasteld query staking validators
pasteld query gov proposals

# Transaction commands
pasteld tx bank send <from> <to> 1000upsl --chain-id pastel-devnet --keyring-backend test
```

#### Common Operations
```bash
# View logs
docker exec -it pastel-validator1 tail -f /root/.pastel/pastel.log

# Check config
docker exec -it pastel-validator1 cat /root/.pastel/config/config.toml

# Monitor sync status
docker exec -it pastel-validator1 watch 'pasteld status | jq .SyncInfo'
```

*Note: The keyring-backend is set to 'test' in the dev environment, so no password prompts appear. For production, use 'file' or 'os' backend.*



