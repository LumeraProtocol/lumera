# Supernode Setup

Each devnet validator runs a Supernode process alongside `lumerad`. The Supernode handles block production coordination, data storage, and price feed aggregation. Setup is managed by `devnet/scripts/supernode-setup.sh`.

## Architecture

```
validator container
  |
  +-- lumerad (chain daemon)
  |     Handles consensus, state machine, tx processing
  |
  +-- supernode (Supernode process)
  |     Registered on-chain via MsgRegisterSupernode
  |     Connects to lumerad via gRPC
  |     Exposes gRPC + P2P + HTTP gateway
  |
  +-- sncli (optional CLI tool)
        Interacts with Supernode for debugging/management
```

## Setup sequence

The `supernode-setup.sh` script runs these steps in order:

```
1.  Check prerequisites (crudini, jq installed)
2.  Stop any leftover supernode process
3.  Install supernode binary from /shared/release/
4.  Install sncli binary (optional)
5.  Wait for lumerad RPC (180s timeout)
6.  Wait for block height >= 5 (chain stability)
7.  Update gas prices for EVM feemarket (if applicable)
8.  Load pre-configured mnemonics from config.json
9.  Configure supernode (keys, config.yml, fund account)
10. Register supernode on-chain (MsgRegisterSupernode)
11. Configure sncli (keys, config.toml, fund account)
12. Start supernode process in background
```

## Key management

### Key naming convention

| Key | Name pattern | Example |
| --- | --- | --- |
| Validator key | `<moniker>_key` | `supernova_validator_1_key` |
| Supernode key | `<moniker with 'validator' replaced by 'supernode'>_key` | `supernova_supernode_1_key` |
| sncli key | `sncli-account` | `sncli-account` |

### EVM key migration

The Supernode supports dual key types for EVM migration:

| Chain version | Key type | HD path | Coin type |
| --- | --- | --- | --- |
| Pre-EVM (< v1.20.0) | `secp256k1` | `m/44'/118'/0'/0/0` | 118 (Cosmos) |
| Post-EVM (>= v1.20.0) | `eth_secp256k1` | `m/44'/60'/0'/0/0` | 60 (Ethereum) |

The setup script maintains two keyrings:
1. **Daemon keyring** (`~/.lumera/keys/`) -- used by `lumerad` for tx signing
2. **Supernode keyring** (`~/.supernode/keys/`) -- used by the Supernode process

When EVM is detected, both legacy and EVM keys are derived. The Supernode config tracks:
- `key_name`: Current active key
- `evm_key_name`: EVM-derived key (set during migration, cleared after completion)

### Mnemonic sources

Mnemonics are loaded in priority order:
1. **Pre-configured** in `config.json` under `sn-account-mnemonics` (deterministic, preferred)
2. **Saved file** at `/shared/status/<moniker>/sn_mnemonic`
3. **Generated** on first run (new random mnemonic)

The `sn-account-mnemonics` array is split by validator index:
- First N entries: Supernode keys (index = validator position in `validators.json`)
- Next N entries: sncli keys

## Supernode configuration (`config.yml`)

The setup script manages `~/.supernode/config.yml` using awk-based helpers:

```yaml
supernode:
  key_name: "supernova_supernode_1_key"
  evm_key_name: ""                       # Set during EVM migration
  identity: "lumera1..."                  # On-chain account address
```

Key config fields are set via `set_supernode_config_value()` and read via `get_supernode_config_value()`.

## On-chain registration

After key setup and funding, the script registers the Supernode:

```bash
lumerad tx supernode register-supernode \
    --from <supernode_key> \
    --keyring-backend test \
    --chain-id lumera-devnet-1 \
    --yes
```

The script checks if the Supernode is already registered and active before attempting registration. Registration state is queried via:

```bash
lumerad query supernode get-supernode <valoper_address>
```

## sncli configuration

If the `sncli` binary is present in `/shared/release/`, the setup script also configures it:

### Binary installation

Sources (in priority order):
1. `/shared/release/sncli`
2. `/shared/release/sncli-linux-amd64`

Installed to: `/usr/local/bin/sncli`

### Config file (`~/.sncli/config.toml`)

```toml
[lumera]
grpc_addr = "localhost:9090"
chain_id = "lumera-devnet-1"

[supernode]
address = "lumera1..."              # Supernode's on-chain address
grpc_endpoint = "172.28.0.11:4444"  # Supernode gRPC (container IP)
p2p_endpoint = "172.28.0.11:4445"   # Supernode P2P

[keyring]
backend = "test"
key_name = "sncli-account"
local_address = "lumera1..."        # sncli's own funded address
```

### Funding

| Parameter | Default |
| --- | --- |
| `SNCLI_FUND_AMOUNT` | `100000ulume` |
| `SNCLI_MIN_AMOUNT` | `10000ulume` |

The sncli account is funded from the validator's genesis account if its balance is below `SNCLI_MIN_AMOUNT`.

## Ports

| Service | Default port | Description |
| --- | --- | --- |
| Supernode gRPC | 4444 | Main Supernode API |
| Supernode P2P | 4445 | Peer-to-peer communication |
| Supernode gateway | 8002 | HTTP gateway |

Host port mapping per validator (N = 1..5):

| Service | Host port formula |
| --- | --- |
| gRPC | 7441 + 2*(N-1) |
| P2P | 7442 + 2*(N-1) |
| Gateway | 18001 + (N-1) |

## Environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `MONIKER` | (required) | Validator moniker |
| `SUPERNODE_PORT` | `4444` | Supernode gRPC port |
| `SUPERNODE_P2P_PORT` | `4445` | Supernode P2P port |
| `SUPERNODE_GATEWAY_PORT` | `8002` | Supernode HTTP gateway port |
| `TX_GAS_PRICES` | `0.03ulume` | Cosmos tx gas prices in `ulume` (numeric price auto-updated for EVM feemarket) |
| `LUMERA_VERSION` | auto-detected | Lumerad version hint |
| `LUMERA_FIRST_EVM_VERSION` | `v1.20.0` | EVM cutover version |
| `INTEGRATION_TEST` | `true` (in compose) | Integration test mode flag |

## EVM feemarket integration

When EVM is active, the feemarket module enforces dynamic base fees. The setup script queries the feemarket parameters and adjusts the numeric `TX_GAS_PRICES` value to 2x the current base fee to ensure transactions are accepted under fee fluctuations. Cosmos tx fees remain in `ulume`; `alume` is the EVM-side 18-decimal representation.

```bash
# The script queries:
lumerad query feemarket params --output json
# And sets TX_GAS_PRICES = 2 * base_fee + ulume
```

## Devnet tests

Validator-specific devnet tests live under `devnet/tests/validator/`:

| Test file | Coverage |
| --- | --- |
| `evm_test.go` | EVM functionality |
| `ibc_test.go` | IBC integration |
| `lep5_test.go` | LEP-5 protocol |
| `ports_test.go` | Port configuration verification |

## Troubleshooting

### Supernode not starting

Check the setup log:
```bash
docker exec lumera-supernova_validator_1 cat /root/logs/supernode-setup.out
```

### Registration failed

Check if the validator account has sufficient funds:
```bash
docker exec lumera-supernova_validator_1 \
    lumerad query bank balances $(lumerad keys show supernova_supernode_1_key -a --keyring-backend test)
```

### Key type mismatch after EVM upgrade

Check the current key type:
```bash
docker exec lumera-supernova_validator_1 \
    lumerad keys show supernova_supernode_1_key --keyring-backend test --output json | jq .pubkey
```

The `@type` field should contain `ethsecp256k1` after EVM migration.
