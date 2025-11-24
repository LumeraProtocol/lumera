# Lumera Protocol Devnet Setup

## 1. Overview

This tool automates the creation of blockchain validator networks by generating Docker configurations and validator scripts from JSON input files. It provides:

- Configuration-driven network generation
- Dynamic validator scaling
- Automated peer discovery and network setup
- Customizable chain parameters
- Docker-based deployment system

The system takes `config.json` and `validators.json` as inputs to generate Docker Compose files and validator scripts, enabling rapid deployment of distributed validator networks of any size.
The system take `claims.json` as input to be used by Claim module on genesis initialization.
The system CAN take existing `genesis.json` as input to be extended to include new validators. 

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
            "minimum_gas_price": "0ulume" // Minimum transaction fee
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
            "account_balance": "1000",
            "validator_stake": "900ulume"
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
            "account_balance": "1000ulume",
            "validator_stake": "900ulume"
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
   - Chain initialization
   - Genesis configuration
   - Account creation
   - Network bootstrapping
2. Secondary Validator (`secondary-validator.go`)
   - Validator initialization
   - Genesis synchronization
   - Peer discovery
   - Network joining

Each generator uses the config package structures to ensure consistent network setup across all components.

## 5. Devnet Docker Test System

The Devnet Docker test system is assembled by `Makefile.devnet`, `docker-compose.yml`, and the helper scripts under `devnet/scripts`. During `make devnet-build`, `devnet/scripts/configure.sh` copies configuration files and every binary from `devnet/bin` into `/tmp/<chain-id>/shared`. Each validator container subsequently runs `start.sh`, `validator-setup.sh`, `supernode-setup.sh`, and `network-maker-setup.sh` so that validators, Supernode, network-maker, and optional Hermes relayer tasks all bootstrap in a consistent order.

### 5.1 Dockerfile Structure
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
COPY lumerad /usr/local/bin/lumerad            # Chain binary executable
COPY libwasmvm.x86_64.so /usr/lib/            # Required for WASM contract support
RUN chmod +x /usr/local/bin/lumerad && ldconfig

