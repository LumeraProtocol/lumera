package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type migrateResult int

const (
	migrateFailed migrateResult = iota
	migrateNew
	migrateAlreadyOnChain
)

func runMigrate() {
	ensureEVMMigrationRuntime("migrate mode")

	af := loadAccounts(*flagFile)
	for i := range af.Accounts {
		af.Accounts[i].normalizeActivityTracking()
	}
	log.Printf("=== MIGRATE MODE: loaded %d accounts from %s ===", len(af.Accounts), *flagFile)

	// Check migration params.
	log.Println("--- Checking migration params ---")
	params, err := queryMigrationParams()
	if err != nil {
		log.Fatalf("query evmigration params: %v", err)
	}
	log.Printf("  params: enable_migration=%v migration_end_time=%d max_migrations_per_block=%d max_validator_delegations=%d",
		params.EnableMigration, params.MigrationEndTime, params.MaxMigrationsPerBlock, params.MaxValidatorDelegations)
	if params.MigrationEndTime > 0 {
		log.Printf("  migration window end: %s", time.Unix(params.MigrationEndTime, 0).UTC().Format(time.RFC3339))
	}
	if !params.EnableMigration {
		log.Fatal("migration preflight failed: enable_migration=false. Submit/execute governance params update first, then rerun migrate mode")
	}
	if params.MigrationEndTime > 0 && time.Now().Unix() > params.MigrationEndTime {
		log.Fatalf("migration preflight failed: migration window closed at %s",
			time.Unix(params.MigrationEndTime, 0).UTC().Format(time.RFC3339))
	}

	// Query initial migration stats.
	log.Println("--- Initial migration stats ---")
	initialStats, haveInitialStats := queryAndLogMigrationStats()
	printMigrationStats()

	// Collect legacy accounts that need migration.
	var legacyIdx []int
	for i, rec := range af.Accounts {
		if rec.IsLegacy && !rec.Migrated {
			legacyIdx = append(legacyIdx, i)
		}
	}
	log.Printf("  %d legacy accounts to migrate", len(legacyIdx))

	if len(legacyIdx) == 0 {
		log.Println("nothing to migrate")
		return
	}

	// Query migration-estimate for a sample of accounts before starting.
	log.Println("--- Pre-migration estimates (sample) ---")
	sampleSize := 5
	if sampleSize > len(legacyIdx) {
		sampleSize = len(legacyIdx)
	}
	for _, idx := range legacyIdx[:sampleSize] {
		rec := &af.Accounts[idx]
		verifyMigrationEstimate(rec, false)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Shuffle the order for randomness.
	rng.Shuffle(len(legacyIdx), func(i, j int) {
		legacyIdx[i], legacyIdx[j] = legacyIdx[j], legacyIdx[i]
	})

	// Process in random batches of 1..5.
	migrated := 0
	alreadyMigrated := 0
	failed := 0
	pos := 0
	batchNum := 0
	for pos < len(legacyIdx) {
		batchSize := 1 + rng.Intn(5)
		if pos+batchSize > len(legacyIdx) {
			batchSize = len(legacyIdx) - pos
		}
		batchNum++

		log.Printf("--- Batch %d: migrating %d accounts ---", batchNum, batchSize)

		for _, idx := range legacyIdx[pos : pos+batchSize] {
			rec := &af.Accounts[idx]
			switch migrateOne(rec) {
			case migrateNew:
				migrated++
			case migrateAlreadyOnChain:
				alreadyMigrated++
			default:
				failed++
			}
		}

		pos += batchSize

		// Save progress after each batch.
		for i := range af.Accounts {
			af.Accounts[i].normalizeActivityTracking()
		}
		saveAccounts(*flagFile, af)
		log.Printf("  batch %d complete, progress saved (%d newly migrated, %d already on-chain, %d failed, %d total)",
			batchNum, migrated, alreadyMigrated, failed, len(legacyIdx))

		// Print stats after each batch.
		printMigrationStats()
	}

	// Final verification: query estimate for a migrated account (should reject as already migrated).
	log.Println("--- Post-migration estimate verification ---")
	for _, rec := range af.Accounts {
		if rec.Migrated {
			verifyMigrationEstimate(&rec, true)
			break
		}
	}

	// Final stats.
	log.Println("--- Final migration stats ---")
	finalStats, haveFinalStats := queryAndLogMigrationStats()
	printMigrationStats()

	if haveInitialStats && haveFinalStats {
		delta := finalStats.TotalMigrated - initialStats.TotalMigrated
		if delta < migrated {
			log.Fatalf("post-check failed: migration-stats delta=%d is lower than newly migrated accounts=%d", delta, migrated)
		}
		log.Printf("  post-check: migration-stats total_migrated delta=%d (newly migrated=%d, already on-chain=%d)",
			delta, migrated, alreadyMigrated)
	}

	log.Printf("=== MIGRATE COMPLETE: %d newly migrated, %d already on-chain, %d failed, %d total ===",
		migrated, alreadyMigrated, failed, len(legacyIdx))
	if failed > 0 {
		log.Fatalf("migration completed with %d failures", failed)
	}
}

// migrateOne migrates a single legacy account and reports whether it was
// migrated in this run, already migrated on-chain, or failed.
func migrateOne(rec *AccountRecord) migrateResult {
	rec.normalizeActivityTracking()

	// Check if already migrated on-chain (handles rerun after partial progress).
	if already, recNewAddr := queryMigrationRecord(rec.Address); already {
		log.Printf("  SKIP (already on-chain): %s -> %s", rec.Name, recNewAddr)
		rec.Migrated = true
		rec.NewAddress = recNewAddr
		if err := validateLegacyPostMigration(rec); err != nil {
			log.Printf("  FAIL: post-migration checks for already-migrated %s: %v", rec.Name, err)
			return migrateFailed
		}
		return migrateAlreadyOnChain
	}

	// Query migration estimate before migrating.
	verifyMigrationEstimate(rec, false)

	// Create destination key from the same mnemonic (coin-type 60 + eth_secp256k1).
	newRec, err := createDestinationAccountFromLegacy(rec)
	if err != nil {
		log.Printf("  WARN: create destination key for %s: %v", rec.Name, err)
		return migrateFailed
	}
	rec.NewName = newRec.Name
	rec.NewAddress = newRec.Address

	// Sign the migration message using the legacy private key.
	var sigB64, pubB64 string
	if rec.Mnemonic != "" {
		sigB64, err = signMigrationMessage("claim", rec.Mnemonic, rec.Address, newRec.Address)
		if err != nil {
			log.Printf("  FAIL: sign for %s: %v", rec.Name, err)
			return migrateFailed
		}
		pubB64 = rec.PubKeyB64
	} else {
		// No mnemonic (key reused from keyring); export private key hex.
		privHex, expErr := exportPrivateKeyHex(rec.Name)
		if expErr != nil {
			log.Printf("  FAIL: export key for %s: %v", rec.Name, expErr)
			return migrateFailed
		}
		sigB64, pubB64, err = signMigrationMessageWithPrivHex("claim", privHex, rec.Address, newRec.Address)
		if err != nil {
			log.Printf("  FAIL: sign for %s: %v", rec.Name, err)
			return migrateFailed
		}
	}

	// Submit the migration transaction.
	// AutoCLI positional args: [new-address] [legacy-address] [legacy-pub-key] [legacy-signature]
	_, err = runTx(
		"tx", "evmigration", "claim-legacy-account",
		newRec.Address, rec.Address, pubB64, sigB64,
		"--from", newRec.Name)
	if err != nil {
		log.Printf("  FAIL: claim-legacy-account %s -> %s: %v", rec.Name, newRec.Address, err)
		return migrateFailed
	}

	// Verify migration-record exists and points to the expected new address.
	hasRecord, recNewAddr := queryMigrationRecord(rec.Address)
	if !hasRecord {
		log.Printf("  FAIL: migration-record missing after tx for %s", rec.Name)
		return migrateFailed
	}
	if recNewAddr != newRec.Address {
		log.Printf("  FAIL: migration-record mismatch for %s: expected=%s got=%s", rec.Name, newRec.Address, recNewAddr)
		return migrateFailed
	}

	rec.Migrated = true
	if err := validateLegacyPostMigration(rec); err != nil {
		log.Printf("  FAIL: post-migration checks for %s: %v", rec.Name, err)
		return migrateFailed
	}

	log.Printf("  OK: %s (%s) -> %s (%s)", rec.Name, rec.Address, newRec.Name, newRec.Address)
	return migrateNew
}

func createDestinationAccountFromLegacy(rec *AccountRecord) (AccountRecord, error) {
	if strings.TrimSpace(rec.Mnemonic) == "" {
		return AccountRecord{}, fmt.Errorf("legacy account %s has no mnemonic; cannot derive coin-type 60 destination from the same mnemonic", rec.Name)
	}
	expectedAddr, err := deriveAddressFromMnemonic(rec.Mnemonic, false)
	if err != nil {
		return AccountRecord{}, fmt.Errorf("derive destination address for %s: %w", rec.Name, err)
	}

	if rec.NewName != "" {
		addr, err := getAddress(rec.NewName)
		if err == nil && addr == expectedAddr {
			return AccountRecord{
				Name:     rec.NewName,
				Mnemonic: rec.Mnemonic,
				Address:  addr,
				IsLegacy: false,
			}, nil
		}
	}
	if legacyName := "new_" + rec.Name; legacyName != rec.NewName {
		addr, err := getAddress(legacyName)
		if err == nil && addr == expectedAddr {
			return AccountRecord{
				Name:     legacyName,
				Mnemonic: rec.Mnemonic,
				Address:  addr,
				IsLegacy: false,
			}, nil
		}
	}

	baseName := migratedAccountBaseName(rec.Name, rec.IsLegacy)
	for i := 0; i < 50; i++ {
		name := baseName
		if i > 0 {
			name = fmt.Sprintf("%s-%02d", baseName, i)
		}

		addr, err := getAddress(name)
		if err == nil {
			if addr == expectedAddr {
				return AccountRecord{
					Name:     name,
					Mnemonic: rec.Mnemonic,
					Address:  addr,
					IsLegacy: false,
				}, nil
			}
			continue
		}

		if err := importKey(name, rec.Mnemonic, false); err != nil {
			low := strings.ToLower(err.Error())
			if strings.Contains(low, "already exists") || strings.Contains(low, "key exists") {
				continue
			}
			return AccountRecord{}, err
		}

		addr, err = getAddress(name)
		if err != nil {
			return AccountRecord{}, fmt.Errorf("resolve imported key %s address: %w", name, err)
		}
		if addr != expectedAddr {
			return AccountRecord{}, fmt.Errorf("imported key %s address mismatch: expected %s got %s", name, expectedAddr, addr)
		}

		return AccountRecord{
			Name:     name,
			Mnemonic: rec.Mnemonic,
			Address:  addr,
			IsLegacy: false,
		}, nil
	}

	return AccountRecord{}, fmt.Errorf("unable to create unique destination key for %s", rec.Name)
}

func migratedAccountBaseName(name string, isLegacy bool) string {
	switch {
	case strings.HasPrefix(name, legacyPreparedAccountPrefix+"-"):
		return migratedAccountPrefix + strings.TrimPrefix(name, legacyPreparedAccountPrefix)
	case strings.HasPrefix(name, extraPreparedAccountPrefix+"-"):
		return migratedExtraAccountPrefix + strings.TrimPrefix(name, extraPreparedAccountPrefix)
	case strings.HasPrefix(name, legacyPreparedAccountPrefixV0+"_"):
		return migratedAccountPrefix + "-" + strings.ReplaceAll(strings.TrimPrefix(name, legacyPreparedAccountPrefixV0+"_"), "_", "-")
	case strings.HasPrefix(name, extraPreparedAccountPrefixV0+"_"):
		return migratedExtraAccountPrefix + "-" + strings.ReplaceAll(strings.TrimPrefix(name, extraPreparedAccountPrefixV0+"_"), "_", "-")
	case strings.HasPrefix(name, "legacy_"):
		return migratedAccountPrefix + "-" + strings.ReplaceAll(strings.TrimPrefix(name, "legacy_"), "_", "-")
	case strings.HasPrefix(name, "extra_"):
		return migratedExtraAccountPrefix + "-" + strings.ReplaceAll(strings.TrimPrefix(name, "extra_"), "_", "-")
	default:
		prefix := migratedExtraAccountPrefix
		if isLegacy {
			prefix = migratedAccountPrefix
		}
		return prefix + "-" + strings.ReplaceAll(strings.Trim(name, "-_ "), "_", "-")
	}
}

// verifyMigrationEstimate queries and logs the migration estimate for an account.
// If expectMigrated is true, it checks for the "already migrated" rejection reason.
func verifyMigrationEstimate(rec *AccountRecord, expectMigrated bool) {
	estimate, err := queryMigrationEstimate(rec.Address)
	if err != nil {
		log.Printf("  WARN: migration-estimate %s: %v", rec.Name, err)
		return
	}

	logEstimateReport(rec, estimate)

	isAlreadyMigrated := estimate.RejectionReason == "already migrated"
	if expectMigrated && !isAlreadyMigrated {
		log.Printf("  ERROR: expected rejection_reason='already migrated' for %s", rec.Name)
	}
	if !expectMigrated && isAlreadyMigrated {
		log.Printf("  INFO: %s is already migrated on-chain; local accounts file may be stale", rec.Name)
	}
}

// printMigrationStats queries and logs the current migration stats.
func printMigrationStats() {
	stats, err := queryMigrationStats()
	if err != nil {
		log.Printf("  WARN: migration-stats: %v", err)
		return
	}

	log.Printf("  stats: migrated=%d legacy=%d legacy_staked=%d validators_migrated=%d validators_legacy=%d",
		stats.TotalMigrated, stats.TotalLegacy, stats.TotalLegacyStaked,
		stats.TotalValidatorsMigrated, stats.TotalValidatorsLegacy)
}

func queryAndLogMigrationStats() (migrationStats, bool) {
	stats, err := queryMigrationStats()
	if err != nil {
		log.Printf("  WARN: migration-stats: %v", err)
		return migrationStats{}, false
	}
	log.Printf("  stats: migrated=%d legacy=%d legacy_staked=%d validators_migrated=%d validators_legacy=%d",
		stats.TotalMigrated, stats.TotalLegacy, stats.TotalLegacyStaked,
		stats.TotalValidatorsMigrated, stats.TotalValidatorsLegacy)
	return stats, true
}

func validateLegacyPostMigration(rec *AccountRecord) error {
	rec.normalizeActivityTracking()

	var issues []string
	if rec.NewAddress == "" {
		issues = append(issues, "missing new address for post-migration checks")
	}

	estimate, err := queryMigrationEstimate(rec.Address)
	if err != nil {
		issues = append(issues, fmt.Sprintf("query migration-estimate failed: %v", err))
	} else if estimate.RejectionReason != "already migrated" {
		issues = append(issues, fmt.Sprintf("expected rejection_reason='already migrated', got %q", estimate.RejectionReason))
	}

	if len(rec.Delegations) > 0 {
		seen := make(map[string]struct{}, len(rec.Delegations))
		for _, d := range rec.Delegations {
			if d.Validator == "" {
				continue
			}
			if _, ok := seen[d.Validator]; ok {
				continue
			}
			seen[d.Validator] = struct{}{}
			currentValidator := resolvePostMigrationValidator(d.Validator)
			newN, err := queryDelegationToValidatorCount(rec.NewAddress, currentValidator)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query new delegation %s failed: %v", currentValidator, err))
			} else if newN == 0 {
				issues = append(issues, fmt.Sprintf("expected delegation on new address to %s, got 0", currentValidator))
			}
			oldN, err := queryDelegationToValidatorCount(rec.Address, d.Validator)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query legacy delegation %s failed: %v", d.Validator, err))
			} else if oldN != 0 {
				issues = append(issues, fmt.Sprintf("expected 0 legacy delegations to %s, got %d", d.Validator, oldN))
			}
		}
	} else if rec.HasDelegation {
		newN, err := queryDelegationCount(rec.NewAddress)
		if err != nil {
			issues = append(issues, fmt.Sprintf("query new delegations failed: %v", err))
		} else if newN == 0 {
			issues = append(issues, "expected delegations on new address, got 0")
		}
		oldN, err := queryDelegationCount(rec.Address)
		if err != nil {
			issues = append(issues, fmt.Sprintf("query legacy delegations failed: %v", err))
		} else if oldN != 0 {
			issues = append(issues, fmt.Sprintf("expected 0 legacy delegations after migration, got %d", oldN))
		}
	}

	if len(rec.Unbondings) > 0 {
		seen := make(map[string]struct{}, len(rec.Unbondings))
		for _, u := range rec.Unbondings {
			if u.Validator == "" {
				continue
			}
			if _, ok := seen[u.Validator]; ok {
				continue
			}
			seen[u.Validator] = struct{}{}
			currentValidator := resolvePostMigrationValidator(u.Validator)
			newN, err := queryUnbondingFromValidatorCount(rec.NewAddress, currentValidator)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query new unbonding %s failed: %v", currentValidator, err))
			} else if newN == 0 {
				issues = append(issues, fmt.Sprintf("expected unbonding on new address from %s, got 0", currentValidator))
			}
			oldN, err := queryUnbondingFromValidatorCount(rec.Address, u.Validator)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query legacy unbonding %s failed: %v", u.Validator, err))
			} else if oldN != 0 {
				issues = append(issues, fmt.Sprintf("expected 0 legacy unbondings from %s, got %d", u.Validator, oldN))
			}
		}
	} else if rec.HasUnbonding {
		newN, err := queryUnbondingCount(rec.NewAddress)
		if err != nil {
			issues = append(issues, fmt.Sprintf("query new unbondings failed: %v", err))
		} else if newN == 0 {
			issues = append(issues, "expected unbonding entries on new address, got 0")
		}
		oldN, err := queryUnbondingCount(rec.Address)
		if err != nil {
			issues = append(issues, fmt.Sprintf("query legacy unbondings failed: %v", err))
		} else if oldN != 0 {
			issues = append(issues, fmt.Sprintf("expected 0 legacy unbondings after migration, got %d", oldN))
		}
	}

	if len(rec.Redelegations) > 0 {
		seen := make(map[string]struct{}, len(rec.Redelegations))
		for _, rd := range rec.Redelegations {
			if rd.SrcValidator == "" || rd.DstValidator == "" {
				continue
			}
			key := rd.SrcValidator + "->" + rd.DstValidator
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			currentSrc := resolvePostMigrationValidator(rd.SrcValidator)
			currentDst := resolvePostMigrationValidator(rd.DstValidator)
			currentKey := currentSrc + "->" + currentDst
			newN, err := queryRedelegationCount(rec.NewAddress, currentSrc, currentDst)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query new redelegation %s failed: %v", currentKey, err))
			} else if newN == 0 {
				issues = append(issues, fmt.Sprintf("expected redelegation on new address for %s, got 0", currentKey))
			}
			oldN, err := queryRedelegationCount(rec.Address, rd.SrcValidator, rd.DstValidator)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query legacy redelegation %s failed: %v", key, err))
			} else if oldN != 0 {
				issues = append(issues, fmt.Sprintf("expected 0 legacy redelegations for %s, got %d", key, oldN))
			}
		}
	} else if rec.HasRedelegation {
		newN, err := queryRedelegationCount(rec.NewAddress, rec.DelegatedTo, rec.RedelegatedTo)
		if err != nil {
			issues = append(issues, fmt.Sprintf("query new redelegations failed: %v", err))
		} else if newN == 0 {
			issues = append(issues, "expected redelegations on new address, got 0")
		}
		oldN, err := queryRedelegationCount(rec.Address, rec.DelegatedTo, rec.RedelegatedTo)
		if err != nil {
			issues = append(issues, fmt.Sprintf("query legacy redelegations failed: %v", err))
		} else if oldN != 0 {
			issues = append(issues, fmt.Sprintf("expected 0 legacy redelegations after migration, got %d", oldN))
		}
	}

	if len(rec.WithdrawAddresses) > 0 || rec.HasThirdPartyWD {
		expected := rec.WithdrawAddress
		if n := len(rec.WithdrawAddresses); n > 0 {
			expected = rec.WithdrawAddresses[n-1].Address
		}
		addr, err := queryWithdrawAddress(rec.NewAddress)
		if err != nil {
			issues = append(issues, fmt.Sprintf("query new withdraw-addr failed: %v", err))
		} else if expected != "" && addr != expected {
			issues = append(issues, fmt.Sprintf("withdraw-addr mismatch: expected %s got %s", expected, addr))
		}
	}

	if len(rec.AuthzGrants) > 0 {
		seen := make(map[string]struct{}, len(rec.AuthzGrants))
		for _, g := range rec.AuthzGrants {
			if g.Grantee == "" {
				continue
			}
			if _, ok := seen[g.Grantee]; ok {
				continue
			}
			seen[g.Grantee] = struct{}{}
			currentGrantee := resolvePostMigrationAddress(g.Grantee)
			ok, err := queryAuthzGrantExists(rec.NewAddress, currentGrantee)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query new authz grant -> %s failed: %v", currentGrantee, err))
			} else if !ok {
				issues = append(issues, fmt.Sprintf("expected authz grant on new address -> %s", currentGrantee))
			}
			legacyOK, err := queryAuthzGrantExists(rec.Address, g.Grantee)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query legacy authz grant -> %s failed: %v", g.Grantee, err))
			} else if legacyOK {
				issues = append(issues, fmt.Sprintf("legacy authz grant still present -> %s", g.Grantee))
			}
		}
	} else if rec.HasAuthzGrant && rec.AuthzGrantedTo != "" {
		currentGrantee := resolvePostMigrationAddress(rec.AuthzGrantedTo)
		ok, err := queryAuthzGrantExists(rec.NewAddress, currentGrantee)
		if err != nil {
			issues = append(issues, fmt.Sprintf("query new authz grant failed: %v", err))
		} else if !ok {
			issues = append(issues, "expected authz grant on new address")
		}
		legacyOK, err := queryAuthzGrantExists(rec.Address, rec.AuthzGrantedTo)
		if err != nil {
			issues = append(issues, fmt.Sprintf("query legacy authz grant failed: %v", err))
		} else if legacyOK {
			issues = append(issues, "legacy authz grant still present")
		}
	}

	if len(rec.AuthzAsGrantee) > 0 {
		seen := make(map[string]struct{}, len(rec.AuthzAsGrantee))
		for _, g := range rec.AuthzAsGrantee {
			if g.Granter == "" {
				continue
			}
			if _, ok := seen[g.Granter]; ok {
				continue
			}
			seen[g.Granter] = struct{}{}
			currentGranter := resolvePostMigrationAddress(g.Granter)
			ok, err := queryAuthzGrantExists(currentGranter, rec.NewAddress)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query authz grant %s -> new failed: %v", currentGranter, err))
			} else if !ok {
				issues = append(issues, fmt.Sprintf("expected authz grant %s -> new address", currentGranter))
			}
			legacyOK, err := queryAuthzGrantExists(g.Granter, rec.Address)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query authz grant %s -> legacy failed: %v", g.Granter, err))
			} else if legacyOK {
				issues = append(issues, fmt.Sprintf("authz grant %s still targets legacy address", g.Granter))
			}
		}
	} else if rec.HasAuthzAsGrantee && rec.AuthzReceivedFrom != "" {
		currentGranter := resolvePostMigrationAddress(rec.AuthzReceivedFrom)
		ok, err := queryAuthzGrantExists(currentGranter, rec.NewAddress)
		if err != nil {
			issues = append(issues, fmt.Sprintf("query authz grant to new address failed: %v", err))
		} else if !ok {
			issues = append(issues, "expected authz grant targeting new address")
		}
		legacyOK, err := queryAuthzGrantExists(rec.AuthzReceivedFrom, rec.Address)
		if err != nil {
			issues = append(issues, fmt.Sprintf("query authz grant to legacy address failed: %v", err))
		} else if legacyOK {
			issues = append(issues, "authz grant still targets legacy address")
		}
	}

	if len(rec.Feegrants) > 0 {
		seen := make(map[string]struct{}, len(rec.Feegrants))
		for _, g := range rec.Feegrants {
			if g.Grantee == "" {
				continue
			}
			if _, ok := seen[g.Grantee]; ok {
				continue
			}
			seen[g.Grantee] = struct{}{}
			currentGrantee := resolvePostMigrationAddress(g.Grantee)
			ok, err := queryFeegrantAllowanceExists(rec.NewAddress, currentGrantee)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query new feegrant -> %s failed: %v", currentGrantee, err))
			} else if !ok {
				issues = append(issues, fmt.Sprintf("expected feegrant on new address -> %s", currentGrantee))
			}
			legacyOK, err := queryFeegrantAllowanceExists(rec.Address, g.Grantee)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query legacy feegrant -> %s failed: %v", g.Grantee, err))
			} else if legacyOK {
				issues = append(issues, fmt.Sprintf("legacy feegrant still present -> %s", g.Grantee))
			}
		}
	} else if rec.HasFeegrant && rec.FeegrantGrantedTo != "" {
		currentGrantee := resolvePostMigrationAddress(rec.FeegrantGrantedTo)
		ok, err := queryFeegrantAllowanceExists(rec.NewAddress, currentGrantee)
		if err != nil {
			issues = append(issues, fmt.Sprintf("query new feegrant failed: %v", err))
		} else if !ok {
			issues = append(issues, "expected feegrant on new address")
		}
		legacyOK, err := queryFeegrantAllowanceExists(rec.Address, rec.FeegrantGrantedTo)
		if err != nil {
			issues = append(issues, fmt.Sprintf("query legacy feegrant failed: %v", err))
		} else if legacyOK {
			issues = append(issues, "legacy feegrant still present")
		}
	}

	if len(rec.FeegrantsReceived) > 0 {
		seen := make(map[string]struct{}, len(rec.FeegrantsReceived))
		for _, g := range rec.FeegrantsReceived {
			if g.Granter == "" {
				continue
			}
			if _, ok := seen[g.Granter]; ok {
				continue
			}
			seen[g.Granter] = struct{}{}
			currentGranter := resolvePostMigrationAddress(g.Granter)
			ok, err := queryFeegrantAllowanceExists(currentGranter, rec.NewAddress)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query feegrant %s -> new failed: %v", currentGranter, err))
			} else if !ok {
				issues = append(issues, fmt.Sprintf("expected feegrant %s -> new address", currentGranter))
			}
			legacyOK, err := queryFeegrantAllowanceExists(g.Granter, rec.Address)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query feegrant %s -> legacy failed: %v", g.Granter, err))
			} else if legacyOK {
				issues = append(issues, fmt.Sprintf("feegrant %s still targets legacy address", g.Granter))
			}
		}
	} else if rec.HasFeegrantGrantee && rec.FeegrantFrom != "" {
		currentGranter := resolvePostMigrationAddress(rec.FeegrantFrom)
		ok, err := queryFeegrantAllowanceExists(currentGranter, rec.NewAddress)
		if err != nil {
			issues = append(issues, fmt.Sprintf("query feegrant to new address failed: %v", err))
		} else if !ok {
			issues = append(issues, "expected feegrant targeting new address")
		}
		legacyOK, err := queryFeegrantAllowanceExists(rec.FeegrantFrom, rec.Address)
		if err != nil {
			issues = append(issues, fmt.Sprintf("query feegrant to legacy address failed: %v", err))
		} else if legacyOK {
			issues = append(issues, "feegrant still targets legacy address")
		}
	}

	// Validate actions: creator field should now point to new address.
	// For SDK-created actions, also validate state, price, metadata, superNodes.
	if len(rec.Actions) > 0 {
		for _, act := range rec.Actions {
			if act.ActionID == "" {
				continue
			}
			full, err := queryFullAction(act.ActionID)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query action %s failed: %v", act.ActionID, err))
				continue
			}
			// Creator should be migrated to new address.
			if full.Creator != rec.NewAddress {
				issues = append(issues, fmt.Sprintf("action %s creator mismatch: expected %s got %s", act.ActionID, rec.NewAddress, full.Creator))
			}
			// State should survive migration. Allow legitimate forward progression
			// from background supernode processing between prepare and migrate.
			if act.State != "" && !isCompatibleActionState(act.State, full.State) {
				issues = append(issues, fmt.Sprintf("action %s state mismatch: expected %s got %s", act.ActionID, act.State, full.State))
			}
			// Price should be preserved.
			if act.Price != "" && full.Price != act.Price {
				issues = append(issues, fmt.Sprintf("action %s price mismatch: expected %s got %s", act.ActionID, act.Price, full.Price))
			}
			// ActionType should be preserved.
			if act.ActionType != "" && full.ActionType != act.ActionType && full.ActionType != "ACTION_TYPE_"+act.ActionType {
				issues = append(issues, fmt.Sprintf("action %s type mismatch: expected %s got %s", act.ActionID, act.ActionType, full.ActionType))
			}
			// SuperNodes may be a mix of legacy and EVM addresses while migrations are
			// still in progress. For each recorded supernode, expect its current
			// post-migration address: migrated peers should appear under the new EVM
			// address, and unmigrated peers may still appear under the legacy address.
			if len(act.SuperNodes) > 0 {
				if len(full.SuperNodes) == 0 {
					issues = append(issues, fmt.Sprintf("action %s lost superNodes after migration", act.ActionID))
				}
				for _, recorded := range act.SuperNodes {
					expected := resolvePostMigrationAddress(recorded)
					if !containsString(full.SuperNodes, expected) {
						issues = append(issues, fmt.Sprintf("action %s missing migrated supernode %s", act.ActionID, expected))
					}
					if expected != recorded && containsString(full.SuperNodes, recorded) {
						issues = append(issues, fmt.Sprintf("action %s still contains legacy supernode %s", act.ActionID, recorded))
					}
				}
			}
			// BlockHeight should be preserved.
			if act.BlockHeight > 0 && full.BlockHeight != "" && full.BlockHeight != "0" {
				if fmt.Sprintf("%d", act.BlockHeight) != full.BlockHeight {
					issues = append(issues, fmt.Sprintf("action %s blockHeight mismatch: expected %d got %s", act.ActionID, act.BlockHeight, full.BlockHeight))
				}
			}
		}
		// Verify legacy address no longer owns any actions.
		if legacyIDs, err := queryActionsByCreator(rec.Address); err != nil {
			issues = append(issues, fmt.Sprintf("query legacy actions failed: %v", err))
		} else if len(legacyIDs) > 0 {
			issues = append(issues, fmt.Sprintf("expected 0 legacy actions after migration, got %d", len(legacyIDs)))
		}
	} else if rec.HasAction {
		// HasAction flag set but no detailed records — just check by creator.
		if newIDs, err := queryActionsByCreator(rec.NewAddress); err != nil {
			issues = append(issues, fmt.Sprintf("query new actions failed: %v", err))
		} else if len(newIDs) == 0 {
			issues = append(issues, "expected actions on new address, got 0")
		}
		if legacyIDs, err := queryActionsByCreator(rec.Address); err != nil {
			issues = append(issues, fmt.Sprintf("query legacy actions failed: %v", err))
		} else if len(legacyIDs) > 0 {
			issues = append(issues, fmt.Sprintf("expected 0 legacy actions after migration, got %d", len(legacyIDs)))
		}
	}

	if len(issues) == 0 {
		return nil
	}
	return errors.New(strings.Join(issues, "; "))
}

func resolvePostMigrationAddress(addr string) string {
	if ok, newAddr := queryMigrationRecord(addr); ok && newAddr != "" {
		return newAddr
	}
	return addr
}

func resolvePostMigrationValidator(valoper string) string {
	valAddr, err := sdk.ValAddressFromBech32(valoper)
	if err != nil {
		return valoper
	}
	legacyAcc := sdk.AccAddress(valAddr).String()
	if ok, newAddr := queryMigrationRecord(legacyAcc); ok && newAddr != "" {
		if newValoper, err := valoperFromAccAddress(newAddr); err == nil && newValoper != "" {
			return newValoper
		}
	}
	return valoper
}

func isCompatibleActionState(expected, actual string) bool {
	if expected == "" || actual == "" || expected == actual {
		return true
	}

	stateRank := func(state string) int {
		switch state {
		case "ACTION_STATE_PENDING":
			return 1
		case "ACTION_STATE_DONE":
			return 2
		case "ACTION_STATE_APPROVED":
			return 3
		default:
			return 0
		}
	}

	expectedRank := stateRank(expected)
	actualRank := stateRank(actual)
	if expectedRank == 0 || actualRank == 0 {
		return false
	}
	return actualRank >= expectedRank
}
