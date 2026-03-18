package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	legacyPreparedAccountPrefix   = "pre-evm"
	extraPreparedAccountPrefix    = "pre-evmex"
	migratedAccountPrefix         = "evm"
	migratedExtraAccountPrefix    = "evmex"
	legacyPreparedAccountPrefixV0 = "evm_test"
	extraPreparedAccountPrefixV0  = "evm_testex"
)

func runPrepare() {
	ensurePrepareRuntime()

	if *flagFunder == "" {
		name, err := detectFunder()
		if err != nil {
			log.Fatalf("no -funder provided and auto-detect failed: %v", err)
		}
		*flagFunder = name
		log.Printf("auto-detected funder from keyring: %s", name)
	}

	log.Printf("=== PREPARE MODE: generating %d legacy + %d extra accounts ===",
		*flagNumAccounts, *flagNumExtra)

	validators, err := getValidators()
	if err != nil {
		log.Fatalf("get validators: %v", err)
	}
	log.Printf("found %d existing validators: %v", len(validators), validators)
	if len(validators) == 0 {
		log.Fatal("no validators found")
	}

	funderAddr, err := getAddress(*flagFunder)
	if err != nil {
		log.Fatalf("get funder address: %v", err)
	}
	log.Printf("funder: %s (%s)", *flagFunder, funderAddr)

	accountTag := resolvePrepareAccountTag(*flagAccountTag, *flagFunder, funderAddr)
	if accountTag == "" {
		log.Printf("account name tag: none (using %s-XXX / %s-XXX)", legacyPreparedAccountPrefix, extraPreparedAccountPrefix)
	} else {
		log.Printf("account name tag: %s (using %s-%s-XXX / %s-%s-XXX)",
			accountTag, legacyPreparedAccountPrefix, accountTag, extraPreparedAccountPrefix, accountTag)
	}

	// Load existing accounts file if present (supports rerun).
	var af *AccountsFile
	if _, statErr := os.Stat(*flagFile); statErr == nil {
		af = loadAccounts(*flagFile)
		log.Printf("  loaded existing accounts file with %d accounts (rerun mode)", len(af.Accounts))
	} else {
		af = &AccountsFile{
			ChainID:   *flagChainID,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
			Funder:    funderAddr,
		}
	}
	af.Validators = validators
	for i := range af.Accounts {
		af.Accounts[i].normalizeActivityTracking()
	}

	// Index existing accounts by name for fast lookup.
	existingByName := make(map[string]int, len(af.Accounts))
	for i, rec := range af.Accounts {
		existingByName[rec.Name] = i
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	legacyIdx := make([]int, 0, *flagNumAccounts)
	extraIdx := make([]int, 0, *flagNumExtra)

	// Generate legacy accounts (will be migrated).
	log.Println("--- Generating legacy accounts ---")
	for i := 0; i < *flagNumAccounts; i++ {
		name := buildPreparedAccountName(legacyPreparedAccountPrefix, accountTag, i)
		if idx, ok := findPreparedAccountIndex(existingByName, legacyPreparedAccountPrefix, accountTag, i); ok {
			legacyIdx = append(legacyIdx, idx)
			log.Printf("  reusing existing %s: %s", af.Accounts[idx].Name, af.Accounts[idx].Address)
			continue
		}
		rec, err := ensureAccount(name, true)
		if err != nil {
			log.Fatalf("ensure account %s: %v", name, err)
		}
		af.Accounts = append(af.Accounts, rec)
		idx := len(af.Accounts) - 1
		existingByName[name] = idx
		legacyIdx = append(legacyIdx, idx)
		log.Printf("  created %s: %s", name, rec.Address)
	}

	// Generate extra legacy accounts (will also be migrated).
	log.Println("--- Generating extra accounts ---")
	for i := 0; i < *flagNumExtra; i++ {
		name := buildPreparedAccountName(extraPreparedAccountPrefix, accountTag, i)
		if idx, ok := findPreparedAccountIndex(existingByName, extraPreparedAccountPrefix, accountTag, i); ok {
			extraIdx = append(extraIdx, idx)
			log.Printf("  reusing existing %s: %s", af.Accounts[idx].Name, af.Accounts[idx].Address)
			continue
		}
		rec, err := ensureAccount(name, true)
		if err != nil {
			log.Fatalf("ensure account %s: %v", name, err)
		}
		af.Accounts = append(af.Accounts, rec)
		idx := len(af.Accounts) - 1
		existingByName[name] = idx
		extraIdx = append(extraIdx, idx)
		log.Printf("  created %s: %s", name, rec.Address)
	}

	// Ensure account file addresses and keyring keys are aligned before funding.
	reconcileAccountsWithKeyring(af)

	// Save after key generation so reruns find accounts even if later steps fail.
	saveAccounts(*flagFile, af)

	// Fund all accounts.
	log.Println("--- Funding accounts ---")
	if err := fundAccountsBatched(af, rng); err != nil {
		log.Printf("  WARN: batched funding failed (%v), falling back to sequential funding", err)
		fundAccountsSequential(af, rng)
	}

	log.Println("--- Waiting for supernode upload readiness ---")
	if waitForEligibleCascadeSupernodes(validators, 90*time.Second) {
		log.Println("  cascade uploads enabled: at least one registered supernode is ACTIVE")
	} else {
		log.Println("  WARN: no ACTIVE cascade supernodes detected within 90s; upload-backed action creation may still fail")
	}

	// Create activity for legacy accounts in parallel batches.
	// Phase 1: own-account operations (--from rec.Name) — safe to parallelize.
	// Phase 2: cross-account operations (--from other account) — run sequentially.
	log.Println("--- Creating legacy account activity (phase 1: own-account ops) ---")
	runParallel(legacyIdx, 5, func(ordinal, idx int) {
		rec := &af.Accounts[idx]
		if !rec.HasBalance {
			return
		}
		if !ensureSenderAccountReady(rec) {
			return
		}

		// Per-account RNG to avoid races on the shared rng.
		localRng := rand.New(rand.NewSource(int64(ordinal) + time.Now().UnixNano()))

		delegatedVals := make([]string, 0, 3)
		if len(rec.Delegations) > 0 {
			for _, d := range rec.Delegations {
				if d.Validator != "" {
					delegatedVals = append(delegatedVals, d.Validator)
				}
			}
		} else if rec.HasDelegation && rec.DelegatedTo != "" {
			delegatedVals = append(delegatedVals, rec.DelegatedTo)
		}

		// 1) Delegate to random validators (1..3) to vary account state.
		nTargets := 1 + localRng.Intn(minInt(3, len(validators)))
		for _, valAddr := range pickRandomValidators(validators, nTargets, localRng) {
			delegateAmt := fmt.Sprintf("%dulume", 100_000+localRng.Intn(400_000))
			_, err := runTx("tx", "staking", "delegate", valAddr, delegateAmt, "--from", rec.Name)
			if err != nil {
				log.Printf("  WARN: delegate %s: %v", rec.Name, err)
				continue
			}
			rec.addDelegation(valAddr, delegateAmt)
			delegatedVals = append(delegatedVals, valAddr)
			log.Printf("  %s delegated %s to %s", rec.Name, delegateAmt, valAddr)
		}

		// 2) Every 4th legacy account: create unbonding entries from a random delegated validator.
		if rec.HasDelegation && ordinal%4 == 0 {
			srcVal := rec.DelegatedTo
			if len(delegatedVals) > 0 {
				srcVal = delegatedVals[localRng.Intn(len(delegatedVals))]
			}
			unbondAmt := "20000ulume"
			_, err := runTx("tx", "staking", "unbond", srcVal, unbondAmt, "--from", rec.Name)
			if err != nil {
				log.Printf("  WARN: unbond %s: %v", rec.Name, err)
			} else {
				rec.addUnbonding(srcVal, unbondAmt)
				log.Printf("  %s unbonded %s from %s", rec.Name, unbondAmt, srcVal)
			}
		}

		// 3) Every 6th legacy account: create 1..3 redelegation attempts.
		if rec.HasDelegation && ordinal%6 == 0 && len(validators) > 1 {
			attempts := 1 + localRng.Intn(minInt(3, len(validators)-1))
			for i := 0; i < attempts; i++ {
				srcVal := rec.DelegatedTo
				if len(delegatedVals) > 0 {
					srcVal = delegatedVals[localRng.Intn(len(delegatedVals))]
				}
				dstVal, ok := pickDifferentValidator(validators, srcVal, localRng)
				if !ok {
					continue
				}
				if n, err := queryRedelegationCount(rec.Address, srcVal, dstVal); err == nil && n > 0 {
					rec.addRedelegation(srcVal, dstVal, "")
					log.Printf("  %s already has redelegation %s -> %s, reusing existing state", rec.Name, srcVal, dstVal)
					continue
				}
				redelAmt := "15000ulume"
				_, err := runTx("tx", "staking", "redelegate", srcVal, dstVal, redelAmt, "--from", rec.Name)
				if err != nil {
					if isPrepareRerunConflict(err) {
						if n, qErr := queryAnyRedelegationCount(rec.Address, validators); qErr == nil && n > 0 {
							if n2, qErr2 := queryRedelegationCount(rec.Address, srcVal, dstVal); qErr2 == nil && n2 > 0 {
								rec.addRedelegation(srcVal, dstVal, "")
							}
							log.Printf("  %s redelegation already in progress, reusing existing state", rec.Name)
						} else {
							log.Printf("  %s redelegation already in progress but no query-visible entry; skipping marker update", rec.Name)
						}
					} else {
						log.Printf("  WARN: redelegate %s: %v", rec.Name, err)
					}
					continue
				}
				rec.addRedelegation(srcVal, dstVal, redelAmt)
				log.Printf("  %s redelegated %s from %s -> %s", rec.Name, redelAmt, srcVal, dstVal)
			}
		}

		// 4) Every 7th legacy account: set third-party withdraw address.
		if ordinal%7 == 0 && len(extraIdx) > 0 {
			thirdParty := af.Accounts[extraIdx[ordinal%len(extraIdx)]].Address
			_, err := runTx("tx", "distribution", "set-withdraw-addr", thirdParty, "--from", rec.Name)
			if err != nil {
				log.Printf("  WARN: set-withdraw-addr %s: %v", rec.Name, err)
			} else {
				rec.addWithdrawAddress(thirdParty)
				log.Printf("  %s set withdraw addr to %s", rec.Name, thirdParty)
			}
		}

		// 5) Every 3rd legacy account: authz grants to up to 3 random legacy peers.
		if ordinal%3 == 0 && len(legacyIdx) > 1 {
			targets := pickRandomLegacyIndices(legacyIdx, idx, 3, localRng)
			for _, granteeIdx := range targets {
				grantee := af.Accounts[granteeIdx].Address
				if ok, err := queryAuthzGrantExists(rec.Address, grantee); err == nil && ok {
					rec.addAuthzGrant(grantee, bankSendMsgType)
					log.Printf("  %s authz grant to %s already exists, reusing existing state", rec.Name, grantee)
					continue
				}
				_, err := runTx("tx", "authz", "grant", grantee, "generic",
					"--msg-type", bankSendMsgType,
					"--from", rec.Name)
				if err != nil {
					if isPrepareRerunConflict(err) {
						rec.addAuthzGrant(grantee, bankSendMsgType)
						log.Printf("  %s authz grant already exists, reusing existing state", rec.Name)
					} else {
						log.Printf("  WARN: authz grant %s: %v", rec.Name, err)
					}
					continue
				}
				rec.addAuthzGrant(grantee, bankSendMsgType)
				log.Printf("  %s granted authz to %s", rec.Name, grantee)
			}
		}

		// 6a) Every 4th legacy account offset by 2: register CASCADE actions
		// via sdk-go to exercise x/action creator migration and supernode upload.
		// Actions are left in different states: PENDING, DONE, APPROVED.
		if ordinal%4 == 2 {
			if rec.hasDelayedClaim() {
				log.Printf("  %s already has delayed-claim activity; skipping sdk actions to avoid vesting-account uploads on old supernode", rec.Name)
			} else if vesting, err := queryAccountIsVesting(rec.Address); err != nil {
				log.Printf("  WARN: query vesting state for %s: %v", rec.Name, err)
			} else if vesting {
				log.Printf("  %s is already a vesting account on-chain; skipping sdk actions to avoid unsupported uploads on old supernode", rec.Name)
			} else {
				nPending, nDone, nApproved := 1, 0, 0
				if ordinal%8 == 2 {
					// Give some accounts actions in all three states.
					nPending, nDone, nApproved = 1, 1, 0
				}
				if ordinal%16 == 2 {
					nPending, nDone, nApproved = 0, 1, 1
				}
				ctx := context.Background()
				if err := createActionsWithSDK(ctx, &af.Accounts[idx], nPending, nDone, nApproved); err != nil {
					log.Printf("  WARN: sdk actions %s: %v", rec.Name, err)
				}
			}
		}

		// 7) Every 5th legacy account: feegrants to up to 3 random legacy peers.
		if ordinal%5 == 0 && len(legacyIdx) > 2 {
			targets := pickRandomLegacyIndices(legacyIdx, idx, 3, localRng)
			for _, granteeIdx := range targets {
				grantee := af.Accounts[granteeIdx].Address
				if ok, err := queryFeegrantAllowanceExists(rec.Address, grantee); err == nil && ok {
					rec.addFeegrant(grantee, "500000ulume")
					log.Printf("  %s feegrant to %s already exists, reusing existing state", rec.Name, grantee)
					continue
				}
				_, err := runTx("tx", "feegrant", "grant", rec.Address, grantee,
					"--spend-limit", "500000ulume",
					"--from", rec.Name)
				if err != nil {
					if isPrepareRerunConflict(err) {
						rec.addFeegrant(grantee, "500000ulume")
						log.Printf("  %s feegrant already exists, reusing existing state", rec.Name)
					} else {
						log.Printf("  WARN: feegrant %s: %v", rec.Name, err)
					}
					continue
				}
				rec.addFeegrant(grantee, "500000ulume")
				log.Printf("  %s granted feegrant to %s", rec.Name, grantee)
			}
		}
	})

	// Phase 2: cross-account operations (--from is a different account).
	// These run sequentially to avoid sequence conflicts on the granter.
	log.Println("--- Creating legacy account activity (phase 2: cross-account ops) ---")
	for ordinal, idx := range legacyIdx {
		rec := &af.Accounts[idx]
		if !rec.HasBalance {
			continue
		}

		localRng := rand.New(rand.NewSource(int64(idx) + time.Now().UnixNano()))

		// 6) Every 4th legacy account offset by 1: receive authz grants from up to 3 peers.
		if ordinal%4 == 1 && len(legacyIdx) > 1 {
			for _, granterIdx := range pickRandomLegacyIndices(legacyIdx, idx, 3, localRng) {
				granter := &af.Accounts[granterIdx]
				if !ensureSenderAccountReady(granter) {
					continue
				}
				if ok, err := queryAuthzGrantExists(granter.Address, rec.Address); err == nil && ok {
					rec.addAuthzAsGrantee(granter.Address, bankSendMsgType)
					log.Printf("  %s already has authz from %s, reusing existing state", rec.Name, granter.Name)
					continue
				}
				_, err := runTx("tx", "authz", "grant", rec.Address, "generic",
					"--msg-type", bankSendMsgType,
					"--from", granter.Name)
				if err != nil {
					if isPrepareRerunConflict(err) {
						rec.addAuthzAsGrantee(granter.Address, bankSendMsgType)
						log.Printf("  %s already has authz from %s, reusing existing state", rec.Name, granter.Name)
					} else {
						log.Printf("  WARN: authz receive %s from %s: %v", rec.Name, granter.Name, err)
					}
					continue
				}
				rec.addAuthzAsGrantee(granter.Address, bankSendMsgType)
				log.Printf("  %s received authz from %s", rec.Name, granter.Name)
			}
		}

		// 8) Every 6th legacy account offset by 1: receive feegrants from up to 3 peers.
		if ordinal%6 == 1 && len(legacyIdx) > 2 {
			for _, granterIdx := range pickRandomLegacyIndices(legacyIdx, idx, 3, localRng) {
				granter := &af.Accounts[granterIdx]
				if !ensureSenderAccountReady(granter) {
					continue
				}
				if ok, err := queryFeegrantAllowanceExists(granter.Address, rec.Address); err == nil && ok {
					rec.addFeegrantAsGrantee(granter.Address, "350000ulume")
					log.Printf("  %s already has feegrant from %s, reusing existing state", rec.Name, granter.Name)
					continue
				}
				_, err := runTx("tx", "feegrant", "grant", granter.Address, rec.Address,
					"--spend-limit", "350000ulume",
					"--from", granter.Name)
				if err != nil {
					if isPrepareRerunConflict(err) {
						rec.addFeegrantAsGrantee(granter.Address, "350000ulume")
						log.Printf("  %s already has feegrant from %s, reusing existing state", rec.Name, granter.Name)
					} else {
						log.Printf("  WARN: feegrant receive %s from %s: %v", rec.Name, granter.Name, err)
					}
					continue
				}
				rec.addFeegrantAsGrantee(granter.Address, "350000ulume")
				log.Printf("  %s received feegrant from %s", rec.Name, granter.Name)
			}
		}
	}

	// Extra accounts: parallel randomized activity to add realistic background noise.
	log.Println("--- Creating extra account activity ---")
	runParallel(extraIdx, 5, func(ordinal, idx int) {
		rec := &af.Accounts[idx]
		if !rec.HasBalance {
			return
		}
		if !ensureSenderAccountReady(rec) {
			return
		}
		localRng := rand.New(rand.NewSource(int64(ordinal) + time.Now().UnixNano()))

		delegatedVals := make([]string, 0, 3)
		for _, d := range rec.Delegations {
			if d.Validator != "" {
				delegatedVals = append(delegatedVals, d.Validator)
			}
		}
		if len(delegatedVals) == 0 && rec.DelegatedTo != "" {
			delegatedVals = append(delegatedVals, rec.DelegatedTo)
		}

		// 1) Stake to 1..3 random validators.
		nDelegations := 1 + localRng.Intn(minInt(3, len(validators)))
		for _, valAddr := range pickRandomValidators(validators, nDelegations, localRng) {
			delegateAmt := fmt.Sprintf("%dulume", 50_000+localRng.Intn(250_000))
			_, err := runTx("tx", "staking", "delegate", valAddr, delegateAmt, "--from", rec.Name)
			if err != nil {
				log.Printf("  WARN: extra delegate %s: %v", rec.Name, err)
				continue
			}
			rec.addDelegation(valAddr, delegateAmt)
			delegatedVals = append(delegatedVals, valAddr)
			log.Printf("  %s delegated %s to %s", rec.Name, delegateAmt, valAddr)
		}

		// 2) Optional bank sends to random extra peers.
		if len(extraIdx) > 1 {
			nSends := localRng.Intn(minInt(3, len(extraIdx)))
			for _, peerIdx := range pickRandomLegacyIndices(extraIdx, idx, nSends, localRng) {
				toAddr := af.Accounts[peerIdx].Address
				sendAmt := fmt.Sprintf("%dulume", 5_000+localRng.Intn(35_000))
				_, err := runTx("tx", "bank", "send", rec.Address, toAddr, sendAmt, "--from", rec.Name)
				if err != nil {
					log.Printf("  WARN: extra send %s -> %s: %v", rec.Name, af.Accounts[peerIdx].Name, err)
					continue
				}
				log.Printf("  %s sent %s to %s", rec.Name, sendAmt, af.Accounts[peerIdx].Name)
			}
		}

		// 3) Optional unbonding from one delegated validator.
		if len(delegatedVals) > 0 && localRng.Intn(100) < 50 {
			srcVal := delegatedVals[localRng.Intn(len(delegatedVals))]
			if n, err := queryUnbondingFromValidatorCount(rec.Address, srcVal); err == nil && n > 0 {
				rec.addUnbonding(srcVal, "")
				log.Printf("  %s already has unbonding from %s, reusing existing state", rec.Name, srcVal)
			} else {
				unbondAmt := "10000ulume"
				_, err := runTx("tx", "staking", "unbond", srcVal, unbondAmt, "--from", rec.Name)
				if err != nil {
					log.Printf("  WARN: extra unbond %s: %v", rec.Name, err)
				} else {
					rec.addUnbonding(srcVal, unbondAmt)
					log.Printf("  %s unbonded %s from %s", rec.Name, unbondAmt, srcVal)
				}
			}
		}

		// 4) Optional redelegations from delegated validators.
		if len(delegatedVals) > 0 && len(validators) > 1 && localRng.Intn(100) < 45 {
			attempts := 1 + localRng.Intn(2)
			for i := 0; i < attempts; i++ {
				srcVal := delegatedVals[localRng.Intn(len(delegatedVals))]
				dstVal, ok := pickDifferentValidator(validators, srcVal, localRng)
				if !ok {
					continue
				}
				if n, err := queryRedelegationCount(rec.Address, srcVal, dstVal); err == nil && n > 0 {
					rec.addRedelegation(srcVal, dstVal, "")
					log.Printf("  %s already has redelegation %s -> %s, reusing existing state", rec.Name, srcVal, dstVal)
					continue
				}
				redelAmt := fmt.Sprintf("%dulume", 5_000+localRng.Intn(15_000))
				_, err := runTx("tx", "staking", "redelegate", srcVal, dstVal, redelAmt, "--from", rec.Name)
				if err != nil {
					if isPrepareRerunConflict(err) {
						if n, qErr := queryAnyRedelegationCount(rec.Address, validators); qErr == nil && n > 0 {
							rec.addRedelegation(srcVal, dstVal, "")
							log.Printf("  %s redelegation already in progress, reusing existing state", rec.Name)
						}
					} else {
						log.Printf("  WARN: extra redelegate %s: %v", rec.Name, err)
					}
					continue
				}
				rec.addRedelegation(srcVal, dstVal, redelAmt)
				log.Printf("  %s redelegated %s from %s -> %s", rec.Name, redelAmt, srcVal, dstVal)
			}
		}

		// 5) Optional third-party withdraw address.
		if len(extraIdx) > 1 && localRng.Intn(100) < 30 {
			peers := pickRandomLegacyIndices(extraIdx, idx, 1, localRng)
			if len(peers) == 1 {
				withdrawAddr := af.Accounts[peers[0]].Address
				_, err := runTx("tx", "distribution", "set-withdraw-addr", withdrawAddr, "--from", rec.Name)
				if err != nil {
					log.Printf("  WARN: extra set-withdraw-addr %s: %v", rec.Name, err)
				} else {
					rec.addWithdrawAddress(withdrawAddr)
					log.Printf("  %s set withdraw addr to %s", rec.Name, withdrawAddr)
				}
			}
		}

		// 6) Optional authz grants to 1..2 extra peers.
		if len(extraIdx) > 1 && localRng.Intn(100) < 55 {
			nTargets := 1 + localRng.Intn(minInt(2, len(extraIdx)-1))
			for _, peerIdx := range pickRandomLegacyIndices(extraIdx, idx, nTargets, localRng) {
				grantee := af.Accounts[peerIdx].Address
				if ok, err := queryAuthzGrantExists(rec.Address, grantee); err == nil && ok {
					rec.addAuthzGrant(grantee, bankSendMsgType)
					log.Printf("  %s authz grant to %s already exists, reusing existing state", rec.Name, af.Accounts[peerIdx].Name)
					continue
				}
				_, err := runTx("tx", "authz", "grant", grantee, "generic",
					"--msg-type", bankSendMsgType,
					"--from", rec.Name)
				if err != nil {
					if isPrepareRerunConflict(err) {
						rec.addAuthzGrant(grantee, bankSendMsgType)
						log.Printf("  %s authz grant already exists, reusing existing state", rec.Name)
					} else {
						log.Printf("  WARN: extra authz grant %s -> %s: %v", rec.Name, af.Accounts[peerIdx].Name, err)
					}
					continue
				}
				rec.addAuthzGrant(grantee, bankSendMsgType)
				log.Printf("  %s granted authz to %s", rec.Name, af.Accounts[peerIdx].Name)
			}
		}

		// 7) Optional feegrants to 1..2 extra peers.
		if len(extraIdx) > 1 && localRng.Intn(100) < 45 {
			nTargets := 1 + localRng.Intn(minInt(2, len(extraIdx)-1))
			for _, peerIdx := range pickRandomLegacyIndices(extraIdx, idx, nTargets, localRng) {
				grantee := af.Accounts[peerIdx].Address
				spendLimit := fmt.Sprintf("%dulume", 150_000+localRng.Intn(300_000))
				if ok, err := queryFeegrantAllowanceExists(rec.Address, grantee); err == nil && ok {
					rec.addFeegrant(grantee, spendLimit)
					log.Printf("  %s feegrant to %s already exists, reusing existing state", rec.Name, af.Accounts[peerIdx].Name)
					continue
				}
				_, err := runTx("tx", "feegrant", "grant", rec.Address, grantee,
					"--spend-limit", spendLimit,
					"--from", rec.Name)
				if err != nil {
					if isPrepareRerunConflict(err) {
						rec.addFeegrant(grantee, spendLimit)
						log.Printf("  %s feegrant already exists, reusing existing state", rec.Name)
					} else {
						log.Printf("  WARN: extra feegrant %s -> %s: %v", rec.Name, af.Accounts[peerIdx].Name, err)
					}
					continue
				}
				rec.addFeegrant(grantee, spendLimit)
				log.Printf("  %s granted feegrant to %s", rec.Name, af.Accounts[peerIdx].Name)
			}
		}
	})

	// Phase 4: Claim activity — exercise the x/claim module with pre-seeded Pastel keypairs.
	// Each legacy account with balance gets 1-2 claims from the pool.
	// ~70% instant (tier 0), ~30% delayed (tiers 1/2/3).
	// When running in parallel across validators, each validator starts from a
	// different offset in the key pool so they don't all compete for the same
	// early indices (which contain the delayed claim slots at 3, 6, 9, ...).
	log.Println("--- Creating claim activity ---")
	if err := verifyClaimKeyIntegrity(); err != nil {
		log.Printf("  WARN: claim key integrity check failed: %v; skipping claim activity", err)
	} else {
		claimKeyIdx := claimKeyStartOffset(accountTag)
		skippedClaimKeysOwnedByOther := 0
		log.Printf("  claim key start offset: %d (tag=%q)", claimKeyIdx, accountTag)
		for ordinal, idx := range legacyIdx {
			rec := &af.Accounts[idx]
			if !rec.HasBalance || claimKeyIdx >= len(preseededClaimKeysByIndex) {
				continue
			}
			if !ensureSenderAccountReady(rec) {
				continue
			}

			// Each legacy account claims 1-2 keys (2 claims for every 3rd account).
			nClaims := 1
			if ordinal%3 == 0 && claimKeyIdx+1 < len(preseededClaimKeysByIndex) {
				nClaims = 2
			}

			for c := 0; c < nClaims && claimKeyIdx < len(preseededClaimKeysByIndex); c++ {
				entry := preseededClaimKeysByIndex[claimKeyIdx]

				// Check if already claimed (rerun support).
				if claimed, destAddr, existingTier, err := queryClaimRecord(entry.OldAddress); err == nil && claimed {
					if destAddr != "" && destAddr != rec.Address {
						skippedClaimKeysOwnedByOther++
						claimKeyIdx++
						c--
						continue
					}
					rec.addClaim(entry.OldAddress, fmt.Sprintf("%dulume", entry.Amount), existingTier, existingTier > 0, claimKeyIdx)
					log.Printf("  %s: claim key %d (%s) already claimed, reusing", rec.Name, claimKeyIdx, entry.OldAddress)
					claimKeyIdx++
					continue
				}

				sig, err := signClaimMessage(entry, rec.Address)
				if err != nil {
					log.Printf("  WARN: sign claim for %s key %d: %v", rec.Name, claimKeyIdx, err)
					claimKeyIdx++
					continue
				}

				// Decide claim type: ~70% instant, ~10% tier 1, ~10% tier 2, ~10% tier 3.
				// Keep delayed entries early in the sequence so low-volume runs still exercise delayed claims.
				tier, delayed := selectPrepareClaimForAccount(rec, claimKeyIdx)
				if plannedTier, plannedDelayed := plannedPrepareClaim(claimKeyIdx); plannedDelayed && (!delayed || tier != plannedTier) {
					log.Printf("  %s already has action activity; forcing instant claim for key %d to avoid turning an upload account into a vesting account", rec.Name, claimKeyIdx)
				}

				amountStr := fmt.Sprintf("%dulume", entry.Amount)
				if delayed {
					_, err = runTx("tx", "claim", "delayed-claim",
						entry.OldAddress, rec.Address, entry.PubKeyHex, sig,
						fmt.Sprintf("%d", tier),
						"--from", rec.Name)
				} else {
					_, err = runTx("tx", "claim", "claim",
						entry.OldAddress, rec.Address, entry.PubKeyHex, sig,
						"--from", rec.Name)
				}
				if err != nil {
					if isPrepareRerunConflict(err) {
						existingTier := tier
						if claimed, destAddr, onChainTier, qErr := queryClaimRecord(entry.OldAddress); qErr == nil && claimed {
							if destAddr != "" && destAddr != rec.Address {
								skippedClaimKeysOwnedByOther++
								claimKeyIdx++
								c--
								continue
							}
							existingTier = onChainTier
						}
						rec.addClaim(entry.OldAddress, amountStr, existingTier, existingTier > 0, claimKeyIdx)
						log.Printf("  %s: claim key %d already claimed (rerun), reusing", rec.Name, claimKeyIdx)
					} else {
						log.Printf("  WARN: claim %s key %d: %v", rec.Name, claimKeyIdx, err)
					}
				} else {
					rec.addClaim(entry.OldAddress, amountStr, tier, delayed, claimKeyIdx)
					claimType := "instant"
					if delayed {
						claimType = fmt.Sprintf("delayed(tier=%d)", tier)
					}
					log.Printf("  %s claimed %s from %s (%s)", rec.Name, amountStr, entry.OldAddress, claimType)
				}
				claimKeyIdx++
			}
		}
		log.Printf("  used %d/%d claim keys", claimKeyIdx, len(preseededClaimKeysByIndex))
		if skippedClaimKeysOwnedByOther > 0 {
			log.Printf("  claim keys already claimed by other addresses skipped: %d", skippedClaimKeysOwnedByOther)
		}
	}

	// Validate prepared scenarios against chain state and fail if critical coverage is missing.
	for i := range af.Accounts {
		af.Accounts[i].normalizeActivityTracking()
	}
	log.Println("--- Validating prepared state ---")
	if errCount := validatePreparedState(af); errCount > 0 {
		log.Fatalf("prepare validation failed: %d errors", errCount)
	}

	// Save accounts file.
	for i := range af.Accounts {
		af.Accounts[i].normalizeActivityTracking()
	}
	saveAccounts(*flagFile, af)
	log.Printf("=== PREPARE COMPLETE: %d accounts saved to %s ===", len(af.Accounts), *flagFile)

	// Print summary.
	var nLegacy, nExtra, nDelegated, nUnbonding, nRedelegation, nWithdraw, nAuthz, nAuthzRecv, nFeegrant, nFeegrantRecv int
	var nClaim, nDelayedClaim, nAction int
	for _, rec := range af.Accounts {
		if rec.IsLegacy {
			nLegacy++
		} else {
			nExtra++
		}
		if rec.HasDelegation {
			nDelegated++
		}
		if rec.HasUnbonding {
			nUnbonding++
		}
		if rec.HasRedelegation {
			nRedelegation++
		}
		if rec.HasThirdPartyWD {
			nWithdraw++
		}
		if rec.HasAuthzGrant {
			nAuthz++
		}
		if rec.HasAuthzAsGrantee {
			nAuthzRecv++
		}
		if rec.HasFeegrant {
			nFeegrant++
		}
		if rec.HasFeegrantGrantee {
			nFeegrantRecv++
		}
		for _, cl := range rec.Claims {
			if cl.Delayed {
				nDelayedClaim++
			} else {
				nClaim++
			}
		}
		nAction += len(rec.Actions)
	}
	log.Printf(
		"  prepare_activity_summary:\n"+
			"    legacy_accounts: %d\n"+
			"    extra_accounts: %d\n"+
			"    delegated_accounts: %d\n"+
			"    unbonding_accounts: %d\n"+
			"    redelegation_accounts: %d\n"+
			"    third_party_withdraw_accounts: %d\n"+
			"    authz_granter_accounts: %d\n"+
			"    authz_grantee_accounts: %d\n"+
			"    feegrant_granter_accounts: %d\n"+
			"    feegrant_grantee_accounts: %d\n"+
			"    instant_claims: %d\n"+
			"    delayed_claims: %d\n"+
			"    actions: %d",
		nLegacy, nExtra, nDelegated, nUnbonding, nRedelegation, nWithdraw,
		nAuthz, nAuthzRecv, nFeegrant, nFeegrantRecv, nClaim, nDelayedClaim, nAction,
	)
}

func buildPreparedAccountName(prefix, tag string, idx int) string {
	if tag == "" {
		return fmt.Sprintf("%s-%03d", prefix, idx)
	}
	return fmt.Sprintf("%s-%s-%03d", prefix, tag, idx)
}

func batchedFundingWaitTimeout(accountCount int) time.Duration {
	if accountCount < 1 {
		accountCount = 1
	}
	timeout := 45*time.Second + time.Duration(accountCount)*5*time.Second
	if timeout > 3*time.Minute {
		return 3 * time.Minute
	}
	return timeout
}

func plannedPrepareClaim(claimKeyIdx int) (tier uint32, delayed bool) {
	switch claimKeyIdx % 10 {
	case 3:
		return 1, true
	case 6:
		return 2, true
	case 9:
		return 3, true
	default:
		return 0, false
	}
}

func selectPrepareClaimForAccount(rec *AccountRecord, claimKeyIdx int) (tier uint32, delayed bool) {
	tier, delayed = plannedPrepareClaim(claimKeyIdx)
	if delayed && rec != nil && rec.hasRecordedAction() {
		return 0, false
	}
	return tier, delayed
}

// claimKeyStartOffset returns a starting index into the pre-seeded claim key
// pool based on the validator account tag (e.g. "val1" → 0, "val2" → 20, ...).
// This ensures parallel validators don't all compete for the same early indices
// and each validator's slice of keys contains delayed claim slots (at offsets
// 3, 6, 9 within each 10-key block).
func claimKeyStartOffset(accountTag string) int {
	const keysPerValidator = 20
	m := regexp.MustCompile(`(\d+)`).FindString(accountTag)
	if m == "" {
		return 0
	}
	n, err := strconv.Atoi(m)
	if err != nil || n < 1 {
		return 0
	}
	offset := (n - 1) * keysPerValidator
	if offset >= len(preseededClaimKeysByIndex) {
		return 0
	}
	return offset
}

func buildPreparedAccountNameV0(prefix, tag string, idx int) string {
	if tag == "" {
		return fmt.Sprintf("%s_%03d", prefix, idx)
	}
	return fmt.Sprintf("%s_%s_%03d", prefix, tag, idx)
}

func findPreparedAccountIndex(existingByName map[string]int, prefix, tag string, idx int) (int, bool) {
	candidates := []string{buildPreparedAccountName(prefix, tag, idx)}
	switch prefix {
	case legacyPreparedAccountPrefix:
		candidates = append(candidates,
			buildPreparedAccountNameV0(legacyPreparedAccountPrefixV0, tag, idx),
			buildPreparedAccountNameV0("legacy", tag, idx),
		)
	case extraPreparedAccountPrefix:
		candidates = append(candidates,
			buildPreparedAccountNameV0(extraPreparedAccountPrefixV0, tag, idx),
			buildPreparedAccountNameV0("extra", tag, idx),
		)
	}

	for _, name := range candidates {
		if recIdx, ok := existingByName[name]; ok {
			return recIdx, true
		}
	}
	return 0, false
}

func resolvePrepareAccountTag(explicitTag, funderKeyName, funderAddr string) string {
	if tag := sanitizePrepareAccountTag(explicitTag); tag != "" {
		return tag
	}

	// Typical devnet funder key names look like "supernova_validator_3_key".
	if m := regexp.MustCompile(`(?i)validator[_-]?(\d+)`).FindStringSubmatch(funderKeyName); len(m) == 2 {
		return fmt.Sprintf("val%s", m[1])
	}

	// Fallback: derive a short stable suffix from funder address.
	if funderAddr != "" {
		addr := strings.ToLower(funderAddr)
		if len(addr) > 6 {
			addr = addr[len(addr)-6:]
		}
		return sanitizePrepareAccountTag("acc" + addr)
	}

	return ""
}

func sanitizePrepareAccountTag(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if tag == "" {
		return ""
	}

	var b strings.Builder
	for _, r := range tag {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	return b.String()
}

func ensureSenderAccountReady(rec *AccountRecord) bool {
	addr, err := getAddress(rec.Name)
	if err != nil {
		rec.HasBalance = false
		log.Printf("  WARN: sender key %s not found in keyring: %v", rec.Name, err)
		return false
	}
	if rec.Address != addr {
		log.Printf("  WARN: account/keyring mismatch for %s: file=%s keyring=%s; using keyring address", rec.Name, rec.Address, addr)
		rec.Address = addr
	}
	bal, err := queryBalance(rec.Address)
	if err != nil || bal == 0 {
		rec.HasBalance = false
		log.Printf("  WARN: sender %s (%s) has no spendable balance; skipping activity", rec.Name, rec.Address)
		return false
	}
	return true
}

func reconcileAccountsWithKeyring(af *AccountsFile) {
	log.Println("--- Reconciling account keys with keyring ---")
	addressUpdates := make(map[string]string)
	for i := range af.Accounts {
		rec := &af.Accounts[i]
		if rec.Name == "" {
			continue
		}
		originalAddr := rec.Address

		expectedAddr := rec.Address
		if rec.Mnemonic != "" {
			if derivedAddr, err := deriveAddressFromMnemonic(rec.Mnemonic, rec.IsLegacy); err == nil {
				expectedAddr = derivedAddr
				if rec.Address != derivedAddr {
					log.Printf("  WARN: %s file address differs from mnemonic-derived address: file=%s mnemonic=%s; updating file",
						rec.Name, rec.Address, derivedAddr)
					rec.Address = derivedAddr
				}
			} else {
				log.Printf("  WARN: derive mnemonic address for %s failed: %v", rec.Name, err)
			}
		}

		keyAddr, err := getAddress(rec.Name)
		if err != nil {
			if rec.Mnemonic == "" {
				log.Printf("  WARN: key %s missing and mnemonic unavailable; cannot recover", rec.Name)
				rec.HasBalance = false
				continue
			}
			if impErr := importKey(rec.Name, rec.Mnemonic, rec.IsLegacy); impErr != nil {
				log.Printf("  WARN: recover key %s from mnemonic failed: %v", rec.Name, impErr)
				rec.HasBalance = false
				continue
			}
			keyAddr, err = getAddress(rec.Name)
			if err != nil {
				log.Printf("  WARN: key %s still unavailable after recovery: %v", rec.Name, err)
				rec.HasBalance = false
				continue
			}
			log.Printf("  restored key %s (%s)", rec.Name, keyAddr)
		}

		if rec.Mnemonic != "" && expectedAddr != "" && keyAddr != expectedAddr {
			reimportCoinType := uint32(118)
			if !rec.IsLegacy {
				reimportCoinType = nonLegacyCoinType
			}
			log.Printf("  WARN: key %s address (%s) differs from expected (%s); reimporting with coin-type=%v",
				rec.Name, keyAddr, expectedAddr, reimportCoinType)
			if err := deleteKey(rec.Name); err != nil {
				log.Printf("  WARN: delete key %s before reimport failed: %v", rec.Name, err)
			}
			if err := importKey(rec.Name, rec.Mnemonic, rec.IsLegacy); err != nil {
				log.Printf("  WARN: reimport key %s failed: %v", rec.Name, err)
			}
			if addr2, err2 := getAddress(rec.Name); err2 == nil {
				keyAddr = addr2
			} else {
				log.Printf("  WARN: read key %s after reimport failed: %v", rec.Name, err2)
			}
		}

		if keyAddr != rec.Address {
			log.Printf("  WARN: account/keyring mismatch for %s during reconcile: file=%s keyring=%s; using keyring address",
				rec.Name, rec.Address, keyAddr)
			rec.Address = keyAddr
		}
		if originalAddr != "" && rec.Address != "" && originalAddr != rec.Address {
			addressUpdates[originalAddr] = rec.Address
		}

		// Force balance state to be recomputed/funded for the reconciled address.
		rec.HasBalance = false
	}

	if len(addressUpdates) == 0 {
		return
	}

	for i := range af.Accounts {
		rec := &af.Accounts[i]

		if mapped, ok := addressUpdates[rec.WithdrawAddress]; ok {
			rec.WithdrawAddress = mapped
		}
		if mapped, ok := addressUpdates[rec.AuthzGrantedTo]; ok {
			rec.AuthzGrantedTo = mapped
		}
		if mapped, ok := addressUpdates[rec.AuthzReceivedFrom]; ok {
			rec.AuthzReceivedFrom = mapped
		}
		if mapped, ok := addressUpdates[rec.FeegrantGrantedTo]; ok {
			rec.FeegrantGrantedTo = mapped
		}
		if mapped, ok := addressUpdates[rec.FeegrantFrom]; ok {
			rec.FeegrantFrom = mapped
		}

		for j := range rec.WithdrawAddresses {
			if mapped, ok := addressUpdates[rec.WithdrawAddresses[j].Address]; ok {
				rec.WithdrawAddresses[j].Address = mapped
			}
		}
		for j := range rec.AuthzGrants {
			if mapped, ok := addressUpdates[rec.AuthzGrants[j].Grantee]; ok {
				rec.AuthzGrants[j].Grantee = mapped
			}
		}
		for j := range rec.AuthzAsGrantee {
			if mapped, ok := addressUpdates[rec.AuthzAsGrantee[j].Granter]; ok {
				rec.AuthzAsGrantee[j].Granter = mapped
			}
		}
		for j := range rec.Feegrants {
			if mapped, ok := addressUpdates[rec.Feegrants[j].Grantee]; ok {
				rec.Feegrants[j].Grantee = mapped
			}
		}
		for j := range rec.FeegrantsReceived {
			if mapped, ok := addressUpdates[rec.FeegrantsReceived[j].Granter]; ok {
				rec.FeegrantsReceived[j].Granter = mapped
			}
		}

		rec.refreshLegacyFields()
	}
}

func isPrepareRerunConflict(err error) bool {
	if err == nil {
		return false
	}
	low := strings.ToLower(err.Error())
	return strings.Contains(low, "already exists") ||
		strings.Contains(low, "already in progress") ||
		strings.Contains(low, "fee allowance already exists") ||
		strings.Contains(low, "authorization already exists") ||
		strings.Contains(low, "claim already claimed") ||
		strings.Contains(low, "code=1105")
}

// runParallel processes indices in parallel batches of the given size.
// The callback receives (ordinal, idx) where ordinal is the position in the
// indices slice and idx is the value (e.g. index into af.Accounts).
func runParallel(indices []int, batchSize int, fn func(ordinal, idx int)) {
	for pos := 0; pos < len(indices); pos += batchSize {
		end := pos + batchSize
		if end > len(indices) {
			end = len(indices)
		}
		var wg sync.WaitGroup
		for i := pos; i < end; i++ {
			wg.Add(1)
			go func(ordinal, idx int) {
				defer wg.Done()
				fn(ordinal, idx)
			}(i, indices[i])
		}
		wg.Wait()
	}
}

func pickDifferentValidator(validators []string, current string, rng *rand.Rand) (string, bool) {
	if len(validators) < 2 {
		return "", false
	}
	start := rng.Intn(len(validators))
	for i := 0; i < len(validators); i++ {
		candidate := validators[(start+i)%len(validators)]
		if candidate != current {
			return candidate, true
		}
	}
	return "", false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func pickRandomValidators(validators []string, n int, rng *rand.Rand) []string {
	if n <= 0 || len(validators) == 0 {
		return nil
	}
	if n > len(validators) {
		n = len(validators)
	}
	order := rng.Perm(len(validators))
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, validators[order[i]])
	}
	return out
}

func pickRandomLegacyIndices(legacyIdx []int, selfIdx int, n int, rng *rand.Rand) []int {
	if n <= 0 {
		return nil
	}
	candidates := make([]int, 0, len(legacyIdx))
	for _, idx := range legacyIdx {
		if idx == selfIdx {
			continue
		}
		candidates = append(candidates, idx)
	}
	if len(candidates) == 0 {
		return nil
	}
	if n > len(candidates) {
		n = len(candidates)
	}
	order := rng.Perm(len(candidates))
	out := make([]int, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, candidates[order[i]])
	}
	return out
}

