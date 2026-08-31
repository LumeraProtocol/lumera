package common

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMigrationEstimate(t *testing.T) {
	t.Run("single-sig with string-encoded counts", func(t *testing.T) {
		out := `{
			"delegation_count": "5",
			"unbonding_count": "2",
			"redelegation_count": "2",
			"authz_grant_count": "5",
			"feegrant_count": "3",
			"total_touched": "19",
			"would_succeed": true,
			"action_count": "2",
			"balance_summary": "4562270ulume"
		}`
		est, err := parseMigrationEstimate(out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !est.WouldSucceed {
			t.Error("WouldSucceed = false, want true")
		}
		if est.DelegationCount != 5 {
			t.Errorf("DelegationCount = %d, want 5", est.DelegationCount)
		}
		if est.IsMultisig {
			t.Error("IsMultisig = true, want false")
		}
		if est.BalanceSummary != "4562270ulume" {
			t.Errorf("BalanceSummary = %q, want 4562270ulume", est.BalanceSummary)
		}
	})

	t.Run("multisig with rejection reason", func(t *testing.T) {
		out := `{"would_succeed": false, "is_multisig": true, "is_validator": false, "rejection_reason": "already migrated"}`
		est, err := parseMigrationEstimate(out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if est.WouldSucceed {
			t.Error("WouldSucceed = true, want false")
		}
		if !est.IsMultisig {
			t.Error("IsMultisig = false, want true")
		}
		if est.RejectionReason != "already migrated" {
			t.Errorf("RejectionReason = %q, want 'already migrated'", est.RejectionReason)
		}
	})
}

func TestParseMigrationParams(t *testing.T) {
	t.Run("wrapped in params envelope", func(t *testing.T) {
		out := `{"params": {"enable_migration": true, "migration_end_time": "0", "max_migrations_per_block": "50", "max_validator_delegations": "2000", "max_multisig_sub_keys": 20}}`
		p, err := parseMigrationParams(out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !p.EnableMigration {
			t.Error("EnableMigration = false, want true")
		}
		if p.MaxMigrationsPerBlock != 50 {
			t.Errorf("MaxMigrationsPerBlock = %d, want 50", p.MaxMigrationsPerBlock)
		}
		if p.MaxMultisigSubKeys != 20 {
			t.Errorf("MaxMultisigSubKeys = %d, want 20", p.MaxMultisigSubKeys)
		}
	})

	t.Run("bare params object", func(t *testing.T) {
		out := `{"enable_migration": false, "migration_end_time": 1781634504, "max_migrations_per_block": 10}`
		p, err := parseMigrationParams(out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.EnableMigration {
			t.Error("EnableMigration = true, want false")
		}
		if p.MigrationEndTime != 1781634504 {
			t.Errorf("MigrationEndTime = %d, want 1781634504", p.MigrationEndTime)
		}
	})
}

func TestMigrationParamsReportsUnsupportedBinary(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "lumerad")
	script := `#!/bin/sh
cat <<'OUT'
Querying subcommands

Usage:
  lumerad query [flags]
  lumerad query [command]

Available Commands:
  action              Querying commands for the action module
  audit               Audit query commands
OUT
exit 1
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake lumerad: %v", err)
	}

	_, err := (&ChainCLI{Bin: bin, ChainID: "lumera-devnet-1", RPC: "tcp://localhost:26657"}).MigrationParams()
	if err == nil {
		t.Fatal("expected unsupported evmigration binary error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "does not support x/evmigration queries") {
		t.Fatalf("error %q missing unsupported evmigration context", msg)
	}
	if !strings.Contains(msg, bin) {
		t.Fatalf("error %q missing configured binary path %q", msg, bin)
	}
}

func TestParseMigrationStats(t *testing.T) {
	out := `{"total_migrated": "14", "total_legacy": "104", "total_legacy_staked": "30", "total_validators_migrated": "3", "total_validators_legacy": "2"}`
	s, err := parseMigrationStats(out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.TotalMigrated != 14 || s.TotalLegacy != 104 || s.TotalLegacyStaked != 30 ||
		s.TotalValidatorsMigrated != 3 || s.TotalValidatorsLegacy != 2 {
		t.Errorf("stats mismatch: %+v", s)
	}
}

func TestParseMigrationRecord(t *testing.T) {
	t.Run("present record", func(t *testing.T) {
		out := `{"record": {"legacy_address": "lumera1legacy", "new_address": "lumera1new", "height": "64566"}}`
		rec, found, err := parseMigrationRecord(out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found {
			t.Fatal("found = false, want true")
		}
		if rec.LegacyAddress != "lumera1legacy" || rec.NewAddress != "lumera1new" {
			t.Errorf("record = %+v, want legacy/new addresses set", rec)
		}
		if rec.Height != 64566 {
			t.Errorf("Height = %d, want 64566", rec.Height)
		}
	})

	t.Run("present record with migration height", func(t *testing.T) {
		out := `{"record": {"legacy_address": "lumera1legacy", "new_address": "lumera1new", "migration_height": "65194"}}`
		rec, found, err := parseMigrationRecord(out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found {
			t.Fatal("found = false, want true")
		}
		if rec.Height != 65194 {
			t.Errorf("Height = %d, want 65194", rec.Height)
		}
	})

	t.Run("absent record", func(t *testing.T) {
		out := `{}`
		_, found, err := parseMigrationRecord(out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Error("found = true, want false for empty response")
		}
	})

	t.Run("empty record object is not found", func(t *testing.T) {
		out := `{"record": {}}`
		_, found, err := parseMigrationRecord(out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Error("found = true, want false for empty record object")
		}
	})
}

func TestFlexInt(t *testing.T) {
	t.Run("empty input is zero", func(t *testing.T) {
		got, err := flexInt(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 0 {
			t.Errorf("flexInt(nil) = %d, want 0", got)
		}
	})

	t.Run("string number", func(t *testing.T) {
		got, err := flexInt(json.RawMessage(`"42"`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 42 {
			t.Errorf("flexInt(\"42\") = %d, want 42", got)
		}
	})

	t.Run("json number", func(t *testing.T) {
		got, err := flexInt(json.RawMessage(`17`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 17 {
			t.Errorf("flexInt(17) = %d, want 17", got)
		}
	})

	t.Run("out of range string", func(t *testing.T) {
		_, err := flexInt(json.RawMessage(`"9223372036854775808"`))
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "parse") {
			t.Errorf("error = %q, want parse context", err)
		}
	})
}
