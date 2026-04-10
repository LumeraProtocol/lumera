# Integration Tests: VM Query & State

Purpose: validates `x/vm` query APIs and consistency against JSON-RPC/accounting/state snapshots.
Suite: `tests/integration/evm/vm/suite_test.go`

| Test | Description |
| --- | --- |
| `VMQueryParamsAndConfigBasic` | Verifies vm params/config query endpoints return expected baseline values. |
| `VMAddressConversionRoundTrip` | Verifies VM address conversion utilities round-trip correctly. |
| `VMQueryAccountMatchesEthRPC` | Verifies VM account query fields match equivalent JSON-RPC account state. |
| `VMQueryAccountRejectsInvalidAddress` | Verifies VM account query rejects invalid addresses. |
| `VMQueryAccountAcceptsHexAndBech32` | Verifies VM account query accepts both hex and Bech32 forms where supported. |
| `VMBalanceBankMatchesBankQuery` | Verifies VM bank-balance query is consistent with bank module query results. |
| `VMStorageQueryKeyFormatEquivalence` | Verifies storage queries are equivalent across supported key encodings. |
| `VMQueryCodeAndStorageMatchJSONRPC` | Verifies VM code/storage queries align with JSON-RPC responses. |
| `VMQueryAccountHistoricalHeightNonceProgression` | Verifies historical-height account queries show expected nonce progression. |
| `VMQueryHistoricalCodeAndStorageSnapshots` | Verifies historical code/storage snapshots are queryable and consistent. |
| `VMBalanceERC20MatchesEthCall` | Verifies VM ERC20 balance query matches direct contract `eth_call` results. |
| `VMBalanceERC20RejectsNonERC20Runtime` | Verifies ERC20 balance query fails cleanly for non-ERC20 runtimes. |
