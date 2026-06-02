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
		// Redelegate when at least two validators exist.
		if src, dst, ok := SelectRedelegationPair(validators, rng); ok {
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

func randomPeer(peers []string, rng *rand.Rand) string {
	return peers[rng.Intn(len(peers))]
}
