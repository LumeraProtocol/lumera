package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"gen/tests/common"
)

// fakeMigrationChain is a scripted migrationChain for unit-testing the executor.
type fakeMigrationChain struct {
	records     map[string]common.MigrationRecord // legacy addr -> record (present == migrated)
	estimate    common.MigrationEstimate
	estimateErr error
	keys        map[string]bool
	importAddr  map[string]string // key name -> address returned by import
	claimHash   string
	claimErr    error

	multisigResult common.MultisigProofResult
	multisigErr    error

	// call tracking
	claimCalls    int
	multisigCalls int
	importCalls   []string
}

func (f *fakeMigrationChain) MigrationRecord(addr string) (common.MigrationRecord, bool, error) {
	r, ok := f.records[addr]
	return r, ok, nil
}
func (f *fakeMigrationChain) MigrationEstimate(addr string) (common.MigrationEstimate, error) {
	return f.estimate, f.estimateErr
}
func (f *fakeMigrationChain) HasKey(name string) bool { return f.keys[name] }
func (f *fakeMigrationChain) ShowAddress(name string) (string, error) {
	if a, ok := f.importAddr[name]; ok {
		return a, nil
	}
	return "lumera1" + name, nil
}
func (f *fakeMigrationChain) ImportKeyWithStyle(name, mnemonic string, style common.KeyStyle) (string, error) {
	f.importCalls = append(f.importCalls, name)
	if f.keys == nil {
		f.keys = map[string]bool{}
	}
	f.keys[name] = true
	if a, ok := f.importAddr[name]; ok {
		return a, nil
	}
	return "lumera1" + name, nil
}
func (f *fakeMigrationChain) ClaimLegacyAccount(legacyKey, newKey string) (string, error) {
	f.claimCalls++
	if f.claimErr != nil {
		return "", f.claimErr
	}
	return f.claimHash, nil
}

func (f *fakeMigrationChain) MigrateMultisigProof(legacyBase, legacyAddr string, members []string, threshold, signers int) (common.MultisigProofResult, error) {
	f.multisigCalls++
	if f.multisigErr != nil {
		return common.MultisigProofResult{}, f.multisigErr
	}
	return f.multisigResult, nil
}

func singleSigItem() migrationWorkItem {
	rec := legacyRec("gen-0001", "lumera1legacy", "seed words here")
	return migrationWorkItem{Index: 1, Rec: rec, Kind: migrationKindSingleSig, AccountType: "regular", CorrelationID: "migrate gen-0001 #1"}
}

func newTestExecutor(chain migrationChain) (*migrationExecutor, *bytes.Buffer) {
	var buf bytes.Buffer
	lg := newMigrationLogger(&buf)
	lg.now = fixedClock()
	return &migrationExecutor{chain: chain, log: lg}, &buf
}

func TestMigrateOneSingleSigHappyPath(t *testing.T) {
	chain := &fakeMigrationChain{
		records:    map[string]common.MigrationRecord{},
		estimate:   common.MigrationEstimate{WouldSucceed: true, DelegationCount: 5, BalanceSummary: "100ulume"},
		keys:       map[string]bool{"gen-0001": true}, // legacy key already in keyring
		importAddr: map[string]string{"gen-0001-evm": "lumera1newevm"},
		claimHash:  "TXHASH123",
	}
	ex, _ := newTestExecutor(chain)

	res := ex.migrateOne(singleSigItem(), false)

	if res.Status != MigrationStatusMigrated {
		t.Fatalf("status = %q, want migrated (err=%v)", res.Status, res.Err)
	}
	if res.NewAddress != "lumera1newevm" {
		t.Errorf("NewAddress = %q, want lumera1newevm", res.NewAddress)
	}
	if res.TxHash != "TXHASH123" {
		t.Errorf("TxHash = %q, want TXHASH123", res.TxHash)
	}
	if chain.claimCalls != 1 {
		t.Errorf("claimCalls = %d, want 1", chain.claimCalls)
	}
}