func fundAccountsBatched(af *AccountsFile, rng *rand.Rand) error {
	ctx := context.Background()
	funderAddr, err := getAddress(*flagFunder)
	if err != nil {
		return fmt.Errorf("get funder address: %w", err)
	}
	sdkClient, err := sdkKeyringClient(ctx, *flagFunder, funderAddr)
	if err != nil {
		return fmt.Errorf("create SDK client for %s: %w", *flagFunder, err)
	}
	defer sdkClient.Close()
	accountNumber, sequence, err := queryAccountNumberAndSequence(funderAddr)
	if err != nil {
		return fmt.Errorf("query funder account number/sequence: %w", err)
	}

	log.Printf("  batched mode: funder account_number=%d start_sequence=%d", accountNumber, sequence)

	type pendingFund struct {
		idx    int
		amount string
		seq    uint64
	}
	waitTimeout := batchedFundingWaitTimeout(len(af.Accounts))
	var lastTxHash string
	pending := make([]pendingFund, 0, len(af.Accounts))
	for i := range af.Accounts {
		rec := &af.Accounts[i]
		amount := fmt.Sprintf("%dulume", 10_000_000+rng.Intn(10_000_000))
		accNum := accountNumber
		seq := sequence

		txHash, err := sdkSendBankTx(ctx, sdkClient.Blockchain, funderAddr, rec.Address, amount, &accNum, &seq)
		if err != nil {
			// Settle any accepted txs before the caller falls back to sequential mode.
			if lastTxHash != "" {
				_ = waitForSDKTxResult(ctx, sdkClient.Blockchain, lastTxHash, waitTimeout)
			}
			return fmt.Errorf("fund %s at sequence %d failed: %w", rec.Name, sequence, err)
		}

		pending = append(pending, pendingFund{idx: i, amount: amount, seq: sequence})
		sequence++
		if txHash != "" {
			lastTxHash = txHash
		}
		log.Printf("  accepted funding tx for %s with %s (seq=%d)", rec.Name, amount, sequence-1)
	}

	if len(pending) == 0 {
		return fmt.Errorf("no funding txs accepted")
	}
	if lastTxHash != "" {
		if err := waitForSDKTxResult(ctx, sdkClient.Blockchain, lastTxHash, waitTimeout); err != nil {
			return fmt.Errorf("wait for last funding tx %s: %w", lastTxHash, err)
		}
	}

	// Verify balances on-chain — some txs may have passed CheckTx but failed in DeliverTx.
	var funded int
	for _, p := range pending {
		rec := &af.Accounts[p.idx]
		bal, err := queryBalance(rec.Address)
		if err != nil || bal == 0 {
			log.Printf("  WARN: %s has no on-chain balance (funding tx may have failed), marking unfunded", rec.Name)
		} else {
			rec.HasBalance = true
			funded++
			log.Printf("  funded %s with %s (seq=%d)", rec.Name, p.amount, p.seq)
		}
	}
	log.Printf("  batched funding verified: %d/%d accounts funded on-chain", funded, len(pending))
	if funded == 0 {
		return fmt.Errorf("no accounts funded on-chain despite %d txs accepted", len(pending))
	}
	return nil
}

