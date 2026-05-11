package main

import (
	"testing"
	"time"
)

func TestBuildPreparedAccountName(t *testing.T) {
	if got := buildPreparedAccountName(legacyPreparedAccountPrefix, "val1", 7); got != "pre-evm-val1-007" {
		t.Fatalf("unexpected prepared account name: %s", got)
	}
	if got := buildPreparedAccountName(extraPreparedAccountPrefix, "", 4); got != "pre-evmex-004" {
		t.Fatalf("unexpected extra prepared account name: %s", got)
	}
}

func TestBatchedFundingWaitTimeout(t *testing.T) {
	if got := batchedFundingWaitTimeout(0); got != 50*time.Second {
		t.Fatalf("batchedFundingWaitTimeout(0) = %s, want %s", got, 50*time.Second)
	}
	if got := batchedFundingWaitTimeout(10); got != 95*time.Second {
		t.Fatalf("batchedFundingWaitTimeout(10) = %s, want %s", got, 95*time.Second)
	}
	if got := batchedFundingWaitTimeout(60); got != 3*time.Minute {
		t.Fatalf("batchedFundingWaitTimeout(60) = %s, want %s", got, 3*time.Minute)
	}
}

func TestPlannedPrepareClaim(t *testing.T) {
	cases := []struct {
		idx     int
		tier    uint32
		delayed bool
	}{
		{idx: 0, tier: 0, delayed: false},
		{idx: 3, tier: 1, delayed: true},
		{idx: 6, tier: 2, delayed: true},
		{idx: 9, tier: 3, delayed: true},
		{idx: 10, tier: 0, delayed: false},
	}

	for _, tc := range cases {
		tier, delayed := plannedPrepareClaim(tc.idx)
		if tier != tc.tier || delayed != tc.delayed {
			t.Fatalf("plannedPrepareClaim(%d) = (%d, %v), want (%d, %v)", tc.idx, tier, delayed, tc.tier, tc.delayed)
		}
	}
}

func TestSelectPrepareClaimForAccount(t *testing.T) {
	actionRec := &AccountRecord{
		Actions: []ActionActivity{{ActionID: "11"}},
	}
	tier, delayed := selectPrepareClaimForAccount(actionRec, 3)
	if tier != 0 || delayed {
		t.Fatalf("expected action account delayed claim to be forced instant, got (%d, %v)", tier, delayed)
	}

	plainRec := &AccountRecord{}
	tier, delayed = selectPrepareClaimForAccount(plainRec, 3)
	if tier != 1 || !delayed {
		t.Fatalf("expected non-action account to keep delayed claim selection, got (%d, %v)", tier, delayed)
	}
}
