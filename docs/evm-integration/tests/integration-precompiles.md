# Integration Tests: Precompiles

Purpose: validates static precompile read/write paths exposed to EVM callers, including standard, custom Lumera, and cross-runtime (Wasm) precompiles.
Suite: `tests/integration/evm/precompiles/suite_test.go`

## Standard Precompiles

| Test | Description |
| --- | --- |
| `BankPrecompileBalancesViaEthCall` | Verifies bank precompile balance queries via `eth_call`. |
| `DistributionPrecompileQueryPathsViaEthCall` | Verifies distribution precompile query methods via `eth_call`. |
| `GovPrecompileQueryPathsViaEthCall` | Verifies governance precompile query methods via `eth_call`. |
| `StakingPrecompileValidatorViaEthCall` | Verifies staking precompile validator query behavior via `eth_call`. |
| `Bech32PrecompileRoundTripViaEthCall` | Verifies Bech32 precompile address conversion round-trips correctly. |
| `P256PrecompileVerifyViaEthCall` | Verifies P256 precompile signature verification behavior. |
| `StakingPrecompileDelegateTxPath` | Verifies staking delegate tx path through precompile execution. |
| `DistributionPrecompileSetWithdrawAddressTxPath` | Verifies distribution withdraw-address tx path via precompile. |
| `GovPrecompileCancelProposalTxPathFailsForUnknownProposal` | Verifies expected failure behavior for canceling unknown proposals. |
| `SlashingPrecompileGetParamsViaEthCall` | Verifies slashing precompile `getParams` returns valid slashing parameters. |
| `SlashingPrecompileGetSigningInfosViaEthCall` | Verifies `getSigningInfos` returns signing info for genesis validator. |
| `SlashingPrecompileUnjailTxPathFailsWhenNotJailed` | Verifies unjail tx reverts when validator is not jailed. |
| `ICS20PrecompileDenomsViaEthCall` | Verifies ICS20 `denoms` query returns well-formed response. |
| `ICS20PrecompileDenomHashViaEthCall` | Verifies ICS20 `denomHash` query for non-existent trace returns empty hash. |
| `ICS20PrecompileDenomViaEthCall` | Verifies ICS20 `denom` query for non-existent hash returns default struct. |

## Custom Lumera Precompiles

| Test | Description |
| --- | --- |
| `SupernodeRegisterTxPath` | Registers supernode via precompile tx, verifies receipt success and listSuperNodes count. |
| `SupernodeReportMetricsTxPath` | Reports metrics via precompile tx from the registered supernode account, verifies success. |
| `SupernodeReportMetricsTxPathFailsForWrongCaller` | Verifies non-supernode account cannot report metrics (auth check on contract.Caller()). |
| `ActionRequestCascadeTxPathFailsWithBadSignature` | Verifies requestCascade rejects invalid signature format via tx path. |
| `ActionApproveActionTxPathFailsForNonExistent` | Verifies approveAction reverts for non-existent action ID. |

## Gas Metering

| Test | Description |
| --- | --- |
| `PrecompileGasMeteringAccuracy` | Verifies each precompile consumes bounded, non-trivial gas (6 precompiles). |
| `PrecompileGasEstimateMatchesActual` | Verifies eth_estimateGas is within 3x of actual gasUsed for bank precompile. |

## Wasm Precompile (EVM -> CosmWasm)

