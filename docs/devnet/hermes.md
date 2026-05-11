# Hermes IBC Relayer

The devnet includes an optional Hermes IBC relayer that operates a dual-chain relay between Lumera and a local Cosmos SDK test chain (`simd`). Both services run inside the `lumera-hermes` container.

## Architecture

```
lumera-hermes container (172.28.0.10)
  |
  +-- simd (Cosmos SDK simapp)
  |     Local test chain for IBC counterparty
  |     Chain ID: hermes-simd-1
  |     Denom: stake
  |
  +-- hermes (IBC relayer daemon)
        Relays packets between Lumera and simd
        Creates clients, connections, and channels
```

The Hermes container depends on `supernova_validator_1` and waits for the Lumera RPC to become reachable before starting.

## Files

| File | Purpose |
| --- | --- |
| `devnet/hermes/Dockerfile` | Builds the Hermes container (golang builder + debian runtime) |
| `devnet/hermes/config.toml` | Hermes config template (modes, telemetry, client refresh) |
| `devnet/hermes/scripts/hermes-start.sh` | Main entrypoint: orchestrates simd + Hermes startup |
| `devnet/hermes/scripts/init-simapp.sh` | Initializes simd chain (genesis, validator key, accounts) |
| `devnet/hermes/scripts/hermes-configure.sh` | Generates Hermes config from template with chain-specific values |
| `devnet/hermes/scripts/hermes-channel.sh` | Establishes IBC clients, connections, and channels |

## Startup sequence

The `hermes-start.sh` entrypoint runs through these phases:

1. **Environment setup** -- load chain config from `/shared/config/config.json`, detect EVM version, determine key style
2. **Initialize simd** -- run `init-simapp.sh` to create the simd chain with validator key, genesis accounts, and initial balances
3. **Start simd** -- launch `simd start` in the background with full archive pruning
4. **Wait for Lumera** -- poll Lumera RPC on `supernova_validator_1:26657` (120s timeout)
5. **Wait for simd** -- poll simd RPC until it reaches block height 1 (120s timeout)
6. **Wait for Lumera blocks** -- wait for 5 blocks to ensure chain stability
7. **Fund accounts** -- fund simd test and relayer accounts
8. **Configure Hermes** -- generate config from template, add chain entries
9. **Create IBC channels** -- establish clients, connections, and channels between both chains
10. **Start Hermes** -- launch the relayer daemon and monitor logs

## Port mapping

| Service | Container port | Host port |
| --- | --- | --- |
| simd P2P | 26656 | 36656 |
| simd RPC | 26657 | 36657 |
| simd REST API | 1317 | 31317 |
| simd gRPC | 9090 | 39090 |
| simd gRPC-Web | 9091 | 39091 |

## Environment variables

### Simd configuration

| Variable | Default | Description |
| --- | --- | --- |
| `SIMD_HOME` | `/root/.simd` | Simd data directory |
| `SIMD_MONIKER` | `hermes-simd` | Simd validator moniker |
| `SIMD_CHAIN_ID` | `hermes-simd-1` | Simd chain ID |
| `SIMD_KEY_NAME` | `validator` | Simd validator key name |
| `SIMD_KEYRING` | `test` | Keyring backend |
| `SIMD_DENOM` | `stake` | Native denomination |
| `SIMD_GENESIS_BALANCE` | `100000000000stake` | Validator genesis balance |
| `SIMD_STAKE_AMOUNT` | `50000000000stake` | Self-delegation amount |
| `SIMD_TEST_KEY_NAME` | `simd-test` | Test account key name |
| `SIMD_TEST_ACCOUNT_BALANCE` | `100000000stake` | Test account funding |
| `SIMD_RELAYER_ACCOUNT_BALANCE` | `100000000stake` | Relayer account funding on simd |

### Lumera configuration

| Variable | Default | Description |
| --- | --- | --- |
| `LUMERA_CHAIN_ID` | from `config.json` | Lumera chain ID |
| `LUMERA_BOND_DENOM` | from `config.json` | Lumera bond denomination |
| `LUMERA_FIRST_EVM_VERSION` | `v1.20.0` | EVM cutover version for key style selection |
| `LUMERA_VERSION` | auto-detected | Running lumerad version |
| `LUMERA_KEY_STYLE` | auto-detected | `cosmos` (secp256k1) or `evm` (ethsecp256k1) |
| `LUMERA_RPC_PORT` | `26657` | Lumera RPC port (inside compose network) |
| `LUMERA_GRPC_PORT` | `9090` | Lumera gRPC port |
| `LUMERA_ACCOUNT_PREFIX` | `lumera` | Bech32 account prefix |

