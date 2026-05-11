# Devnet Tests

The devnet includes three test suites that run inside Docker containers against the live network. They are compiled as standalone Go test binaries and copied into the containers via `configure.sh`.

## Test suites

| Binary | Source | Runs in | Purpose |
| --- | --- | --- | --- |
| `tests_validator` | `devnet/tests/validator/` | Any validator container | EVM JSON-RPC, IBC from Lumera side, LEP-5 cascade, port accessibility |
| `tests_hermes` | `devnet/tests/hermes/` | Hermes container | IBC from simd side, Interchain Accounts (ICA), cascade via ICA |
| `tests_evmigration` | `devnet/tests/evmigration/` | Hermes container | End-to-end EVM migration (see [../evm-integration/evmigration/devnet-tests.md](../evm-integration/evmigration/devnet-tests.md)) |

### Shared utilities

`devnet/tests/ibcutil/` provides common helpers used by both validator and hermes IBC tests:

- IBC channel/connection/client queries via CLI
- Balance queries via CLI and REST
- IBC transfer submission and balance polling
- Channel info JSON loading and IBC denom computation
- CLI execution with timeout handling

## Building

```bash
make devnet-tests-build
```

This produces three binaries in `devnet/bin/`:
- `tests_validator` -- compiled from `devnet/tests/validator/` via `go test -c`
- `tests_hermes` -- compiled from `devnet/tests/hermes/` via `go test -c`
- `tests_evmigration` -- compiled from `devnet/tests/evmigration/` via `go build`

The binaries are copied into containers by `configure.sh` and land in `/shared/release/`.

## Running

Tests run inside their respective containers:

```bash
# Validator tests (run on any validator)
docker exec lumera-supernova_validator_1 /shared/release/tests_validator -test.v -test.run TestEVM

# Hermes tests (run on hermes container)
docker exec lumera-hermes /shared/release/tests_hermes -test.v -test.run TestICA

# All validator tests
docker exec lumera-supernova_validator_1 /shared/release/tests_validator -test.v

# All hermes tests
docker exec lumera-hermes /shared/release/tests_hermes -test.v
```

---

## Validator tests (`tests_validator`)

### EVM JSON-RPC tests (`evm_test.go`)

These tests validate the EVM JSON-RPC endpoint exposed by each validator. They are **automatically skipped** if the lumerad version is below v1.20.0 (pre-EVM).

| Test | Description |
| --- | --- |
| `TestEVMJSONRPCBasicMethods` | Validates `eth_chainId`, `eth_blockNumber`, `net_version` return correct values |
| `TestEVMJSONRPCNamespacesExposed` | Confirms all configured namespaces are available (`web3`, `eth`, `personal`, `net`, `txpool`, `debug`, `rpc`) |
| `TestEVMFeeMarketBaseFeeActive` | Verifies `baseFeePerGas` is present and positive in latest block; tests `eth_feeHistory` |
| `TestEVMSendRawTransactionAndReceipt` | Sends a dynamic-fee (EIP-1559) transaction and validates receipt properties |
| `TestEVMGetTransactionByHashRoundTrip` | Retrieves tx by hash, compares fields with receipt |
| `TestEVMNonceIncrementsAfterMinedTx` | Verifies nonce increments from pending to latest after a mined transaction |
| `TestEVMBlockLookupByHashAndNumberConsistent` | Confirms block retrieval by hash and by number return identical data |
| `TestEVMTransactionVisibleAcrossPeerValidator` | Sends EVM tx to local validator, verifies it is visible on a peer validator's RPC |

**Key environment variables:**

| Variable | Default | Description |
| --- | --- | --- |
| `LUMERA_JSONRPC_ADDR` | `http://supernova_validator_1:8545` | JSON-RPC endpoint |
| `LUMERA_HOME` | `/root/.lumera` | Lumera home directory for keyring access |

### IBC tests (`ibc_test.go`)

These tests validate IBC functionality from the Lumera side using a testify suite (`lumeraValidatorSuite`).