| Test | Description |
| --- | --- |
| `WasmPrecompileDeployAndQuery` | Deploys hackatom.wasm via CLI, queries `{"verifier":{}}` via precompile `query` method, verifies response contains verifier address. |
| `WasmPrecompileContractInfoViaEthCall` | Verifies `contractInfo` returns correct code ID, creator, admin, and label for deployed contract. |
| `WasmPrecompileRawQueryViaEthCall` | Verifies `rawQuery` reads raw `"config"` storage key from deployed hackatom contract. |
| `WasmPrecompileExecuteTxPath` | Executes hackatom `{"release":{}}` via precompile `execute`, verifies receipt status=0x1. |
| `WasmPrecompileExecuteEmitsEvent` | Verifies `WasmExecuted` EVM log is emitted with correct topic and decodable payload. |
| `WasmPrecompileSenderIdentity` | Verifies `execute` succeeds when sender matches verifier (proves `contract.Caller()` is forwarded, not tx.origin). |
| `WasmPrecompileGasConsumption` | Verifies non-zero, meaningful gas consumption (>21k) for cross-runtime execute calls. |
| `WasmPrecompileEstimateGas` | Verifies `eth_estimateGas` returns a bounded estimate for wasm precompile query calls. |
| `WasmPrecompileExecuteFailsWithBadMessage` | Verifies unrecognized execute message causes receipt status=0x0. |
| `WasmPrecompileExecuteRejectedInEthCall` | Verifies the state-changing wasm `execute` entrypoint is rejected when invoked via read-only `eth_call`. |
| `WasmPrecompileQueryInvalidContract` | Verifies querying a non-existent bech32 contract returns a JSON-RPC error. |
| `WasmPrecompileContractInfoNotFound` | Verifies `contractInfo` for a non-existent contract returns a JSON-RPC error. |
| `WasmPrecompileInvalidBech32Fails` | Verifies invalid bech32 address causes tx revert (status=0x0). |

## CosmWasm -> EVM Plugin Unit Tests

Purpose: validates custom message handler and query handler decorator control flow.
Suite: `app/wasm_evm_plugin_test.go`

| Test | Description |
| --- | --- |
| `EVMMessageHandler_NilCustomPassesThrough` | Verifies nil Custom field delegates to next handler in chain. |
| `EVMMessageHandler_NonEVMCustomPassesThrough` | Verifies non-`evm_call` custom JSON delegates to next handler. |
| `EVMMessageHandler_MalformedJSONPassesThrough` | Verifies malformed JSON in Custom delegates to next handler. |
| `EVMMessageHandler_EVMCallNilPassesThrough` | Verifies `{"evm_call":null}` delegates to next handler. |
| `EVMMessageHandler_ReentrancyBlocked` | Verifies EVM call from depth=1 returns `ErrReentrancyNotAllowed`. |
| `EVMMessageHandler_InvalidContractAddress` | Verifies malformed EVM hex address returns "invalid target contract" error. |
| `EVMMessageHandler_InvalidCalldataHex` | Verifies invalid calldata hex returns "invalid calldata hex" error. |
| `EVMQueryHandler_NilCustomPassesThrough` | Verifies nil Custom query field delegates to wrapped handler. |
| `EVMQueryHandler_NonEVMCustomPassesThrough` | Verifies non-EVM custom query JSON delegates to wrapped handler. |
| `EVMQueryHandler_MalformedJSONPassesThrough` | Verifies malformed JSON in custom query delegates to wrapped handler. |
| `EVMQueryHandler_EVMCallReentrancyBlocked` | Verifies `evm_call` query at depth=1 returns reentrancy error. |
| `EVMQueryHandler_EVMAccountReentrancyBlocked` | Verifies `evm_account` query at depth=1 returns reentrancy error. |

## CosmWasm -> EVM Plugin End-to-End (Planned)

Purpose: validates full Wasm->EVM call paths with actual EVM contract execution.
Suite: `tests/integration/wasm/evm_plugin_test.go` (requires custom CosmWasm contract with `evm_call` support)

| Test | Description |
| --- | --- |
| `WasmToEVMCallTxPath` | CosmWasm contract calls an EVM contract via `evm_call` custom message, verifies state change. |
| `WasmToEVMCallQueryPath` | CosmWasm contract queries an EVM contract via `evm_call` custom query, verifies return data. |
| `WasmToEVMAccountQuery` | CosmWasm contract queries EVM account info via `evm_account` custom query. |
| `WasmToEVMCallGasCapEnforced` | Verifies per-call gas cap is enforced for Wasm->EVM calls. |
| `WasmToEVMCallSenderIdentity` | Verifies the EVM contract sees the wasm contract address as `msg.sender`. |
