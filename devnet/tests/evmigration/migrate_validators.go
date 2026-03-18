package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type validatorCandidate struct {
	KeyName       string
	LegacyAddress string
	LegacyValoper string
}

// Destination keys created during migrate-validator runs use eth_secp256k1 and
// now use the evm- prefix. Older reruns may still have new_* destination keys.
// They must not be treated as legacy validator candidates on reruns, otherwise
// auto-detection sees both old and new keys.
func isDestinationValidatorKey(k keyRecord) bool {
	name := strings.ToLower(strings.TrimSpace(k.Name))
	if strings.HasPrefix(name, migratedAccountPrefix+"-") || strings.HasPrefix(name, "new_") {
		return true
	}

	pubKey := strings.ToLower(k.PubKey)
	return strings.Contains(pubKey, "ethsecp256k1") || strings.Contains(pubKey, "eth_secp256k1")
}

func isLegacyValidatorKey(k keyRecord) bool {
	return !isDestinationValidatorKey(k)
}

func valoperFromAccAddress(accAddr string) (string, error) {
	addr, err := sdk.AccAddressFromBech32(accAddr)
	if err != nil {
		return "", err
	}
	return sdk.ValAddress(addr).String(), nil
}

func runMigrateValidator() {
	log.Println("=== MIGRATE-VALIDATOR MODE ===")
	ensureEVMMigrationRuntime("migrate-validator mode")

	params, err := queryMigrationParams()
	if err != nil {
		log.Fatalf("query evmigration params: %v", err)
	}
	log.Printf("  params: enable_migration=%v migration_end_time=%d max_migrations_per_block=%d max_validator_delegations=%d",
		params.EnableMigration, params.MigrationEndTime, params.MaxMigrationsPerBlock, params.MaxValidatorDelegations)
	if !params.EnableMigration {
		log.Fatal("migration preflight failed: enable_migration=false. Submit/execute governance params update first, then rerun migrate-validator mode")
	}

	validators, err := getValidators()
	if err != nil {
		log.Fatalf("get validators: %v", err)
	}
	if len(validators) == 0 {
		log.Fatal("no validators found")
	}

	keys, err := listKeys()
	if err != nil {
		log.Fatalf("list keys: %v", err)
	}
	if len(keys) == 0 {
		log.Println("no local keys found in keyring; nothing to migrate")
		return
	}

	candidates := pickValidatorCandidates(validators, keys)
	if len(candidates) == 0 {
		log.Println("no local validator key matched staking validators; nothing to do")
		return
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].KeyName < candidates[j].KeyName
	})

	if len(candidates) > 1 {
		names := make([]string, 0, len(candidates))
		for _, c := range candidates {
			names = append(names, c.KeyName)
		}
		log.Fatalf("found %d local validator candidates (%s); set -validator-keys to exactly one key name", len(candidates), strings.Join(names, ","))
	}
	c := candidates[0]
	log.Printf("selected local validator candidate: key=%s legacy=%s valoper=%s", c.KeyName, c.LegacyAddress, c.LegacyValoper)

	initialStats, haveInitialStats := queryAndLogMigrationStats()
	ok, skipped := migrateOneValidator(c)
	failCount := 0
	okCount := 0
	skipCount := 0
	if ok {
		okCount = 1
	} else if skipped {
		skipCount = 1
	} else {
		failCount = 1
	}

	if ok && haveInitialStats {
		finalStats, haveFinalStats := queryAndLogMigrationStats()
		if haveFinalStats && finalStats.TotalValidatorsMigrated <= initialStats.TotalValidatorsMigrated {
			log.Fatalf("post-check failed: validators_migrated did not increase (before=%d after=%d)",
				initialStats.TotalValidatorsMigrated, finalStats.TotalValidatorsMigrated)
		}
	}

	log.Printf("validator migration summary: migrated=%d skipped=%d failed=%d", okCount, skipCount, failCount)
	if failCount > 0 {
		log.Fatalf("validator migration completed with %d failures", failCount)
	}
}

