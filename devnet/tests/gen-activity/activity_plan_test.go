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

func TestPlanActivitiesRedelegatesFromPlannedDelegation(t *testing.T) {
	vals := []string{"valA", "valB", "valC", "valD"}
	for seed := int64(0); seed < 100; seed++ {
		plan := planActivities(vals, nil, rand.New(rand.NewSource(seed)), 1000)
		delegated := map[string]bool{}
		for _, a := range plan {
			if a.Kind == actDelegate {
				delegated[a.Validator] = true
			}
		}
		for _, a := range plan {
			if a.Kind == actRedelegate && !delegated[a.SrcValidator] {
				t.Fatalf("seed %d planned redelegation from %q without a planned delegation; plan=%v", seed, a.SrcValidator, plan)
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

func TestBuildActivityPlansExcludesSelfPeers(t *testing.T) {
	accounts := []*AccountRecord{
		{AccountIdentity: common.AccountIdentity{Name: "a", Address: "addr-a"}},
		{AccountIdentity: common.AccountIdentity{Name: "b", Address: "addr-b"}},
		{AccountIdentity: common.AccountIdentity{Name: "c", Address: "addr-c"}},
	}
	plans := buildActivityPlans(accounts, []string{"valA", "valB"}, rand.New(rand.NewSource(3)), 1000)

	if len(plans) != len(accounts) {
		t.Fatalf("plans = %d, want %d", len(plans), len(accounts))
	}
	for _, acct := range accounts {
		for _, act := range plans[acct] {
			switch act.Kind {
			case actBankSend, actAuthzGrant, actFeegrant, actWithdrawAddr:
				if act.Peer == acct.Address {
					t.Fatalf("account %s planned self-targeting activity %v", acct.Name, act)
				}
			}
		}
	}
}

func TestActivityTargetsRespectExistingFlag(t *testing.T) {
	existing := &AccountRecord{AccountIdentity: common.AccountIdentity{Name: "old", Address: "addr-old"}, Funded: true}
	created := &AccountRecord{AccountIdentity: common.AccountIdentity{Name: "new", Address: "addr-new"}, Funded: true}
	unfunded := &AccountRecord{AccountIdentity: common.AccountIdentity{Name: "dry", Address: "addr-dry"}}
	reg := NewRegistry("c", "f", "addr-f", "evm", "t0")
	reg.UpsertAccount(existing)
	reg.UpsertAccount(created)
	reg.UpsertAccount(unfunded)

	onlyNew := activityTargets(reg, []*AccountRecord{created}, false)
	if len(onlyNew) != 1 || onlyNew[0] != created {
		t.Fatalf("targets without existing activity = %v, want only newly created funded account", onlyNew)
	}

	withExisting := activityTargets(reg, []*AccountRecord{created}, true)
	if len(withExisting) != 2 {
		t.Fatalf("targets with existing activity = %d, want 2 funded accounts", len(withExisting))
	}
	if withExisting[0] != existing || withExisting[1] != created {
		t.Fatalf("targets with existing activity = %v, want existing then created", withExisting)
	}

	resumeTargets := activityTargets(reg, nil, false)
	if len(resumeTargets) != 2 {
		t.Fatalf("targets on resume with no new records = %d, want 2 funded accounts", len(resumeTargets))
	}
	if resumeTargets[0] != existing || resumeTargets[1] != created {
		t.Fatalf("targets on resume with no new records = %v, want existing then created", resumeTargets)
	}
}
