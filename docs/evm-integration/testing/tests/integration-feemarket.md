# Integration Tests: Fee Market (EIP-1559)

Purpose: validates EIP-1559 RPC behavior, effective gas price accounting, and dynamic-fee admission rules.
Suite: `tests/integration/evm/feemarket/suite_test.go`

| Test | Description |
| --- | --- |
| `FeeHistoryReportsCanonicalShape` | Checks `eth_feeHistory` response shape and core fields for compatibility. |
| `ReceiptEffectiveGasPriceRespectsBlockBaseFee` | Verifies receipt `effectiveGasPrice` reflects block base fee constraints. |
| `FeeHistoryRewardPercentilesShape` | Validates reward percentile formatting/structure in fee history results. |
| `MaxPriorityFeePerGasReturnsValidHex` | Ensures `eth_maxPriorityFeePerGas` returns a valid hex value. |
| `GasPriceIsAtLeastLatestBaseFee` | Ensures `eth_gasPrice` is not below current base fee expectations. |
| `DynamicFeeType2EffectiveGasPriceFormula` | Verifies type-2 tx effective gas price calculation is correct. |
| `DynamicFeeType2RejectsFeeCapBelowBaseFee` | Ensures txs with fee cap below base fee are rejected. |
