// estimate.go implements the "estimate" mode, which queries and reports
// migration estimates for all legacy accounts without performing any migrations.
package main

import "log"

// classifyEstimateStatus categorizes a migration estimate into one of
// "already_migrated", "ready_to_migrate", or "blocked".
func classifyEstimateStatus(estimate migrationEstimate) (status string, reason string) {
	if estimate.RejectionReason == "already migrated" {
		return "already_migrated", ""
	}
	if estimate.WouldSucceed {
		return "ready_to_migrate", ""
	}
	return "blocked", estimate.RejectionReason
}

// logEstimateReport prints a detailed migration estimate report for a single account.
func logEstimateReport(rec *AccountRecord, estimate migrationEstimate) {
	status, reason := classifyEstimateStatus(estimate)
	totalLinkedRecords := estimate.DelegationCount +
		estimate.UnbondingCount +
		estimate.RedelegationCount +
		estimate.AuthzGrantCount +
		estimate.FeegrantCount +
		estimate.ActionCount +
		estimate.ValDelegationCount
	if reason == "" {
		reason = "n/a"
	}
	log.Printf(
		"  account: %s (%s)\n"+
			"    status: %s\n"+
			"    can_migrate_now: %v\n"+
			"    block_reason: %s\n"+
			"    is_validator_operator: %v\n"+
			"    migration_record_links:\n"+
			"      delegations_to_migrate: %d\n"+
			"      unbondings_to_migrate: %d\n"+
			"      redelegations_to_migrate: %d\n"+
			"      authz_grants_to_migrate: %d\n"+
			"      feegrants_to_migrate: %d\n"+
			"      actions_to_migrate: %d\n"+
			"      validator_delegations_to_migrate: %d\n"+
			"      total_linked_records: %d",
		rec.Name, rec.Address,
		status,
		estimate.WouldSucceed,
		reason,
		estimate.IsValidator,
		estimate.DelegationCount,
		estimate.UnbondingCount,
		estimate.RedelegationCount,
		estimate.AuthzGrantCount,
		estimate.FeegrantCount,
		estimate.ActionCount,
		estimate.ValDelegationCount,
		totalLinkedRecords,
	)
}

// runEstimate queries migration estimates for all legacy accounts and prints a summary.
func runEstimate() {
	ensureEVMMigrationRuntime("estimate mode")

	af := loadAccounts(*flagFile)
	for i := range af.Accounts {
		af.Accounts[i].normalizeActivityTracking()
	}
	log.Printf("=== ESTIMATE MODE: loaded %d accounts from %s ===", len(af.Accounts), *flagFile)

	log.Println("--- Checking migration params ---")
	params, err := queryMigrationParams()
	if err != nil {
		log.Printf("WARN: query evmigration params: %v", err)
	} else {
		log.Printf("  params: enable_migration=%v migration_end_time=%d max_migrations_per_block=%d max_validator_delegations=%d",
			params.EnableMigration, params.MigrationEndTime, params.MaxMigrationsPerBlock, params.MaxValidatorDelegations)
		if !params.EnableMigration {
			log.Printf("  note: migration txs are currently disabled by params (enable_migration=false)")
		}
	}

	log.Println("--- Current migration stats ---")
	printMigrationStats()

	log.Println("--- Migration estimates (legacy accounts) ---")
	var totalLegacy, estimatable, wouldSucceed, alreadyMigrated, rejected, estimateErrors int
	for i := range af.Accounts {
		rec := &af.Accounts[i]
		if !rec.IsLegacy {
			continue
		}
		totalLegacy++

		estimate, err := queryMigrationEstimate(rec.Address)
		if err != nil {
			estimateErrors++
			log.Printf("  WARN: estimate %s (%s): %v", rec.Name, rec.Address, err)
			continue
		}
		estimatable++

		if estimate.RejectionReason == "already migrated" {
			alreadyMigrated++
		} else if estimate.WouldSucceed {
			wouldSucceed++
		} else {
			rejected++
		}

		logEstimateReport(rec, estimate)
	}

	log.Printf(
		"  migration_estimate_summary:\n"+
			"    legacy_accounts: %d\n"+
			"    estimates_fetched: %d\n"+
			"    ready_to_migrate: %d\n"+
			"    already_migrated: %d\n"+
			"    blocked: %d\n"+
			"    estimate_query_errors: %d",
		totalLegacy, estimatable, wouldSucceed, alreadyMigrated, rejected, estimateErrors,
	)

	log.Printf("=== ESTIMATE COMPLETE: legacy=%d estimated=%d ready=%d already_migrated=%d blocked=%d errors=%d ===",
		totalLegacy, estimatable, wouldSucceed, alreadyMigrated, rejected, estimateErrors)
}
