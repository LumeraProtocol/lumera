package main

import (
	"math/rand"

	"gen/tests/common"
)

// activityKind enumerates the per-account activities the generator can submit.
type activityKind int

const (
	actDelegate activityKind = iota
	actUnbond
	actRedelegate
	actWithdrawAddr
	actAuthzGrant
	actFeegrant
	actBankSend
)

func (k activityKind) String() string {
	switch k {
	case actDelegate:
		return "delegate"
	case actUnbond:
		return "unbond"
	case actRedelegate:
		return "redelegate"
	case actWithdrawAddr:
		return "withdraw-address"
	case actAuthzGrant:
		return "authz-grant"
	case actFeegrant:
		return "feegrant"
	case actBankSend:
		return "bank-send"
	default:
		return "unknown"
	}
}

// plannedActivity is a single intended activity for an account. Only the fields
// relevant to its Kind are populated.
type plannedActivity struct {
	Kind         activityKind
	Validator    string // delegate, unbond
	SrcValidator string // redelegate
	DstValidator string // redelegate
	Peer         string // bank-send, authz-grant, feegrant, withdraw-address
	Amount       string // ulume amount where applicable (empty for withdraw-address)
}

// planActivities decides the activity mix for one account given the live
// validator set, the addresses of peer generated accounts, an RNG (injected for
// determinism), and a base unit amount in ulume for activity sizing.
//
// Coverage-oriented: an activity is planned whenever its preconditions hold
// (validators present for staking, peers present for transfers/grants), so a
// single run exercises the full surface. Re-recording is prevented downstream by
// ActivityLog deduplication.
func planActivities(validators, peers []string, rng *rand.Rand, unit int64) []plannedActivity {
	var plan []plannedActivity

	amount := func(mult int64) string {
		return common.Coin{Amount: unit * mult, Denom: common.ChainDenom}.String()
	}

	// Delegations: 1..3 distinct validators (clamped to availability).
	if len(validators) > 0 {
		n := 1 + rng.Intn(3)
		chosen := SelectValidators(validators, n, rng)
		for _, v := range chosen {
			plan = append(plan, plannedActivity{Kind: actDelegate, Validator: v, Amount: amount(2)})
		}
		// Unbond a portion from the first delegation.
		if len(chosen) > 0 {
			plan = append(plan, plannedActivity{Kind: actUnbond, Validator: chosen[0], Amount: amount(1)})
		}
		// Redelegate from a validator this account just delegated to.
		if src, dst, ok := selectRedelegationFromDelegations(chosen, validators, rng); ok {
			plan = append(plan, plannedActivity{Kind: actRedelegate, SrcValidator: src, DstValidator: dst, Amount: amount(1)})
		}
	}

	// Peer-targeting activities.
	if len(peers) > 0 {
		plan = append(plan, plannedActivity{Kind: actBankSend, Peer: randomPeer(peers, rng), Amount: amount(1)})
		plan = append(plan, plannedActivity{Kind: actAuthzGrant, Peer: randomPeer(peers, rng)})
		plan = append(plan, plannedActivity{Kind: actFeegrant, Peer: randomPeer(peers, rng), Amount: amount(1)})
		plan = append(plan, plannedActivity{Kind: actWithdrawAddr, Peer: randomPeer(peers, rng)})
	}

	return plan
}

func buildActivityPlans(accounts []*AccountRecord, validators []string, rng *rand.Rand, unit int64) map[*AccountRecord][]plannedActivity {
	addrs := make([]string, len(accounts))
	for i, rec := range accounts {
		addrs[i] = rec.Address
	}
	plans := make(map[*AccountRecord][]plannedActivity, len(accounts))
	for _, rec := range accounts {
		plans[rec] = planActivities(validators, peersExcluding(addrs, rec.Address), rng, unit)
	}
	return plans
}

// activityTargets returns the funded accounts that take part in the regular
// single-sig activity mix. Multisig composites are excluded: they cannot sign an
// ordinary single-sig --from tx and have their own dedicated exercise path
// (exerciseMultisigAccounts), so including them here would only produce
// guaranteed-failing delegate/bank-send/action attempts.
func activityTargets(reg *ActivityRegistry, newRecords []*AccountRecord, includeExisting bool) []*AccountRecord {
	var out []*AccountRecord
	if includeExisting || len(newRecords) == 0 {
		for _, rec := range reg.Accounts {
			if rec.Funded && rec.Multisig == nil {
				out = append(out, rec)
			}
		}
		return out
	}
	for _, rec := range newRecords {
		if rec.Funded && rec.Multisig == nil {
			out = append(out, rec)
		}
	}
	return out
}

func randomPeer(peers []string, rng *rand.Rand) string {
	return peers[rng.Intn(len(peers))]
}

func selectRedelegationFromDelegations(delegated, validators []string, rng *rand.Rand) (src, dst string, ok bool) {
	if len(delegated) == 0 || len(validators) < 2 {
		return "", "", false
	}
	src = delegated[rng.Intn(len(delegated))]
	var candidates []string
	for _, v := range validators {
		if v != src {
			candidates = append(candidates, v)
		}
	}
	if len(candidates) == 0 {
		return "", "", false
	}
	return src, candidates[rng.Intn(len(candidates))], true
}
