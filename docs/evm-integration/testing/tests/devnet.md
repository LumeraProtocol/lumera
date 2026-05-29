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
| `TestEVMContractPersistsAcrossLocalLumeradRestart` | Opt-in destructive check gated by `LUMERA_DEVNET_RESTART_TESTS=true`; deploys a contract, restarts local `lumerad`, waits for JSON-RPC, and verifies code plus call behavior persist. |
| `TestEVMActionPrecompileQueryDevnet` | Calls the Lumera Action precompile through `eth_call` and verifies ABI-shaped fee output. |
| `TestEVMBankPrecompileTotalSupplyQueryDevnet` | Calls the standard Bank precompile `totalSupply()` query and verifies ABI-shaped output. |
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

## Hermes EVM/IBC Tests

Test source: `devnet/tests/hermes/ibc_test.go`

| Test | Description |
| --- | --- |
| `TestIBCUnapprovedBaseDenomDoesNotRegisterERC20Pair` | In EVM mode, sends the simd base denom to Lumera over IBC, verifies the bank voucher arrives, and confirms the unapproved voucher denom does not auto-register an ERC20 token pair. Skips if the devnet profile has already registered that pair. |

## Coverage Gaps

The current devnet coverage does not yet explicitly exercise:

| Scenario | Current coverage |
| --- | --- |
| Public JSON-RPC rate-limit profile | Conditional devnet coverage when rate limiting is enabled |
| JSON-RPC restart persistence | Opt-in destructive devnet coverage gated by `LUMERA_DEVNET_RESTART_TESTS=true` |
| Full standard and custom precompile tx matrix | Devnet covers Bank query, gov tx smoke, and Action query; integration covers broader tx/query paths |
| ERC20 wrong-provenance rejection for an allowlisted base denom on the wrong channel | Devnet covers unapproved base-denom rejection; integration covers provenance-bound policy branches |
