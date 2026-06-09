package main

import (
	"testing"

	"gen/tests/common"
)

func TestPlannedNewAccountCount(t *testing.T) {
	withAccounts := func(n int) *ActivityRegistry {
		r := NewRegistry("c", "f", "addr", "evm", "t0")
		for i := range n {
			r.UpsertAccount(newRec(string(rune('a'+i)), string(rune('a'+i))))
		}
		return r
	}

	t.Run("fresh registry fills up to num-accounts", func(t *testing.T) {
		cfg := &Config{NumAccounts: 10}
		if got := plannedNewAccountCount(cfg, withAccounts(0)); got != 10 {
			t.Errorf("got %d, want 10", got)
		}
	})

	t.Run("partially filled registry tops up the deficit", func(t *testing.T) {
		cfg := &Config{NumAccounts: 10}
		if got := plannedNewAccountCount(cfg, withAccounts(4)); got != 6 {
			t.Errorf("got %d, want 6", got)
		}
	})

	t.Run("already full registry adds none", func(t *testing.T) {
		cfg := &Config{NumAccounts: 3}
		if got := plannedNewAccountCount(cfg, withAccounts(5)); got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})

	t.Run("add-accounts always adds num-accounts more", func(t *testing.T) {
		cfg := &Config{NumAccounts: 4, AddAccounts: true}
		if got := plannedNewAccountCount(cfg, withAccounts(5)); got != 4 {
			t.Errorf("got %d, want 4", got)
		}
	})

	t.Run("activity-existing alone adds none", func(t *testing.T) {
		cfg := &Config{NumAccounts: 10, ActivityExisting: true}
		if got := plannedNewAccountCount(cfg, withAccounts(2)); got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})
}

func TestReconcilePreservesAccountKeyStyle(t *testing.T) {
	reg := NewRegistry("old-chain", "old-funder", "", "legacy", "t0")
	// An account created under the legacy style.
	acct := newRec("gen-0001", "lumera1a")
	acct.AccountIdentity.KeyStyle = "legacy"
	reg.UpsertAccount(acct)

	cfg := &Config{ChainID: "new-chain", FundingKey: "new-funder"}
	reconcile(reg, cfg, common.KeyStyleEVM)

	if reg.ChainID != "new-chain" || reg.FunderKey != "new-funder" || reg.KeyStyle != "evm" {
		t.Errorf("envelope not updated: %+v", reg)
	}
	// The pre-existing account keeps its original creation style.
	if reg.Accounts[0].AccountIdentity.KeyStyle != "legacy" {
		t.Errorf("account key style = %q, want legacy (must not be rewritten)", reg.Accounts[0].AccountIdentity.KeyStyle)
	}
}
