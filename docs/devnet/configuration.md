# Devnet Configuration Reference

This document describes the JSON configuration files that drive the Lumera devnet. All files live under `devnet/config/`.

## config.json

Global chain parameters shared by every validator. Loaded by `devnet/config/config.go` (`ChainConfig` struct) and read by shell scripts via `jq`.

### Full schema

```json
{
    "chain": {
        "id": "lumera-devnet-1",
        "evm_from_version": "v1.20.0",
        "denom": {
            "bond": "ulume",
            "mint": "ulume",
            "minimum_gas_price": "0.025ulume"
        }
    },
    "docker": {
        "network_name": "lumera-network",
        "container_prefix": "lumera",
        "volume_prefix": "lumera"
    },
    "paths": {
        "base": {
            "host": "~",
            "container": "/root"
        },
        "directories": {
            "daemon": ".lumera"
        }
    },
    "daemon": {
        "binary": "lumerad",
        "keyring_backend": "test"
    },
    "genesis-account-mnemonics": [ "..." ],
    "sn-account-mnemonics": [ "..." ],
    "api": {
        "enable_unsafe_cors": true
    },
    "json-rpc": {
        "enable": true,
        "address": "0.0.0.0:8545",
        "ws_address": "0.0.0.0:8546",
        "api": "web3,eth,personal,net,txpool,debug,rpc",
        "enable_indexer": true
    },
    "lumera-uploader": {
        "enabled": true,
        "grpc_port": 50051,
        "http_port": 8080,
        "max_accounts": 3,
        "account_balance": "10000000ulume"
    },
    "hermes": {
        "enabled": false
    }
}
```

### Field reference

#### `chain`

| Field | Type | Description |
| --- | --- | --- |
| `id` | string | Chain ID used by all validators and in genesis |
| `evm_from_version` | string | First lumerad version that activates EVM key style. Used by scripts to decide `secp256k1` vs `eth_secp256k1` key derivation. Default: `v1.20.0` |
| `denom.bond` | string | Staking/bond denomination |
| `denom.mint` | string | Minting denomination |
| `denom.minimum_gas_price` | string | Minimum gas price with denom suffix |

#### `docker`

| Field | Type | Description |
| --- | --- | --- |
| `network_name` | string | Docker bridge network name |
| `container_prefix` | string | Prefix for container names |
| `volume_prefix` | string | Prefix for volume names |

#### `paths`

| Field | Type | Description |
| --- | --- | --- |
| `base.host` | string | Base path on the host machine |
| `base.container` | string | Base path inside containers |
| `directories.daemon` | string | Daemon home directory name (relative to base) |

#### `daemon`

| Field | Type | Description |
| --- | --- | --- |
| `binary` | string | Daemon binary name |
| `keyring_backend` | string | Keyring backend (`test`, `file`, or `os`) |

#### `genesis-account-mnemonics`

Array of BIP-39 mnemonics. Each validator gets one mnemonic (by index) for its genesis account. Must have at least as many entries as validators.

#### `sn-account-mnemonics`

Array of BIP-39 mnemonics for Supernode and sncli accounts. The first N entries (where N = number of validators) are used for Supernode keys; the next N are used for sncli keys. Must have at least 2 * N entries.

#### `api`

| Field | Type | Description |
| --- | --- | --- |
| `enable_unsafe_cors` | bool | Enable CORS headers on REST API for browser access |

#### `json-rpc`

| Field | Type | Description |
| --- | --- | --- |
| `enable` | bool | Enable EVM JSON-RPC endpoint |
| `address` | string | HTTP listen address |
| `ws_address` | string | WebSocket listen address |
| `api` | string | Comma-separated list of enabled API namespaces |
| `enable_indexer` | bool | Enable EVM transaction indexer |

#### `lumera-uploader`

| Field | Type | Description |
| --- | --- | --- |
| `enabled` | bool | Global enable flag |
| `grpc_port` | int | gRPC listen port (default 50051) |
| `http_port` | int | HTTP gateway listen port (default 8080) |
| `max_accounts` | int | Number of funded uploader accounts to create per validator (minimum 1) |
| `account_balance` | string | Funding amount per account (with or without denom suffix) |

> **Note:** For Lumera < v1.11.0, this section was called `"network-maker"`. Scripts accept both keys for backward compatibility.

#### `hermes`

| Field | Type | Description |
| --- | --- | --- |
| `enabled` | bool | Whether to start the Hermes IBC relayer container |

---

## validators.json

Array of validator specifications. Each entry defines one validator container with its ports, keys, and optional service configurations.

### Full schema (one entry)