| Test | Description |
| --- | --- |
| `TestChannelOpen` | Verifies IBC channel is in OPEN state on Lumera |
| `TestConnectionOpen` | Checks IBC connection is OPEN |
| `TestClientActive` | Validates IBC client status is ACTIVE |
| `TestChannelClientState` | Verifies client-state height is positive and client ID matches |
| `TestTransferToSimd` | Executes a real IBC transfer Lumera -> simd, waits for balance increase |
| `TestIBCTransferWithEVMModeStillRelays` | Tests IBC transfer works when Lumera is in EVM mode (skipped if not EVM) |

**Key environment variables:**

| Variable | Default | Description |
| --- | --- | --- |
| `CHANNEL_INFO_FILE` | `/shared/status/hermes/channel_transfer.json` | IBC channel metadata |
| `LUMERA_RPC_ADDR` | `http://supernova_validator_1:26657` | Lumera RPC endpoint |
| `LUMERA_CHAIN_ID` | `lumera-devnet-1` | Chain ID |
| `LUMERA_KEY_NAME` | `hermes-relayer` | Key for signing transfers |
| `SIMD_REST_ADDR` | `http://hermes:1317` | Simd REST for balance queries |
| `LUMERA_KEY_STYLE` | auto-detected | `evm` or `cosmos` |

### LEP-5 cascade availability commitment tests (`lep5_test.go`)

These tests exercise the full LEP-5 cascade availability commitment flow: file chunking, Merkle tree construction, action registration, finalization, and metadata queries.

| Test | Description |
| --- | --- |
| `TestLEP5CascadeAvailabilityCommitment` | Full register -> finalize -> DONE cycle with default chunk size (256 KB) |
| `TestLEP5VariableChunkSizes` | Parameterized test with 3 subtests: 5 KB/1024 chunks, 500 KB/128 KB chunks, 4 B/1 B chunks |
| `TestLEP5CascadeAvailabilityCommitmentFailure` | Tests failure scenarios in the commitment flow |
| `TestLEP5QueryActionMetadata` | Validates action metadata queries after commitment |

**Key constants:**
- Chunk size: 262,144 bytes (256 KB default)
- Commitment type: `lep5/chunk-merkle/v1`
- Hash algorithm: BLAKE3
- Top supernodes limit: 25

**Endpoint:** `localhost:9090` (Lumera gRPC)

### Port accessibility tests (`ports_test.go`)

| Test | Description |
| --- | --- |
| `TestLocalLumeradRequiredPortsAccessible` | Validates TCP connectivity to P2P (26656), RPC (26657), REST (1317), gRPC (9090); optionally JSON-RPC (8545) and WebSocket (8546) if EVM enabled |
| `TestLocalLumeradJSONRPCCORSAllowsMetaMaskHeaders` | Tests JSON-RPC CORS preflight accepts MetaMask extension origin headers |

Port values are read from the actual `config.toml` and `app.toml` in the daemon home directory. JSON-RPC tests are skipped if the version is pre-EVM or if JSON-RPC is disabled in `app.toml`.

### Version utilities (`version_mode.go`, `version_mode_test.go`)

- `resolveLumeraBinaryVersion()` -- executes `lumerad version --long --output json`, cached via `sync.Once`
- `requireEVMVersionOrSkip()` -- skips EVM tests if version < v1.20.0
- `TestVersionGTE` -- unit tests for semver comparison logic

---

## Hermes tests (`tests_hermes`)

### IBC tests (`ibc_test.go`)

Mirror of the validator IBC tests, but running from the **simd side** (inside the Hermes container). Uses testify suite `lumeraHermesSuite`.

| Test | Description |
| --- | --- |
| `TestChannelOpen` | Verifies channel is OPEN on simd side |
| `TestConnectionOpen` | Verifies connection is OPEN |
| `TestClientActive` | Verifies client status is ACTIVE |
| `TestChannelClientState` | Verifies client-state height > 0 |
| `TestTransferToLumera` | Executes IBC transfer simd -> Lumera, waits for balance increase |
| `TestIBCTransferWithEVMModeStillRelays` | Tests transfer when Lumera is EVM-enabled |