func pickValidatorCandidates(validators []string, keys []keyRecord) []validatorCandidate {
	keyByAddr := make(map[string]keyRecord, len(keys))
	keyByName := make(map[string]keyRecord, len(keys))
	for _, k := range keys {
		keyByAddr[k.Address] = k
		keyByName[k.Name] = k
	}

	if strings.TrimSpace(*flagValidatorKeys) != "" {
		selected := make([]validatorCandidate, 0)
		validatorSet := make(map[string]struct{}, len(validators))
		for _, v := range validators {
			validatorSet[v] = struct{}{}
		}
		for _, name := range strings.Split(*flagValidatorKeys, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			k, ok := keyByName[name]
			if !ok {
				log.Printf("WARN: validator key %q not found in keyring", name)
				continue
			}
			accAddr, err := sdk.AccAddressFromBech32(k.Address)
			if err != nil {
				log.Printf("WARN: invalid key address for %q: %v", name, err)
				continue
			}
			if !isLegacyValidatorKey(k) {
				log.Printf("WARN: key %q (%s) is a migrated destination key, not a legacy validator key", name, k.Address)
				continue
			}
			valoper := sdk.ValAddress(accAddr).String()
			if _, ok := validatorSet[valoper]; !ok {
				log.Printf("WARN: key %q (%s) is not a current validator", name, k.Address)
				continue
			}
			selected = append(selected, validatorCandidate{
				KeyName:       name,
				LegacyAddress: k.Address,
				LegacyValoper: valoper,
			})
		}
		return selected
	}

	selected := make([]validatorCandidate, 0)
	for _, valoper := range validators {
		valAddr, err := sdk.ValAddressFromBech32(valoper)
		if err != nil {
			continue
		}
		accAddr := sdk.AccAddress(valAddr).String()
		k, ok := keyByAddr[accAddr]
		if !ok {
			continue
		}
		if !isLegacyValidatorKey(k) {
			continue
		}
		selected = append(selected, validatorCandidate{
			KeyName:       k.Name,
			LegacyAddress: accAddr,
			LegacyValoper: valoper,
		})
	}
	return selected
}

