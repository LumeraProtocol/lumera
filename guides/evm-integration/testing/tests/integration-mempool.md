# Integration Tests: Mempool

Purpose: validates app-side EVM mempool behavior for ordering, pending visibility, nonce handling, replacement policy, and metrics observability.

Suites:
- `tests/integration/evm/mempool/suite_test.go`
- `tests/integration/evm/mempool/metrics_txpool_status_test.go`
- `tests/integration/evm/mempool/metrics_prometheus_e2e_test.go`
- `tests/integration/evm/mempool/evmigration_zero_signer_test.go`

| Test | Description |
| --- | --- |
| `DeterministicOrderingUnderContention` | Verifies deterministic inclusion ordering under concurrent submission pressure. |
| `EVMFeePriorityOrderingSameBlock` | Verifies higher-fee tx priority ordering when txs land in the same block. |
| `PendingTxSubscriptionEmitsHash` | Verifies pending subscription emits tx hashes for pending EVM txs. |
| `NonceGapPromotionAfterGapFilled` | Verifies queued nonce-gap txs are promoted once missing nonce is filled. |
| `TestMempoolDisabledWithJSONRPCFailsFast` | Verifies txpool namespace behavior when app-side mempool is disabled. |
| `TestNonceReplacementRequiresPriceBump` | Verifies same-nonce replacement requires configured fee bump threshold. |
| `TestMempoolCapacityRejectsOverflow` | Floods a low-capacity mempool until rejection, verifying max-txs enforcement. |
| `RapidReplacementRace` | Concurrent goroutines race to replace the same nonce; verifies no panics/deadlock. |
| `NewHeadsSubscriptionEmitsBlocks` | WS `newHeads` subscription receives block header with expected fields. |
| `LogsSubscriptionEmitsEvents` | WS `logs` subscription receives LOG1 event from a deployed contract. |
| `NewHeadsSubscriptionMultipleBlocks` | WS `newHeads` delivers 3 consecutive headers with monotonically increasing numbers. |
| `TestTxPoolStatusReflectsPendingAndQueued` | Verifies txpool_status JSON-RPC reports correct pending/queued counts. |
| `TestTxPoolStatusOverflowKeepsPoolBounded` | Verifies flooding a low-capacity mempool results in rejections and bounded pool size. |
| `TestPrometheusMetricsExposeMempoolGauges` | E2E: starts node with Prometheus telemetry, scrapes /metrics, verifies gauges. |
| `TestPrometheusRejectionsCountedViaCometCheckTx` | E2E: submits malformed bytes via CometBFT broadcast_tx_sync, verifies rejection counter. |
| `TestEVMigrationZeroSignerTxBroadcastSyncWithMempoolEnabled` | Real-node `broadcast_tx_sync`: a valid zero-signer `MsgClaimLegacyAccount` passes CheckTx with the app-side EVM mempool enabled. |
| `TestEVMigrationZeroSignerTxBroadcastSyncAfterLegacyMainnetConfigMigration` | Real-node upgrade-profile check: starts from a pre-EVM `app.toml` with `mempool.max-txs = -1`, verifies migration rewrites it to `10000`, emits the real Cosmos EVM mempool defaults, and still admits a valid zero-signer `MsgClaimLegacyAccount`. |
| `TestEVMigrationProofValidNonexistentLegacyAccountRejectedByAnte` | Real-node `broadcast_tx_sync`: a proof-valid zero-signer migration tx is rejected by ante state admission when the legacy account does not exist. |
| `TestEVMigrationMalformedLegacyAddressRejectedByValidateBasic` | Real-node `broadcast_tx_sync`: malformed migration `legacy_address` is rejected by `ValidateBasic` in the ante chain, before mempool admission. |
| `TestZeroSignerNonMigrationBroadcastSyncStillRejected` | Negative control: a zero-signer non-migration tx is still rejected, proving the evmigration adapter does not widen signer bypass behavior. |