# Copy validator scripts
COPY primary-validator.sh /root/scripts/
COPY secondary-validator.sh /root/scripts/
RUN chmod +x /root/scripts/*.sh

# Expose ports
EXPOSE 26656 26657 1317 9090  # P2P, RPC, REST, gRPC respectively

WORKDIR /root
```

### 5.2 Docker Compose
Generated from `docker-compose.go`
```yaml
services:
  validator1:
    build: .
    container_name: lumera-validator1
    ports:
      - "26656:26656"  # P2P
      - "26657:26657"  # RPC
      - "1317:1317"    # REST API
      - "9090:9090"    # gRPC
    volumes:
      - /tmp/lumera-devnet/validator1-data:/root/.lumera  # Chain data directory
      - /tmp/lumera-devnet/shared:/shared                 # Shared directory for validator coordination
    environment:
      MONIKER: validator1
    command: bash /root/scripts/primary-validator.sh

  validator2:
    # ... similar config with incremented ports
    depends_on:
      - validator1
    command: bash /root/scripts/secondary-validator.sh bob 900000000000000ulume validator2
```

### 5.3 Devnet Binary Bundle (`devnet/bin`)

`make devnet-build` (or `./devnet/scripts/configure.sh --bin-dir devnet/bin`) expects the following assets inside `devnet/bin`. Required files must always be present; optional files only need to exist when the related service is enabled.

| File | Required? | Purpose |
| --- | --- | --- |
| `lumerad` | Yes | Primary Lumera daemon executed by every validator container. |
| `libwasmvm.x86_64.so` | Yes | CosmWasm runtime shared library mounted into every validator so contracts can execute. |
| `supernode-linux-amd64` | Optional (required when Supernode service is used) | Binary started by `supernode-setup.sh` to aggregate price feeds and update `/shared/status`. |
| `sncli` | Optional | CLI utility that interacts with Supernode. Copied for convenience when present. |
| `sncli-config.toml` | Optional (used only if `sncli` exists) | Configuration consumed by `sncli`; copied next to the binary when provided. |
| `network-maker` | Optional (required for validators with `"network-maker": true`) | Service that mints and rotates accounts used for Supernode/NFT operations. |
| `nm-config.toml` | Required whenever `network-maker` is bundled | Template applied by `network-maker-setup.sh` to produce `/root/.network-maker/config.toml`. |

> Tip: Keep versioned folders such as `devnet/bin-v1.8.4` in sync with the required binaries so you can point `DEVNET_BIN_DIR` at a tested bundle when reproducing historical upgrades.

### 5.4 Network Maker Multi-Account Support

`devnet/scripts/network-maker-setup.sh` now provisions **multiple** network-maker accounts per validator. The defaults come from `config/config.json`:

```json
"network-maker": {
    "max_accounts": 5,
    "account_balance": "10000000ulume"
}
```

Enable the service on specific validators by setting the flag inside `validators.json`:

```json
{
    "name": "validator1",
    "moniker": "validator1",
    "network-maker": true,
    "port": 26656,
    "rpc_port": 26657
}
```

When the flag is true and the `network-maker` binary/template exist in `devnet/bin`, the setup script:

- Waits for `lumerad` RPC and the Supernode endpoint to become healthy before executing.
- Produces keys named `nm-account`, `nm-account-2`, … up to `max_accounts`, storing mnemonics under `/shared/status/<moniker>/nm_mnemonic[-N]` and addresses inside `/shared/status/<moniker>/nm-address`.
- Funds any empty account from the validator’s genesis key (`/shared/status/<moniker>/genesis-address`) with the configured `account_balance` and confirms the transactions on-chain.
- Writes the final addresses into `/root/.network-maker/config.toml` via repeated `[[keyring.accounts]]` blocks and records scanner directories `/root/nm-files` plus `/shared/nm-files` for convenient document drop zones.

Adjust `max_accounts` when you need additional faucet-style wallets and update `account_balance` with a denom-suffixed amount (or a raw number that automatically adds the staking denom). This allows automated funding for Supernode scenarios or any devnet tests that require multiple funded signers per validator container.

## 6. Usage Guide

### 6.1 Build and Setup

> Be sure there is only ONE version of the `wasmvm` go package.
> ```bash
> ls -l ~/go/pkg/mod/github.com/\!cosm\!wasm/wasmvm/
> total 4.0K
> dr-xr-xr-x 13 user user 4.0K Oct 22 13:56 v2@v2.1.2
> ```
> If you have multiple versions, remove all of them and run `make devnet-build` again.  

#### Full build process

1. If using pre-existing genesis file
```bash
make devnet-build EXTERNAL_CLAIMS_FILE=/paht-to/claims.csv EXTERNAL_GENESIS_FILE=/paht-to/genesis-template.json
```
> NOTE: if EXTERNAL_GENESIS_FILE is provided, new validators will be added to the existing genesis file

2. Creating fresh genesis file
```bash
make devnet-build EXTERNAL_CLAIMS_FILE=/paht-to/claims.csv EXTERNAL_GENESIS_FILE=/paht-to/genesis-template.json
```
> NOTE: if EXTERNAL_GENESIS_FILE is not provided, new genesis will be generated based on the validators.json file

These will:
1. Downloads WasmVM v2.1.2 library
2. Builds chain binary with Ignite
3. Extracts binary from release archive
4. Copies files to devnet/:
   - lumerad binary
   - libwasmvm.x86_64.so
   - claims.csv
5. Generates network configuration
6. Builds Docker images

#### Clean old data
```bash
make devnet-clean   # Removes /tmp/lumera-devnet/shared and all ~/validator*-data directories
```

### 6.2 Network Operations
```bash
# Start network with console output
make devnet-up

# Start network in background
make devnet-up-detach

# Clean start (stops network, cleans, rebuilds defaults)
make devnet-new

# Stop network and cleanup containers
make devnet-down
```

### 6.3 Network Files Location
```bash
# Final Genesis File
/tmp/lumera-devnet/validator1-data/config/genesis.json   # On host machine
/root/.lumera/config/genesis.json       # Inside containers

# Node IDs, IPs and Ports
/tmp/lumera-devnet/shared/validator*_nodeid              # Node IDs 
/tmp/lumera-devnet/shared/validator*_ip                  # Container IPs
/tmp/lumera-devnet/shared/validator*_port                  # Container P2P Ports
```

### 6.4 Joining New Node to Network

#### 1. Get Validator Info
```bash
# Get node ID
VALIDATOR1_ID=$(cat /tmp/lumera-devnet/shared/validator1_nodeid)
# e.g., 4cb8e2eb7bb90fd026e02f693927230fe3fb9c89

# Get container IP
VALIDATOR1_IP=$(cat /tmp/lumera-devnet/shared/validator1_ip) 
# e.g., 172.20.0.2
```

> NOTE: It might be better to use `localhost` instead of validators internal docker IP address.
> ```bash
> VALIDATOR1_IP=localhost
> ```


#### 2. Initialize New Node
```bash
# Clean previous data (if needed)
rm -rf ~/.lumerad

# Initialize node with same chain-id
lumerad init my-local-node --chain-id lumera-devnet
```

#### 3. Copy Genesis
```bash
# Copy from validator1's data directory /shared
cp /tmp/lumera-devnet/validator1-data/config/genesis.json ~/.lumera/config/
```

#### 4. Start Node
```bash
# Start with container IP peer connection
lumerad start --minimum-gas-prices 0ulume \
    --p2p.persistent_peers "${VALIDATOR1_ID}@${VALIDATOR1_IP}:26656" \
    --p2p.laddr tcp://0.0.0.0:26626 \
    --rpc.laddr tcp://127.0.0.1:26627
```

*Note: You can use any validator as a peer by using their respective node ID and IP from the shared directory.*

### 6.5 Verify Connection
```bash
# Check peer connections
lumerad net-info

# Check sync status 
lumerad status | jq .SyncInfo
```

### 6.6 CLI Sessions

#### Access Container Shell
```bash
# Access primary validator
docker exec -it lumera-validator1 bash

# Access any validator (n = 1-5)
docker exec -it lumera-validator{n} bash
```

#### Direct CLI Commands
```bash
# Primary validator commands
docker exec -it lumera-validator1 lumerad keys list --keyring-backend test
docker exec -it lumera-validator1 lumerad query bank balances <address>

# Secondary validator commands (n = 2-5)
docker exec -it lumera-validator{n} lumerad status
```

#### Interactive CLI Sessions
From inside container after `docker exec -it lumera-validator1 bash`:
```bash
# Query commands
lumerad query bank balances <address>
lumerad query staking validators
lumerad query gov proposals

# Transaction commands
lumerad tx bank send <from> <to> 1000ulume --chain-id lumera-devnet --keyring-backend test
```

#### Common Operations
```bash
# View logs
docker exec -it lumera-validator1 tail -f /root/.lumera/lumera.log

# Check config
docker exec -it lumera-validator1 cat /root/.lumera/config/config.toml

# Monitor sync status
docker exec -it lumera-validator1 watch 'lumerad status | jq .SyncInfo'
```

*Note: The keyring-backend is set to 'test' in the dev environment, so no password prompts appear. For production, use 'file' or 'os' backend.*

### 6.7 Devnet Makefile Commands

Targets declared in `Makefile.devnet` (and exposed through the root `Makefile`) control the Docker test system end-to-end.

| Command | Description |
| --- | --- |
| `make devnet-build` | Build Lumera, copy `lumerad`/`libwasmvm.x86_64.so` into `/tmp/<chain-id>/shared/release`, and rerun the generators with the active `config.json`/`validators.json`. |
| `make devnet-build-default` | Run `devnet-build` with the repository default config, validators, genesis template, and claims CSV. |
| `make devnet-build-172` | Use the legacy `devnet/bin-v1.7.2` bundle and default configs to reproduce the v1.7.2 network. |
| `make devnet-up` | Start Docker Compose in the foreground with `START_MODE=auto` so logs stream to the terminal. |
| `make devnet-up-detach` | Start Docker Compose in the background (`docker compose up -d`). |
| `make devnet-down` | Stop the stack and remove containers (`docker compose down --remove-orphans`). |
| `make devnet-stop` | Gracefully stop containers without removing them. |
| `make devnet-start` | Start previously stopped containers with `START_MODE=run`. |
| `make devnet-reset` | Clear each validator’s `genesis.json` and `priv_validator_key.json`, then restart to rebuild `gentx`. |
| `make devnet-clean` | Remove `/tmp/<chain-id>/shared`, validator data folders, Hermes volumes, and the generated `docker-compose.yml`. |
| `make devnet-new` | Convenience target: `devnet-down` + `devnet-clean` + `devnet-build-default`. |
| `make devnet-new-172` | Clean and rebuild the network using the v1.7.2 binary bundle, then start it. |
| `make devnet-upgrade` | Rebuild binaries (if requested), stop containers, refresh `/shared/release`, and rerun `configure.sh`. |
| `make devnet-upgrade-binaries` | Copy freshly built `lumerad` and `libwasmvm` into running containers through `devnet/scripts/upgrade-binaries.sh`. |
| `make devnet-upgrade-180` | Execute `devnet/scripts/upgrade.sh` for the v1.8.0 release bundle. |
| `make devnet-upgrade-184` | Execute `devnet/scripts/upgrade.sh` for the v1.8.4 release bundle. |
| `make devnet-update-scripts` | Copy updated `start.sh`, `validator-setup.sh`, `supernode-setup.sh`, and `network-maker-setup.sh` (plus Hermes scripts) into running containers. |
| `make devnet-deploy-tar` | Package dockerfile, compose file, binaries, configs, claims, and optional genesis into `devnet-deploy.tar.gz` for distribution. |