func TestMigrateOneAlreadyMigrated(t *testing.T) {
	chain := &fakeMigrationChain{
		records: map[string]common.MigrationRecord{
			"lumera1legacy": {LegacyAddress: "lumera1legacy", NewAddress: "lumera1existing"},
		},
	}
	ex, _ := newTestExecutor(chain)

	res := ex.migrateOne(singleSigItem(), false)

	if res.Status != MigrationStatusAlreadyMigrated {
		t.Fatalf("status = %q, want already_migrated", res.Status)
	}
	if res.NewAddress != "lumera1existing" {
		t.Errorf("NewAddress = %q, want lumera1existing", res.NewAddress)
	}
	if chain.claimCalls != 0 {
		t.Errorf("claimCalls = %d, want 0 (no submit when already migrated)", chain.claimCalls)
	}
}

func TestMigrateOneSkippedWhenEstimateRejects(t *testing.T) {
	chain := &fakeMigrationChain{
		records:  map[string]common.MigrationRecord{},
		estimate: common.MigrationEstimate{WouldSucceed: false, RejectionReason: "blocked"},
	}
	ex, _ := newTestExecutor(chain)

	res := ex.migrateOne(singleSigItem(), false)

	if res.Status != MigrationStatusSkipped {
		t.Fatalf("status = %q, want skipped", res.Status)
	}
	if chain.claimCalls != 0 {
		t.Errorf("claimCalls = %d, want 0 when estimate rejects", chain.claimCalls)
	}
}

func TestMigrateOneDryRunDoesNotSubmit(t *testing.T) {
	chain := &fakeMigrationChain{
		records:  map[string]common.MigrationRecord{},
		estimate: common.MigrationEstimate{WouldSucceed: true},
		keys:     map[string]bool{"gen-0001": true},
	}
	ex, _ := newTestExecutor(chain)

	res := ex.migrateOne(singleSigItem(), true)

	if !res.Planned {
		t.Errorf("Planned = false, want true in dry-run")
	}
	if chain.claimCalls != 0 {
		t.Errorf("claimCalls = %d, want 0 in dry-run", chain.claimCalls)
	}
	if len(chain.importCalls) != 0 {
		t.Errorf("importCalls = %v, want none in dry-run", chain.importCalls)
	}
}

func multisigItem() migrationWorkItem {
	rec := &AccountRecord{AccountIdentity: common.AccountIdentity{Name: "gen-msig35-0001", Address: "lumera1ms", KeyStyle: "legacy"}}
	rec.Multisig = &MultisigInfo{
		MemberNames: []string{"gen-msig35-0001-signer-1", "gen-msig35-0001-signer-2", "gen-msig35-0001-signer-3", "gen-msig35-0001-signer-4", "gen-msig35-0001-signer-5"},
		Threshold:   3, Signers: 5,
	}
	return migrationWorkItem{Index: 1, Rec: rec, Kind: migrationKindMultisig, AccountType: "multisig-3-of-5", CorrelationID: "migrate gen-msig35-0001 #1"}
}

func TestMigrateOneMultisigHappyPath(t *testing.T) {
	chain := &fakeMigrationChain{
		records:  map[string]common.MigrationRecord{},
		estimate: common.MigrationEstimate{WouldSucceed: true, IsMultisig: true},
		keys: map[string]bool{ // all member keys already in keyring
			"gen-msig35-0001-signer-1": true, "gen-msig35-0001-signer-2": true,
			"gen-msig35-0001-signer-3": true, "gen-msig35-0001-signer-4": true, "gen-msig35-0001-signer-5": true,
		},
		multisigResult: common.MultisigProofResult{NewName: "evm-gen-msig35-0001", NewAddress: "lumera1newms", TxHash: "MSTX"},
	}
	ex, _ := newTestExecutor(chain)

	res := ex.migrateOne(multisigItem(), false)

	if res.Status != MigrationStatusMigrated {
		t.Fatalf("status = %q, want migrated (err=%v)", res.Status, res.Err)
	}
	if res.NewAddress != "lumera1newms" || res.TxHash != "MSTX" {
		t.Errorf("result = %+v, want new=lumera1newms tx=MSTX", res)
	}
	if chain.multisigCalls != 1 {
		t.Errorf("multisigCalls = %d, want 1", chain.multisigCalls)
	}
	if chain.claimCalls != 0 {
		t.Errorf("claimCalls = %d, want 0 (multisig path must not use single-sig claim)", chain.claimCalls)
	}
}

