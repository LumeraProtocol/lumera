package main

import (
	"fmt"
	"sync"
)

// migrationRunOpts configures a migration run.
type migrationRunOpts struct {
	Parallelism int
	MaxPerBlock int // chain's max_migrations_per_block; 0 means unthrottled
	DryRun      bool
	// Now returns the timestamp stamped onto applied MigrationInfo records.
	Now func() string
}

// migrationOutcomeCounts is the run summary: per-status tallies plus the list of
// failed item descriptions for triage.
type migrationOutcomeCounts struct {
	Migrated        int
	AlreadyMigrated int
	Skipped         int
	Failed          int
	Planned         int
	Failures        []string
}

// effectiveConcurrency caps worker parallelism by the chain's per-block migration
// limit so a burst never exceeds what a block can absorb. A non-positive
// parallelism floors to 1; a non-positive cap means "no per-block limit".
func effectiveConcurrency(parallelism, maxPerBlock int) int {
	n := parallelism
	if n < 1 {
		n = 1
	}
	if maxPerBlock > 0 && maxPerBlock < n {
		n = maxPerBlock
	}
	return n
}

// applyMigrationResult writes a result's outcome onto the registry record and
// reports whether it changed anything. Dry-run (Planned) results never mutate
// the registry.
func applyMigrationResult(rec *AccountRecord, res migrationResult, now string) bool {
	if res.Planned {
		return false
	}
	info := &MigrationInfo{Status: res.Status, MigratedAt: now}
	switch res.Status {
	case MigrationStatusMigrated:
		info.NewName = res.NewName
		info.NewAddress = res.NewAddress
		info.TxHash = res.TxHash
		info.Height = res.Height
	case MigrationStatusAlreadyMigrated:
		info.NewAddress = res.NewAddress
		info.Height = res.Height
	case MigrationStatusSkipped:
		info.Error = res.Reason
	case MigrationStatusFailed:
		if res.Err != nil {
			info.Error = res.Err.Error()
		}
	}
	rec.Migration = info
	return true
}

// summarizeMigration tallies results by status for the run summary.
func summarizeMigration(results []migrationResult) migrationOutcomeCounts {
	var c migrationOutcomeCounts
	for _, r := range results {
		switch {
		case r.Planned:
			c.Planned++
		case r.Status == MigrationStatusMigrated:
			c.Migrated++
		case r.Status == MigrationStatusAlreadyMigrated:
			c.AlreadyMigrated++
		case r.Status == MigrationStatusSkipped:
			c.Skipped++
		case r.Status == MigrationStatusFailed:
			c.Failed++
			reason := "unknown error"
			if r.Err != nil {
				reason = r.Err.Error()
			}
			c.Failures = append(c.Failures, fmt.Sprintf("%s: %s", r.Item.CorrelationID, reason))
		}
	}
	return c
}

// runMigration executes the migration work queue with a bounded worker pool and
// a single coordinator goroutine that applies results to the registry and saves
// it after each change. Workers never touch the registry directly. The per-block
// migration cap bounds in-flight submissions (each submission waits for
// inclusion before releasing its slot, so a block is never over-filled).
func runMigration(reg *ActivityRegistry, exec *migrationExecutor, lg *migrationLogger, opts migrationRunOpts, save func(*ActivityRegistry) error) (migrationOutcomeCounts, error) {
	queue := buildMigrationQueue(reg)
	total := len(queue)
	if total == 0 {
		lg.logf("coordinator", "no eligible accounts to migrate")
		return migrationOutcomeCounts{}, nil
	}

	concurrency := effectiveConcurrency(opts.Parallelism, opts.MaxPerBlock)
	lg.logf("coordinator", "starting migration: total=%d concurrency=%d max_per_block=%d dry_run=%v",
		total, concurrency, opts.MaxPerBlock, opts.DryRun)

	now := opts.Now
	if now == nil {
		now = func() string { return "" }
	}

	results := make(chan migrationResult, total)
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, item := range queue {
		wg.Add(1)
		go func(it migrationWorkItem) {
			defer wg.Done()
			sem <- struct{}{}        // acquire a throttle slot
			defer func() { <-sem }() // release after inclusion (migrateOne waits)
			lg.logf(it.CorrelationID, "worker acquired slot (in-flight cap %d)", concurrency)
			results <- exec.migrateOne(it, opts.DryRun)
		}(item)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Single coordinator: apply each result to the registry and persist.
	all := make([]migrationResult, 0, total)
	done := 0
	var saveErr error
	for res := range results {
		all = append(all, res)
		done++
		if applyMigrationResult(res.Item.Rec, res, now()) {
			if err := save(reg); err != nil {
				lg.logf("coordinator", "WARN: registry save failed after %s: %v", res.Item.CorrelationID, err)
				if saveErr == nil {
					saveErr = err
				}
			}
		}
		lg.logf("coordinator", "progress %d/%d applied: %s status=%s", done, total, res.Item.CorrelationID, resultStatusLabel(res))
	}

	sum := summarizeMigration(all)
	lg.logf("coordinator", "summary: migrated=%d already=%d skipped=%d failed=%d planned=%d",
		sum.Migrated, sum.AlreadyMigrated, sum.Skipped, sum.Failed, sum.Planned)
	for _, f := range sum.Failures {
		lg.logf("coordinator", "FAILED ITEM: %s", f)
	}
	if saveErr != nil {
		return sum, fmt.Errorf("save migration registry: %w", saveErr)
	}
	return sum, nil
}

func resultStatusLabel(res migrationResult) string {
	if res.Planned {
		return "planned"
	}
	return res.Status
}
