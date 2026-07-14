package main

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"gen/tests/common"
)

// activityChain is the per-account chain operations an activity worker needs.
// Each method signs with the given account key name. It is an interface so the
// worker-pool and recording logic are testable against a fake.
type activityChain interface {
	Delegate(fromKey, valoper, amount string) (txHash string, err error)
	Unbond(fromKey, valoper, amount string) (txHash string, err error)
	Redelegate(fromKey, srcValoper, dstValoper, amount string) (txHash string, err error)
	SetWithdrawAddress(fromKey, withdrawAddr string) (txHash string, err error)
	GrantAuthzSend(fromKey, granteeAddr string) (txHash string, err error)
	GrantFeegrant(fromKey, granteeAddr, spendLimit string) (txHash string, err error)
	BankSend(fromKey, toAddr, amount string) (txHash string, err error)
	// AlreadyDone reports whether the activity's resulting on-chain state already
	// exists, so a rerun can skip a redundant (and conflict-prone) resubmission.
	AlreadyDone(acct *AccountRecord, act plannedActivity) (bool, error)
}

// executeActivity carries out one planned activity for an account. On a rerun
// where the on-chain state already exists, it records the existing state and
// skips the tx; otherwise it submits (signing with the account's key) and
// records only on success. Recording the actor's side only; reciprocal
// received-grant arrays are filled by reconcileReceivedGrants afterward.
func executeActivity(chain activityChain, acct *AccountRecord, act plannedActivity) error {
	// Best-effort pre-query: a query error doesn't block the submission.
	if done, err := chain.AlreadyDone(acct, act); err != nil {
		log.Printf("  WARN: %s %s pre-check failed (will attempt): %v", acct.Name, act.Kind, err)
	} else if done {
		recordActivity(acct, act, "")
		return nil
	}
	txHash, err := submitActivity(chain, acct, act)
	if err != nil {
		return err
	}
	recordActivity(acct, act, txHash)
	return nil
}

// submitActivity issues the chain transaction for an activity, returning the tx
// hash where the operation produces one.
func submitActivity(chain activityChain, acct *AccountRecord, act plannedActivity) (string, error) {
	switch act.Kind {
	case actDelegate:
		return chain.Delegate(acct.Name, act.Validator, act.Amount)
	case actUnbond:
		return chain.Unbond(acct.Name, act.Validator, act.Amount)
	case actRedelegate:
		return chain.Redelegate(acct.Name, act.SrcValidator, act.DstValidator, act.Amount)
	case actWithdrawAddr:
		return chain.SetWithdrawAddress(acct.Name, act.Peer)
	case actAuthzGrant:
		return chain.GrantAuthzSend(acct.Name, act.Peer)
	case actFeegrant:
		return chain.GrantFeegrant(acct.Name, act.Peer, act.Amount)
	case actBankSend:
		return chain.BankSend(acct.Name, act.Peer, act.Amount)
	default:
		return "", fmt.Errorf("unknown activity kind %v", act.Kind)
	}
}

// recordActivity records an activity into the account's log. ActivityLog dedup
// makes this idempotent across reruns.
func recordActivity(acct *AccountRecord, act plannedActivity, txHash string) {
	switch act.Kind {
	case actDelegate:
		acct.AddDelegation(act.Validator, act.Amount)
	case actUnbond:
		acct.AddUnbonding(act.Validator, act.Amount)
	case actRedelegate:
		acct.AddRedelegation(act.SrcValidator, act.DstValidator, act.Amount)
	case actWithdrawAddr:
		acct.AddWithdrawAddress(act.Peer)
	case actAuthzGrant:
		acct.AddAuthzGrant(act.Peer, common.BankSendMsgType)
	case actFeegrant:
		acct.AddFeegrant(act.Peer, act.Amount)
	case actBankSend:
		acct.AddBankSend(common.BankSendActivity{To: act.Peer, Amount: act.Amount, TxHash: txHash})
	}
}

// runActivityWorkers runs each account's planned activities, with accounts
// processed concurrently up to `parallelism` workers and each account's own
// activities run sequentially (one signer, one sequence). Per-activity failures
// are logged and skipped; one bad activity never aborts the account or the run.
func runActivityWorkers(chain activityChain, accounts []*AccountRecord, planFor func(*AccountRecord) []plannedActivity, parallelism int) {
	if parallelism < 1 {
		parallelism = 1
	}
	totalAccounts := len(accounts)
	if totalAccounts == 0 {
		log.Printf("activity progress: no accounts to process")
		return
	}
	accountStep := progressEvery(totalAccounts)
	sem := make(chan struct{}, parallelism)
	var wg sync.WaitGroup
	var completedAccounts atomic.Int64
	var completedActivities atomic.Int64
	log.Printf("activity progress: starting %d account(s) with parallelism %d", totalAccounts, parallelism)
	for _, acct := range accounts {
		wg.Add(1)
		go func(acct *AccountRecord) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			plan := planFor(acct)
			log.Printf("activity progress: %s starting %d planned operation(s)", acct.Name, len(plan))
			localDone := 0
			for _, act := range plan {
				if err := executeActivity(chain, acct, act); err != nil {
					log.Printf("  WARN: %s %s failed: %v", acct.Name, act.Kind, err)
				} else {
					localDone++
				}
				doneActs := completedActivities.Add(1)
				if doneActs%50 == 0 {
					log.Printf("activity progress: %d operation(s) attempted", doneActs)
				}
			}
			doneAccounts := completedAccounts.Add(1)
			if doneAccounts == int64(totalAccounts) || doneAccounts%int64(accountStep) == 0 {
				log.Printf("activity progress: %d/%d account(s) completed; latest %s succeeded %d/%d operation(s)",
					doneAccounts, totalAccounts, acct.Name, localDone, len(plan))
			}
		}(acct)
	}
	wg.Wait()
	log.Printf("activity progress: completed %d account(s), attempted %d operation(s)", completedAccounts.Load(), completedActivities.Load())
}

// reconcileReceivedGrants backfills the received-grant arrays: for every authz
// or fee grant an account sent to a peer address, the matching peer record gets
// the reciprocal received entry. Dedup is handled by the ActivityLog adders.
func reconcileReceivedGrants(accounts []*AccountRecord) {
	byAddr := make(map[string]*AccountRecord, len(accounts))
	for _, a := range accounts {
		byAddr[a.Address] = a
	}
	for _, granter := range accounts {
		for _, g := range granter.AuthzGrants {
			if peer, ok := byAddr[g.Grantee]; ok {
				peer.AddAuthzAsGrantee(granter.Address, g.MsgType)
			}
		}
		for _, f := range granter.Feegrants {
			if peer, ok := byAddr[f.Grantee]; ok {
				peer.AddFeegrantAsGrantee(granter.Address, f.SpendLimit)
			}
		}
	}
}
