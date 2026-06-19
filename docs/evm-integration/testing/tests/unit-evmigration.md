# Unit Tests: EVM Migration (x/evmigration)

Purpose: validates the `x/evmigration` module — dual-signature verification, account/bank/staking/distribution/authz/feegrant/supernode/action/claim migration, preChecks, and full ClaimLegacyAccount message handler flow.

Files: `x/evmigration/keeper/verify_test.go`, `x/evmigration/keeper/migrate_test.go`, `x/evmigration/keeper/msg_server_claim_legacy_test.go`, `x/evmigration/keeper/msg_server_migrate_validator_test.go`, `x/evmigration/keeper/query_test.go`

| Test | Description |
| --- | --- |
| `TestVerifyLegacySignature_Valid` | Verifies a correctly signed migration message passes verification. |
| `TestVerifyLegacySignature_InvalidPubKeySize` | Rejects public keys that are not exactly 33 bytes (compressed secp256k1). |
| `TestVerifyLegacySignature_PubKeyAddressMismatch` | Rejects when the public key does not derive to the claimed legacy address. |
| `TestVerifyLegacySignature_InvalidSignature` | Rejects a signature produced by a different private key. |
| `TestVerifyLegacySignature_WrongMessage` | Rejects a valid signature produced over a different new address. |
| `TestVerifyLegacySignature_EmptySignature` | Rejects a nil/empty signature. |
| `TestVerifyNewSignature_EIP191` | Verifies EIP-191 personal_sign signature (Keplr/Leap wallet path) passes new key verification. |
| `TestVerifyNewSignature_EIP191_Validator` | Verifies EIP-191 path works for the "validator" migration kind. |
| `TestVerifyNewSignature_EIP191_WrongKey` | Rejects an EIP-191 signature from the wrong private key. |
| `TestVerifyLegacySignature_ADR036` | Verifies ADR-036 signArbitrary signature (Keplr/Leap wallet path) passes legacy key verification. |
| `TestVerifyLegacySignature_ADR036_Validator` | Verifies ADR-036 path works for the "validator" migration kind. |
| `TestVerifyLegacySignature_ADR036_WrongKey` | Rejects an ADR-036 signature from the wrong private key. |
| `TestVerifyLegacySignature_ADR036_WrongSigner` | Rejects ADR-036 signature with mismatched signer field in the sign doc. |
| `TestVerifyLegacySignature_ADR036_DocFormat` | Verifies canonical ADR-036 JSON structure matches expected format byte-for-byte. |
| `TestVerifyNewSignature_EIP191_PayloadFormat` | Verifies EIP-191 prefix construction is correct for a known payload. |
| `TestVerifyLegacySignature_BothPathsRejectGarbage` | Verifies neither raw nor ADR-036 path accepts a garbage signature. |
| `TestVerifyNewSignature_BothPathsRejectGarbage` | Verifies neither raw nor EIP-191 path accepts a garbage signature. |
| `TestVerifyLegacySignature_ChainIDMismatch` | Signs legacy proof with wrong chain ID, verifies error includes the expected chain ID to help diagnose mismatches. |
| `TestVerifyNewSignature_ChainIDMismatch` | Signs new proof with wrong chain ID, verifies address-mismatch error includes chain ID hint. |
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
| `TestEVMMempool_CheckTxRejectsZeroSignerNonMigrationTx` | End-to-end pin: zero-signer non-migration txs are rejected on the live CheckTx path (by the ante's signature verification, before mempool admission). |
| `TestEVMMempool_InsertRejectsZeroSignerNonMigrationTx` | Adapter-layer security pin: drives `mempool.Insert` directly (bypassing the ante) to prove a non-migration tx gets no synthetic signer and is rejected with "tx must have at least one signer". |
| `TestEVMMempool_InsertAcceptsZeroSignerValidatorMigrationTx` | App mempool accepts zero-signer `MsgMigrateValidator`. |
| `TestEVMMempool_InsertRejectsMalformedMigrationLegacyAddress` | App mempool rejects malformed migration `legacy_address`. |
| `TestEVMMempool_InsertRejectsZeroSignerMixedMigrationTx` | Mixed migration/non-migration txs do not get synthetic signer treatment. |
| `TestEVMMempool_DuplicateLegacyMigrationTxDoesNotGrowMempool` | Duplicate txs for the same synthetic legacy-address signer do not grow the mempool. |
| `TestEVMMempool_PrepareProposalIncludesZeroSignerMigrationTx` | Accepted zero-signer migration txs are selected by `PrepareProposal`. |
| `TestVerifyMigrationProofsForAnte_AdmissionGate` | Admission gate: proof-valid migration txs are rejected at the ante (`ErrMigrationDisabled` / `ErrMigrationWindowClosed`) when migration is off or the window has closed, bounding the zero-fee mempool-spam surface to the operator-defined window. |

## Multisig support tests

### Multisig verifier tests (`x/evmigration/keeper/verify_test.go`)

| Test | Description |
| ---- | ----------- |
| `TestVerifyLegacyProof_Multisig_ValidCLI` | 2-of-3 multisig with CLI sig format passes verifier. |
| `TestVerifyLegacyProof_Multisig_ValidADR036` | 2-of-3 multisig with ADR-036 sig format passes verifier. |
| `TestVerifyLegacyProof_Multisig_1of1` | 1-of-1 multisig (degenerate edge case) passes verifier. |
| `TestVerifyLegacyProof_Multisig_WrongAddress` | Proof whose recovered address does not match `legacy_address` is rejected. |
| `TestVerifyLegacyProof_Multisig_InvalidSubSig` | One corrupted sub-signature causes rejection. |
| `TestVerifyLegacyProof_Multisig_N20Boundary` | N=20 (at `MaxMultisigSubKeys`) passes; N=21 is rejected by `ValidateParams`. |

### Multisig query tests (`x/evmigration/keeper/query_test.go`)

| Test | Description |
| ---- | ----------- |
| `TestLegacyAccounts_Multisig` | `LegacyAccounts` response includes `is_multisig=true`, correct `threshold` and `num_signers`. |
| `TestMigrationEstimate_Multisig_Supported` | Estimate returns `would_succeed=true` for a valid K-of-N secp256k1 multisig. |
| `TestMigrationEstimate_Multisig_TooManySubKeys` | Estimate returns `would_succeed=false` when `num_signers > MaxMultisigSubKeys`. |
| `TestMigrationEstimate_Multisig_NonSecp256k1` | Estimate returns `would_succeed=false` when any sub-key is not secp256k1. |

### Type validation tests (`x/evmigration/types/proof_test.go`)

| Test | Description |
| ---- | ----------- |
| `TestSingleKeyProof_ValidateBasic` | Valid and invalid `SingleKeyProof` shapes (nil pub_key, nil sig, unspecified format). |
| `TestMultisigProof_ValidateBasic` | Valid and invalid `MultisigProof` shapes (zero threshold, mismatched indices/sigs length, non-ascending indices, wrong sub-key size, unspecified format). |
| `TestMultisigProof_ValidateParams_SizeCap` | `ValidateParams` rejects when `len(sub_pub_keys) > MaxMultisigSubKeys`. |
| `TestLegacyProof_ValidateBasic_Dispatch` | `LegacyProof.ValidateBasic` dispatches to the correct sub-validator and rejects a nil oneof. |
