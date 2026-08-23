# Integration Tests: Ante Handler

Purpose: validates Cosmos-path ante behavior after EVM integration, including fee enforcement and authz message filtering.
Suite: `tests/integration/evm/ante/suite_test.go`

| Test | Description |
| --- | --- |
| `CosmosTxFeeEnforcement` | Verifies low-fee Cosmos txs are rejected and valid-fee txs pass under current ante settings. |
| `AuthzGenericGrantRejectsBlockedMsgTypes` | Ensures authz generic grants cannot authorize blocked EVM message types. |
| `AuthzGenericGrantAllowsNonBlockedMsgType` | Ensures authz generic grants still work for allowed non-EVM message types. |
