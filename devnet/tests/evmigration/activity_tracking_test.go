package main

import "testing"

func TestIsCompatibleActionState(t *testing.T) {
	cases := []struct {
		expected string
		actual   string
		ok       bool
	}{
		{expected: "ACTION_STATE_PENDING", actual: "ACTION_STATE_PENDING", ok: true},
		{expected: "ACTION_STATE_PENDING", actual: "ACTION_STATE_DONE", ok: true},
		{expected: "ACTION_STATE_PENDING", actual: "ACTION_STATE_APPROVED", ok: true},
		{expected: "ACTION_STATE_DONE", actual: "ACTION_STATE_APPROVED", ok: true},
		{expected: "ACTION_STATE_DONE", actual: "ACTION_STATE_PENDING", ok: false},
		{expected: "ACTION_STATE_APPROVED", actual: "ACTION_STATE_DONE", ok: false},
		{expected: "ACTION_STATE_PENDING", actual: "ACTION_STATE_FAILED", ok: false},
	}

	for _, tc := range cases {
		if got := isCompatibleActionState(tc.expected, tc.actual); got != tc.ok {
			t.Fatalf("isCompatibleActionState(%q, %q) = %v, want %v", tc.expected, tc.actual, got, tc.ok)
		}
	}
}

func TestAccountRecordDelayedClaimAndActionHelpers(t *testing.T) {
	rec := &AccountRecord{}
	if rec.hasDelayedClaim() {
		t.Fatal("expected empty record not to report delayed claims")
	}
	if rec.hasRecordedAction() {
		t.Fatal("expected empty record not to report actions")
	}

	rec.Claims = []ClaimActivity{{OldAddress: "pastel1", Tier: 2}}
	if !rec.hasDelayedClaim() {
		t.Fatal("expected tiered claim to be treated as delayed")
	}

	rec.Claims = []ClaimActivity{{OldAddress: "pastel1"}}
	rec.Actions = []ActionActivity{{ActionID: "7"}}
	if !rec.hasRecordedAction() {
		t.Fatal("expected action slice to be treated as recorded action activity")
	}
}
