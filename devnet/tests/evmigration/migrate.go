package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"
)

func runMigrate() {
	ensureEVMMigrationRuntime("migrate mode")

	if *flagFunder == "" {
		if name, err := detectFunder(); err == nil {
			*flagFunder = name
			log.Printf("auto-detected funder from keyring: %s", name)
		}
	}

	af := loadAccounts(*flagFile)
	for i := range af.Accounts {
		af.Accounts[i].normalizeActivityTracking()
	}
	log.Printf("=== MIGRATE MODE: loaded %d accounts from %s ===", len(af.Accounts), *flagFile)

	// Check migration params.
	log.Println("--- Checking migration params ---")
	out, err := run("query", "evmigration", "params")
	if err != nil {
		log.Printf("WARN: query evmigration params: %v\n%s", err, out)
	} else {
		log.Printf("  evmigration params: %s", out)
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
			ok := migrateOne(rec)
			if ok {
				migrated++
			} else {
				failed++
			}
		}

		pos += batchSize

		// Save progress after each batch.
		for i := range af.Accounts {
			af.Accounts[i].normalizeActivityTracking()
		}
		saveAccounts(*flagFile, af)
		log.Printf("  batch %d complete, progress saved (%d/%d migrated, %d failed)",
			batchNum, migrated, len(legacyIdx), failed)

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
			log.Fatalf("post-check failed: migration-stats delta=%d is lower than successful migrations=%d", delta, migrated)
		}
		log.Printf("  post-check: migration-stats total_migrated delta=%d (local successful=%d)", delta, migrated)
	}

	log.Printf("=== MIGRATE COMPLETE: %d/%d migrated, %d failed ===",
		migrated, len(legacyIdx), failed)
	if failed > 0 {
		log.Fatalf("migration completed with %d failures", failed)
	}
}

// migrateOne migrates a single legacy account. Returns true on success.
func migrateOne(rec *AccountRecord) bool {
	rec.normalizeActivityTracking()

	// Check if already migrated on-chain (handles rerun after partial progress).
	if already, recNewAddr := queryMigrationRecord(rec.Address); already {
		log.Printf("  SKIP (already on-chain): %s -> %s", rec.Name, recNewAddr)
		rec.Migrated = true
		rec.NewAddress = recNewAddr
		if err := validateLegacyPostMigration(rec); err != nil {
			log.Printf("  FAIL: post-migration checks for already-migrated %s: %v", rec.Name, err)
			return false
		}
		return true
	}

	// Query migration estimate before migrating.
	verifyMigrationEstimate(rec, false)

	// Create destination key from the same mnemonic (coin-type 60 + eth_secp256k1).
	newRec, err := createDestinationAccountFromLegacy(rec)
	if err != nil {
		log.Printf("  WARN: create destination key for %s: %v", rec.Name, err)
		return false
	}
	rec.NewName = newRec.Name
	rec.NewAddress = newRec.Address

	// Fund the new address minimally so it can pay tx fees.
	if *flagFunder != "" {
		_, _ = runTx("tx", "bank", "send", *flagFunder, newRec.Address, "100000ulume", "--from", *flagFunder)
	}

	// Sign the migration message using the legacy private key.
	var sigB64, pubB64 string
	if rec.Mnemonic != "" {
		sigB64, err = signMigrationMessage(rec.Mnemonic, rec.Address, newRec.Address)
		if err != nil {
			log.Printf("  FAIL: sign for %s: %v", rec.Name, err)
			return false
		}
		pubB64 = rec.PubKeyB64
	} else {
		// No mnemonic (key reused from keyring); export private key hex.
		privHex, expErr := exportPrivateKeyHex(rec.Name)
		if expErr != nil {
			log.Printf("  FAIL: export key for %s: %v", rec.Name, expErr)
			return false
		}
		sigB64, pubB64, err = signMigrationMessageWithPrivHex(privHex, rec.Address, newRec.Address)
		if err != nil {
			log.Printf("  FAIL: sign for %s: %v", rec.Name, err)
			return false
		}
	}

	// Submit the migration transaction.
	// AutoCLI positional args: [new-address] [legacy-address] [legacy-pub-key] [legacy-signature]
	_, err = runTx("tx", "evmigration", "claim-legacy-account",
		newRec.Address, rec.Address, pubB64, sigB64,
		"--from", newRec.Name)
	if err != nil {
		log.Printf("  FAIL: claim-legacy-account %s -> %s: %v", rec.Name, newRec.Address, err)
		return false
	}

	// Verify migration-record exists and points to the expected new address.
	hasRecord, recNewAddr := queryMigrationRecord(rec.Address)
	if !hasRecord {
		log.Printf("  FAIL: migration-record missing after tx for %s", rec.Name)
		return false
	}
	if recNewAddr != newRec.Address {
		log.Printf("  FAIL: migration-record mismatch for %s: expected=%s got=%s", rec.Name, newRec.Address, recNewAddr)
		return false
	}

	rec.Migrated = true
	if err := validateLegacyPostMigration(rec); err != nil {
		log.Printf("  FAIL: post-migration checks for %s: %v", rec.Name, err)
		return false
	}

	log.Printf("  OK: %s (%s) -> %s (%s)", rec.Name, rec.Address, newRec.Name, newRec.Address)
	return true
}

