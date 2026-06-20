# Devnet Upgrade Testing

This document covers the software-upgrade workflow, versioned binary bundles, and the EVM upgrade pipeline.

## Overview

Lumera uses Cosmos SDK's `x/upgrade` module for coordinated chain upgrades. The devnet provides a complete end-to-end workflow: submit a governance proposal, vote, wait for the halt height, swap binaries, and restart.

## Versioned binary bundles

Each historical version has a pre-populated binary directory under `devnet/`:

| Directory | Version | Contents |
| --- | --- | --- |
| `devnet/bin-v1.7.2` | v1.7.2 | lumerad, libwasmvm, supernode, network-maker |
| `devnet/bin-v1.9.1` | v1.9.1 | lumerad, libwasmvm, supernode, network-maker |
| `devnet/bin-v1.11.1` | v1.11.1 | lumerad, libwasmvm, supernode |

### Downloading binaries

The `download-binaries.sh` script fetches binaries from GitHub releases based on `devnet/config/binaries.json`:

```bash
# Download all binaries for a specific version
./devnet/scripts/download-binaries.sh v1.11.1

# Binaries are placed in devnet/bin-v1.11.1/
```

See [configuration.md](configuration.md#binariesjson) for the `binaries.json` schema.

## Upgrade workflow

### Scripts involved

| Script | Location | Purpose |
| --- | --- | --- |
| `upgrade.sh` | `devnet/scripts/` | Orchestrates the full upgrade cycle |
| `upgrade-binaries.sh` | `devnet/scripts/` | Stops containers, swaps binaries, restarts |
| `submit-upgrade-proposal.sh` | `devnet/scripts/` | Submits a software-upgrade governance proposal |
| `vote-all.sh` | `devnet/scripts/` | Votes YES on a proposal from all validators |
| `wait-for-height.sh` | `devnet/scripts/` | Blocks until chain reaches a target height |

### `upgrade.sh` execution flow

```
upgrade.sh <release-name> <upgrade-height|auto-height> <binaries-dir>

1. Check if upgrade halt already occurred (docker logs scan)
2. Compare running version to target release
3. Determine upgrade height:
   - Explicit: use provided height
   - auto-height: current_height + 100
4. Submit software-upgrade governance proposal
5. Retrieve proposal ID
6. Vote YES from all validators
7. Wait for chain to halt at upgrade height
8. Run upgrade-binaries.sh to swap and restart
```

### `upgrade-binaries.sh` execution flow

```
upgrade-binaries.sh <binaries-dir> [<expected-release-name>]

1. Verify lumerad version in binaries-dir matches expected release
2. docker compose stop (graceful, 30s timeout)
3. Copy lumerad + libwasmvm from binaries-dir to /shared/release/
4. docker compose up -d --no-build (START_MODE=run)
5. Poll until all services are running (90s timeout)
```

### Timeouts

| Constant | Default | Purpose |
| --- | --- | --- |
| `AUTO_HEIGHT_OFFSET` | 100 | Blocks ahead of current height for auto-height |
| `COMPOSE_STOP_TIMEOUT` | 30s | Docker compose stop grace period |
| `COMPOSE_UP_TIMEOUT` | 120s | Docker compose up command timeout |
| `COMPOSE_READY_TIMEOUT` | 90s | Wait for all services to report running |

## EVM upgrade pipeline

The `make devnet-evm-upgrade` target automates a full upgrade from pre-EVM to EVM-enabled lumerad:

```
1. Build v1.20.0 lumerad binary
2. Submit software-upgrade proposal for "v1.20.0"
3. Vote YES from all 5 validators
4. Wait for chain to halt at upgrade height
5. Copy new lumerad + libwasmvm into containers
6. Restart all containers
7. Wait for chain to resume producing blocks
```

Use this upgrade pipeline, not a fresh EVM init, for release qualification. It preserves the pre-EVM `app.toml` shape and exercises the startup config migration that adds `[evm]`, `[evm.mempool]`, `[json-rpc]`, `[tls]`, and `[lumera.*]`; this is the path that catches legacy no-op mempool settings such as `mempool.max-txs = -1`.

### Running the full EVM migration test

```bash
# 1. Start on pre-EVM version (v1.11.1)
make devnet-new-1111

# 2. Create legacy accounts and on-chain activity
make devnet-evmigration-prepare

# 3. Run the EVM upgrade
make devnet-evm-upgrade

# 4. Execute migrations (parallel mode for speed)
make devnet-evmigrationp-migrate-all

# 5. Verify all state migrated correctly
make devnet-evmigrationp-verify

# 6. Clean up
make devnet-evmigrationp-cleanup
```

See [../evm-integration/evmigration/devnet-tests.md](../evm-integration/evmigration/devnet-tests.md) for comprehensive documentation of the `tests_evmigration` tool, including all operating modes, module coverage, and verification strategies.

## Manual upgrade walkthrough

If you want to perform an upgrade step-by-step instead of using the Makefile:

```bash
# 1. Start old version
make devnet-new-1111

# 2. Enter the primary validator container
docker exec -it lumera-supernova_validator_1 bash

# 3. Submit upgrade proposal (inside container)
lumerad tx upgrade software-upgrade v1.20.0 \
    --title "Upgrade to v1.20.0" \
    --summary "EVM support" \
    --upgrade-height 200 \
    --from supernova_validator_1_key \
    --keyring-backend test \
    --chain-id lumera-devnet-1 \
    --yes

# 4. Vote from all validators
for i in 1 2 3 4 5; do
    docker exec lumera-supernova_validator_$i \
        lumerad tx gov vote 1 yes \
            --from supernova_validator_${i}_key \
            --keyring-backend test \
            --chain-id lumera-devnet-1 \
            --yes
done

# 5. Wait for halt, then swap binaries and restart
make devnet-upgrade-binaries DEVNET_BIN_DIR=devnet/bin-v1.20.0
```

## Version matching

The upgrade scripts use relaxed version matching:

- Exact match: `v1.20.0` == `v1.20.0`
- Core version match: `v1.20.0-rc1` matches expected `v1.20.0` (pre-release suffixes are stripped for comparison)

This is handled by `versions_match()` in `upgrade-binaries.sh` and `release_core_version()` in `common.sh`.
