# Devnet Tests

Devnet tests run inside the Docker multi-validator testnet (`make devnet-new`).

## Validator EVM Tests

Test source: `devnet/tests/validator/evm_test.go`

| Test | Description |
| --- | --- |
| `TestEVMJSONRPCBasicMethods` | Verifies basic JSON-RPC health and chain metadata methods on a validator RPC endpoint. |
| `TestEVMJSONRPCNamespacesExposed` | Verifies the expected public EVM JSON-RPC namespaces are exposed. |
| `TestEVMJSONRPCRateLimitPublicProfileIfEnabled` | When `[lumera.json-rpc-ratelimit] enable = true` in the local devnet `app.toml`, bursts JSON-RPC requests through the public endpoint and requires HTTP 429 responses from the rate limiter. Skipped for default profiles where rate limiting is disabled. |
| `TestEVMFeeMarketBaseFeeActive` | Validates `eth_gasPrice` returns a non-zero base fee on an active devnet. |
| `TestEVMSendRawTransactionAndReceipt` | Sends a raw EVM transaction and verifies the successful receipt. |
| `TestEVMGetTransactionByHashRoundTrip` | Sends a transaction and verifies it can be fetched by hash. |
| `TestEVMNonceIncrementsAfterMinedTx` | Verifies account nonce increments after a mined EVM transaction. |
| `TestEVMBlockLookupByHashAndNumberConsistent` | Compares block lookup by hash and by number for consistent block metadata. |
| `TestEVMTransactionVisibleAcrossPeerValidator` | Sends a tx to one validator and verifies the receipt is visible on a peer validator with matching `blockHash`; exercises the broadcast worker re-gossip path. |
| `TestEVMWebSocketNewHeadsSubscription` | Subscribes to `newHeads` over the EVM WebSocket endpoint, sends a transaction, and verifies a header notification arrives. |
| `TestEVMContractDeployCallAndLogsDevnet` | Deploys a small EVM contract, verifies deployment logs, calls the runtime with `eth_call`, and queries logs by topic. |
| `TestEVMActionPrecompileQueryDevnet` | Calls the Lumera Action precompile through `eth_call` and verifies ABI-shaped fee output. |
| `TestEVMGovPrecompileTxPathDevnet` | Sends a transaction to the governance precompile and verifies an unknown-proposal failure is returned as an EVM receipt failure. |

## EVM Migration Devnet Tests

Tool source: `devnet/tests/evmigration/`

See [devnet-tests.md](../../evmigration/devnet-tests.md) for full details on the EVM migration devnet test binary, Makefile targets, upgrade walkthrough, and module coverage.

| Mode | Description |
| --- | --- |
| `prepare` | Creates legacy coin-type 118 accounts and module state before the EVM upgrade. |
| `estimate` | Queries migration readiness and touched-state counts after the EVM upgrade. |
| `migrate` | Migrates regular legacy accounts with `MsgClaimLegacyAccount`. |
| `migrate-validator` | Migrates a validator operator with `MsgMigrateValidator`. |
| `migrate-all` | Interleaves account and validator migrations in randomized batches. |
| `verify` | Scans modules to confirm migrated legacy addresses no longer own live state. |
| `multisig` | Exercises the focused multisig account migration proof flow. |
| `multisig-vesting` | Exercises multisig migration for a vesting account fixture. |
| `multisig-validator` | Exercises validator migration from a multisig legacy account. |
| `cleanup` | Removes generated test keys from the local keyring. |

## Coverage Gaps

The current devnet coverage does not yet explicitly exercise:

| Scenario | Current coverage |
| --- | --- |
| Public JSON-RPC rate-limit profile | Conditional devnet coverage when rate limiting is enabled |
| JSON-RPC restart persistence | Integration coverage only |
| Full standard and custom precompile tx matrix | Devnet covers gov tx smoke and Action query; integration covers broader tx/query paths |
| ERC20 wrong-provenance rejection | Integration coverage only |