func fundAccountsSequential(af *AccountsFile, rng *rand.Rand) {
	ctx := context.Background()
	funderAddr, err := getAddress(*flagFunder)
	if err != nil {
		log.Printf("  WARN: get funder address: %v", err)
		return
	}
	sdkClient, err := sdkKeyringClient(ctx, *flagFunder, funderAddr)
	if err != nil {
		log.Printf("  WARN: create SDK client for %s: %v", *flagFunder, err)
		return
	}
	defer sdkClient.Close()

	for i := range af.Accounts {
		rec := &af.Accounts[i]
		if rec.HasBalance {
			continue
		}
		amount := fmt.Sprintf("%dulume", 10_000_000+rng.Intn(10_000_000))
		txHash, err := sdkSendBankTx(ctx, sdkClient.Blockchain, funderAddr, rec.Address, amount, nil, nil)
		if err != nil {
			low := strings.ToLower(err.Error())
			if strings.Contains(low, "incorrect account sequence") {
				_ = waitForNextBlock(20 * time.Second)
				txHash, err = sdkSendBankTx(ctx, sdkClient.Blockchain, funderAddr, rec.Address, amount, nil, nil)
			}
		}
		if err == nil {
			err = waitForSDKTxResult(ctx, sdkClient.Blockchain, txHash, 45*time.Second)
		}
		if err != nil {
			log.Printf("  WARN: fund %s: %v", rec.Name, err)
			continue
		}
		rec.HasBalance = true
		log.Printf("  funded %s with %s", rec.Name, amount)
	}
}

