# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Lumera is a Cosmos SDK blockchain (v0.53.6) built with Ignite CLI, supporting CosmWasm smart contracts, IBC cross-chain messaging, and four custom modules. The binary is `lumerad`, the native token denom is `ulume`, and addresses use the `lumera` Bech32 prefix.

## Build & Development Commands

```bash
# Build
make build                    # Build lumerad binary -> build/lumerad
make build-debug              # Build with debug symbols
make build-proto              # Regenerate protobuf files (cleans first)
make install-tools            # Install all dev tools (buf, golangci-lint, goimports, etc.)

# Lint
make lint                     # golangci-lint run ./... --timeout=5m

# Tests
make unit-tests               # go test ./x/... -v -coverprofile=coverage.out
make integration-tests        # go test ./tests/integration/... -v
make system-tests             # go test -tags=system ./tests/system/... -v
make systemex-tests           # cd tests/systemtests && go test -tags=system_test -v .
make simulation-tests         # ignite chain simulate

# Run a single test
go test ./x/claim/... -v -run TestClaimRecord
go test -tags=integration ./tests/integration/... -v -run TestMsgClaim
cd tests/systemtests && go test -tags=system_test -v . -run 'TestSupernodeMetricsE2E'

# evmigration integration tests REQUIRE -tags='integration test'
# (without 'test', the cosmos-evm chainConfig guard makes every subtest
# silently skip). The package's TestMain fails fast when the tag is missing.
go test -tags='integration test' ./tests/integration/evmigration/... -v

# EVM-specific
make openrpc                  # Regenerate OpenRPC spec -> docs/openrpc.json + app/openrpc/openrpc.json.gz

# EVM integration tests (under tests/integration/evm/)
# Most EVM suites use -tags='integration test'; IBC ERC20 suite uses -tags='test'
go test -tags='integration test' ./tests/integration/evm/contracts/... -v -timeout 10m
go test -tags='integration test' ./tests/integration/evm/jsonrpc/... -v -timeout 10m
go test -tags='integration test' ./tests/integration/evm/feemarket/... -v -timeout 10m
go test -tags='integration test' ./tests/integration/evm/mempool/... -v -timeout 10m
go test -tags='integration test' ./tests/integration/evm/precompiles/... -v -timeout 10m
go test -tags='integration test' ./tests/integration/evm/precisebank/... -v -timeout 10m
go test -tags='integration test' ./tests/integration/evm/vm/... -v -timeout 10m
go test -tags='integration test' ./tests/integration/evm/ante/... -v -timeout 10m
go test -tags='test' ./tests/integration/evm/ibc/... -v -timeout 5m
# All EVM integration tests at once:
go test -tags='integration test' ./tests/integration/evm/... -v -timeout 15m

# Devnet (local Docker testnet with 5 validators + Hermes relayer)
make devnet-new               # Full clean rebuild + start
make devnet-build-default     # Build devnet from default config
make devnet-up                # Start containers (attached)
make devnet-up-detach         # Start containers (detached)
make devnet-down              # Stop and remove containers
make devnet-stop              # Stop containers (keep state)
make devnet-start             # Start stopped containers
make devnet-clean             # Remove all devnet data (/tmp/lumera-devnet-1/)
make devnet-refresh-bin       # Copy build/lumerad into devnet/bin/ (run after make build)
make devnet-upgrade-binaries  # Copy devnet/bin/ into all containers + restart (run devnet-refresh-bin first)
make devnet-update-scripts    # Update devnet scripts in containers
make devnet-reset             # Reset chain state, keep config
make devnet-evm-upgrade       # Run EVM upgrade on devnet
```

### Current shared devnet host notes

For the shared EVM migration/devnet environment, SSH via:

```bash
ssh lumera-devnet
```

Important paths and containers:

- Host devnet root: `/tmp/lumera-devnet-1`
- Shared host path: `/tmp/lumera-devnet-1/shared`
- Shared container path: `/shared`
- Accounts file: `/tmp/lumera-devnet-1/shared/release/accounts-devnet.json` on the host, `/shared/release/accounts-devnet.json` in containers
- Validator containers: `lumera-supernova_validator_1` through `lumera-supernova_validator_5`
- Hermes container: `lumera-hermes`
- Container shell: `docker exec -it lumera-supernova_validator_1 bash`
- Validator data bind mounts: `/tmp/lumera-devnet-1/supernova_validator_N-data` on the host maps to `/root/.lumera` in `lumera-supernova_validator_N`
- Migration scripts in validator containers: `/root/scripts/migration/migrate-account.sh`, `/root/scripts/migration/migrate-validator.sh`, `/root/scripts/migration/migrate-multisig.sh`

For manual multisig migration work, run from inside a validator container that has the relevant keyring entries. The generated devnet multisig records in `accounts-devnet.json` list the composite and member key names, while the actual keyring entries live under the validator container's `/root/.lumera`. Save full transcripts and proof artifacts under `/shared/release/migration-transcripts/<account>-<timestamp>/` so they appear on the host under `/tmp/lumera-devnet-1/shared/release/migration-transcripts/`.

**Note**: `claims.csv` is only needed if genesis `TotalClaimableAmount > 0` (claiming period ended 2025-01-01; default is now 0).

**Rule**: After completing any multi-file code change, run `make lint` and fix any issues before considering the task done. Lint must pass cleanly (0 issues).

