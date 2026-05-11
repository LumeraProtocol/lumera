# Unit Tests: EVM Module, Config Guard & Genesis

Purpose: verifies EVM module registration/genesis defaults and production guardrails around test-only global resets.

Primary files:
- `app/evm/config_modules_genesis_test.go`
- `app/evm/prod_guard_test.go`

| Test | Description |
| --- | --- |
| `TestConfigureNoOp` | Verifies `Configure()` remains a safe no-op with current x/vm global config lifecycle. |
| `TestProvideCustomGetSigners` | Verifies custom signer provider exposes MsgEthereumTx custom get-signer registration. |
| `TestLumeraGenesisDefaults` | Verifies Lumera EVM and feemarket genesis defaults match expected chain settings. |
| `TestRegisterModulesMatrix` | Verifies CLI-side registration map includes all EVM modules and wrappers. |
| `TestUpstreamDefaultEvmDenomIsNotLumera` | Documents that cosmos/evm v0.6.0 `DefaultParams().EvmDenom` = `"aatom"` (not `"ulume"`), validating why the v1.20.0 upgrade handler must skip InitGenesis for EVM modules. |
| `TestResetGlobalStateRequiresTestTag` | Verifies reset helper is guarded and requires `test` build tag. |
| `TestSetKeeperDefaultsRequiresTestTag` | Verifies keeper-default mutation helper is guarded behind `test` tag. |
