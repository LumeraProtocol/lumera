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

func runMigrateValidator() {
	log.Println("=== MIGRATE-VALIDATOR MODE ===")
	ensureEVMMigrationRuntime("migrate-validator mode")

	if *flagFunder == "" {
		if name, err := detectFunder(); err == nil {
			*flagFunder = name
			log.Printf("auto-detected funder from keyring: %s", name)
		}
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
		log.Printf("  SKIP: already migrated to %s", recNewAddr)
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

	newRec, err := createUniqueAccount(fmt.Sprintf("new_%s", c.KeyName), false)
	if err != nil {
		log.Printf("  FAIL: create destination key: %v", err)
		return false, false
	}

	// Optional seed for tx fees (in case fee waiver is not active in this environment).
	if *flagFunder != "" {
		if _, err := runTx("tx", "bank", "send", *flagFunder, newRec.Address, "200000ulume", "--from", *flagFunder); err != nil {
			log.Printf("  WARN: could not pre-fund destination %s: %v", newRec.Address, err)
		}
	}

	privHex, err := exportPrivateKeyHex(c.KeyName)
	if err != nil {
		log.Printf("  FAIL: export validator key: %v", err)
		return false, false
	}
	sigB64, pubB64, err := signMigrationMessageWithPrivHex(privHex, c.LegacyAddress, newRec.Address)
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

	_, err = runTx("tx", "evmigration", "migrate-validator",
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

	if _, err := run("query", "staking", "validator", c.LegacyValoper); err == nil {
		log.Printf("  FAIL: legacy validator record still exists at %s", c.LegacyValoper)
		return false, false
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

	log.Printf("  OK: validator migrated %s -> %s (new key=%s)", c.LegacyAddress, newRec.Address, newRec.Name)
	return true, false
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
			name = fmt.Sprintf("%s_%02d", baseName, i)
		}
		rec, err := generateAccount(name, isLegacy)
		if err != nil {
			return AccountRecord{}, err
		}
		if err := importKey(name, rec.Mnemonic, isLegacy); err != nil {
			low := strings.ToLower(err.Error())
			if strings.Contains(low, "already exists") || strings.Contains(low, "key exists") {
				continue
			}
			return AccountRecord{}, err
		}
		rec.Name = name
		return rec, nil
	}
	return AccountRecord{}, fmt.Errorf("unable to create unique key with base name %s", baseName)
}