## Architecture

### Cosmos SDK App (depinject wiring)

The app uses Cosmos SDK's **depinject** for module wiring. Configuration is declarative in `app/app_config.go` (module list, genesis order, begin/end blocker ordering). The main `App` struct with all keeper fields is in `app/app.go`. Chain upgrades are registered in `app/upgrades/` with version-specific handlers.

### Custom Modules (`x/`)

| Module | Path | Purpose |
|--------|------|---------|
| **action** | `x/action/v1/` | Distributed action processing for GPU compute jobs |
| **claim** | `x/claim/` | Token claim distribution (Bitcoin-to-Cosmos bridge) |
| **lumeraid** | `x/lumeraid/` | Identity management (Lumera ID / PastelID) |
| **supernode** | `x/supernode/v1/` | Supernode registration, governance, metrics, and evidence |

Each module follows standard Cosmos SDK layout:
- `keeper/` - State management and message server implementation
- `module/` - Module definition, depinject providers, AppModule interface
- `types/` - Message types, params, errors, keys, protobuf-generated code
- `simulation/` - Simulation parameters
- `mocks/` - Generated mocks (go.uber.org/mock)

### IBC Stack

IBC v10 with: core IBC, transfer, interchain accounts (host + controller), packet-forward-middleware. Light clients: Tendermint (07-tendermint), Solo Machine (06-solomachine). IBC router and middleware wiring is in `app/app.go` (search for `ibcRouter`).

### Protobuf

Proto definitions live in `proto/lumera/`. Code generation uses `buf` with two templates:
- `proto/buf.gen.gogo.yaml` - Go message/gRPC code
- `proto/buf.gen.swagger.yaml` - OpenAPI specs

Generated files land in `x/*/types/` as `*.pb.go`, `*_pb.gw.go`, `*.pulsar.go`.

### Ante Handlers

Custom ante handler in `ante/delayed_claim_fee_decorator.go` - a fee decorator specific to claim transactions. Dual-route EVM ante handler in `app/evm/ante.go` routes Ethereum extension txs to the EVM path and all others to the Cosmos path.

### EVM Stack (Cosmos EVM v0.6.0)

Four EVM modules wired in `app/evm.go`:

| Module | Purpose |
| -------- | ------- |
| `x/vm` | Core EVM execution, JSON-RPC, receipts/logs |
| `x/feemarket` | EIP-1559 dynamic base fee |
| `x/precisebank` | 6-decimal `ulume` ↔ 18-decimal `alume` bridge |
| `x/erc20` | STRv2 token pair registration, IBC ERC20 middleware |

Key files:

- `app/evm.go` - Keeper wiring, circular dependency resolution (`&app.Erc20Keeper` pointer)
- `app/evm/ante.go` - Dual-route ante handler (EVM vs Cosmos path)
- `app/evm/precompiles.go` - Static precompiles (bank, staking, distribution, gov, ics20, bech32, p256, slashing)
- `app/evm_mempool.go` - EVM-aware app-side mempool wiring + wrapped CheckTx rejection counter
- `app/evm_mempool_metrics.go` - Prometheus collector (gauges: size, pending, queued, broadcast_queue_depth; labeled counter: rejections_total{source,reason})
- `app/evm_broadcast.go` - Async broadcast queue (prevents mempool deadlock)
- `app/evm_runtime.go` - RegisterTxService/Close overrides for EVM lifecycle
- `app/ibc.go` - IBC router with ERC20 middleware for v1 and v2 transfer stacks
- `config/evm.go` - Chain ID, base fee, consensus max gas constants
- `app/openrpc/` - Gzip-compressed embedded OpenRPC spec served via `rpc_discover` and `/openrpc.json`; POST proxy for playground compatibility

EVM integration tests live in `tests/integration/evm/` with subpackages: ante, contracts, feemarket, ibc, jsonrpc, mempool, precisebank, precompiles, vm. Most use `//go:build integration` tag; the IBC ERC20 tests use `//go:build test`.

**Rule**: When adding or modifying EVM tests, update `docs/evm-integration/tests.md` — add new tests to the appropriate table (Unit Tests, Integration Tests, or Devnet Tests) and reference them from the related bug entry in `docs/evm-integration/bugs.md` if applicable.

**Rule**: When making significant changes to EVM code (precompile ABI changes, new module integrations, ante handler updates, new precompiles), update the relevant docs in `docs/evm-integration/` — especially `precompiles/*.md` for precompile changes and `main.md` for architectural changes.

### Test Utilities

`testutil/` provides:
- `keeper/` - Per-module keeper test setup helpers (action, claim, supernode, pastelid)
- `sample/` - Sample data generators for test fixtures
- `network/` - Test network configuration
- `mocks/` - Keyring mocks

### Key Configuration

- Go toolchain: 1.26.1
- Bech32 prefixes defined in `config/config.go` (lumera, lumeravaloper, lumeravalcons)
- Chain denom: `ulume` (coin type 60 / Ethereum-compatible, EVM extended denom `alume` at 18 decimals)
- EVM chain ID: `76857769`, key type: `eth_secp256k1`
- CosmWasm: wasmd v0.61.6 with wasmvm v3.0.3 (requires `libwasmvm.x86_64.so` at runtime)
- Ignite scaffolding comments (`# stargate/app/...`) mark extension points - preserve these when editing
