# Lumera Uploader

Lumera Uploader (formerly network-maker) is a multi-account management service used for NFT/scanner operations. It runs on a single validator (typically `supernova_validator_3`) and provides gRPC + HTTP APIs for managing accounts, scanning files, and submitting transactions.

> Starting with Lumera v1.11.0, the project was renamed from "network-maker" to "lumera-uploader". All devnet scripts support both names for backward compatibility.

## Version-based naming

| Lumera version | Binary name         | Home directory             | Log file                | Config TOML section   |
| -------------- | ------------------- | -------------------------- | ----------------------- | --------------------- |
| >= v1.11.0     | `lumera-uploader` | `/root/.lumera-uploader` | `lumera-uploader.log` | `[lumera-uploader]` |
| < v1.11.0      | `network-maker`   | `/root/.network-maker`   | `network-maker.log`   | `[network-maker]`   |

The binary name is resolved at runtime by `resolve_uploader_name()` in `common.sh`, which compares the running `lumerad` version against the threshold `LUMERA_FIRST_UPLOADER_VERSION` (default: `1.11.0`).

## Configuration

### Global settings (`config.json`)

```json
"lumera-uploader": {
    "enabled": true,
    "grpc_port": 50051,
    "http_port": 8080,
    "max_accounts": 3,
    "account_balance": "10000000ulume"
}
```

| Field               | Type   | Default           | Description                                       |
| ------------------- | ------ | ----------------- | ------------------------------------------------- |
| `enabled`         | bool   | `true`          | Global enable flag                                |
| `grpc_port`       | int    | `50051`         | gRPC listen port                                  |
| `http_port`       | int    | `8080`          | HTTP gateway listen port                          |
| `max_accounts`    | int    | `1`             | Number of funded accounts to create per validator |
| `account_balance` | string | `10000000ulume` | Funding amount per account                        |

### Per-validator settings (`validators.json`)

Enable the uploader on specific validators:

```json
{
    "name": "supernova_validator_3",
    "moniker": "supernova_validator_3",
    "lumera-uploader": {
        "enabled": true,
        "grpc_port": 50051,
        "http_port": 8080
    }
}
```

Per-validator `grpc_port` and `http_port` override the global defaults.

> For backward compatibility, scripts also accept the `"network-maker"` key name in both JSON files.

## Setup script

**File**: `devnet/scripts/lumera-uploader-setup.sh`

### Prerequisites

The setup script is a no-op (exits 0) if:

- The uploader binary is missing from`/shared/release/`
- `validators.json` has the uploader disabled for this validator's moniker

### Execution flow

```
1. Resolve binary name (lumera-uploader or network-maker)
2. Stop any leftover uploader process
3. Install binary from /shared/release/ to /usr/local/bin/
4. Wait for lumerad RPC (180s timeout)
5. Wait for Supernode endpoint (300s timeout)
6. Create/fund uploader accounts
7. Migrate accounts to EVM keys (if chain upgraded to >= v1.20.0)
8. Build config.toml from template + runtime values
9. Start uploader process in background
```

### Account management

The setup script creates `max_accounts` keyring keys:

- First account:`nm-account`
- Additional accounts:`nm-account-2`,`nm-account-3`, ...

For each account:

1. **Key creation**: Recover from saved mnemonic (if exists), or generate new
2. **Mnemonic storage**: Saved to`/shared/status/<moniker>/nm_mnemonic[-N]`
3. **Address storage**: Written to`/shared/status/<moniker>/nm-address`
4. **Funding**: Send`account_balance` from the validator's genesis account
5. **Registry**: Recorded in`/shared/status/<moniker>/accounts.json`

### Config generation

The active config is built from the template (`uploader-config.toml`):

```toml
[lumera]
grpc_endpoint = "localhost:9090"
rpc_endpoint = "http://localhost:26657"
chain_id = "lumera-devnet-1"
denom = "ulume"

[lumera-uploader]     # section name matches the resolved binary name
grpc_listen = "0.0.0.0:50051"
http_gateway_listen = "0.0.0.0:8080"

[keyring]
backend = "test"
dir = "/root/.lumera"

[[keyring.accounts]]
key_name = "nm-account"
address = "lumera1..."

[scanner]
directories = ["/root/nm-files", "/shared/nm-files"]
```

### EVM account migration

When the chain upgrades to v1.20.0+, uploader account keys are migrated from `secp256k1` (coin-type 118) to `eth_secp256k1` (coin-type 60):

1. Detect key type via pubkey`@type` field
2. Delete and re-add each key using`--key-type eth_secp256k1 --hd-path m/44'/60'/0'/0/0`
3. Fund the new address if balance is zero
4. Update address files and config

## Ports

| Service      | Default port | Description                   |
| ------------ | ------------ | ----------------------------- |
| gRPC         | 50051        | Uploader gRPC API             |
| HTTP gateway | 8080         | Uploader HTTP gateway         |
| UI (nginx)   | 8088         | Static web UI served by nginx |

## UI

If the `uploader-ui/` directory is present in the release, nginx serves the static web UI on port 8088. The `VITE_API_BASE` environment variable can override the default API endpoint (`http://127.0.0.1:8080`) baked into the UI bundle.

## Environment variables

| Variable                          | Default                          | Description                                                            |
| --------------------------------- | -------------------------------- | ---------------------------------------------------------------------- |
| `MONIKER`                       | (required)                       | Validator moniker, set by docker-compose                               |
| `START_MODE`                    | `run`                          | `run` = full setup + start; `wait` = wait for chain readiness only |
| `NM_GRPC_PORT`                  | `50051`                        | Override gRPC port                                                     |
| `NM_HTTP_PORT`                  | `8080`                         | Override HTTP gateway port                                             |
| `NM_LOG`                        | `/root/logs/<binary-name>.log` | Log file path                                                          |
| `NM_UI_PORT`                    | `8088`                         | Nginx UI port                                                          |
| `VITE_API_BASE`                 | (unset)                          | Override API base URL injected into UI bundle                          |
| `LUMERA_FIRST_UPLOADER_VERSION` | `1.11.0`                       | Version threshold for the rename                                       |

## Managing the uploader

### Stop/start/restart

From inside the container:

```bash
# Stop
/root/scripts/stop.sh nm

# Start
/root/scripts/restart.sh nm

# The stop/restart scripts handle both binary names automatically
```

### From the host

```bash
# Restart uploader in a specific validator
docker exec lumera-supernova_validator_3 /root/scripts/restart.sh nm
```

## Scanner directories

The uploader monitors these directories for files to process:

| Directory            | Scope  | Description                                            |
| -------------------- | ------ | ------------------------------------------------------ |
| `/root/nm-files`   | Local  | Per-container scanner directory                        |
| `/shared/nm-files` | Shared | Cross-container directory (writable by all validators) |

Drop files into either directory and the uploader will pick them up for processing.
