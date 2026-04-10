# Unit Tests: Fee Market (EIP-1559)

Purpose: verifies feemarket arithmetic, lifecycle hooks, query APIs, and type validation invariants.

Primary files:
- `app/feemarket_test.go`
- `app/feemarket_types_test.go`

| Test | Description |
| --- | --- |
| `TestFeeMarketCalculateBaseFee` | Verifies base-fee calculation matrix across target gas and min-gas-price scenarios. |
| `TestFeeMarketBeginBlockUpdatesBaseFee` | Verifies BeginBlock updates base fee from prior gas usage inputs. |
| `TestFeeMarketEndBlockGasWantedClamp` | Verifies EndBlock clamps block gas wanted using configured multiplier logic. |
| `TestFeeMarketQueryMethods` | Verifies keeper query methods return consistent params/base-fee/block-gas values. |
| `TestFeeMarketUpdateParamsAuthority` | Verifies only authorized authority can update feemarket params. |
| `TestFeeMarketGRPCQueryClient` | Verifies gRPC query client paths for feemarket endpoints. |
| `TestFeeMarketTypesParamsValidateMatrix` | Verifies feemarket params validation rules across valid/invalid combinations. |
| `TestFeeMarketTypesMsgUpdateParamsValidateBasic` | Verifies basic validation for fee market MsgUpdateParams messages. |
| `TestFeeMarketTypesGenesisValidateMatrix` | Verifies genesis validation matrix for feemarket state. |