func TestMigrateOneMultisigDryRunDoesNotSubmit(t *testing.T) {
	chain := &fakeMigrationChain{
		records:  map[string]common.MigrationRecord{},
		estimate: common.MigrationEstimate{WouldSucceed: true, IsMultisig: true},
	}
	ex, _ := newTestExecutor(chain)

	res := ex.migrateOne(multisigItem(), true)
	if !res.Planned {
		t.Error("Planned = false, want true in dry-run")
	}
	if chain.multisigCalls != 0 {
		t.Errorf("multisigCalls = %d, want 0 in dry-run", chain.multisigCalls)
	}
}

func TestMigrateMultisigImportsMissingMembersFromMnemonic(t *testing.T) {
	item := multisigItem()
	// Attach member material with mnemonics; keyring starts empty.
	var members []MultisigMember
	for _, n := range item.Rec.Multisig.MemberNames {
		members = append(members, MultisigMember{Name: n, Mnemonic: "seed-" + n})
	}
	item.Rec.Multisig.Members = members

	chain := &fakeMigrationChain{
		records:        map[string]common.MigrationRecord{},
		estimate:       common.MigrationEstimate{WouldSucceed: true, IsMultisig: true},
		keys:           map[string]bool{}, // members NOT present -> must be imported
		multisigResult: common.MultisigProofResult{NewName: "evm-gen-msig35-0001", NewAddress: "lumera1newms", TxHash: "MSTX"},
	}
	ex, _ := newTestExecutor(chain)

	res := ex.migrateOne(item, false)

	if res.Status != MigrationStatusMigrated {
		t.Fatalf("status = %q, want migrated (err=%v)", res.Status, res.Err)
	}
	if chain.multisigCalls != 1 {
		t.Errorf("multisigCalls = %d, want 1", chain.multisigCalls)
	}
	// All five members should have been imported from their stored mnemonics.
	if len(chain.importCalls) != 5 {
		t.Errorf("importCalls = %v, want 5 member imports", chain.importCalls)
	}
}

func TestMigrateMultisigFailsFastWhenMembersMissingNoMnemonic(t *testing.T) {
	item := multisigItem() // MemberNames only, no Members mnemonics
	chain := &fakeMigrationChain{
		records:  map[string]common.MigrationRecord{},
		estimate: common.MigrationEstimate{WouldSucceed: true, IsMultisig: true},
		keys:     map[string]bool{}, // members absent and unrecoverable
	}
	ex, _ := newTestExecutor(chain)

	res := ex.migrateOne(item, false)

	if res.Status != MigrationStatusFailed {
		t.Fatalf("status = %q, want failed", res.Status)
	}
	if chain.multisigCalls != 0 {
		t.Errorf("multisigCalls = %d, want 0 (must fail before the ceremony / destination keys)", chain.multisigCalls)
	}
	if res.Err == nil || !strings.Contains(res.Err.Error(), "gen-msig35-0001-signer-1") {
		t.Errorf("error should name the missing member keys, got: %v", res.Err)
	}
}

func TestMigrateOneFailedClaim(t *testing.T) {
	chain := &fakeMigrationChain{
		records:  map[string]common.MigrationRecord{},
		estimate: common.MigrationEstimate{WouldSucceed: true},
		keys:     map[string]bool{"gen-0001": true},
		claimErr: fmt.Errorf("broadcast rejected"),
	}
	ex, _ := newTestExecutor(chain)

	res := ex.migrateOne(singleSigItem(), false)

	if res.Status != MigrationStatusFailed {
		t.Fatalf("status = %q, want failed", res.Status)
	}
	if res.Err == nil {
		t.Error("expected Err to be set on failed claim")
	}
}
