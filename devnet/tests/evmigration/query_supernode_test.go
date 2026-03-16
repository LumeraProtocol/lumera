package main

import "testing"

func TestQuerySupernodeMetricsArgs(t *testing.T) {
	got := querySupernodeMetricsArgs("lumeravaloper1test")
	want := []string{"query", "supernode", "get-metrics", "lumeravaloper1test"}
	if len(got) != len(want) {
		t.Fatalf("unexpected arg count: got %d want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d] = %q, want %q (all args: %v)", i, got[i], want[i], got)
		}
	}
}

func TestLatestSupernodeState(t *testing.T) {
	record := &SuperNodeRecord{
		States: []struct {
			State  string `json:"state"`
			Height string `json:"height"`
			Reason string `json:"reason"`
		}{
			{State: "SUPERNODE_STATE_STOPPED", Height: "10"},
			{State: "SUPERNODE_STATE_ACTIVE", Height: "12"},
			{State: "SUPERNODE_STATE_POSTPONED", Height: "11"},
		},
	}

	if got := latestSupernodeState(record); got != "SUPERNODE_STATE_ACTIVE" {
		t.Fatalf("latestSupernodeState() = %q, want %q", got, "SUPERNODE_STATE_ACTIVE")
	}
}
