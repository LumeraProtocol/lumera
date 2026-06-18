package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"gen/tests/common"
)

// migrationPreflight rejects a migration run when the chain's migration window
// is not open. Fatal preflight conditions stop the run before any worker starts.
func migrationPreflight(params common.MigrationParams) error {
	return migrationPreflightAt(params, time.Now().Unix())
}

func migrationPreflightAt(params common.MigrationParams, nowUnix int64) error {
	if !params.EnableMigration {
		return fmt.Errorf("migration is disabled on this chain (params.enable_migration=false); the migration window is closed")
	}
	if params.MigrationEndTime > 0 && nowUnix > params.MigrationEndTime {
		return fmt.Errorf("migration window closed at %s (migration_end_time=%d)",
			time.Unix(params.MigrationEndTime, 0).UTC().Format(time.RFC3339), params.MigrationEndTime)
	}
	return nil
}

// runMigrateMode is the entry point for -mode=migrate. It loads the registry,
// preflights the chain's migration params, then runs the bounded worker pool.
// It is invoked from run() when the resolved mode is migrate.
func runMigrateMode(cfg *Config) error {
	cli := newChainCLI(cfg)
	lg := newMigrationLogger(os.Stdout)

	reg, err := LoadRegistry(cfg.AccountsPath)
	if err != nil {
		return fmt.Errorf("migrate mode requires an existing registry at %s: %w", cfg.AccountsPath, err)
	}

	// Preflight against chain params (skipped in dry-run so planning works offline).
	maxPerBlock := 0
	if !cfg.DryRun {
		params, perr := cli.MigrationParams()
		if perr != nil {
			return fmt.Errorf("query migration params: %w", perr)
		}
		if err := migrationPreflight(params); err != nil {
			return err
		}
		maxPerBlock = params.MaxMigrationsPerBlock
		lg.logf("coordinator", "chain migration params: enabled=%v max_per_block=%d end_time=%d",
			params.EnableMigration, params.MaxMigrationsPerBlock, params.MigrationEndTime)
	}

	exec := &migrationExecutor{chain: cli, log: lg}
	save := func(r *ActivityRegistry) error {
		return r.Save(cfg.AccountsPath, time.Now().UTC().Format(time.RFC3339))
	}
	opts := migrationRunOpts{
		Parallelism: cfg.Parallelism,
		MaxPerBlock: maxPerBlock,
		DryRun:      cfg.DryRun,
		Now:         func() string { return time.Now().UTC().Format(time.RFC3339) },
	}

	sum, err := runMigration(reg, exec, lg, opts, save)
	if err != nil {
		return err
	}
	log.Printf("migrate mode complete: migrated=%d already=%d skipped=%d failed=%d planned=%d",
		sum.Migrated, sum.AlreadyMigrated, sum.Skipped, sum.Failed, sum.Planned)
	if sum.Failed > 0 {
		return fmt.Errorf("%d account(s) failed to migrate", sum.Failed)
	}
	return nil
}
