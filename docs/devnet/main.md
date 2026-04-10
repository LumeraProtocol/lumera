# Lumera Devnet

## Overview

The Lumera devnet is a Docker-based local test network that runs 5 validator nodes, optional Supernode services, Lumera Uploader, and an IBC Hermes relayer with a companion `simd` chain. It is driven entirely by two JSON configuration files (`config.json` and `validators.json`) and orchestrated through `Makefile.devnet`.

### Key capabilities

- Configuration-driven network generation (any number of validators)
- Automated peer discovery, genesis assembly, and account funding
- Integrated Supernode registration and Lumera Uploader account provisioning
- IBC relayer (Hermes) with a local Cosmos SDK test chain (`simd`)
- Software-upgrade testing with versioned binary bundles
- EVM migration end-to-end test harness

## Documentation index

| Document | Description |
| --- | --- |
| [configuration.md](configuration.md) | `config.json`, `validators.json`, `binaries.json` reference |
| [makefile-commands.md](makefile-commands.md) | All `make devnet-*` targets |
| [upgrade-testing.md](upgrade-testing.md) | Software-upgrade workflow, binary bundles, version-specific builds |
| [hermes.md](hermes.md) | Hermes IBC relayer and `simd` companion chain |
| [lumera-uploader.md](lumera-uploader.md) | Lumera Uploader (formerly network-maker) multi-account service |
| [supernode.md](supernode.md) | Supernode setup, on-chain registration, and `sncli` |
| [tests.md](tests.md) | Validator, Hermes, and EVM migration devnet tests |

### Related EVM integration docs

| Document | Description |
| --- | --- |
| [../evm-integration/evmigration/devnet-tests.md](../evm-integration/evmigration/devnet-tests.md) | `tests_evmigration` end-to-end migration test tool |
| [../evm-integration/testing/tests.md](../evm-integration/testing/tests.md) | Full test inventory (unit, integration, devnet) |
| [../lumera-ports.md](../lumera-ports.md) | Port defaults, config keys, and CLI flags for `lumerad` |

## Architecture

### Service topology

The devnet runs the following containers on a single Docker bridge network (`172.28.0.0/24`):

| Service | Container | IP | Role |
| --- | --- | --- | --- |
| `supernova_validator_1` | `lumera-supernova_validator_1` | `172.28.0.11` | Primary validator (genesis creator) |
| `supernova_validator_2..5` | `lumera-supernova_validator_N` | `172.28.0.12..15` | Secondary validators |
| `hermes` | `lumera-hermes` | `172.28.0.10` | IBC relayer + `simd` chain |

Each validator container runs:
1. **`start.sh`** -- entrypoint that orchestrates all other scripts
2. **`validator-setup.sh`** -- genesis assembly, gentx, account creation
3. **`supernode-setup.sh`** -- Supernode key management, on-chain registration, `sncli` setup
4. **`lumera-uploader-setup.sh`** -- Uploader account provisioning and binary lifecycle

### Boot sequence

```
start.sh (entrypoint)
  |
  +-- [background] lumera-uploader-setup.sh
  |     waits for lumerad RPC + Supernode readiness
  |     installs binary, creates/funds accounts, starts process
  |
  +-- [background] supernode-setup.sh
  |     installs supernode binary
  |     waits for lumerad RPC
  |     creates keys, funds account, registers on-chain
  |     configures sncli, starts supernode process
  |
  +-- [background] validator-setup.sh
  |     initializes chain, creates genesis accounts
  |     assembles genesis from all validators
  |     writes setup_complete flag
  |
  +-- wait for setup_complete
  +-- start lumerad
  +-- start nginx (Uploader UI, if present)
  +-- tail logs
```

### Shared volume layout

All containers mount `/tmp/<chain-id>/shared/` as `/shared/`:

```
/shared/
  config/
    config.json             # Global chain parameters
    validators.json         # Per-validator specs
  release/
    lumerad                 # Chain binary
    libwasmvm.x86_64.so    # CosmWasm runtime library
    supernode-linux-amd64   # Supernode binary (optional)
    lumera-uploader         # Uploader binary (optional, or network-maker for <v1.11.0)
    uploader-config.toml    # Uploader config template (optional)
    uploader-ui/            # Uploader static web UI (optional)
    sncli                   # Supernode CLI tool (optional)
    sncli-config.toml       # sncli config template (optional)
  status/
    setup_complete          # Global flag: genesis is ready
    <moniker>/
      setup_complete        # Per-validator flag
      accounts.json         # Account registry (addresses, mnemonics, funding txs)
      nm_mnemonic[-N]       # Uploader account mnemonics
      nm-address            # Uploader account addresses
  hermes/
    lumera-hermes-relayer.mnemonic  # Hermes relayer key
```

### Devnet binary bundle (`devnet/bin`)

`make devnet-build` copies these from `devnet/bin/` into `/shared/release/`:

