package main

import "fmt"

// Migration flow kinds: single-sig (mnemonic-derived destination, used by
// regular/vesting/permanent-locked accounts) and multisig (four-step proof flow).
const (
	migrationKindSingleSig = "single-sig"
	migrationKindMultisig  = "multisig"
)

// migrationWorkItem is one account queued for migration, carrying a stable
// correlation id used in every log line for that item so concurrent runs stay
// traceable.
type migrationWorkItem struct {
	Index         int
	Rec           *AccountRecord
	Kind          string
	AccountType   string
	CorrelationID string
}

// memberKeyMaterial is one multisig member's keyring name and (optional) seed
// mnemonic, used by migrate mode to re-import a missing member key.
type memberKeyMaterial struct {
	Name     string
	Mnemonic string
}

// multisigMembers returns the per-member key material for a multisig, preferring
// the richer Members list (carries mnemonics) and falling back to MemberNames
// (names only) for registries written before member mnemonics were persisted.
func multisigMembers(ms *MultisigInfo) []memberKeyMaterial {
	if ms == nil {
		return nil
	}
	if len(ms.Members) > 0 {
		out := make([]memberKeyMaterial, 0, len(ms.Members))
		for _, m := range ms.Members {
			out = append(out, memberKeyMaterial{Name: m.Name, Mnemonic: m.Mnemonic})
		}
		return out
	}
	out := make([]memberKeyMaterial, 0, len(ms.MemberNames))
	for _, n := range ms.MemberNames {
		out = append(out, memberKeyMaterial{Name: n})
	}
	return out
}

// migrationKindOf returns the migration flow a record requires.
func migrationKindOf(rec *AccountRecord) string {
	if rec.Multisig != nil && len(rec.Multisig.MemberNames) > 0 {
		return migrationKindMultisig
	}
	return migrationKindSingleSig
}

// migrationAccountType returns a human-readable account type for logging:
// "regular", a vesting type (e.g. "continuous", "permanent_locked"), or
// "multisig-K-of-N".
func migrationAccountType(rec *AccountRecord) string {
	if rec.Multisig != nil {
		return fmt.Sprintf("multisig-%d-of-%d", rec.Multisig.Threshold, rec.Multisig.Signers)
	}
	if rec.Vesting != nil && rec.Vesting.Type != "" {
		return rec.Vesting.Type
	}
	return "regular"
}

// buildMigrationQueue turns the eligible registry records into an ordered,
// indexed work queue. Indices start at 1 and correlation ids embed the account
// name and index for log traceability.
func buildMigrationQueue(reg *ActivityRegistry) []migrationWorkItem {
	cands := selectMigrationCandidates(reg)
	items := make([]migrationWorkItem, 0, len(cands))
	for i, rec := range cands {
		idx := i + 1
		items = append(items, migrationWorkItem{
			Index:         idx,
			Rec:           rec,
			Kind:          migrationKindOf(rec),
			AccountType:   migrationAccountType(rec),
			CorrelationID: fmt.Sprintf("migrate %s #%d", rec.Name, idx),
		})
	}
	return items
}

// migrationEligible reports whether a registry record should be migrated.
//
// A record is eligible when:
//   - it was created with the legacy (coin-type 118) key style, and
//   - it carries signing material: either a recorded mnemonic (single-sig,
//     vesting, permanent-locked) or multisig member metadata, and
//   - it has not already been migrated successfully (a prior "failed" attempt is
//     retryable, so it stays eligible).
//
// The authoritative on-chain "already migrated" check happens at execution time
// against the migration record; this is the cheap registry-level pre-filter.
func migrationEligible(rec *AccountRecord) bool {
	if rec == nil {
		return false
	}
	if rec.KeyStyle != "" && rec.KeyStyle != "legacy" {
		return false
	}
	if rec.Migration != nil {
		switch rec.Migration.Status {
		case MigrationStatusMigrated, MigrationStatusAlreadyMigrated:
			return false
		}
	}
	hasMnemonic := rec.Mnemonic != ""
	isMultisig := rec.Multisig != nil && len(rec.Multisig.MemberNames) > 0
	return hasMnemonic || isMultisig
}

// selectMigrationCandidates returns the registry records eligible for migration,
// preserving registry order so a run is deterministic.
func selectMigrationCandidates(reg *ActivityRegistry) []*AccountRecord {
	var out []*AccountRecord
	for _, rec := range reg.Accounts {
		if migrationEligible(rec) {
			out = append(out, rec)
		}
	}
	return out
}
