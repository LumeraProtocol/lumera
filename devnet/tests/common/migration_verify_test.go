package common

import "testing"

func TestValidateMigrationRecord(t *testing.T) {
	t.Run("valid mapping passes", func(t *testing.T) {
		rec := MigrationRecord{LegacyAddress: "lumera1legacy", NewAddress: "lumera1new", Height: 100}
		if err := ValidateMigrationRecord(rec, true, "lumera1legacy", "lumera1new"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing record fails", func(t *testing.T) {
		if err := ValidateMigrationRecord(MigrationRecord{}, false, "lumera1legacy", "lumera1new"); err == nil {
			t.Error("expected error when record not found")
		}
	})

	t.Run("new address mismatch fails", func(t *testing.T) {
		rec := MigrationRecord{LegacyAddress: "lumera1legacy", NewAddress: "lumera1other"}
		if err := ValidateMigrationRecord(rec, true, "lumera1legacy", "lumera1new"); err == nil {
			t.Error("expected error on new-address mismatch")
		}
	})

	t.Run("legacy address mismatch fails", func(t *testing.T) {
		rec := MigrationRecord{LegacyAddress: "lumera1other", NewAddress: "lumera1new"}
		if err := ValidateMigrationRecord(rec, true, "lumera1legacy", "lumera1new"); err == nil {
			t.Error("expected error on legacy-address mismatch")
		}
	})

	t.Run("empty expected new address skips that check", func(t *testing.T) {
		rec := MigrationRecord{LegacyAddress: "lumera1legacy", NewAddress: "lumera1whatever"}
		if err := ValidateMigrationRecord(rec, true, "lumera1legacy", ""); err != nil {
			t.Errorf("unexpected error when expectedNew empty: %v", err)
		}
	})
}

func TestEstimateConfirmsAlreadyMigrated(t *testing.T) {
	if !EstimateConfirmsAlreadyMigrated(MigrationEstimate{RejectionReason: "already migrated"}) {
		t.Error("expected 'already migrated' rejection to confirm migrated")
	}
	if EstimateConfirmsAlreadyMigrated(MigrationEstimate{WouldSucceed: true}) {
		t.Error("a would-succeed estimate must not confirm already-migrated")
	}
	if EstimateConfirmsAlreadyMigrated(MigrationEstimate{RejectionReason: "is a validator"}) {
		t.Error("an unrelated rejection must not confirm already-migrated")
	}
}