| File | Required | Purpose |
| --- | --- | --- |
| `lumerad` | Yes | Primary chain daemon |
| `libwasmvm.x86_64.so` | Yes | CosmWasm runtime shared library |
| `supernode-linux-amd64` | Optional | Supernode binary |
| `sncli` | Optional | Supernode CLI utility |
| `sncli-config.toml` | Optional | sncli configuration template |
| `lumera-uploader` | Optional | Uploader service (or `network-maker` for < v1.11.0) |
| `uploader-config.toml` | Required if uploader is bundled | Uploader config template |
| `uploader-ui/` | Optional | Uploader static web UI served by nginx |

> **Tip:** Keep versioned folders such as `devnet/bin-v1.8.4` in sync with the required binaries so you can point `DEVNET_BIN_DIR` at a tested bundle when reproducing historical upgrades.

### Port mapping

Each validator gets unique host ports to avoid conflicts:

| Service | Container port | Host port formula (N = 1..5) |
| --- | --- | --- |
| P2P | 26656 | 26656 + 10*(N-1) |
| RPC | 26657 | 26657 + 10*(N-1) |
| REST API | 1317 | 1317 + 10*(N-1) |
| gRPC | 9090 | 9090 + (N-1) |
| Supernode gRPC | 4444 | 7441 + 2*(N-1) |
| Supernode P2P | 4445 | 7442 + 2*(N-1) |
| Supernode gateway | 8002 | 18001 + (N-1) |
| EVM JSON-RPC | 8545 | 8545 + 10*(N-1) |
| EVM WebSocket | 8546 | 8546 + 10*(N-1) |

Hermes/simd uses offset ports: P2P 36656, RPC 36657, API 31317, gRPC 39090.

See [../lumera-ports.md](../lumera-ports.md) for `lumerad` port defaults and config keys.

## Quick start

```bash
# Full clean build + start (foreground with log streaming)
make devnet-new

# Or step-by-step:
make devnet-build-default   # Build binaries, generate configs, build Docker images
make devnet-up              # Start in foreground
make devnet-up-detach       # Start in background

# Access a validator shell
docker exec -it lumera-supernova_validator_1 bash

# Common queries inside the container
lumerad status | jq .SyncInfo
lumerad query bank balances <address>
lumerad query staking validators
```

### Joining a new node to the network

```bash
# 1. Get validator info from shared volume
VALIDATOR1_ID=$(cat /tmp/lumera-devnet-1/shared/supernova_validator_1_nodeid)
VALIDATOR1_IP=localhost

# 2. Initialize
lumerad init my-local-node --chain-id lumera-devnet-1

# 3. Copy genesis
cp /tmp/lumera-devnet-1/supernova_validator_1-data/config/genesis.json ~/.lumera/config/

# 4. Start
lumerad start --minimum-gas-prices 0ulume \
    --p2p.persistent_peers "${VALIDATOR1_ID}@${VALIDATOR1_IP}:26656" \
    --p2p.laddr tcp://0.0.0.0:26626 \
    --rpc.laddr tcp://127.0.0.1:26627

# 5. Verify
lumerad status | jq .SyncInfo
```

## Core components

### Config package (`devnet/config/config.go`)

Go structs that deserialize `config.json` and `validators.json`. Used by the generator package to produce Docker Compose files. See [configuration.md](configuration.md).

### Generators package (`devnet/generators/`)

| Generator | Purpose |
| --- | --- |
| `docker-compose.go` | Produces `docker-compose.yml` with service definitions, port mappings, volumes, and dependencies |
| `config.go` | Default port constants and environment variable names |

### Scripts (`devnet/scripts/`)

| Script | Runs on | Purpose |
| --- | --- | --- |
| `start.sh` | Container | Entrypoint: orchestrates setup scripts, starts `lumerad`, tails logs |
| `stop.sh` | Container | Stops services by name (`nm`, `sn`, `lumera`, `nginx`, `all`) |
| `restart.sh` | Container | Stops + starts services |
| `validator-setup.sh` | Container | Genesis assembly, account creation, gentx collection |
| `supernode-setup.sh` | Container | Supernode key management, registration, sncli setup |
| `lumera-uploader-setup.sh` | Container | Uploader account provisioning and binary lifecycle |
| `configure.sh` | Host | Copies configs and binaries into the shared volume |
| `download-binaries.sh` | Host | Downloads versioned binaries from GitHub releases |
| `upgrade.sh` | Host | Orchestrates a full software-upgrade workflow |
| `upgrade-binaries.sh` | Host | Stops containers, swaps binaries, restarts |
| `common.sh` | Container | Shared utilities (version comparison, tx waiting, name resolution) |
| `account-registry.sh` | Container | JSON-based account registry for cross-script coordination |
| `lumera-helper.sh` | Container | Additional chain interaction helpers |

### Dockerfile (`devnet/dockerfile`)

Based on `debian:trixie-slim`. Installs system tools (`jq`, `crudini`, `nginx-light`, `ripgrep`, `Node.js`) and copies all setup scripts. Entrypoint: `/root/scripts/start.sh`.

Exposed ports: `26656 26657 1317 9090 4444 8002 50051 8080 8088`.
