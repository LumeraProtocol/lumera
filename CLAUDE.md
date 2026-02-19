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

# Devnet (local Docker testnet with 3 validators + Hermes relayer)
make devnet-new               # Full clean rebuild + start
make devnet-build-default     # Build devnet from default config
make devnet-up                # Start containers
make devnet-down              # Stop containers
make devnet-clean             # Remove all devnet data (/tmp/lumera-devnet-1/)
```

**Prerequisite**: `claims.csv` must exist in the repo root (CI copies it to `$HOME/`).

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

Custom ante handler in `ante/delayed_claim_fee_decorator.go` - a fee decorator specific to claim transactions.

### Test Utilities

`testutil/` provides:
- `keeper/` - Per-module keeper test setup helpers (action, claim, supernode, pastelid)
- `sample/` - Sample data generators for test fixtures
- `network/` - Test network configuration
- `mocks/` - Keyring mocks

### Key Configuration

- Go toolchain: 1.25.5
- Bech32 prefixes defined in `config/config.go` (lumera, lumeravaloper, lumeravalcons)
- Chain denom: `ulume` (coin type 118)
- CosmWasm: wasmd v0.61.6 with wasmvm v3.0.2 (requires `libwasmvm.x86_64.so` at runtime)
- Ignite scaffolding comments (`# stargate/app/...`) mark extension points - preserve these when editing
