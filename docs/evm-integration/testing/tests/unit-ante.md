# Unit Tests: EVM Ante Decorators

Purpose: verifies dual-route ante behavior and decorator-level Ethereum/Cosmos transaction validation logic.

Primary files:
- `app/evm/ante_decorators_test.go`
- `app/evm/ante_fee_checker_test.go`
- `app/evm/ante_gas_wanted_test.go`
- `app/evm/ante_handler_test.go`
- `app/evm/ante_min_gas_price_test.go`
- `app/evm/ante_mono_decorator_test.go`
- `app/evm/ante_nonce_test.go`
- `app/evm/ante_sigverify_test.go`

| Test | Description |
| --- | --- |
| `TestRejectMessagesDecorator` | Verifies Cosmos ante path rejects blocked message types (for example MsgEthereumTx). |
| `TestAuthzLimiterDecorator` | Verifies authz limiter blocks grants for restricted message types. |
| `TestDynamicFeeCheckerMatrix` | Verifies dynamic fee checker decisions across representative gas-fee inputs. |
| `TestGasWantedDecoratorMatrix` | Verifies gas-wanted accounting updates are applied correctly per tx path. |
| `TestNewAnteHandlerRequiredDependencies` | Verifies NewAnteHandler fails fast when required keeper/dependency inputs are missing. |
| `TestNewAnteHandlerRoutesEthereumExtension` | Verifies extension option routes Ethereum txs to EVM ante chain. |
| `TestNewAnteHandlerRoutesDynamicFeeExtensionToCosmosPath` | Verifies dynamic-fee extension routes tx to Cosmos ante path. |
| `TestNewAnteHandlerDefaultRouteWithoutExtension` | Verifies txs without EVM extension use default Cosmos ante path. |
| `TestNewAnteHandlerPendingTxListenerTriggeredForEVMCheckTx` | Verifies pending-tx listener fires for EVM CheckTx path. |
| `TestNewAnteHandlerPendingTxListenerNotTriggeredOnCosmosPath` | Verifies pending-tx listener does not trigger on Cosmos ante path. |
| `TestMinGasPriceDecoratorMatrix` | Verifies min gas price decorator behavior across accepted/rejected fee cases. |
| `TestEVMMonoDecoratorMatrix` | Verifies EVM mono decorator baseline validation matrix. |
| `TestEVMMonoDecoratorRejectsInvalidTxType` | Verifies EVM mono decorator rejects unsupported tx types. |
| `TestEVMMonoDecoratorRejectsNonEthereumMessage` | Verifies EVM mono decorator rejects non-Ethereum message payloads. |
| `TestEVMMonoDecoratorRejectsSenderMismatch` | Verifies EVM mono decorator rejects signer/from mismatches. |
| `TestEVMMonoDecoratorRejectsInsufficientBalance` | Verifies EVM mono decorator rejects txs with insufficient sender balance for fees/value. |
| `TestEVMMonoDecoratorRejectsNonEOASender` | Verifies EVM mono decorator rejects non-EOA senders where required. |
| `TestEVMMonoDecoratorAllowsDelegatedCodeSender` | Verifies delegated-code sender case is accepted when rules permit it. |
| `TestEVMMonoDecoratorRejectsGasFeeCapBelowBaseFee` | Verifies tx is rejected when fee cap is below current base fee. |
| `TestIncrementNonceMatrix` | Verifies nonce increment semantics across successful tx paths. |
| `TestSigVerificationGasConsumerMatrix` | Verifies signature verification gas charging across key/signature types. |
