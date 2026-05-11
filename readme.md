# Lumera
[![Release Workflow](https://github.com/LumeraProtocol/lumera/actions/workflows/release.yml/badge.svg)](https://github.com/LumeraProtocol/lumera/actions/workflows/release.yml)

**Lumera** is a Cosmos SDK blockchain (v0.53.6) supporting CosmWasm smart contracts, IBC cross-chain messaging, Cosmos EVM, and four custom modules (action, claim, lumeraid, supernode).

## Get started

### Prerequisites

- Go 1.26+ (see `go.mod` for exact version)
- `make`
- `libwasmvm` shared library (built automatically or available from CosmWasm releases)

### Build

```bash
make build
```

This produces the `lumerad` binary in `build/lumerad`.

Other useful build targets:

```bash
make build-debug          # Build with debug symbols
make build-proto          # Regenerate protobuf files
make lint                 # Run golangci-lint
make unit-tests           # Run unit tests
make integration-tests    # Run integration tests
```

### Initialize

You only need to run this command once.
```bash
lumerad init my-node
```

### Get latest `genesis.json`

```
https://github.com/LumeraProtocol/lumera-networks
```

### Get seeds

```
https://github.com/LumeraProtocol/lumera-networks
```

### Start

```bash
lumerad start
```

## Documentation

- [EVM Integration](docs/evm-integration/main.md) — Cosmos EVM architecture, precompiles, JSON-RPC, migration guides
- [Devnet](docs/devnet/main.md) — Local Docker test network setup, configuration, upgrade testing
- [Port Reference](docs/lumera-ports.md) — Network port defaults, config keys, and CLI flags

## Learn more

- [Cosmos SDK docs](https://docs.cosmos.network)
- [CometBFT docs](https://docs.cometbft.com)
- [IBC Protocol](https://ibc.cosmos.network)
- [CosmWasm](https://cosmwasm.com)
