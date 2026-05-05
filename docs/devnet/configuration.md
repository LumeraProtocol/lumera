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
    "multisig": {
        "enabled": true,
        "threshold": 2,
        "signer_count": 3,
        "vesting_type": "PermanentLocked"
    },
    "test_accounts": {
        "count": 5,
        "balance_base": "20000ulume",
        "balance_increment": "10000ulume",
        "multisig": true
    },
    "initial_distribution": {
        "account_balance": "2000000000000ulume",
        "validator_stake": "1000000000000ulume"
    }
}
```

> **Note:** All sub-objects except `initial_distribution` are optional and use `omitempty`. Real validators rarely set every block — see `devnet/config/validators.json` for the canonical examples (V2 carries `multisig` + multisig-flagged `test_accounts`; V3 carries `lumera-uploader`; V4 carries single-sig `test_accounts`).

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

#### `multisig` (optional)

Wraps this validator's genesis account as a multisig account at chain-init time. When `enabled: true`, the validator's `key_name` is registered as a multisig key composed of `signer_count` deterministically-generated signer keys with the given `threshold`. Used by the EVM-migration test suites (`tests_evmigration -mode=multisig*`) and end-user multisig migration walkthroughs.

| Field | Type | Description |
| --- | --- | --- |
| `enabled` | bool | Activate multisig wrapping for this validator. |
| `threshold` | int | Minimum number of signers required to authorize a transaction (`k` in `k-of-n`). |
| `signer_count` | int | Total number of signer keys generated (`n` in `k-of-n`). Must satisfy `threshold ≤ signer_count`. |
| `vesting_type` | string | Optional. If set, the validator's genesis account is post-processed into a vesting account variant. Currently only `"PermanentLocked"` is implemented (rewrites the BaseAccount into a [`/cosmos.vesting.v1beta1.PermanentLockedAccount`](../../scripts/migrate-multisig.sh)); any other value aborts setup with an "unsupported multisig.vesting_type" error. Omit for a plain multisig BaseAccount. |

> **Why a vesting wrapper?** The Cosmos SDK CLI's `add-genesis-account` can only emit `Delayed`/`ContinuousVesting` (which require `end_time > 0`). `PermanentLocked` requires `end_time == 0`, so the devnet rewrites the genesis JSON directly after account creation. See [`devnet/scripts/validator-setup.sh`](../../devnet/scripts/validator-setup.sh) (`Wrapping multisig validator … as PermanentLockedAccount`).

#### `test_accounts` (optional)

Creates `count` extra funded accounts on this validator beyond the standard genesis account. Used to give migration tests, EVM tests, and the lumera-uploader fixture multiple sender keys without polluting the global mnemonic list.

| Field | Type | Description |
| --- | --- | --- |
| `count` | int | Number of test accounts to create. Set to `0` or omit to disable. |
| `balance_base` | string | Funding amount for the first account (e.g. `"20000ulume"`). |
| `balance_increment` | string | Per-account increment added to `balance_base` for subsequent accounts. The N-th account (1-indexed) gets `balance_base + (N-1) * balance_increment`. Useful for distinguishing accounts in test assertions by balance fingerprint. |
| `multisig` | bool | If `true`, generate the test accounts as multisig accounts (uses the parent validator's `multisig.threshold` / `signer_count`). Requires `multisig.enabled = true` on the same validator. Default: `false` (single-sig test accounts). |

#### `initial_distribution`

| Field | Type | Description |
| --- | --- | --- |
| `account_balance` | string | Total tokens allocated to this validator's genesis account |
| `validator_stake` | string | Tokens self-delegated at genesis |

### Validator network matrix (default `devnet/config/validators.json`)

The devnet runs five validators plus a Hermes IBC relayer on a private Docker bridge (`172.28.0.0/24`, network name `lumera-network`). Each container exposes the **same set of internal ports** — only the host-side mapping differs per validator. From inside any container, reach a peer via `<service-name>:<internal-port>` (e.g. `http://supernova_validator_1:26657`); from your host machine, use `localhost:<host-port>`.

#### Internal ports (constant across all validator containers)