func validatePreparedState(af *AccountsFile) int {
	var errCount int
	var legacyWithBalance int
	var scenarioUnbonding, scenarioRedelegation, scenarioWithdraw, scenarioAuthzAsGrantee, scenarioFeegrantAsGrantee int
	var scenarioClaim, scenarioDelayedClaim, scenarioAction int

	for i := range af.Accounts {
		rec := &af.Accounts[i]
		rec.normalizeActivityTracking()
		if !rec.IsLegacy || !rec.HasBalance {
			continue
		}
		legacyWithBalance++

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
				n, err := queryDelegationToValidatorCount(rec.Address, d.Validator)
				if err != nil {
					log.Printf("  ERROR: query delegation %s -> %s: %v", rec.Name, d.Validator, err)
					errCount++
				} else if n == 0 {
					log.Printf("  ERROR: expected delegation %s -> %s", rec.Name, d.Validator)
					errCount++
				}
			}
		} else if rec.HasDelegation {
			n, err := queryDelegationCount(rec.Address)
			if err != nil {
				log.Printf("  ERROR: query delegations %s: %v", rec.Name, err)
				errCount++
			} else if n == 0 {
				log.Printf("  ERROR: expected delegations for %s, got 0", rec.Name)
				errCount++
			}
		}

		if len(rec.Unbondings) > 0 {
			scenarioUnbonding++
			seen := make(map[string]struct{}, len(rec.Unbondings))
			for _, u := range rec.Unbondings {
				if u.Validator == "" {
					continue
				}
				if _, ok := seen[u.Validator]; ok {
					continue
				}
				seen[u.Validator] = struct{}{}
				n, err := queryUnbondingFromValidatorCount(rec.Address, u.Validator)
				if err != nil {
					log.Printf("  ERROR: query unbonding %s from %s: %v", rec.Name, u.Validator, err)
					errCount++
				} else if n == 0 {
					// Older reruns could persist synthetic legacy fallback entries with empty amount.
					// If any unbonding exists for the account, treat this stale per-validator record as reconciled.
					if u.Amount == "" {
						if anyN, anyErr := queryUnbondingCount(rec.Address); anyErr == nil && anyN > 0 {
							log.Printf("  INFO: stale unbonding marker %s from %s; account has %d unbonding entries, keeping run green",
								rec.Name, u.Validator, anyN)
							continue
						}
					}
					log.Printf("  ERROR: expected unbonding %s from %s", rec.Name, u.Validator)
					errCount++
				}
			}
		} else if rec.HasUnbonding {
			scenarioUnbonding++
			n, err := queryUnbondingCount(rec.Address)
			if err != nil {
				log.Printf("  ERROR: query unbonding %s: %v", rec.Name, err)
				errCount++
			} else if n == 0 {
				log.Printf("  ERROR: expected unbonding entries for %s, got 0", rec.Name)
				errCount++
			}
		}

		if len(rec.Redelegations) > 0 {
			scenarioRedelegation++
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
				n, err := queryRedelegationCount(rec.Address, rd.SrcValidator, rd.DstValidator)
				if err != nil {
					log.Printf("  ERROR: query redelegation %s %s -> %s: %v", rec.Name, rd.SrcValidator, rd.DstValidator, err)
					errCount++
				} else if n == 0 {
					// Older reruns could persist synthetic legacy fallback entries with empty amount.
					// If any redelegation exists for the account, treat this stale pair as reconciled.
					if rd.Amount == "" {
						if anyN, anyErr := queryAnyRedelegationCount(rec.Address, af.Validators); anyErr == nil && anyN > 0 {
							log.Printf("  INFO: stale redelegation marker %s %s -> %s; account has %d redelegations, keeping run green",
								rec.Name, rd.SrcValidator, rd.DstValidator, anyN)
							continue
						}
					}
					log.Printf("  ERROR: expected redelegation %s %s -> %s", rec.Name, rd.SrcValidator, rd.DstValidator)
					errCount++
				}
			}
		} else if rec.HasRedelegation {
			scenarioRedelegation++
			n, err := queryRedelegationCount(rec.Address, rec.DelegatedTo, rec.RedelegatedTo)
			if err == nil && n == 0 {
				n, err = queryAnyRedelegationCount(rec.Address, af.Validators)
			}
			if err != nil {
				log.Printf("  ERROR: query redelegation %s: %v", rec.Name, err)
				errCount++
			} else if n == 0 {
				log.Printf("  ERROR: expected redelegation entries for %s, got 0", rec.Name)
				errCount++
			}
		}

		if len(rec.WithdrawAddresses) > 0 || rec.HasThirdPartyWD {
			scenarioWithdraw++
			addr, err := queryWithdrawAddress(rec.Address)
			if err != nil {
				log.Printf("  ERROR: query withdraw addr %s: %v", rec.Name, err)
				errCount++
			} else if addr == "" || addr == rec.Address {
				log.Printf("  ERROR: expected third-party withdraw addr for %s, got %s", rec.Name, addr)
				errCount++
			} else {
				expected := rec.WithdrawAddress
				if n := len(rec.WithdrawAddresses); n > 0 {
					expected = rec.WithdrawAddresses[n-1].Address
				}
				if expected != "" && addr != expected {
					// Reruns can legitimately rotate the withdraw address. Reconcile with chain state.
					log.Printf("  INFO: withdraw addr changed for %s: expected %s got %s; updating record", rec.Name, expected, addr)
					rec.addWithdrawAddress(addr)
				} else if expected == "" {
					rec.addWithdrawAddress(addr)
				}
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
				ok, err := queryAuthzGrantExists(rec.Address, g.Grantee)
				if err != nil {
					log.Printf("  ERROR: query authz grant %s -> %s: %v", rec.Name, g.Grantee, err)
					errCount++
				} else if !ok {
					log.Printf("  ERROR: expected authz grant %s -> %s", rec.Name, g.Grantee)
					errCount++
				}
			}
		} else if rec.HasAuthzGrant && rec.AuthzGrantedTo != "" {
			ok, err := queryAuthzGrantExists(rec.Address, rec.AuthzGrantedTo)
			if err != nil {
				log.Printf("  ERROR: query authz grant %s -> %s: %v", rec.Name, rec.AuthzGrantedTo, err)
				errCount++
			} else if !ok {
				log.Printf("  ERROR: expected authz grant %s -> %s", rec.Name, rec.AuthzGrantedTo)
				errCount++
			}
		}

		if len(rec.AuthzAsGrantee) > 0 {
			scenarioAuthzAsGrantee++
			seen := make(map[string]struct{}, len(rec.AuthzAsGrantee))
			for _, g := range rec.AuthzAsGrantee {
				if g.Granter == "" {
					continue
				}
				if _, ok := seen[g.Granter]; ok {
					continue
				}
				seen[g.Granter] = struct{}{}
				ok, err := queryAuthzGrantExists(g.Granter, rec.Address)
				if err != nil {
					log.Printf("  ERROR: query authz grant %s -> %s: %v", g.Granter, rec.Name, err)
					errCount++
				} else if !ok {
					log.Printf("  ERROR: expected authz grant %s -> %s", g.Granter, rec.Name)
					errCount++
				}
			}
		} else if rec.HasAuthzAsGrantee && rec.AuthzReceivedFrom != "" {
			scenarioAuthzAsGrantee++
			ok, err := queryAuthzGrantExists(rec.AuthzReceivedFrom, rec.Address)
			if err != nil {
				log.Printf("  ERROR: query authz grant %s -> %s: %v", rec.AuthzReceivedFrom, rec.Name, err)
				errCount++
			} else if !ok {
				log.Printf("  ERROR: expected authz grant %s -> %s", rec.AuthzReceivedFrom, rec.Name)
				errCount++
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
				ok, err := queryFeegrantAllowanceExists(rec.Address, g.Grantee)
				if err != nil {
					log.Printf("  ERROR: query feegrant %s -> %s: %v", rec.Name, g.Grantee, err)
					errCount++
				} else if !ok {
					log.Printf("  ERROR: expected feegrant allowance %s -> %s", rec.Name, g.Grantee)
					errCount++
				}
			}
		} else if rec.HasFeegrant && rec.FeegrantGrantedTo != "" {
			ok, err := queryFeegrantAllowanceExists(rec.Address, rec.FeegrantGrantedTo)
			if err != nil {
				log.Printf("  ERROR: query feegrant %s -> %s: %v", rec.Name, rec.FeegrantGrantedTo, err)
				errCount++
			} else if !ok {
				log.Printf("  ERROR: expected feegrant allowance %s -> %s", rec.Name, rec.FeegrantGrantedTo)
				errCount++
			}
		}

		if len(rec.FeegrantsReceived) > 0 {
			scenarioFeegrantAsGrantee++
			seen := make(map[string]struct{}, len(rec.FeegrantsReceived))
			for _, g := range rec.FeegrantsReceived {
				if g.Granter == "" {
					continue
				}
				if _, ok := seen[g.Granter]; ok {
					continue
				}
				seen[g.Granter] = struct{}{}
				ok, err := queryFeegrantAllowanceExists(g.Granter, rec.Address)
				if err != nil {
					log.Printf("  ERROR: query feegrant %s -> %s: %v", g.Granter, rec.Name, err)
					errCount++
				} else if !ok {
					log.Printf("  ERROR: expected feegrant allowance %s -> %s", g.Granter, rec.Name)
					errCount++
				}
			}
		} else if rec.HasFeegrantGrantee && rec.FeegrantFrom != "" {
			scenarioFeegrantAsGrantee++
			ok, err := queryFeegrantAllowanceExists(rec.FeegrantFrom, rec.Address)
			if err != nil {
				log.Printf("  ERROR: query feegrant %s -> %s: %v", rec.FeegrantFrom, rec.Name, err)
				errCount++
			} else if !ok {
				log.Printf("  ERROR: expected feegrant allowance %s -> %s", rec.FeegrantFrom, rec.Name)
				errCount++
			}
		}

		// Validate action records.
		if len(rec.Actions) > 0 {
			scenarioAction++
			for _, act := range rec.Actions {
				if act.ActionID == "" {
					continue
				}
				creator, err := queryActionCreator(act.ActionID)
				if err != nil {
					log.Printf("  ERROR: query action %s for %s: %v", act.ActionID, rec.Name, err)
					errCount++
				} else if creator != rec.Address {
					log.Printf("  ERROR: action %s creator mismatch: expected %s got %s", act.ActionID, rec.Address, creator)
					errCount++
				}
			}
		}

		// Validate claim records.
		if len(rec.Claims) > 0 {
			for _, cl := range rec.Claims {
				claimed, destAddr, _, err := queryClaimRecord(cl.OldAddress)
				if err != nil {
					log.Printf("  ERROR: query claim record %s for %s: %v", cl.OldAddress, rec.Name, err)
					errCount++
					continue
				}
				if !claimed {
					log.Printf("  ERROR: claim record %s should be claimed for %s", cl.OldAddress, rec.Name)
					errCount++
				} else if destAddr != rec.Address {
					log.Printf("  ERROR: claim record %s dest=%s, expected %s", cl.OldAddress, destAddr, rec.Address)
					errCount++
				}
				if cl.Delayed {
					scenarioDelayedClaim++
				} else {
					scenarioClaim++
				}
			}
		}
	}

	// Coverage expectations: with enough legacy accounts, each scenario should exist at least once.
	if legacyWithBalance >= 4 && scenarioUnbonding == 0 {
		log.Printf("  ERROR: no legacy account with unbonding scenario created")
		errCount++
	}
	if legacyWithBalance >= 6 && len(af.Validators) > 1 && scenarioRedelegation == 0 {
		log.Printf("  ERROR: no legacy account with redelegation scenario created")
		errCount++
	}
	if legacyWithBalance >= 7 && scenarioWithdraw == 0 {
		log.Printf("  ERROR: no legacy account with third-party withdraw address created")
		errCount++
	}
	if legacyWithBalance >= 4 && scenarioAuthzAsGrantee == 0 {
		log.Printf("  ERROR: no legacy account exercised authz-as-grantee scenario")
		errCount++
	}
	if legacyWithBalance >= 6 && scenarioFeegrantAsGrantee == 0 {
		log.Printf("  ERROR: no legacy account exercised feegrant-as-grantee scenario")
		errCount++
	}
	if legacyWithBalance >= 4 && scenarioAction == 0 {
		log.Printf("  ERROR: no legacy account with action scenario created")
		errCount++
	}
	if legacyWithBalance >= 2 && scenarioClaim == 0 {
		log.Printf("  ERROR: no instant claim scenario exercised")
		errCount++
	}
	if legacyWithBalance >= 2 && scenarioDelayedClaim == 0 {
		// Reruns on old datasets may have only instant claims pre-created.
		// If chain state has no delayed claims at all, warn but don't fail prepare.
		hasDelayed, err := queryHasAnyDelayedClaim()
		if err != nil {
			log.Printf("  ERROR: query delayed-claim coverage: %v", err)
			errCount++
		} else if hasDelayed {
			log.Printf("  ERROR: no delayed claim scenario exercised")
			errCount++
		} else {
			log.Printf("  WARN: no delayed claim scenario exercised and chain has no delayed claims yet")
		}
	}

	return errCount
}