func createDestinationAccountFromLegacy(rec *AccountRecord) (AccountRecord, error) {
	if strings.TrimSpace(rec.Mnemonic) == "" {
		return AccountRecord{}, fmt.Errorf("legacy account %s has no mnemonic; cannot derive coin-type 60 destination from the same mnemonic", rec.Name)
	}
	baseName := fmt.Sprintf("new_%s", rec.Name)
	expectedAddr, err := deriveAddressFromMnemonic(rec.Mnemonic, false)
	if err != nil {
		return AccountRecord{}, fmt.Errorf("derive destination address for %s: %w", rec.Name, err)
	}

	for i := 0; i < 50; i++ {
		name := baseName
		if i > 0 {
			name = fmt.Sprintf("%s_%02d", baseName, i)
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

// verifyMigrationEstimate queries and logs the migration estimate for an account.
// If expectMigrated is true, it checks for the "already migrated" rejection reason.
func verifyMigrationEstimate(rec *AccountRecord, expectMigrated bool) {
	estimate, err := queryMigrationEstimate(rec.Address)
	if err != nil {
		log.Printf("  WARN: migration-estimate %s: %v", rec.Name, err)
		return
	}

	log.Printf("  estimate %s: would_succeed=%v reason=%q dels=%d unbonding=%d redels=%d authz=%d feegrant=%d val_dels=%d is_validator=%v",
		rec.Name, estimate.WouldSucceed, estimate.RejectionReason,
		estimate.DelegationCount, estimate.UnbondingCount, estimate.RedelegationCount,
		estimate.AuthzGrantCount, estimate.FeegrantCount, estimate.ValDelegationCount, estimate.IsValidator)

	isAlreadyMigrated := estimate.RejectionReason == "already migrated"
	if expectMigrated && !isAlreadyMigrated {
		log.Printf("  ERROR: expected rejection_reason='already migrated' for %s", rec.Name)
	}
	if !expectMigrated && isAlreadyMigrated {
		log.Printf("  ERROR: unexpected already-migrated rejection for %s", rec.Name)
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
			newN, err := queryDelegationToValidatorCount(rec.NewAddress, d.Validator)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query new delegation %s failed: %v", d.Validator, err))
			} else if newN == 0 {
				issues = append(issues, fmt.Sprintf("expected delegation on new address to %s, got 0", d.Validator))
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
			newN, err := queryUnbondingFromValidatorCount(rec.NewAddress, u.Validator)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query new unbonding %s failed: %v", u.Validator, err))
			} else if newN == 0 {
				issues = append(issues, fmt.Sprintf("expected unbonding on new address from %s, got 0", u.Validator))
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
			newN, err := queryRedelegationCount(rec.NewAddress, rd.SrcValidator, rd.DstValidator)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query new redelegation %s failed: %v", key, err))
			} else if newN == 0 {
				issues = append(issues, fmt.Sprintf("expected redelegation on new address for %s, got 0", key))
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
			ok, err := queryAuthzGrantExists(rec.NewAddress, g.Grantee)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query new authz grant -> %s failed: %v", g.Grantee, err))
			} else if !ok {
				issues = append(issues, fmt.Sprintf("expected authz grant on new address -> %s", g.Grantee))
			}
			legacyOK, err := queryAuthzGrantExists(rec.Address, g.Grantee)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query legacy authz grant -> %s failed: %v", g.Grantee, err))
			} else if legacyOK {
				issues = append(issues, fmt.Sprintf("legacy authz grant still present -> %s", g.Grantee))
			}
		}
	} else if rec.HasAuthzGrant && rec.AuthzGrantedTo != "" {
		ok, err := queryAuthzGrantExists(rec.NewAddress, rec.AuthzGrantedTo)
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
			ok, err := queryAuthzGrantExists(g.Granter, rec.NewAddress)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query authz grant %s -> new failed: %v", g.Granter, err))
			} else if !ok {
				issues = append(issues, fmt.Sprintf("expected authz grant %s -> new address", g.Granter))
			}
			legacyOK, err := queryAuthzGrantExists(g.Granter, rec.Address)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query authz grant %s -> legacy failed: %v", g.Granter, err))
			} else if legacyOK {
				issues = append(issues, fmt.Sprintf("authz grant %s still targets legacy address", g.Granter))
			}
		}
	} else if rec.HasAuthzAsGrantee && rec.AuthzReceivedFrom != "" {
		ok, err := queryAuthzGrantExists(rec.AuthzReceivedFrom, rec.NewAddress)
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
			ok, err := queryFeegrantAllowanceExists(rec.NewAddress, g.Grantee)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query new feegrant -> %s failed: %v", g.Grantee, err))
			} else if !ok {
				issues = append(issues, fmt.Sprintf("expected feegrant on new address -> %s", g.Grantee))
			}
			legacyOK, err := queryFeegrantAllowanceExists(rec.Address, g.Grantee)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query legacy feegrant -> %s failed: %v", g.Grantee, err))
			} else if legacyOK {
				issues = append(issues, fmt.Sprintf("legacy feegrant still present -> %s", g.Grantee))
			}
		}
	} else if rec.HasFeegrant && rec.FeegrantGrantedTo != "" {
		ok, err := queryFeegrantAllowanceExists(rec.NewAddress, rec.FeegrantGrantedTo)
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
			ok, err := queryFeegrantAllowanceExists(g.Granter, rec.NewAddress)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query feegrant %s -> new failed: %v", g.Granter, err))
			} else if !ok {
				issues = append(issues, fmt.Sprintf("expected feegrant %s -> new address", g.Granter))
			}
			legacyOK, err := queryFeegrantAllowanceExists(g.Granter, rec.Address)
			if err != nil {
				issues = append(issues, fmt.Sprintf("query feegrant %s -> legacy failed: %v", g.Granter, err))
			} else if legacyOK {
				issues = append(issues, fmt.Sprintf("feegrant %s still targets legacy address", g.Granter))
			}
		}
	} else if rec.HasFeegrantGrantee && rec.FeegrantFrom != "" {
		ok, err := queryFeegrantAllowanceExists(rec.FeegrantFrom, rec.NewAddress)
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

	if len(issues) == 0 {
		return nil
	}
	return errors.New(strings.Join(issues, "; "))
}