| Service | Internal port | Protocol details |
| --- | --- | --- |
| CometBFT P2P | `26656` | See [lumera-ports.md → P2P](../lumera-ports.md#1-p2p-listener-peer-gossip) |
| CometBFT RPC | `26657` | See [lumera-ports.md → RPC](../lumera-ports.md#2-cometbft-rpc-listener) |
| Cosmos REST API | `1317` | See [lumera-ports.md → REST](../lumera-ports.md#4-cosmos-sdk-rest-api) |
| Cosmos gRPC | `9090` | See [lumera-ports.md → gRPC](../lumera-ports.md#5-cosmos-sdk-grpc-api) |
| EVM JSON-RPC HTTP | `8545` | See [lumera-ports.md → JSON-RPC HTTP](../lumera-ports.md#7-evm-json-rpc-http) |
| EVM JSON-RPC WS | `8546` | See [lumera-ports.md → JSON-RPC WS](../lumera-ports.md#8-evm-json-rpc-websocket) |
| Supernode gRPC | `4444` | Action processing service |
| Supernode P2P | `4445` | Supernode-to-supernode gossip |
| Supernode HTTP gateway | `8002` | Supernode REST gateway |
| Lumera-uploader gRPC | `50051` | Only when `lumera-uploader.enabled = true` |
| Lumera-uploader HTTP | `8080` | Only when `lumera-uploader.enabled = true` |
| Delve debugger | `40000` | Only when the binary is built in debug mode |

#### Per-validator host ports + container DNS / IP

| # | Container DNS / `name` | Static IP | P2P (host) | RPC | REST | gRPC | EVM HTTP | EVM WS | SN gRPC | SN P2P | SN GW | Debug | Uploader |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | `supernova_validator_1` | `172.28.0.11` | 26666 | 26667 | 1327 | 9091 | 8545 | 8546 | 7441 | 7442 | 18001 | 40000 | — |
| 2 | `supernova_validator_2` | `172.28.0.12` | 26676 | 26677 | 1337 | 9092 | 8555 | 8556 | 7443 | 7444 | 18002 | 40001 | — |
| 3 | `supernova_validator_3` | `172.28.0.13` | 26686 | 26687 | 1347 | 9093 | 8565 | 8566 | 7445 | 7446 | 18003 | 40002 | 50051 / 8080 |
| 4 | `supernova_validator_4` | `172.28.0.14` | 26696 | 26697 | 1357 | 9094 | 8575 | 8576 | 7447 | 7448 | 18004 | 40003 | — |
| 5 | `supernova_validator_5` | `172.28.0.15` | 26606 | 26607 | 1367 | 9095 | 8585 | 8586 | 7449 | 7450 | 18005 | 40004 | — |
| — | `hermes` | `172.28.0.10` | 36656 | 36657 | 31317 | 39090 / 39091 | — | — | — | — | — | — | — |

> **Reading the table:** the **Host** column gives the port published on `localhost` (i.e. the port your laptop talks to). All in-container traffic uses the **internal** port from the previous table — e.g. validator 4's CometBFT RPC is reached as `http://localhost:26697` from your host but `http://supernova_validator_1:26657` from another container.

#### Port assignment conventions

- **Per-validator host-port stride.** P2P / RPC / REST / EVM-HTTP / EVM-WS step by **+10** per validator slot (`26666 → 26676 → 26686 → 26696 → 26606`). Validator 5 wraps around because the +40 offset would collide with V1's debug span; the script intentionally keeps each validator's host-port block in its own decade.
- **gRPC** steps by **+1** (`9091..9095`) — gRPC traffic is rarely diagnosed on the host so the dense packing is fine.
- **Supernode** ports come in a `(gRPC, P2P, gateway)` triple per validator: `(7441, 7442, 18001) … (7449, 7450, 18005)`.
- **Debug (delve)** is `40000 + (i-1)` where `i` is the validator slot.
- **Hermes** uses a `+10000` offset on the standard CometBFT/REST ports so it can run an independent `simd` chain alongside `lumerad` validators without conflict.

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