### Hermes configuration

| Variable | Default | Description |
| --- | --- | --- |
| `HERMES_CONFIG` | `/root/.hermes/config.toml` | Generated config path |
| `HERMES_TEMPLATE_PATH` | `/root/scripts/hermes-config-template.toml` | Config template |
| `HERMES_KEY_NAME` | `relayer` | Key name used by Hermes on both chains |

## Config template

The Hermes config template (`devnet/hermes/config.toml`) defines global relay behavior:

```toml
[global]
log_level = 'info'

[mode.clients]
enabled = true
refresh = true          # Auto-refresh clients at 2/3 of trusting period

[mode.connections]
enabled = true

[mode.channels]
enabled = true

[mode.packets]
enabled = true
clear_interval = 10     # Clear pending packets every 10 blocks
clear_on_start = true   # Clear pending packets on startup
tx_confirmation = true  # Confirm tx via /tx_search

[rest]
enabled = false

[telemetry]
enabled = true
host = '127.0.0.1'
port = 3001
```

Chain-specific `[[chains]]` blocks are added dynamically by `hermes-configure.sh`:

```toml
[[chains]]
id = 'lumera-devnet-1'
type = 'CosmosSdk'
rpc_addr = 'http://supernova_validator_1:26657'
grpc_addr = 'http://supernova_validator_1:9090'
event_source = { mode = 'push', url = 'ws://supernova_validator_1:26657/websocket' }
account_prefix = 'lumera'
address_type = { derivation = 'cosmos' }   # or 'ethermint' for EVM chains
key_name = 'relayer'
gas_price = { price = 0.025, denom = 'ulume' }
max_gas = 1000000
trusting_period = '14days'
```

## Key management

The relayer uses a single mnemonic for both chains:

1. **Mnemonic file**: `/shared/hermes/lumera-hermes-relayer.mnemonic`
2. **Legacy fallback**: `/shared/hermes/hermes-relayer.mnemonic`
3. **Key name**: `relayer` (same on both Lumera and simd)

### EVM key style

When the Lumera chain version >= `LUMERA_FIRST_EVM_VERSION`:
- `address_type` is set to `{ derivation = 'ethermint' }` in the Hermes config
- HD path uses coin-type 60 (`m/44'/60'/0'/0/0`)

Otherwise:
- `address_type` is `{ derivation = 'cosmos' }`
- HD path uses coin-type 118 (`m/44'/118'/0'/0/0`)

## Validator selection

Hermes connects to the first validator that has `"lumera-uploader"` (or `"network-maker"`) enabled in `validators.json`. This is typically `supernova_validator_3`. If no validator has the flag, it falls back to the first validator in the list.

## Docker image

Built from `devnet/hermes/Dockerfile`:

- **Builder stage**: `golang:1.25.5-bookworm` -- builds `simd` from ibc-go v10.5.0 source
- **Runtime stage**: `debian:trixie-slim` with `jq`, `crudini`, `curl`, `net-tools`
- **Hermes binary**: Pre-compiled v1.13.3 downloaded from GitHub releases
- **Entrypoint**: `/root/scripts/hermes-start.sh`

## Devnet tests

IBC-specific devnet tests live under `devnet/tests/hermes/`:

| Test file | Coverage |
| --- | --- |
| `ibc_test.go` | IBC token transfer packet relay |
| `ibc_ica_test.go` | Interchain Accounts integration |
| `ibc_ica_app_pubkey_test.go` | ICA application pubkey handling |

These tests require the Hermes container to be running. Build and run with:

```bash
make devnet-tests-build
# Tests are executed inside the Hermes container
```

## Troubleshooting

### Hermes fails to connect to Lumera

Check that `supernova_validator_1` is producing blocks:
```bash
docker logs lumera-supernova_validator_1 --tail 20
curl http://localhost:26667/status | jq .result.sync_info.latest_block_height
```

### IBC channel creation fails

Check Hermes logs for client/connection errors:
```bash
docker exec lumera-hermes cat /root/logs/hermes.log | tail -50
```

### simd not starting

Check simd initialization log:
```bash
docker exec lumera-hermes cat /root/logs/simapp-init.log
```
