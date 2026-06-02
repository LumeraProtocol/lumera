package main

import (
	"math/rand"
	"slices"
	"testing"

	"gen/tests/common"
)

func planRNG() *rand.Rand { return rand.New(rand.NewSource(7)) }

// kindsIn returns the set of activity kinds present in a plan.
func kindsIn(plan []plannedActivity) map[activityKind]int {
	m := map[activityKind]int{}
	for _, a := range plan {
		m[a.Kind]++
	}
	return m
}

func TestPlanActivitiesNoValidatorsNoPeers(t *testing.T) {
	plan := planActivities(nil, nil, planRNG(), 1000)
	if len(plan) != 0 {
		t.Errorf("plan = %v, want empty when no validators and no peers", plan)
	}
}

func TestPlanActivitiesValidatorsOnly(t *testing.T) {
	vals := []string{"valA", "valB", "valC"}
	plan := planActivities(vals, nil, planRNG(), 1000)
	kinds := kindsIn(plan)

	if kinds[actDelegate] < 1 || kinds[actDelegate] > 3 {
		t.Errorf("delegations = %d, want 1..3", kinds[actDelegate])
	}
	// With >= 2 validators a redelegation is planned.
	if kinds[actRedelegate] != 1 {
		t.Errorf("redelegations = %d, want 1 for >=2 validators", kinds[actRedelegate])
	}
	// No peers: no peer-targeting activities.
	for _, k := range []activityKind{actBankSend, actAuthzGrant, actFeegrant, actWithdrawAddr} {
		if kinds[k] != 0 {
			t.Errorf("kind %v = %d, want 0 without peers", k, kinds[k])
		}
	}
	// Every delegation/redelegation references a real validator.
	for _, a := range plan {
		switch a.Kind {
		case actDelegate, actUnbond:
			if !slices.Contains(vals, a.Validator) {
				t.Errorf("activity references unknown validator %q", a.Validator)
			}
		case actRedelegate:
			if !slices.Contains(vals, a.SrcValidator) || !slices.Contains(vals, a.DstValidator) {
				t.Errorf("redelegation references unknown validator(s) %q->%q", a.SrcValidator, a.DstValidator)
			}
			if a.SrcValidator == a.DstValidator {
				t.Error("redelegation src == dst")
			}
		}
	}
}

func TestPlanActivitiesSingleValidatorHasNoRedelegation(t *testing.T) {
	plan := planActivities([]string{"valA"}, nil, planRNG(), 1000)
	if kindsIn(plan)[actRedelegate] != 0 {
		t.Error("redelegation planned with only one validator")
	}
}

func TestPlanActivitiesWithPeers(t *testing.T) {
	vals := []string{"valA", "valB"}
	peers := []string{"lumera1peer1", "lumera1peer2"}
	plan := planActivities(vals, peers, planRNG(), 1000)
	kinds := kindsIn(plan)

	for _, k := range []activityKind{actBankSend, actAuthzGrant, actFeegrant, actWithdrawAddr} {
		if kinds[k] != 1 {
			t.Errorf("kind %v = %d, want 1 with peers present", k, kinds[k])
		}
	}
	// Peer-targeting activities reference a real peer.
	for _, a := range plan {
		switch a.Kind {
		case actBankSend, actAuthzGrant, actFeegrant, actWithdrawAddr:
			if !slices.Contains(peers, a.Peer) {
				t.Errorf("kind %v references unknown peer %q", a.Kind, a.Peer)
			}
		}
	}
}

func TestPlanActivitiesAmountsArePositiveUlume(t *testing.T) {
	plan := planActivities([]string{"valA", "valB"}, []string{"lumera1peer1"}, planRNG(), 1000)
	for _, a := range plan {
		if a.Amount == "" {
			continue // withdraw-address has no amount
		}
		c, err := common.ParseCoin(a.Amount)
		if err != nil {
			t.Fatalf("activity %v has unparseable amount %q: %v", a.Kind, a.Amount, err)
		}
		if c.Amount <= 0 || c.Denom != common.ChainDenom {
			t.Errorf("activity %v amount %q not a positive ulume amount", a.Kind, a.Amount)
		}
	}
}
