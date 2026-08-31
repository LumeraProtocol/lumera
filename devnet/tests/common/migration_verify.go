package common

import (
	"fmt"
	"strings"
	"time"
)

// ValidateMigrationRecord checks that a migration record exists and maps the
// expected legacy address to the expected new address. An empty expectedNew
// skips the new-address check (useful when the destination is derived on-chain
// and not known a priori).
func ValidateMigrationRecord(rec MigrationRecord, found bool, legacyAddr, expectedNew string) error {
	if !found {
		return fmt.Errorf("no migration record for legacy address %s", legacyAddr)
	}
	if rec.LegacyAddress != "" && rec.LegacyAddress != legacyAddr {
		return fmt.Errorf("migration record legacy address %s != expected %s", rec.LegacyAddress, legacyAddr)
	}
	if expectedNew != "" && rec.NewAddress != expectedNew {
		return fmt.Errorf("migration record new address %s != expected %s", rec.NewAddress, expectedNew)
	}
	return nil
}

// EstimateConfirmsAlreadyMigrated reports whether a migration estimate's
// rejection reason indicates the account is already migrated. This is the
// post-migration invariant: the legacy address should now be rejected as
// already migrated.
func EstimateConfirmsAlreadyMigrated(est MigrationEstimate) bool {
	return strings.Contains(strings.ToLower(est.RejectionReason), "already migrated")
}

// VerifyMigration confirms an account migrated correctly: the on-chain record
// maps legacyAddr -> expectedNew (expectedNew may be empty to skip that check),
// and a fresh estimate now rejects the legacy address as already migrated.
func (c *ChainCLI) VerifyMigration(legacyAddr, expectedNew string) error {
	rec, found, err := c.MigrationRecord(legacyAddr)
	if err != nil {
		return fmt.Errorf("verify: query migration record: %w", err)
	}
	if err := ValidateMigrationRecord(rec, found, legacyAddr, expectedNew); err != nil {
		return err
	}
	if est, err := c.MigrationEstimate(legacyAddr); err == nil && !EstimateConfirmsAlreadyMigrated(est) && est.WouldSucceed {
		return fmt.Errorf("verify: legacy address %s still reports migratable after migration", legacyAddr)
	}
	return nil
}

// WaitForMigrationRecord polls until a migration record for legacyAddr appears
// or the timeout elapses, returning the record. Useful right after a submit when
// indexing may lag a block.
func (c *ChainCLI) WaitForMigrationRecord(legacyAddr string, timeout time.Duration) (MigrationRecord, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		rec, found, err := c.MigrationRecord(legacyAddr)
		if err != nil {
			lastErr = err
		} else if found {
			return rec, nil
		}
		time.Sleep(time.Second)
	}
	if lastErr != nil {
		return MigrationRecord{}, fmt.Errorf("wait for migration record %s: %w", legacyAddr, lastErr)
	}
	return MigrationRecord{}, fmt.Errorf("timed out waiting for migration record %s", legacyAddr)
}