func migrateOneValidator(c validatorCandidate) (ok bool, skipped bool) {
	log.Printf("--- migrate validator: key=%s legacy=%s valoper=%s ---", c.KeyName, c.LegacyAddress, c.LegacyValoper)

	already, recNewAddr := queryMigrationRecord(c.LegacyAddress)
	if already {
		newValoper, err := valoperFromAccAddress(recNewAddr)
		if err != nil {
			log.Printf("  SKIP: already migrated to %s (new valoper: <invalid address: %v>)", recNewAddr, err)
		} else {
			log.Printf("  SKIP: already migrated to %s (new valoper: %s)", recNewAddr, newValoper)
		}
		return false, true
	}

	estimate, err := queryMigrationEstimate(c.LegacyAddress)
	if err != nil {
		log.Printf("  FAIL: query migration-estimate: %v", err)
		return false, false
	}
	log.Printf("  estimate: is_validator=%v would_succeed=%v reason=%q val_delegations=%d",
		estimate.IsValidator, estimate.WouldSucceed, estimate.RejectionReason, estimate.ValDelegationCount)
	if !estimate.IsValidator {
		log.Printf("  FAIL: account is not a validator according to migration-estimate")
		return false, false
	}
	if !estimate.WouldSucceed {
		if estimate.RejectionReason == "already migrated" {
			log.Printf("  SKIP: already migrated")
			return false, true
		}
		log.Printf("  FAIL: validator migration would not succeed: %s", estimate.RejectionReason)
		return false, false
	}

	preDelegators, err := queryValidatorDelegationsToCount(c.LegacyValoper)
	if err != nil {
		log.Printf("  FAIL: query pre-migration validator delegations: %v", err)
		return false, false
	}
	preCreatorActionIDs, err := queryActionsByCreator(c.LegacyAddress)
	if err != nil {
		log.Printf("  FAIL: query pre-migration creator actions: %v", err)
		return false, false
	}
	preSupernodeActionIDs, err := queryActionsBySupernode(c.LegacyAddress)
	if err != nil {
		log.Printf("  FAIL: query pre-migration supernode actions: %v", err)
		return false, false
	}

	// Capture pre-migration supernode record and metrics for field-level validation.
	preSupernode, err := querySupernodeByValoper(c.LegacyValoper)
	if err != nil {
		log.Printf("  FAIL: query pre-migration supernode: %v", err)
		return false, false
	}
	var preMetrics *SuperNodeMetricsState
	if preSupernode != nil {
		preMetrics, err = querySupernodeMetricsByValoper(c.LegacyValoper)
		if err != nil {
			log.Printf("  FAIL: query pre-migration supernode metrics: %v", err)
			return false, false
		}
		log.Printf("  pre-migration supernode: account=%s evidence=%d prev_accounts=%d has_metrics=%v",
			preSupernode.SupernodeAccount, len(preSupernode.Evidence), len(preSupernode.PrevSupernodeAccounts), preMetrics != nil)
	} else {
		log.Printf("  INFO: no supernode registered for %s", c.LegacyValoper)
	}

	newRec, err := createUniqueAccount(migratedAccountPrefix+"-"+strings.ReplaceAll(c.KeyName, "_", "-"), false)
	if err != nil {
		log.Printf("  FAIL: create destination key: %v", err)
		return false, false
	}
	privHex, err := exportPrivateKeyHex(c.KeyName)
	if err != nil {
		log.Printf("  FAIL: export validator key: %v", err)
		return false, false
	}
	sigB64, pubB64, err := signMigrationMessageWithPrivHex("validator", privHex, c.LegacyAddress, newRec.Address)
	if err != nil {
		log.Printf("  FAIL: sign migration payload: %v", err)
		return false, false
	}

	pubBz, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil {
		log.Printf("  FAIL: decode pubkey b64: %v", err)
		return false, false
	}
	legacyPub := &secp256k1.PubKey{Key: pubBz}
	if sdk.AccAddress(legacyPub.Address()).String() != c.LegacyAddress {
		log.Printf("  FAIL: exported key does not match legacy validator address")
		return false, false
	}

	_, err = runTx(
		"tx", "evmigration", "migrate-validator",
		newRec.Address, c.LegacyAddress, pubB64, sigB64,
		"--from", newRec.Name)
	if err != nil {
		log.Printf("  FAIL: migrate-validator tx failed: %v", err)
		return false, false
	}

	// Verify migration record.
	hasRecord, recNewAddr := queryMigrationRecord(c.LegacyAddress)
	if !hasRecord {
		log.Printf("  FAIL: migration-record not found after tx")
		return false, false
	}
	if recNewAddr != newRec.Address {
		log.Printf("  FAIL: migration-record new_address mismatch, expected=%s got=%s", newRec.Address, recNewAddr)
		return false, false
	}

	postEstimate, err := queryMigrationEstimate(c.LegacyAddress)
	if err != nil {
		log.Printf("  FAIL: query post-migration estimate: %v", err)
		return false, false
	}
	if postEstimate.RejectionReason != "already migrated" {
		log.Printf("  FAIL: expected post-migration rejection_reason='already migrated', got %q", postEstimate.RejectionReason)
		return false, false
	}

	// The old validator KV entry may remain orphaned by design. Validator
	// migration re-keys the active indexes and linked state to the new valoper,
	// but it does not delete the legacy staking record because the SDK removal
	// path is not safe for bonded validators during migration.
	if _, err := run("query", "staking", "validator", c.LegacyValoper); err == nil {
		log.Printf("  INFO: legacy validator record still queryable at %s (expected orphaned entry)", c.LegacyValoper)
	}

	// Verify the validator exists under new valoper address.
	newAcc, err := sdk.AccAddressFromBech32(newRec.Address)
	if err != nil {
		log.Printf("  FAIL: parse new address: %v", err)
		return false, false
	}
	newValoper := sdk.ValAddress(newAcc).String()
	if _, err := run("query", "staking", "validator", newValoper); err != nil {
		log.Printf("  FAIL: new validator record not found at %s: %v", newValoper, err)
		return false, false
	}

	postDelegators, err := queryValidatorDelegationsToCount(newValoper)
	if err != nil {
		log.Printf("  FAIL: query post-migration validator delegations: %v", err)
		return false, false
	}
	if postDelegators != preDelegators {
		log.Printf("  FAIL: validator delegator count mismatch pre=%d post=%d", preDelegators, postDelegators)
		return false, false
	}
	if err := verifyValidatorActionMigration(c.LegacyAddress, newRec.Address, preCreatorActionIDs, preSupernodeActionIDs); err != nil {
		log.Printf("  FAIL: validator action migration checks: %v", err)
		return false, false
	}

	if preSupernode != nil {
		if err := verifySupernodeMigration(c.LegacyValoper, newValoper, c.LegacyAddress, newRec.Address, preSupernode, preMetrics); err != nil {
			log.Printf("  FAIL: supernode migration checks: %v", err)
			return false, false
		}
	}

	log.Printf("  OK: validator migrated %s (%s) -> %s (%s) (new key=%s)",
		c.LegacyAddress, c.LegacyValoper, newRec.Address, newValoper, newRec.Name)
	return true, false
}

