# Unit Tests: EVM Migration (x/evmigration)

Purpose: validates the `x/evmigration` module — dual-signature verification, account/bank/staking/distribution/authz/feegrant/supernode/action/claim migration, preChecks, and full ClaimLegacyAccount message handler flow.

Files: `x/evmigration/types/sigverify/sigverify_test.go`, `x/evmigration/keeper/verify_test.go`, `x/evmigration/keeper/migrate_test.go`, `x/evmigration/keeper/msg_server_claim_legacy_test.go`, `x/evmigration/keeper/msg_server_migrate_validator_test.go`, `x/evmigration/keeper/query_test.go`

| Test | Description |
| --- | --- |
| `TestVerifyCosmosSecp256k1_CLI` | Legacy-side cosmos secp256k1: a CLI-format (SHA256) signature verifies. |
| `TestVerifyCosmosSecp256k1_ADR036` | Legacy-side cosmos secp256k1: an ADR-036 signArbitrary signature (Keplr/Leap path) verifies. |
| `TestVerifyCosmosSecp256k1_EIP191_Rejected` | Legacy-side cosmos secp256k1: EIP-191 format is rejected (wrong side for that format). |
| `TestVerifyCosmosSecp256k1_InvalidSigFormat` | Legacy-side cosmos secp256k1: `SIG_FORMAT_UNSPECIFIED` (default switch branch) is rejected with a clear error. |
| `TestVerifyCosmosSecp256k1_WrongKey` | Legacy-side cosmos secp256k1: a valid-format signature does not verify under a different pubkey (CLI and ADR-036). |
| `TestVerifyEthSecp256k1_CLI_65byte` | New-side eth secp256k1: a 65-byte (R\|\|S\|\|V) CLI signature verifies. |
| `TestVerifyEthSecp256k1_ADR036_65byte` | New-side eth secp256k1: a 65-byte ADR-036 signature verifies. |
| `TestVerifyEthSecp256k1_EIP191_65byte` | New-side eth secp256k1: a 65-byte EIP-191 personal_sign signature verifies. |
| `TestVerifyEthSecp256k1_VByteIgnoredByVerifier` | New-side eth secp256k1: clobbering the recovery V byte does not invalidate an otherwise-valid R\|\|S signature (verifier uses R\|\|S only). |
| `TestVerifyEthSecp256k1_Reject64Byte` | New-side eth secp256k1: a 64-byte signature (R\|\|S, no V) is rejected. |
| `TestVerifyEthSecp256k1_RejectOtherLengths` | New-side eth secp256k1: signatures of any length other than 65 bytes are rejected. |
| `TestVerifyEthSecp256k1_InvalidSigFormat` | New-side eth secp256k1: `SIG_FORMAT_UNSPECIFIED` is rejected. |
| `TestVerifyEthSecp256k1_WrongKey` | New-side eth secp256k1: a valid 65-byte signature does not verify under a different pubkey. |
| `TestMigrateAuth_BaseAccount` | Verifies BaseAccount removal and new account creation. |
| `TestMigrateAuth_ContinuousVesting` | Verifies ContinuousVestingAccount parameters are captured in VestingInfo. |
| `TestMigrateAuth_DelayedVesting` | Verifies DelayedVestingAccount parameters are captured in VestingInfo. |
| `TestMigrateAuth_PeriodicVesting` | Verifies PeriodicVestingAccount parameters including periods are captured. |
| `TestMigrateAuth_PermanentLocked` | Verifies PermanentLockedAccount parameters are captured in VestingInfo. |
| `TestMigrateAuth_ModuleAccount` | Verifies module accounts are rejected. |
| `TestMigrateAuth_AccountNotFound` | Verifies error when legacy account does not exist. |
| `TestMigrateAuth_NewAddressAlreadyExists` | Verifies existing new address account is reused. |
| `TestFinalizeVestingAccount_Continuous` | Verifies ContinuousVestingAccount is recreated from VestingInfo. |
| `TestFinalizeVestingAccount_AccountNotFound` | Verifies error when new account does not exist at finalization. |
| `TestMigrateBank_WithBalance` | Verifies all balances are transferred via SendCoins. |
| `TestMigrateBank_ZeroBalance` | Verifies SendCoins is not called when balance is zero. |
| `TestMigrateBank_MultiDenom` | Verifies multi-denom balances are transferred correctly. |
| `TestMigrateDistribution_WithDelegations` | Verifies pending rewards are withdrawn for all delegations. |
| `TestMigrateDistribution_NoDelegations` | Verifies no-op when there are no delegations. |
| `TestMigrateAuthz_AsGranter` | Verifies grants where legacy is the granter are re-keyed. |
| `TestMigrateAuthz_AsGrantee` | Verifies grants where legacy is the grantee are re-keyed. |
| `TestMigrateAuthz_NoGrants` | Verifies no-op when there are no authz grants. |
| `TestMigrateFeegrant_AsGranter` | Verifies fee allowances where legacy is the granter are re-created. |
| `TestMigrateFeegrant_NoAllowances` | Verifies no-op when there are no fee allowances. |
| `TestMigrateSupernode_Found` | Verifies supernode account field is updated. |
| `TestMigrateSupernode_NotFound` | Verifies no-op when legacy is not a supernode. |
| `TestMigrateActions_CreatorAndSuperNodes` | Verifies Creator and SuperNodes fields are updated. |
| `TestMigrateActions_NoMatch` | Verifies no-op when no actions reference legacy address. |
| `TestMigrateClaim_Found` | Verifies claim record DestAddress is updated. |
| `TestMigrateClaim_NotFound` | Verifies no-op when there is no claim record. |
| `TestMigrateStaking_ActiveDelegations` | Verifies full staking migration: delegation re-keying, starting info, withdraw addr. |
| `TestMigrateStaking_NoDelegations` | Verifies no-op when delegator has no delegations. |
| `TestMigrateStaking_ThirdPartyWithdrawAddress` | Verifies third-party withdraw address is preserved via origWithdrawAddr parameter (bug #16). |
| `TestMigrateStaking_MigratedThirdPartyWithdrawAddress` | Verifies migrated third-party withdraw address is resolved to its new address via MigrationRecords (bug #16). |
| `TestPreChecks_MigrationDisabled` | Verifies rejection when enable_migration is false. |
| `TestPreChecks_MigrationWindowClosed` | Verifies rejection after the configured end time. |
| `TestPreChecks_BlockRateLimitExceeded` | Verifies rejection when per-block migration count exceeds limit. |
| `TestPreChecks_SameAddress` | Verifies rejection when legacy and new addresses are identical. |
| `TestPreChecks_AlreadyMigrated` | Verifies a legacy address cannot be migrated twice. |
| `TestPreChecks_NewAddressWasMigrated` | Verifies new address cannot be a previously-migrated legacy address. |
| `TestPreChecks_NewAddressAlreadyUsed` | Verifies new address cannot be reused as a migration destination (bug #23). |
| `TestPreChecks_ModuleAccount` | Verifies module accounts cannot be migrated. |
| `TestPreChecks_LegacyAccountNotFound` | Verifies error when legacy account does not exist in x/auth. |
| `TestClaimLegacyAccount_ValidatorMustUseMigrateValidator` | Verifies validator operators are directed to MigrateValidator. |
| `TestClaimLegacyAccount_InvalidSignature` | Verifies invalid legacy signature is rejected. |
| `TestClaimLegacyAccount_Success` | Verifies full happy-path: preChecks, signature, migration, record, counters. |
| `TestClaimLegacyAccount_FailAtDistribution` | Failure at step 1 (reward withdrawal) propagates error, no record stored. |
| `TestClaimLegacyAccount_FailAtStaking` | Failure at step 2 (delegation re-keying) propagates error, no record stored. |
| `TestClaimLegacyAccount_FailAtBank` | Failure at step 3b (bank transfer) after auth removal propagates error. Critical atomicity test. |
| `TestClaimLegacyAccount_FailAtAuthz` | Failure at step 4 (authz grant re-keying) propagates error. |
| `TestClaimLegacyAccount_FailAtFeegrant` | Failure at step 5 (feegrant migration) propagates error. |
| `TestClaimLegacyAccount_FailAtSupernode` | Failure at step 6 (supernode migration) propagates error. |
| `TestClaimLegacyAccount_FailAtActions` | Failure at step 7 (action migration) propagates error. |
| `TestClaimLegacyAccount_FailAtClaim` | Failure at step 8 (claim migration, last before finalize) propagates error. |
| `TestClaimLegacyAccount_WithDelegations` | Verifies rewards withdrawal and delegation re-keying during claim. |
| `TestClaimLegacyAccount_MigratedThirdPartyWithdrawAddress` | End-to-end message-server test: third-party withdraw addr resolved to migrated destination (bug #16). |
| `TestMigrateValidator_NotValidator` | Verifies rejection when legacy address is not a validator operator. |
| `TestMigrateValidator_UnbondingValidator` | Verifies rejection when validator is unbonding or unbonded. |
| `TestMigrateValidator_JailedValidator` | Verifies jailed validators are rejected before any validator migration mutation path. |
| `TestMigrateValidator_TooManyDelegators` | Verifies rejection when delegation records exceed MaxValidatorDelegations. |
| `TestMigrateValidator_Success` | Verifies full validator migration: commission, record, delegations, distribution, supernode, account. |
| `TestMigrateValidator_ThirdPartyWithdrawAddrPreserved` | Verifies temporary redirect->withdraw->restore for delegators with already-migrated third-party withdraw addresses (bug #18). |
| `TestQueryMigrationRecord_Found` | Verifies query returns a stored migration record. |
| `TestQueryMigrationRecord_NotFound` | Verifies query returns empty response for unknown address. |
| `TestQueryMigrationRecords_Paginated` | Verifies paginated listing of all migration records. |
| `TestQueryMigrationStats` | Verifies counters and computed stats are returned. |
| `TestQueryMigrationEstimate_NonValidator` | Verifies estimate for non-validator address with delegations. |
| `TestQueryMigrationEstimate_AlreadyMigrated` | Verifies already-migrated addresses report would_succeed=false. |
| `TestQueryLegacyAccounts_WithSecp256k1` | Verifies accounts with secp256k1 pubkeys are listed as legacy. |
| `TestQueryLegacyAccounts_Pagination` | Multi-page offset pagination: page 1 has NextKey, page 2 returns remainder without NextKey. |
| `TestQueryLegacyAccounts_Empty` | Empty response when no legacy accounts exist; Total=0, no NextKey. |
| `TestQueryLegacyAccounts_OffsetBeyondTotal` | Offset beyond total returns empty slice without panic. |
| `TestQueryLegacyAccounts_DefaultLimit` | Nil pagination uses default limit (100) without panic. |
| `TestQueryMigratedAccounts` | Verifies paginated listing of migrated account records. |
| `TestGenesis` | Full genesis round-trip: params, migration records, and counters survive InitGenesis/ExportGenesis. |
| `TestGenesis_DefaultEmpty` | Default empty genesis round-trip: zero records and counters exported correctly. |
| `TestMigrateValidator_FailAtValidatorRecord` | Failure at step V2 (validator record re-key) propagates error. |
| `TestMigrateValidator_FailAtValidatorDistribution` | Failure at step V3 (distribution re-key) propagates error. |
| `TestMigrateValidator_FailAtValidatorDelegations` | Failure at step V4 (delegation re-key) propagates error. |
| `TestMigrateValidator_FailAtValidatorSupernode` | Failure at step V5 (supernode re-key) propagates error. |
| `TestMigrateValidator_FailAtValidatorActions` | Failure at step V6 (action re-key) propagates error. |
| `TestMigrateValidator_FailAtAuth` | Failure at step V7 (auth migration) propagates error. |
| `TestMigrateStaking_WithUnbondingDelegation` | Unbonding delegations re-keyed with queue and UnbondingId indexes. |
| `TestMigrateStaking_WithRedelegation` | Redelegations re-keyed with queue and UnbondingId indexes. |
| `TestMigrateValidatorDelegations_WithUnbondingAndRedelegation` | Validator delegation re-key covers unbonding/redelegation with UnbondingId. |
| `TestMigrateValidatorSupernode_WithMetrics` | Supernode metrics state re-keyed when metrics exist; old key deleted. |
| `TestMigrateValidatorSupernode_MetricsWriteFails` | Metrics write failure propagates as error. |
| `TestMigrateValidatorSupernode_NotFound` | No-op when validator is not a supernode. |
| `TestMigrateValidatorSupernode_EvidenceAddressMigrated` | Evidence entries matching old valoper get ValidatorAddress updated. |
| `TestMigrateValidatorSupernode_AccountHistoryMigrated` | PrevSupernodeAccounts entries matching old account updated; new migration history entry appended. |
| `TestMigrateValidatorSupernode_IndependentAccountPreserved` | Validator migration preserves an already-migrated or otherwise independent supernode account. |
| `TestFinalizeVestingAccount_Delayed` | DelayedVestingAccount correctly recreated at new address. |
| `TestFinalizeVestingAccount_Periodic` | PeriodicVestingAccount recreated with original periods. |
| `TestFinalizeVestingAccount_PermanentLocked` | PermanentLockedAccount correctly recreated at new address. |
| `TestFinalizeVestingAccount_NonBaseAccountFallback` | Non-BaseAccount fallback extracts base account and recreates vesting. |
| `TestQueryParams_NilRequest` | Nil request returns InvalidArgument error. |
| `TestQueryParams_Valid` | Valid request returns stored params. |
| `TestUpdateParams_InvalidAuthority` | Non-authority address rejected with ErrInvalidSigner. |
| `TestUpdateParams_ValidAuthority` | Correct authority updates params successfully. |
| `TestMigrateValidatorDelegations_UsesScopedRedelegationIndexes` | V4's internal scoped scan discovers redelegations where the validator is source or destination (via the val-src/val-dst indexes) and re-keys exactly those, skipping unrelated ones. |
| `TestMigrateValidatorDelegations_DeduplicatesSourceAndDestinationIndexes` | A redelegation with the migrating validator as both source and destination appears in both indexes but is collected and re-keyed exactly once. |
| `TestMigrateValidatorDelegations_UsesPreloadedRedelegations` | A non-nil caller-supplied redelegation slice is re-keyed directly with no scoped-index rescan (store intentionally holds no redelegation rows). |
| `TestMigrateValidatorDelegations_ReturnsErrorForStaleRedelegationIndex` | A redelegation index entry with no backing record aborts V4 with a "points to missing record" error before re-keying anything. |
| `TestMigrateValidatorDistribution_UsesScopedDistributionPrefixes` | Distribution migration reads only the migrating validator's historical-rewards/slash-event prefixes, leaving an unrelated validator's rows untouched. |
| `TestMigrateValidatorDelegations_RekeysMultipleSourceRedelegations` | Two redelegations sharing the val-src index prefix are both collected (iterator advances past the first key) and re-keyed to the new operator. |
| `TestMigrateValidatorDelegations_SetsHistoricalRewardsRefCountOnce` | The target period's historical-rewards reference count is written exactly once as base(1)+N via the scoped O(1) lookup, not reset-then-incremented per delegation. |
| `TestMigrateValidatorDistribution_RekeysAllPeriodsAndSlashEvents` | All of the validator's historical-rewards periods and slash events are re-keyed to the new address; height (from key) and period (from value) are not swapped. |
| `TestMigrateValidatorScopedIteration_SimulatesGlobalStateImprovement` | Simulates a large-chain state shape and asserts validator-scoped iteration touches ~215x fewer KV keys than the old full-chain scan. |
| `TestMigrateValidatorDelegations_RedelegationReplayIsDeterministic` | Redelegations from the scoped scan replay in deterministic store-key order, guarding against Go map-iteration nondeterminism that would diverge app hashes across nodes. |
| `TestMigrateValidator_TooManyDelegatorsIncludesScopedRedelegations` | The MaxValidatorDelegations pre-check counts scoped redelegations (source and destination); exceeding the limit rejects with `ErrTooManyDelegators` even with no plain delegations/unbondings. |
| `TestQueryMigrationEstimate_ValidatorUsesScopedRedelegationIndexesForLimit` | The `MigrationEstimate` query counts a validator's redelegations via scoped indexes (source and destination, excluding unrelated) when reporting the delegation count. |
| `TestMigrateValidatorDelegations_SlashedValidatorUsesTokenStake` | V4 stores `DelegatorStartingInfo.Stake` as tokens-from-shares (truncated) via the re-keyed validator's exchange rate, not raw shares, so an ever-slashed validator (tokens < shares) cannot trip the SDK's final-stake panic on later withdrawals. |
| `TestMigrateStaking_SlashedValidatorUsesTokenStake` | Account-path delegation re-keying (`migrateActiveDelegations`) applies the same shares→token stake conversion for a delegation to an ever-slashed validator. |

**Additional regression coverage**: `TestKeeper_GetSuperNodeByAccount` (in `x/supernode/v1/keeper/`) confirms `GetSuperNodeByAccount` returns the correct supernode for a given account address, exercising the index used by `MigrateSupernode`.

## App-side mempool signer adapter tests

Migration txs are intentionally zero-signer at the Cosmos tx envelope layer; authorization lives in the embedded legacy and new-address proofs. The app-level tests below cover the mempool-specific signer adapter that lets those txs pass app-side mempool admission without weakening non-migration tx validation.

Files: `app/evmigration_signer_extraction_adapter_test.go`, `app/evm_mempool_evmigration_test.go`

| Test | Description |
| ---- | ----------- |
| `TestEVMigrationSignerExtractionAdapter_MigrationOnlyTx_SyntheticSigner` | Extracts a deterministic synthetic signer from `legacy_address` for `MsgClaimLegacyAccount`. |
| `TestEVMigrationSignerExtractionAdapter_MigrationOnlyTx_MigrateValidator` | Extracts the same synthetic signer shape for `MsgMigrateValidator`. |
| `TestEVMigrationSignerExtractionAdapter_NonMigrationTx_DelegatesToFallback` | Non-migration txs keep the normal fallback signer extraction path. |
| `TestEVMigrationSignerExtractionAdapter_MixedTx_DelegatesToFallback` | Mixed migration + non-migration txs are not treated as migration-only. |
| `TestEVMigrationSignerExtractionAdapter_MultipleMigrationMessages_Rejected` | Multi-message migration txs are rejected so one tx cannot map to multiple synthetic signer buckets. |
| `TestEVMigrationSignerExtractionAdapter_EmptyLegacyAddress_Rejected` | Empty `legacy_address` cannot produce a mempool signer. |
| `TestEVMigrationSignerExtractionAdapter_InvalidBech32_Rejected` | Malformed bech32 `legacy_address` is rejected before mempool insertion. |
| `TestEVMigrationSignerExtractionAdapter_NilFallback_FallsBackToDefault` | Nil fallback is replaced with the SDK default adapter without panicking. |
| `TestEVMigrationSignerAdapter_DefaultExtractor_PinsFailureMode` | Pins the upstream SDK default extractor behavior: zero-signer migration txs produce no signers. |
| `TestEVMMempool_SDKPriorityNonceMempoolRejectsZeroSignerMigrationTx` | Demonstrates the raw SDK `PriorityNonceMempool` rejection that the app adapter fixes. |
| `TestEVMMempool_CheckTxAcceptsZeroSignerMigrationTx` | Full app CheckTx path accepts a valid zero-signer migration tx. |
| `TestEVMMempool_CheckTxRejectsProofValidNonexistentLegacyAccount` | Full app CheckTx path rejects a proof-valid zero-signer migration tx when the legacy account is absent from state, before falling back to the generic signer error. |
| `TestEVMMempool_CheckTxRejectsZeroSignerNonMigrationTx` | End-to-end pin: zero-signer non-migration txs are rejected on the live CheckTx path (by the ante's signature verification, before mempool admission). |
| `TestEVMMempool_InsertRejectsZeroSignerNonMigrationTx` | Adapter-layer security pin: drives `mempool.Insert` directly (bypassing the ante) to prove a non-migration tx gets no synthetic signer and is rejected with "tx must have at least one signer". |
| `TestEVMMempool_InsertAcceptsZeroSignerValidatorMigrationTx` | App mempool accepts zero-signer `MsgMigrateValidator`. |
| `TestEVMMempool_InsertRejectsMalformedMigrationLegacyAddress` | App mempool rejects malformed migration `legacy_address`. |
| `TestEVMMempool_InsertRejectsZeroSignerMixedMigrationTx` | Mixed migration/non-migration txs do not get synthetic signer treatment. |
| `TestEVMMempool_DuplicateLegacyMigrationTxDoesNotGrowMempool` | Duplicate txs for the same synthetic legacy-address signer do not grow the mempool. |
| `TestEVMMempool_PrepareProposalIncludesZeroSignerMigrationTx` | Accepted zero-signer migration txs are selected by `PrepareProposal`. |
| `TestVerifyMigrationProofsForAnte_AdmissionGate` | Admission gate: proof-valid migration txs are rejected at the ante (`ErrMigrationDisabled` / `ErrMigrationWindowClosed`) when migration is off or the window has closed, bounding the zero-fee mempool-spam surface to the operator-defined window. |
| `TestVerifyMigrationProofsForAnte_CheapStateAdmission` | Cheap state gate: proof-valid migration txs are rejected at ante admission when the legacy account is missing, the source is already migrated, the destination is already used, or a validator migration source is not a validator. |

## Multisig support tests

### Multisig verifier tests (`x/evmigration/keeper/verify_test.go`)

| Test | Description |
| ---- | ----------- |
| `TestVerifyMigrationProof_NewSide_Multisig_Valid2of3` | New-side 2-of-3 multisig (sub-signers 0 and 2, CLI format) passes the proof verifier. |
| `TestVerifyMigrationProof_NewSide_Multisig_SubSigInvalid_UnderCosmosKeyBytes` | New-side multisig is rejected when a sub-signature is a SHA256-convention Cosmos signature padded to 65 bytes: the outer bound-address check passes but `VerifyEthSecp256k1`'s R\|\|S verify fails. |
| `TestVerifyMigrationProof_NewSide_Multisig_AminoAddressMismatch_OnKeyTypeSwap` | New-side multisig is rejected when the bound address was built under the Cosmos interpretation but the verifier wraps the sub-keys as eth secp256k1 (key-type swap → amino address mismatch). |

### Multisig query tests (`x/evmigration/keeper/query_test.go`)

| Test | Description |
| ---- | ----------- |
| `TestLegacyAccounts_Multisig` | `LegacyAccounts` response includes `is_multisig=true`, correct `threshold` and `num_signers`. |
| `TestMigrationEstimate_Multisig_Supported` | Estimate returns `would_succeed=true` for a valid K-of-N secp256k1 multisig. |
| `TestMigrationEstimate_Multisig_TooManySubKeys` | Estimate returns `would_succeed=false` when `num_signers > MaxMultisigSubKeys`. |
| `TestMigrationEstimate_Multisig_NonSecp256k1SubKey` | Estimate returns `would_succeed=false` when any sub-key is not secp256k1. |

### Type validation tests (`x/evmigration/types/proof_test.go`)

| Test | Description |
| ---- | ----------- |
| `TestSingleKeyProof_ValidateBasic` | Valid and invalid `SingleKeyProof` shapes (nil pub_key, nil sig, unspecified format). |
| `TestMultisigProof_ValidateBasic` | Valid and invalid `MultisigProof` shapes (zero threshold, mismatched indices/sigs length, non-ascending indices, wrong sub-key size, unspecified format). |
| `TestMultisigProof_ValidateParams_SizeCap` | `ValidateParams` rejects when `len(sub_pub_keys) > MaxMultisigSubKeys`. |
| `TestMigrationProof_ValidateBasic_Dispatch` | `MigrationProof.ValidateBasic` dispatches to the correct sub-validator and rejects a nil oneof. |
