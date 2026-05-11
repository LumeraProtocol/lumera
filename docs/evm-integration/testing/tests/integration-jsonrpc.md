# Integration Tests: JSON-RPC & Indexer

Purpose: validates JSON-RPC compatibility, tx/receipt lookup/indexer behavior, mixed Cosmos+EVM block behavior, and restart durability.

Suites:
- `tests/integration/evm/jsonrpc/suite_test.go`
- `tests/integration/evm/jsonrpc/mixed_block_suite_test.go`

| Test | Description |
| --- | --- |
| `BasicRPCMethods` | Verifies baseline RPC methods (`eth_chainId`, `eth_blockNumber`, etc.) return expected values. |
| `BackendBlockCountAndUncleSemantics` | Validates block-count and uncle-related method semantics on this backend. |
| `BackendNetAndWeb3UtilityMethods` | Verifies `net_*` and `web3_*` utility methods return sane values. |
| `BlockLookupIncludesTransaction` | Ensures block queries include expected transaction objects/hashes. |
| `TransactionLookupByBlockAndIndex` | Validates tx lookup by block hash/number + index works correctly. |
| `MultiTxOrderingSameBlock` | Verifies deterministic `transactionIndex` ordering for multiple txs in one block. |
| `ReceiptIncludesCanonicalFields` | Ensures receipts expose canonical Ethereum fields and expected encodings. |
| `MixedCosmosAndEVMTransactionsCanShareBlock` | Confirms Cosmos and EVM txs can be included together in the same committed block. |
| `MixedBlockOrderingPersistsAcrossRestart` | Confirms mixed-block tx ordering is preserved across restart. |
| `TestEOANonceByBlockTagAndRestart` | Verifies nonce query semantics by block tag and restart persistence. |
| `TestSelfTransferFeeAccounting` | Verifies self-transfer balance delta equals `gasUsed * effectiveGasPrice`. |
| `TestIndexerDisabledLookupUnavailable` | Verifies tx/receipt lookups are unavailable when indexers are disabled. |
| `TestLogsIndexerPathAcrossRestart` | Verifies `eth_getLogs` indexer queries remain correct across restart. |
| `TestReceiptPersistsAcrossRestart` | Verifies `eth_getTransactionReceipt` remains available after restart. |
| `TestIndexerStartupSmoke` | Smoke-tests JSON-RPC/WebSocket/indexer startup path and startup logs. |
| `TestTransactionByHashPersistsAcrossRestart` | Verifies `eth_getTransactionByHash` consistency before/after restart. |
| `OpenRPCDiscoverMethodCatalog` | Verifies `rpc_discover` returns non-empty, deduplicated catalog with required namespace coverage. |
| `OpenRPCDiscoverMatchesEmbeddedSpec` | Verifies runtime `rpc_discover` output matches the embedded OpenRPC document in the node binary. |
| `TestOpenRPCHTTPDocumentEndpoint` | Verifies `/openrpc.json` (API server) is served and matches JSON-RPC `rpc_discover` method set. |
| `BatchJSONRPCReturnsAllResponses` | Sends a batch of 4 different methods and verifies all responses return with correct IDs. |
| `BatchJSONRPCMixedErrorsAndResults` | Batch with valid + invalid requests; verifies per-request errors don't break the batch. |
| `BatchJSONRPCSingleElementBatch` | Edge case: single-element batch array returns one response correctly. |
| `BatchJSONRPCDuplicateMethods` | Batch of 3 identical `eth_blockNumber` calls returns 3 independent results. |
