package main

import (
	"fmt"

	"gen/tests/common"
)

// migrationChain is the chain seam the migration executor depends on. It is
// satisfied in production by *common.ChainCLI and in tests by a fake, so the
// executor's control flow is unit-testable without a live chain.
type migrationChain interface {
	MigrationRecord(legacyAddr string) (common.MigrationRecord, bool, error)
	MigrationEstimate(addr string) (common.MigrationEstimate, error)
	HasKey(name string) bool
	ShowAddress(name string) (string, error)
	ImportKeyWithStyle(name, mnemonic string, style common.KeyStyle) (string, error)
	ClaimLegacyAccount(legacyKey, newKey string) (string, error)
	MigrateMultisigProof(legacyBase, legacyAddr string, members []string, threshold, signers int) (common.MultisigProofResult, error)
}

// migrationResult is the outcome of attempting one account's migration. It is
// produced by a worker and applied to the registry by the single coordinator.
type migrationResult struct {
	Item       migrationWorkItem
	Status     string // migrated, already_migrated, skipped, failed
	NewName    string
	NewAddress string
	TxHash     string
	Height     int64
	Reason     string // skip/reject reason
	Err        error
	Planned    bool // dry-run: nothing was submitted, registry must not change
}

// migrationExecutor runs individual account migrations against a migrationChain,
// emitting correlated log lines for every lifecycle step.
type migrationExecutor struct {
	chain migrationChain
	log   *migrationLogger
}

// evmKeyName is the keyring name for an account's EVM-compatible destination key.
func evmKeyName(legacyName string) string { return legacyName + "-evm" }

// migrateOne executes the full lifecycle for one work item and returns its
// result. In dry-run it stops after the estimate, never importing keys or
// submitting a tx. Multisig items are dispatched to the four-step flow.
func (e *migrationExecutor) migrateOne(item migrationWorkItem, dryRun bool) migrationResult {
	rec := item.Rec
	cid := item.CorrelationID
	e.log.logf(cid, "queued: account=%s type=%s kind=%s addr=%s", rec.Name, item.AccountType, item.Kind, rec.Address)

	// 1. Already migrated on-chain? Authoritative skip.
	if mr, found, err := e.chain.MigrationRecord(rec.Address); err != nil {
		return e.fail(item, fmt.Errorf("query migration-record: %w", err))
	} else if found {
		e.log.logf(cid, "already migrated on-chain -> %s (height %d)", mr.NewAddress, mr.Height)
		return migrationResult{Item: item, Status: MigrationStatusAlreadyMigrated, NewAddress: mr.NewAddress, Height: mr.Height}
	}

	// 2. Estimate — confirms migratability and surfaces the rejection reason.
	est, err := e.chain.MigrationEstimate(rec.Address)
	if err != nil {
		return e.fail(item, fmt.Errorf("query migration-estimate: %w", err))
	}
	e.log.logf(cid, "estimate: would_succeed=%v multisig=%v validator=%v delegations=%d balance=%q",
		est.WouldSucceed, est.IsMultisig, est.IsValidator, est.DelegationCount, est.BalanceSummary)
	if !est.WouldSucceed {
		e.log.logf(cid, "SKIP: estimate rejects migration: %s", est.RejectionReason)
		return migrationResult{Item: item, Status: MigrationStatusSkipped, Reason: est.RejectionReason}
	}

	// 3. Dry-run stops before any mutation.
	if dryRun {
		e.log.logf(cid, "dry-run: would migrate %s (%s) via %s flow", rec.Name, item.AccountType, item.Kind)
		return migrationResult{Item: item, Planned: true}
	}

	// 4. Dispatch by flow.
	switch item.Kind {
	case migrationKindMultisig:
		return e.migrateMultisig(item)
	default:
		return e.migrateSingleSig(item)
	}
}

// migrateSingleSig migrates a regular/vesting/permanent-locked legacy account by
// deriving an EVM-compatible destination key from the same mnemonic and
// submitting a single-sig claim.
func (e *migrationExecutor) migrateSingleSig(item migrationWorkItem) migrationResult {
	rec := item.Rec
	cid := item.CorrelationID

	if rec.Mnemonic == "" {
		return e.fail(item, fmt.Errorf("no recorded mnemonic; cannot derive keys"))
	}

	// Ensure the legacy signing key is in the keyring.
	legacyName := rec.Name
	if !e.chain.HasKey(legacyName) {
		if _, err := e.chain.ImportKeyWithStyle(legacyName, rec.Mnemonic, common.KeyStyleLegacy); err != nil {
			return e.fail(item, fmt.Errorf("import legacy key: %w", err))
		}
		e.log.logf(cid, "imported legacy key %s", legacyName)
	}

	// Derive (or reuse) the EVM-compatible destination key.
	newName := evmKeyName(legacyName)
	var newAddr string
	if e.chain.HasKey(newName) {
		addr, err := e.chain.ShowAddress(newName)
		if err != nil {
			return e.fail(item, fmt.Errorf("resolve existing destination key: %w", err))
		}
		newAddr = addr
	} else {
		addr, err := e.chain.ImportKeyWithStyle(newName, rec.Mnemonic, common.KeyStyleEVM)
		if err != nil {
			return e.fail(item, fmt.Errorf("derive EVM destination key: %w", err))
		}
		newAddr = addr
	}
	e.log.logf(cid, "destination EVM key %s -> %s", newName, newAddr)

	// Submit the claim and wait for inclusion (ClaimLegacyAccount waits).
	e.log.logf(cid, "submitting claim-legacy-account %s -> %s", rec.Address, newAddr)
	txHash, err := e.chain.ClaimLegacyAccount(legacyName, newName)
	if err != nil {
		return e.fail(item, fmt.Errorf("claim-legacy-account: %w", err))
	}
	e.log.logf(cid, "claim included: tx=%s", txHash)

	return e.finalizeMigration(item, newName, newAddr, txHash)
}

