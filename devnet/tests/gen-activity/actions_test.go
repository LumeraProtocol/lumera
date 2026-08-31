package main

import (
	"testing"

	"gen/tests/common"
)

func TestPlanActionsRoundRobin(t *testing.T) {
	accounts := []*AccountRecord{acct("a"), acct("b")}
	states := []common.ActionState{common.ActionStatePending, common.ActionStateDone, common.ActionStateApproved}

	plan := planActions(states, accounts, 5)
	if len(plan) != 5 {
		t.Fatalf("plan = %d, want 5 (capped)", len(plan))
	}
	// States cycle round-robin: pending, done, approved, pending, done.
	wantStates := []common.ActionState{
		common.ActionStatePending, common.ActionStateDone, common.ActionStateApproved,
		common.ActionStatePending, common.ActionStateDone,
	}
	for i, a := range plan {
		if a.State != wantStates[i] {
			t.Errorf("plan[%d].State = %q, want %q", i, a.State, wantStates[i])
		}
	}
	// Accounts cycle round-robin: a, b, a, b, a.
	wantAccts := []string{"a", "b", "a", "b", "a"}
	for i, a := range plan {
		if a.Account.Name != wantAccts[i] {
			t.Errorf("plan[%d].Account = %q, want %q", i, a.Account.Name, wantAccts[i])
		}
	}
}

func TestPlanActionsPerAccountIndexAccountsForExisting(t *testing.T) {
	a := acct("a")
	a.AddAction(common.ActionActivity{ActionID: "pre-existing"}) // a already has 1 action
	accounts := []*AccountRecord{a, acct("b")}
	states := []common.ActionState{common.ActionStatePending}

	plan := planActions(states, accounts, 4)
	// Assignments: a(idx?), b, a, b. a's indices continue past its existing action.
	var aIdx, bIdx []int
	for _, asg := range plan {
		if asg.Account.Name == "a" {
			aIdx = append(aIdx, asg.Index)
		} else {
			bIdx = append(bIdx, asg.Index)
		}
	}
	// a had 1 existing action -> its new indices start at 1.
	if len(aIdx) != 2 || aIdx[0] != 1 || aIdx[1] != 2 {
		t.Errorf("a indices = %v, want [1 2]", aIdx)
	}
	// b had none -> starts at 0.
	if len(bIdx) != 2 || bIdx[0] != 0 || bIdx[1] != 1 {
		t.Errorf("b indices = %v, want [0 1]", bIdx)
	}
}

func TestPlanActionsEdgeCases(t *testing.T) {
	accounts := []*AccountRecord{acct("a")}
	states := []common.ActionState{common.ActionStatePending}

	if got := planActions(states, accounts, 0); got != nil {
		t.Errorf("cap 0 = %v, want nil", got)
	}
	if got := planActions(nil, accounts, 3); got != nil {
		t.Errorf("no states = %v, want nil", got)
	}
	if got := planActions(states, nil, 3); got != nil {
		t.Errorf("no accounts = %v, want nil", got)
	}
}