func verifyValidatorActionMigration(legacyAddr, newAddr string, preCreatorActionIDs, preSupernodeActionIDs []string) error {
	if legacyIDs, err := queryActionsByCreator(legacyAddr); err != nil {
		return fmt.Errorf("query legacy creator actions: %w", err)
	} else if len(legacyIDs) > 0 {
		return fmt.Errorf("expected 0 legacy creator actions after migration, got %d", len(legacyIDs))
	}
	if legacyIDs, err := queryActionsBySupernode(legacyAddr); err != nil {
		return fmt.Errorf("query legacy supernode actions: %w", err)
	} else if len(legacyIDs) > 0 {
		return fmt.Errorf("expected 0 legacy supernode actions after migration, got %d", len(legacyIDs))
	}

	if len(preCreatorActionIDs) > 0 {
		newIDs, err := queryActionsByCreator(newAddr)
		if err != nil {
			return fmt.Errorf("query new creator actions: %w", err)
		}
		if missing := missingIDs(preCreatorActionIDs, newIDs); len(missing) > 0 {
			return fmt.Errorf("new creator action index missing migrated actions %s", strings.Join(missing, ","))
		}
		for _, actionID := range preCreatorActionIDs {
			creator, err := queryActionCreator(actionID)
			if err != nil {
				return fmt.Errorf("query creator for action %s: %w", actionID, err)
			}
			if creator != newAddr {
				return fmt.Errorf("action %s creator mismatch: expected %s got %s", actionID, newAddr, creator)
			}
		}
	}

	if len(preSupernodeActionIDs) > 0 {
		newIDs, err := queryActionsBySupernode(newAddr)
		if err != nil {
			return fmt.Errorf("query new supernode actions: %w", err)
		}
		if missing := missingIDs(preSupernodeActionIDs, newIDs); len(missing) > 0 {
			return fmt.Errorf("new supernode action index missing migrated actions %s", strings.Join(missing, ","))
		}
		for _, actionID := range preSupernodeActionIDs {
			supernodes, err := queryActionSupernodes(actionID)
			if err != nil {
				return fmt.Errorf("query supernodes for action %s: %w", actionID, err)
			}
			if !containsString(supernodes, newAddr) {
				return fmt.Errorf("action %s missing migrated supernode %s", actionID, newAddr)
			}
			if containsString(supernodes, legacyAddr) {
				return fmt.Errorf("action %s still contains legacy supernode %s", actionID, legacyAddr)
			}
		}
	}

	return nil
}

