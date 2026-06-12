package main

import (
	"math/rand"
	"testing"

	"gen/tests/common"
)

func vestingRecs(n int) []*AccountRecord {
	recs := make([]*AccountRecord, n)
	for i := range recs {
		recs[i] = &AccountRecord{AccountIdentity: common.AccountIdentity{
			Name: "gen-acct", Address: "lumera1a",
		}}
	}
	return recs
}

func TestPlanVestingSelectsPercentage(t *testing.T) {
	recs := vestingRecs(10)
	rng := rand.New(rand.NewSource(1))
	selected := planVesting(recs, 30, "1000000ulume", rng, 1_700_000_000)
	if len(selected) != 3 { // floor(10 * 30 / 100)
		t.Fatalf("selected %d accounts, want 3", len(selected))
	}
	for _, rec := range selected {
		if rec.Vesting == nil {
			t.Fatalf("selected account %s has no Vesting info", rec.Name)
		}
		if rec.Vesting.Type != string(common.VestingContinuous) && rec.Vesting.Type != string(common.VestingDelayed) {
			t.Errorf("vesting type = %q, want continuous or delayed", rec.Vesting.Type)
		}
		if rec.Vesting.EndTime <= 1_700_000_000 {
			t.Errorf("end-time %d must be after now", rec.Vesting.EndTime)
		}
		if rec.Vesting.LockedAmount != "1000000ulume" {
			t.Errorf("locked amount = %q, want 1000000ulume", rec.Vesting.LockedAmount)
		}
	}
}

func TestPlanVestingZeroPercentSelectsNone(t *testing.T) {
	recs := vestingRecs(10)
	rng := rand.New(rand.NewSource(1))
	if selected := planVesting(recs, 0, "1000000ulume", rng, 1_700_000_000); len(selected) != 0 {
		t.Errorf("0%% selected %d accounts, want 0", len(selected))
	}
}

func TestNewPermanentLockedInfo(t *testing.T) {
	info := newPermanentLockedInfo("5000000ulume")
	if info.Type != string(common.VestingPermanentLocked) {
		t.Errorf("type = %q, want permanent_locked", info.Type)
	}
	if info.EndTime != 0 {
		t.Errorf("permanent-locked end-time = %d, want 0", info.EndTime)
	}
	if info.LockedAmount != "5000000ulume" {
		t.Errorf("locked amount = %q, want 5000000ulume", info.LockedAmount)
	}
}

func TestSplitFundingTargets(t *testing.T) {
	reg := NewRegistry("c", "f", "", "legacy", "t")
	reg.Accounts = []*AccountRecord{
		{AccountIdentity: common.AccountIdentity{Name: "regular"}},                                                // bank
		{AccountIdentity: common.AccountIdentity{Name: "msig"}, Multisig: &MultisigInfo{Threshold: 2}},            // bank (composite funded from funder)
		{AccountIdentity: common.AccountIdentity{Name: "vest"}, Vesting: &VestingInfo{Type: "continuous"}},        // vesting
		{AccountIdentity: common.AccountIdentity{Name: "plock"}, Vesting: &VestingInfo{Type: "permanent_locked"}}, // vesting
		{AccountIdentity: common.AccountIdentity{Name: "already"}, Funded: true},                                  // skipped
	}
	bank, vesting := splitFundingTargets(reg)

	names := func(recs []*AccountRecord) []string {
		var out []string
		for _, r := range recs {
			out = append(out, r.Name)
		}
		return out
	}
	gotBank := names(bank)
	if len(gotBank) != 2 || gotBank[0] != "regular" || gotBank[1] != "msig" {
		t.Errorf("bank targets = %v, want [regular msig]", gotBank)
	}
	gotVest := names(vesting)
	if len(gotVest) != 2 || gotVest[0] != "vest" || gotVest[1] != "plock" {
		t.Errorf("vesting targets = %v, want [vest plock]", gotVest)
	}
}