**Key environment variables:**

| Variable | Default | Description |
| --- | --- | --- |
| `SIMD_RPC_ADDR` | `http://127.0.0.1:26657` | Simd RPC (local in hermes container) |
| `SIMD_GRPC_ADDR` | `http://127.0.0.1:9090` | Simd gRPC |
| `SIMD_CHAIN_ID` | `hermes-simd-1` | Simd chain ID |
| `SIMD_DENOM` | `stake` | Simd denomination |
| `LUMERA_RPC_ADDR` | `http://supernova_validator_1:26657` | Lumera RPC |
| `LUMERA_REST_ADDR` | `http://supernova_validator_1:1317` | Lumera REST |

### Interchain Accounts tests (`ibc_ica_test.go`)

End-to-end ICA (Interchain Accounts) flow testing the full cascade upload/download/approve cycle via ICA.

| Test | Description |
| --- | --- |
| `TestICACascadeFlow` | Full ICA workflow: register ICA account, fund it, upload files via cascade, register actions via ICA `SendRequestAction`, download and verify payloads, approve actions via ICA `ApproveAction` |

**Timeout:** 20 minutes

**Workflow:**
1. Load Lumera keyring, import simd key for app pubkey derivation
2. Create ICA controller on simd, register ICA account on Lumera (host chain)
3. Fund ICA account from relayer account
4. Create test files (128 B, 2 KB, 8 KB)
5. Register actions via ICA `SendRequestAction`, collect action IDs from acknowledgements
6. Download payloads, verify content matches source
7. Wait for actions to reach DONE state
8. Approve actions via ICA `ApproveAction`
9. Verify actions reach APPROVED state

### ICA app pubkey validation (`ibc_ica_app_pubkey_test.go`)

| Test | Description |
| --- | --- |
| `TestICARequestActionAppPubkeyRequired` | Validates that `RequestAction` via ICA requires `app_pubkey`; fails without it, succeeds with it |

### Version utilities (`version_mode.go`)

- `readDevnetChainConfig()` -- reads chain config from multiple candidate paths
- `resolveLumeraKeyStyle()` -- determines `evm` vs `cosmos` based on version comparison
- Supports `LUMERA_KEY_STYLE`, `LUMERA_VERSION`, `LUMERA_FIRST_EVM_VERSION` environment overrides

---

## EVM migration tests (`tests_evmigration`)

This is a standalone binary (not a Go test binary) with 6 operating modes. See [../evm-integration/evmigration/devnet-tests.md](../evm-integration/evmigration/devnet-tests.md) for comprehensive documentation.

Modes: `prepare`, `estimate`, `migrate`, `migrate-validator`, `verify`, `cleanup`

Makefile targets: `make devnet-evmigration-*` (sequential) and `make devnet-evmigrationp-*` (parallel)

---

## Key file locations inside containers

| File | Purpose |
| --- | --- |
| `/shared/release/tests_validator` | Validator test binary |
| `/shared/release/tests_hermes` | Hermes test binary |
| `/shared/release/tests_evmigration` | EVM migration test binary |
| `/shared/status/hermes/channel_transfer.json` | IBC channel metadata (created by Hermes channel setup) |
| `/shared/hermes/simd-test.address` | Simd test account address |
| `/shared/hermes/lumera-hermes-relayer.address` | Relayer address on Lumera |
| `/shared/hermes/*.mnemonic` | Key mnemonics for relayer accounts |
| `/shared/config/validators.json` | Validator specs (used for key/moniker discovery) |

## EVM version gating

Both test suites gate EVM-specific tests on the running lumerad version:

- **Validator tests**: `requireEVMVersionOrSkip()` checks `lumerad version` >= `v1.20.0`
- **Hermes tests**: `resolveLumeraKeyStyle()` reads version from config.json or env

Tests that require EVM are automatically **skipped** (not failed) on pre-EVM chains, so the same test binary works across upgrade boundaries.
