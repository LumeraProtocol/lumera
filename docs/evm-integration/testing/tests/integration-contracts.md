# Integration Tests: Contract Lifecycle

Purpose: exercises contract lifecycle paths (deploy/call/revert) and persistence guarantees across restarts.
Suite: `tests/integration/evm/contracts/suite_test.go`

| Test | Description |
| --- | --- |
| `ContractDeployCallAndLogsE2E` | Deploys a contract, executes calls, and validates receipt/log behavior end to end. |
| `ContractRevertTxReceiptAndGasE2E` | Sends a reverting tx and checks expected revert/receipt/gas semantics. |
| `CALLBetweenContracts` | Deploys caller/callee pair, validates CALL opcode returns data cross-contract. |
| `DELEGATECALLPreservesContext` | Verifies DELEGATECALL writes to proxy's storage, not target contract's storage. |
| `CREATE2DeterministicAddress` | Factory deploys child via CREATE2; verifies deterministic address off-chain. |
| `STATICCALLCannotModifyState` | Confirms STATICCALL reverts when the target contract attempts SSTORE. |
| `TestContractCodePersistsAcrossRestart` | Confirms deployed runtime bytecode remains queryable after node restart. |
| `TestContractStoragePersistsAcrossRestart` | Confirms contract storage values remain intact after node restart. |
| `TestEVMStatePreservationAcrossRestart` | Deploys contract, restarts node, verifies code/storage/receipts survive intact. |
| `TestConcurrentMixedEVMOperations` | 5 concurrent goroutines (transfers + deploys) verify no panics/deadlocks/lost txs. |
| `TestERC20ApproveAllowanceTransferFrom` | Full ERC20 flow: deploy, approve, check allowance, transferFrom, verify balances. |
| `ContractProxiesActionGetParams` | Deploys STATICCALL proxy -> action precompile (0x0901), verifies getParams() response. |
| `ContractProxiesSupernodeGetParams` | Deploys STATICCALL proxy -> supernode precompile (0x0902), verifies getParams() response. |
| `ContractProxiesActionGetActionFee` | Proxy forwards getActionFee(100) with ABI-encoded args, validates fee arithmetic. |
| `ContractQueriesBothPrecompiles` | Two proxies query action + supernode precompiles in same test, cross-validates results. |
