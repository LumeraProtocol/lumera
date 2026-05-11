# Integration Tests: Precisebank

Purpose: validates transaction-level and query-level behavior of fractional balance accounting under EVM flows.
Suite: `tests/integration/evm/precisebank/suite_test.go`

| Test | Description |
| --- | --- |
| `PreciseBankFractionalBalanceQueryMatrix` | Verifies fractional-balance query responses across representative account states. |
| `PreciseBankFractionalBalanceRejectsInvalidAddress` | Verifies invalid address formats are rejected by precisebank queries. |
| `PreciseBankEVMTransferSendSplitMatrix` | Verifies integer/fractional split behavior across EVM transfer scenarios. |
| `PreciseBankSecondarySenderBurnMintWorkflow` | Verifies mint/send/burn workflow behavior using secondary sender flows. |
| `TestPreciseBankRemainderQueryPersistsAcrossRestart` | Verifies precisebank remainder query results persist after restart. |
| `TestPreciseBankModuleAccountFractionalBalanceIsZero` | Verifies module account fractional balance invariants remain zero as expected. |
