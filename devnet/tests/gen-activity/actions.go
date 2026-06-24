package main

import (
	"fmt"
	"log"
	"time"

	"gen/tests/common"
)

// supernodeGate reports whether the supernode set is ready to accept CASCADE
// uploads. It is an interface so the action orchestration is testable.
type supernodeGate interface {
	WaitForReadySupernodes(validators []string, timeout time.Duration) bool
}

// actionCreator creates one CASCADE action for an account in a target state and
// returns the recorded activity. It is an interface so the orchestration is
// testable without sdk-go or a live chain.
type actionCreator interface {
	CreateAction(acct *AccountRecord, state common.ActionState, idx int) (common.ActionActivity, error)
}

// generateActions creates CASCADE actions across the funded accounts. It first
// gates on supernode readiness: when supernodes are not ready, a non-require run
// logs the skip and returns nil, while -require-actions makes it fatal. Each
// per-action failure is similarly non-fatal by default and fatal under
// -require-actions. Successfully created actions are recorded on their account.
func generateActions(
	creator actionCreator,
	gate supernodeGate,
	accounts []*AccountRecord,
	validators []string,
	states []common.ActionState,
	maxActions int,
	requireActions bool,
	readinessTimeout time.Duration,
) error {
	plan := planActions(states, accounts, maxActions)
	if len(plan) == 0 {
		return nil
	}

	if !gate.WaitForReadySupernodes(validators, readinessTimeout) {
		if requireActions {
			return fmt.Errorf("no CASCADE-eligible supernodes ready within %s", readinessTimeout)
		}
		log.Printf("no CASCADE-eligible supernodes ready within %s; skipping action generation", readinessTimeout)
		return nil
	}

	created := 0
	for _, asg := range plan {
		act, err := creator.CreateAction(asg.Account, asg.State, asg.Index)
		if err != nil {
			if requireActions {
				return fmt.Errorf("create %s action for %s: %w", asg.State, asg.Account.Name, err)
			}
			log.Printf("  WARN: create %s action for %s failed: %v", asg.State, asg.Account.Name, err)
			continue
		}
		asg.Account.AddAction(act)
		created++
	}
	log.Printf("created %d/%d CASCADE action(s)", created, len(plan))
	return nil
}

// actionAssignment is one planned CASCADE action: which account creates it, the
// target lifecycle state, and the per-account action index (used for sample-file
// naming and to continue past already-recorded actions on rerun).
type actionAssignment struct {
	Account *AccountRecord
	State   common.ActionState
	Index   int
}

// planActions distributes up to maxActions CASCADE actions across the requested
// target states and the funded accounts, round-robin on both. Per-account
// indices continue past any actions already recorded so reruns don't collide.
func planActions(states []common.ActionState, accounts []*AccountRecord, maxActions int) []actionAssignment {
	if maxActions <= 0 || len(states) == 0 || len(accounts) == 0 {
		return nil
	}
	// Seed each account's running index from its already-recorded actions.
	nextIndex := make(map[*AccountRecord]int, len(accounts))
	for _, a := range accounts {
		nextIndex[a] = len(a.Actions)
	}

	plan := make([]actionAssignment, 0, maxActions)
	for i := range maxActions {
		acct := accounts[i%len(accounts)]
		plan = append(plan, actionAssignment{
			Account: acct,
			State:   states[i%len(states)],
			Index:   nextIndex[acct],
		})
		nextIndex[acct]++
	}
	return plan
}
