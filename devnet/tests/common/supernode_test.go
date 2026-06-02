package common

import "testing"

func TestLatestSupernodeStatePicksHighestHeight(t *testing.T) {
	sn := &SupernodeRecord{
		States: []SupernodeStateEntry{
			{State: "SUPERNODE_STATE_DISABLED", Height: "5"},
			{State: "SUPERNODE_STATE_ACTIVE", Height: "20"},
			{State: "SUPERNODE_STATE_STOPPED", Height: "12"},
		},
	}
	if got := LatestSupernodeState(sn); got != "SUPERNODE_STATE_ACTIVE" {
		t.Errorf("got %q, want SUPERNODE_STATE_ACTIVE", got)
	}

	if got := LatestSupernodeState(&SupernodeRecord{}); got != "" {
		t.Errorf("empty record state = %q, want empty", got)
	}
	if got := LatestSupernodeState(nil); got != "" {
		t.Errorf("nil record state = %q, want empty", got)
	}
}

func TestLatestSupernodeHostPicksHighestHeightAndStripsPort(t *testing.T) {
	sn := &SupernodeRecord{
		PrevIPAddresses: []SupernodeIPEntry{
			{Address: "10.0.0.1:4445", Height: "3"},
			{Address: "10.0.0.9:4445", Height: "30"},
		},
	}
	if got := LatestSupernodeHost(sn); got != "10.0.0.9" {
		t.Errorf("got %q, want 10.0.0.9", got)
	}

	// An address without a port is returned as-is.
	sn2 := &SupernodeRecord{PrevIPAddresses: []SupernodeIPEntry{{Address: "host.local", Height: "1"}}}
	if got := LatestSupernodeHost(sn2); got != "host.local" {
		t.Errorf("got %q, want host.local", got)
	}

	if got := LatestSupernodeHost(&SupernodeRecord{}); got != "" {
		t.Errorf("empty record host = %q, want empty", got)
	}
}
