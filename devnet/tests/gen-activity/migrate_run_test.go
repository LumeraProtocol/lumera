package main

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"

	"gen/tests/common"
)

func errBoom() error { return errors.New("boom") }

func TestEffectiveConcurrency(t *testing.T) {
	cases := []struct {
		parallelism, maxPerBlock, want int
	}{
		{10, 50, 10}, // parallelism is the binding limit
		{50, 10, 10}, // per-block cap is the binding limit
		{5, 0, 5},    // no cap configured -> parallelism
		{0, 10, 1},   // floor of 1
	}
	for _, tc := range cases {
		if got := effectiveConcurrency(tc.parallelism, tc.maxPerBlock); got != tc.want {
			t.Errorf("effectiveConcurrency(%d,%d) = %d, want %d", tc.parallelism, tc.maxPerBlock, got, tc.want)
		}
	}
}

func TestApplyMigrationResult(t *testing.T) {
	t.Run("migrated sets full migration info", func(t *testing.T) {
		rec := legacyRec("gen-0001", "lumera1a", "seed")
		res := migrationResult{Status: MigrationStatusMigrated, NewName: "gen-0001-evm", NewAddress: "lumera1new", TxHash: "TX", Height: 100}
		if !applyMigrationResult(rec, res, "2026-06-16T00:00:00Z") {
			t.Fatal("expected registry change")
		}
		if rec.Migration == nil || rec.Migration.Status != MigrationStatusMigrated ||
			rec.Migration.NewAddress != "lumera1new" || rec.Migration.TxHash != "TX" ||
			rec.Migration.Height != 100 || rec.Migration.MigratedAt != "2026-06-16T00:00:00Z" {
			t.Errorf("migration info not set correctly: %+v", rec.Migration)
		}
	})

	t.Run("failed records error and status", func(t *testing.T) {
		rec := legacyRec("gen-0001", "lumera1a", "seed")
		res := migrationResult{Status: MigrationStatusFailed, Err: errBoom()}
		applyMigrationResult(rec, res, "t")
		if rec.Migration == nil || rec.Migration.Status != MigrationStatusFailed || rec.Migration.Error == "" {
			t.Errorf("failed info not set: %+v", rec.Migration)
		}
	})

	t.Run("planned (dry-run) does not change the registry", func(t *testing.T) {
		rec := legacyRec("gen-0001", "lumera1a", "seed")
		res := migrationResult{Planned: true}
		if applyMigrationResult(rec, res, "t") {
			t.Error("dry-run result must not change the registry")
		}
		if rec.Migration != nil {
			t.Error("dry-run must not set migration info")
		}
	})
}

func TestRunMigrationProcessesAllAndAppliesToRegistry(t *testing.T) {
	reg := NewRegistry("lumera-devnet-1", "funder", "lumera1funder", "legacy", "t0")
	reg.UpsertAccount(legacyRec("gen-0001", "lumera1legacy1", "seed1"))
	reg.UpsertAccount(legacyRec("gen-0002", "lumera1legacy2", "seed2"))

	chain := &fakeMigrationChain{
		records:    map[string]common.MigrationRecord{},
		estimate:   common.MigrationEstimate{WouldSucceed: true},
		keys:       map[string]bool{"gen-0001": true, "gen-0002": true},
		claimHash:  "TX",
		importAddr: map[string]string{},
	}
	var buf bytes.Buffer
	lg := newMigrationLogger(&buf)
	lg.now = fixedClock()
	exec := &migrationExecutor{chain: chain, log: lg}

	var saves int
	var mu sync.Mutex
	save := func(*ActivityRegistry) error { mu.Lock(); saves++; mu.Unlock(); return nil }

	sum, err := runMigration(reg, exec, lg, migrationRunOpts{Parallelism: 2, MaxPerBlock: 50, Now: func() string { return "t1" }}, save)
	if err != nil {
		t.Fatalf("runMigration: %v", err)
	}
	if sum.Migrated != 2 {
		t.Errorf("Migrated = %d, want 2 (summary: %+v)", sum.Migrated, sum)
	}
	for _, rec := range reg.Accounts {
		if rec.Migration == nil || rec.Migration.Status != MigrationStatusMigrated {
			t.Errorf("account %s not marked migrated: %+v", rec.Name, rec.Migration)
		}
	}
	if saves == 0 {
		t.Error("expected registry to be saved at least once")
	}
}

func TestRunMigrationReturnsSaveError(t *testing.T) {
	reg := NewRegistry("lumera-devnet-1", "funder", "lumera1funder", "legacy", "t0")
	reg.UpsertAccount(legacyRec("gen-0001", "lumera1legacy1", "seed1"))

	chain := &fakeMigrationChain{
		records:    map[string]common.MigrationRecord{},
		estimate:   common.MigrationEstimate{WouldSucceed: true},
		keys:       map[string]bool{"gen-0001": true},
		claimHash:  "TX",
		importAddr: map[string]string{},
	}
	var buf bytes.Buffer
	lg := newMigrationLogger(&buf)
	lg.now = fixedClock()
	exec := &migrationExecutor{chain: chain, log: lg}

	sum, err := runMigration(reg, exec, lg, migrationRunOpts{Parallelism: 1, MaxPerBlock: 50, Now: func() string { return "t1" }}, func(*ActivityRegistry) error {
		return errBoom()
	})
	if err == nil {
		t.Fatal("expected registry save error")
	}
	if sum.Migrated != 1 {
		t.Errorf("Migrated = %d, want 1 (summary: %+v)", sum.Migrated, sum)
	}
	if got := buf.String(); !strings.Contains(got, "WARN: registry save failed") {
		t.Errorf("log output missing save warning:\n%s", got)
	}
}

func TestRunMigrationDryRunDoesNotMutate(t *testing.T) {
	reg := NewRegistry("lumera-devnet-1", "funder", "lumera1funder", "legacy", "t0")
	reg.UpsertAccount(legacyRec("gen-0001", "lumera1legacy1", "seed1"))

	chain := &fakeMigrationChain{
		records:  map[string]common.MigrationRecord{},
		estimate: common.MigrationEstimate{WouldSucceed: true},
		keys:     map[string]bool{"gen-0001": true},
	}
	var buf bytes.Buffer
	lg := newMigrationLogger(&buf)
	lg.now = fixedClock()
	exec := &migrationExecutor{chain: chain, log: lg}

	saves := 0
	sum, err := runMigration(reg, exec, lg, migrationRunOpts{Parallelism: 2, MaxPerBlock: 50, DryRun: true, Now: func() string { return "t1" }}, func(*ActivityRegistry) error { saves++; return nil })
	if err != nil {
		t.Fatalf("runMigration: %v", err)
	}
	if sum.Planned != 1 {
		t.Errorf("Planned = %d, want 1", sum.Planned)
	}
	if reg.Accounts[0].Migration != nil {
		t.Error("dry-run must not mutate registry")
	}
	if chain.claimCalls != 0 {
		t.Errorf("claimCalls = %d, want 0 in dry-run", chain.claimCalls)
	}
	if saves != 0 {
		t.Errorf("dry-run must not save registry, saves = %d", saves)
	}
}