```json
{
    "name": "supernova_validator_1",
    "moniker": "supernova_validator_1",
    "key_name": "supernova_validator_1_key",
    "port": 26656,
    "rpc_port": 26657,
    "rest_port": 1317,
    "grpc_port": 9090,
    "primary": true,
    "supernode": {
        "port": 4444,
        "p2p_port": 4445,
        "gateway_port": 8002
    },
    "json-rpc": {
        "port": 8545,
        "ws_port": 8546
    },
    "lumera-uploader": {
        "enabled": true,
        "grpc_port": 50051,
        "http_port": 8080
    },
    "initial_distribution": {
        "account_balance": "2000000000000ulume",
        "validator_stake": "1000000000000ulume"
    }
}
```

### Field reference

| Field | Type | Description |
| --- | --- | --- |
| `name` | string | Docker service name (also used as container suffix) |
| `moniker` | string | CometBFT moniker displayed on the network |
| `key_name` | string | Keyring key identifier for the validator account |
| `port` | int | Host-mapped P2P port |
| `rpc_port` | int | Host-mapped RPC port |
| `rest_port` | int | Host-mapped REST API port |
| `grpc_port` | int | Host-mapped gRPC port |
| `primary` | bool | If `true`, this validator creates genesis and starts first |

#### `supernode` (optional)

| Field | Type | Description |
| --- | --- | --- |
| `port` | int | Supernode gRPC port |
| `p2p_port` | int | Supernode P2P port |
| `gateway_port` | int | Supernode HTTP gateway port |

#### `json-rpc` (optional)

| Field | Type | Description |
| --- | --- | --- |
| `port` | int | EVM JSON-RPC HTTP port |
| `ws_port` | int | EVM JSON-RPC WebSocket port |

#### `lumera-uploader` (optional)

| Field | Type | Description |
| --- | --- | --- |
| `enabled` | bool | Enable uploader on this validator |
| `grpc_port` | int | Override global gRPC port |
| `http_port` | int | Override global HTTP gateway port |

> **Note:** For Lumera < v1.11.0, use `"network-maker"` as the key name. Scripts accept both.

#### `initial_distribution`

| Field | Type | Description |
| --- | --- | --- |
| `account_balance` | string | Total tokens allocated to this validator's genesis account |
| `validator_stake` | string | Tokens self-delegated at genesis |

### Port assignment conventions

- **Primary validator**: Uses standard CometBFT ports (26656, 26657, 1317, 9090)
- **Secondary validators**: Increment ports to avoid host conflicts
  - P2P: +10 per validator (26666, 26676, ...)
  - RPC: +10 per validator (26667, 26677, ...)
  - REST: +10 per validator (1327, 1337, ...)
  - gRPC: +1 per validator (9091, 9092, ...)

---

## binaries.json

Maps Lumera release versions to their download coordinates. Used by `devnet/scripts/download-binaries.sh` to populate versioned `bin-<version>/` directories.

### Schema

```json
{
    "_comment": "Devnet binary versions.",
    "github_org": "LumeraProtocol",
    "versions": {
        "v1.9.1": {
            "bin_dir": "bin-v1.9.1",
            "lumera": { "tag": "v1.9.1" },
            "supernode": { "tag": "v2.4.27" },
            "network_maker": { "tag": "v1.0.7" }
        },
        "v1.11.1": {
            "bin_dir": "bin-v1.11.1",
            "lumera": { "tag": "v1.11.1" },
            "supernode": { "tag": "v2.4.72" },
            "lumera_uploader": { "tag": "" }
        }
    }
}
```

### Field reference

| Field | Type | Description |
| --- | --- | --- |
| `github_org` | string | GitHub organization for all downloads |
| `versions.<ver>.bin_dir` | string | Target directory name under `devnet/` |
| `versions.<ver>.lumera.tag` | string | GitHub release tag for `lumerad` + `libwasmvm` tarball |
| `versions.<ver>.supernode.tag` | string | GitHub release tag for Supernode binary |
| `versions.<ver>.network_maker.tag` | string | GitHub release tag for network-maker (< v1.11.0) |
| `versions.<ver>.lumera_uploader.tag` | string | GitHub release tag for lumera-uploader (>= v1.11.0) |

### Usage

```bash
# Download all binaries for a specific version
./devnet/scripts/download-binaries.sh v1.11.1

# Then build devnet using that bundle
make devnet-build DEVNET_BIN_DIR=devnet/bin-v1.11.1
```

### Version-based binary naming

Starting with Lumera v1.11.0, the "network-maker" project was renamed to "lumera-uploader". The `download-binaries.sh` script handles this automatically:

- `>= v1.11.0`: looks for `lumera_uploader` in `binaries.json`, downloads from `lumera-uploader` GitHub repo
- `< v1.11.0`: looks for `network_maker`, downloads from `network-maker` GitHub repo