// finalizeMigration verifies the on-chain migration record using the shared
// validator and returns the migrated result. A validation warning does not fail
// the result (the tx already committed); it is logged for troubleshooting.
func (e *migrationExecutor) finalizeMigration(item migrationWorkItem, newName, newAddr, txHash string) migrationResult {
	rec := item.Rec
	cid := item.CorrelationID
	height := int64(0)

	mr, found, qerr := e.chain.MigrationRecord(rec.Address)
	switch {
	case qerr != nil:
		e.log.logf(cid, "WARN: post-migration record query failed: %v", qerr)
	case common.ValidateMigrationRecord(mr, found, rec.Address, newAddr) != nil:
		// Destination may be derived on-chain; re-validate without pinning the
		// expected new address and adopt the recorded one.
		if err := common.ValidateMigrationRecord(mr, found, rec.Address, ""); err != nil {
			e.log.logf(cid, "WARN: post-migration validation: %v", err)
		} else {
			if mr.NewAddress != "" {
				newAddr = mr.NewAddress
			}
			height = mr.Height
		}
	default:
		height = mr.Height
	}

	e.log.logf(cid, "MIGRATED: %s -> %s tx=%s height=%d", rec.Address, newAddr, txHash, height)
	return migrationResult{
		Item: item, Status: MigrationStatusMigrated,
		NewName: newName, NewAddress: newAddr, TxHash: txHash, Height: height,
	}
}

// migrateMultisig migrates a legacy K-of-N multisig account via the four-step
// proof flow: it creates a fresh destination EVM multisig and runs
// generate -> sign(×K) -> combine -> submit. The member sub-keys must already
// be in the keyring (gen-activity creates them at generation time).
func (e *migrationExecutor) migrateMultisig(item migrationWorkItem) migrationResult {
	rec := item.Rec
	cid := item.CorrelationID
	if rec.Multisig == nil || len(rec.Multisig.MemberNames) == 0 {
		return e.fail(item, fmt.Errorf("multisig metadata missing member names"))
	}
	ms := rec.Multisig
	e.log.logf(cid, "multisig proof flow: %d-of-%d members=%v", ms.Threshold, ms.Signers, ms.MemberNames)

	// Ensure every legacy member sub-key is in the keyring before the ceremony.
	// The proof flow signs with these keys; unlike single-sig, they cannot be
	// derived on the fly. Import any missing key from its stored mnemonic; if a
	// key is missing and unrecoverable, fail fast BEFORE creating destination
	// keys so the run leaves no orphan keys and the operator gets a clear reason.
	var missing []string
	for _, m := range multisigMembers(ms) {
		if e.chain.HasKey(m.Name) {
			continue
		}
		if m.Mnemonic == "" {
			missing = append(missing, m.Name)
			continue
		}
		if _, err := e.chain.ImportKeyWithStyle(m.Name, m.Mnemonic, common.KeyStyleLegacy); err != nil {
			return e.fail(item, fmt.Errorf("import multisig member %s from stored mnemonic: %w", m.Name, err))
		}
		e.log.logf(cid, "imported missing member key %s from stored mnemonic", m.Name)
	}
	if len(missing) > 0 {
		return e.fail(item, fmt.Errorf(
			"multisig member keys not in keyring and no stored mnemonic to import them: %v; "+
				"run from the keyring that generated them (--home/--keyring-backend) or regenerate the "+
				"registry so member mnemonics are recorded", missing))
	}

	res, err := e.chain.MigrateMultisigProof(rec.Name, rec.Address, ms.MemberNames, ms.Threshold, ms.Signers)
	if err != nil {
		return e.fail(item, fmt.Errorf("multisig proof migration: %w", err))
	}

	return e.finalizeMigration(item, res.NewName, res.NewAddress, res.TxHash)
}

// fail logs and wraps a failed migration result.
func (e *migrationExecutor) fail(item migrationWorkItem, err error) migrationResult {
	e.log.logf(item.CorrelationID, "FAILED: %v", err)
	return migrationResult{Item: item, Status: MigrationStatusFailed, Err: err}
}