func missingIDs(expected, got []string) []string {
	gotSet := make(map[string]struct{}, len(got))
	for _, id := range got {
		gotSet[id] = struct{}{}
	}
	missing := make([]string, 0)
	for _, id := range expected {
		if _, ok := gotSet[id]; !ok {
			missing = append(missing, id)
		}
	}
	return missing
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func queryMigrationRecord(legacyAddr string) (exists bool, newAddr string) {
	out, err := run("query", "evmigration", "migration-record", legacyAddr)
	if err != nil {
		return false, ""
	}
	var resp struct {
		Record *struct {
			NewAddress string `json:"new_address"`
		} `json:"record"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return false, ""
	}
	if resp.Record == nil {
		return false, ""
	}
	return true, resp.Record.NewAddress
}

func createUniqueAccount(baseName string, isLegacy bool) (AccountRecord, error) {
	for i := 0; i < 50; i++ {
		name := baseName
		if i > 0 {
			name = fmt.Sprintf("%s-%02d", baseName, i)
		}
		// Skip names that already exist in the keyring (e.g. from a previous
		// interrupted run). This avoids the SDK's interactive overwrite prompt
		// which produces an "aborted" error when stdin doesn't provide "y".
		if keyExists(name) {
			continue
		}
		rec, err := generateAccount(name, isLegacy)
		if err != nil {
			return AccountRecord{}, err
		}
		if err := importKey(name, rec.Mnemonic, isLegacy); err != nil {
			low := strings.ToLower(err.Error())
			if strings.Contains(low, "already exists") || strings.Contains(low, "key exists") || strings.Contains(low, "aborted") {
				continue
			}
			return AccountRecord{}, err
		}
		rec.Name = name
		return rec, nil
	}
	return AccountRecord{}, fmt.Errorf("unable to create unique key with base name %s", baseName)
}

// verifySupernodeMigration checks that the supernode record, evidence,
// account history, and metrics state were correctly re-keyed by migration.
func verifySupernodeMigration(
	oldValoper, newValoper string,
	legacyAddr, newAddr string,
	preSN *SuperNodeRecord,
	preMetrics *SuperNodeMetricsState,
) error {
	// 1. Supernode record should exist under the new valoper.
	postSN, err := querySupernodeByValoper(newValoper)
	if err != nil {
		return fmt.Errorf("query post-migration supernode by new valoper %s: %w", newValoper, err)
	}
	if postSN == nil {
		return fmt.Errorf("supernode not found under new valoper %s", newValoper)
	}

	// 2. ValidatorAddress must be the new valoper.
	if postSN.ValidatorAddress != newValoper {
		return fmt.Errorf("supernode ValidatorAddress mismatch: expected %s got %s", newValoper, postSN.ValidatorAddress)
	}

	// 3. SupernodeAccount: if it matched the validator's legacy address, it should
	// now be the validator's new address. Otherwise it was an independent account
	// (already migrated or a separate entity) and should be preserved unchanged.
	if preSN.SupernodeAccount == legacyAddr {
		if postSN.SupernodeAccount != newAddr {
			return fmt.Errorf("supernode SupernodeAccount mismatch: expected %s got %s", newAddr, postSN.SupernodeAccount)
		}
	} else {
		if postSN.SupernodeAccount != preSN.SupernodeAccount {
			// The independent supernode account may have been legitimately migrated
			// via MsgClaimLegacyAccount between our pre/post snapshots. Verify by
			// checking whether a migration record exists for the old SN account
			// pointing to the new one.
			if migrated, newSNAddr := queryMigrationRecord(preSN.SupernodeAccount); migrated && newSNAddr == postSN.SupernodeAccount {
				log.Printf("  supernode account migrated independently: %s -> %s (OK)", preSN.SupernodeAccount, postSN.SupernodeAccount)
			} else {
				return fmt.Errorf("supernode SupernodeAccount was overwritten unexpectedly: pre=%s post=%s (no migration record found)",
					preSN.SupernodeAccount, postSN.SupernodeAccount)
			}
		} else {
			log.Printf("  supernode account preserved (independent): %s", postSN.SupernodeAccount)
		}
	}

	// 4. Evidence: every entry that referenced old valoper should now reference new valoper.
	for i, ev := range postSN.Evidence {
		if ev.ValidatorAddress == oldValoper {
			return fmt.Errorf("evidence[%d] still references old valoper %s", i, oldValoper)
		}
	}
	// If pre-migration had evidence pointing to old valoper, post-migration should have them pointing to new.
	for i, preEv := range preSN.Evidence {
		if preEv.ValidatorAddress == oldValoper {
			if i >= len(postSN.Evidence) {
				return fmt.Errorf("evidence[%d] missing after migration", i)
			}
			if postSN.Evidence[i].ValidatorAddress != newValoper {
				return fmt.Errorf("evidence[%d] ValidatorAddress not migrated: expected %s got %s",
					i, newValoper, postSN.Evidence[i].ValidatorAddress)
			}
		}
	}
	log.Printf("  supernode evidence: %d entries verified", len(postSN.Evidence))

	// 5. PrevSupernodeAccounts: only updated when the supernode account matched
	// the validator's legacy address (i.e. the validator was its own supernode
	// account). Independent supernode accounts have their history left untouched.
	if preSN.SupernodeAccount == legacyAddr {
		expectedHistoryLen := len(preSN.PrevSupernodeAccounts) + 1
		if len(postSN.PrevSupernodeAccounts) != expectedHistoryLen {
			return fmt.Errorf("PrevSupernodeAccounts length mismatch: expected %d got %d",
				expectedHistoryLen, len(postSN.PrevSupernodeAccounts))
		}
		// The last entry should record the migration (new account).
		lastEntry := postSN.PrevSupernodeAccounts[len(postSN.PrevSupernodeAccounts)-1]
		if lastEntry.Account != newAddr {
			return fmt.Errorf("PrevSupernodeAccounts last entry account mismatch: expected %s got %s",
				newAddr, lastEntry.Account)
		}
		// Existing history entries matching old account should now reference new account.
		for i, preHist := range preSN.PrevSupernodeAccounts {
			if preHist.Account == legacyAddr {
				if postSN.PrevSupernodeAccounts[i].Account != newAddr {
					return fmt.Errorf("PrevSupernodeAccounts[%d] account not migrated: expected %s got %s",
						i, newAddr, postSN.PrevSupernodeAccounts[i].Account)
				}
			}
		}
		log.Printf("  supernode account history: %d entries (including migration entry)", len(postSN.PrevSupernodeAccounts))
	} else {
		// Independent supernode account — history should not have been modified
		// by the validator migration. The length may differ from pre-migration
		// if the account was migrated independently (which appends its own entry),
		// but the validator migration itself must not touch it.
		log.Printf("  supernode account history: %d entries (independent account, not modified by validator migration)", len(postSN.PrevSupernodeAccounts))
	}

	// 6. Metrics state: if it existed pre-migration, it should be re-keyed.
	if preMetrics != nil {
		postMetrics, err := querySupernodeMetricsByValoper(newValoper)
		if err != nil {
			return fmt.Errorf("query post-migration metrics by new valoper: %w", err)
		}
		if postMetrics == nil {
			return fmt.Errorf("metrics state missing under new valoper %s (was present under old)", newValoper)
		}
		if postMetrics.ValidatorAddress != newValoper {
			return fmt.Errorf("metrics ValidatorAddress mismatch: expected %s got %s",
				newValoper, postMetrics.ValidatorAddress)
		}
		// Old metrics key should be deleted.
		oldMetrics, err := querySupernodeMetricsByValoper(oldValoper)
		if err != nil {
			return fmt.Errorf("query old metrics by old valoper: %w", err)
		}
		if oldMetrics != nil {
			return fmt.Errorf("stale metrics still exist under old valoper %s", oldValoper)
		}
		log.Printf("  supernode metrics: re-keyed and old key deleted")
	} else {
		log.Printf("  supernode metrics: none (skipped)")
	}

	log.Printf("  supernode migration verified: %s -> %s", oldValoper, newValoper)
	return nil
}
