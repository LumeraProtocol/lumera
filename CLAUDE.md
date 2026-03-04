# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Lumera is a Cosmos SDK blockchain (v0.53.5) built with Ignite CLI, supporting CosmWasm smart contracts, IBC cross-chain messaging, and four custom modules. The binary is `lumerad`, the native token denom is `ulume`, and addresses use the `lumera` Bech32 prefix.

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

# EVM-specific
make openrpc                  # Regenerate OpenRPC spec -> docs/openrpc.json + app/openrpc/openrpc.json

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

# Devnet (local Docker testnet with 3 validators + Hermes relayer)
make devnet-new               # Full clean rebuild + start
make devnet-build-default     # Build devnet from default config
make devnet-up                # Start containers
make devnet-down              # Stop containers
make devnet-clean             # Remove all devnet data (/tmp/lumera-devnet-1/)
```

**Note**: `claims.csv` is only needed if genesis `TotalClaimableAmount > 0` (claiming period ended 2025-01-01; default is now 0).

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

### EVM Stack (Cosmos EVM v0.5.1)

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
- `app/evm_mempool.go` - EVM-aware app-side mempool wiring
- `app/evm_broadcast.go` - Async broadcast queue (prevents mempool deadlock)
- `app/evm_runtime.go` - RegisterTxService/Close overrides for EVM lifecycle
- `app/ibc.go` - IBC router with ERC20 middleware for v1 and v2 transfer stacks
- `config/evm.go` - Chain ID, base fee, consensus max gas constants
- `app/openrpc/` - Embedded OpenRPC spec served via `rpc_discover` and `/openrpc.json`

EVM integration tests live in `tests/integration/evm/` with subpackages: ante, contracts, feemarket, ibc, jsonrpc, mempool, precisebank, precompiles, vm. Most use `//go:build integration` tag; the IBC ERC20 tests use `//go:build test`.

**Rule**: When adding or modifying EVM tests, update `docs/evm-integration.md` — add new tests to the appropriate table (Unit Tests, Integration Tests, or Devnet Tests) and reference them from the related bug entry if applicable.

### Test Utilities

`testutil/` provides:
- `keeper/` - Per-module keeper test setup helpers (action, claim, supernode, pastelid)
- `sample/` - Sample data generators for test fixtures
- `network/` - Test network configuration
- `mocks/` - Keyring mocks

### Key Configuration

- Go toolchain: 1.25.5
- Bech32 prefixes defined in `config/config.go` (lumera, lumeravaloper, lumeravalcons)
- Chain denom: `ulume` (coin type 60 / Ethereum-compatible, EVM extended denom `alume` at 18 decimals)
- EVM chain ID: `76857769`, key type: `eth_secp256k1`
- CosmWasm: wasmd v0.61.6 with wasmvm v3.0.2 (requires `libwasmvm.x86_64.so` at runtime)
- Ignite scaffolding comments (`# stargate/app/...`) mark extension points - preserve these when editing