func runCleanup() {
	log.Println("=== CLEANUP MODE: removing test keys from keyring ===")

	keys, err := listKeys()
	if err != nil {
		log.Fatalf("list keys: %v", err)
	}

	removed := 0
	for _, k := range keys {
		if !isTestKeyName(k.Name) {
			continue
		}
		if err := deleteKey(k.Name); err != nil {
			log.Printf("  WARN: delete %s: %v", k.Name, err)
			continue
		}
		removed++
		log.Printf("  deleted %s", k.Name)
	}

	// Remove accounts file if it exists.
	if err := os.Remove(*flagFile); err != nil && !os.IsNotExist(err) {
		log.Printf("  WARN: remove %s: %v", *flagFile, err)
	} else if err == nil {
		log.Printf("  removed %s", *flagFile)
	}

	log.Printf("=== CLEANUP COMPLETE: %d keys removed ===", removed)
}

// isTestKeyName returns true for key names created by the evmigration test tool.
func isTestKeyName(name string) bool {
	return strings.HasPrefix(name, legacyPreparedAccountPrefix+"-") ||
		strings.HasPrefix(name, extraPreparedAccountPrefix+"-") ||
		strings.HasPrefix(name, migratedAccountPrefix+"-") ||
		strings.HasPrefix(name, migratedExtraAccountPrefix+"-") ||
		strings.HasPrefix(name, legacyPreparedAccountPrefixV0+"_") ||
		strings.HasPrefix(name, extraPreparedAccountPrefixV0+"_") ||
		strings.HasPrefix(name, "new_"+legacyPreparedAccountPrefixV0+"_") ||
		strings.HasPrefix(name, "new_"+extraPreparedAccountPrefixV0+"_") ||
		strings.HasPrefix(name, "legacy_") || // backward compatibility with old naming
		strings.HasPrefix(name, "extra_") || // backward compatibility with old naming
		strings.HasPrefix(name, "new_legacy_") || // backward compatibility with old naming
		strings.HasPrefix(name, "new_extra_") || // backward compatibility with old naming
		strings.HasPrefix(name, "new_supernova_") ||
		strings.HasPrefix(name, "new_validator")
}
