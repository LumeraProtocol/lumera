# Devnet Tests

Devnet tests run inside the Docker multi-validator testnet (`make devnet-new`).
Test source: `devnet/tests/validator/evm_test.go`

| Test | Description |
| --- | --- |
| `TestEVMFeeMarketBaseFeeActive` | Validates `eth_gasPrice` returns a non-zero base fee on an active devnet. |
| `TestEVMDynamicFeeTxE2E` | Sends a type-2 (EIP-1559) self-transfer and verifies receipt status 0x1. |
| `TestEVMTransactionVisibleAcrossPeerValidator` | Sends a tx to the local validator and verifies the receipt is visible on a peer validator with matching blockHash — exercises the broadcast worker re-gossip path. |

## EVM Migration Devnet Tests

See [devnet-tests.md](../../evmigration/devnet-tests.md) for full details on the EVM migration devnet test binary (modes, usage, and coverage).
