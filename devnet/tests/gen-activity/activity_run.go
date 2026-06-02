package main

import (
	"fmt"
	"log"
	"sync"

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
}

// executeActivity submits one planned activity for an account (signing with the
// account's key name) and, only on success, records it into the account's
// activity log. Recording the actor's side only; reciprocal received-grant
// arrays are filled by reconcileReceivedGrants after all workers finish.
func executeActivity(chain activityChain, acct *AccountRecord, act plannedActivity) error {
	switch act.Kind {
	case actDelegate:
		if _, err := chain.Delegate(acct.Name, act.Validator, act.Amount); err != nil {
			return err
		}
		acct.AddDelegation(act.Validator, act.Amount)
	case actUnbond:
		if _, err := chain.Unbond(acct.Name, act.Validator, act.Amount); err != nil {
			return err
		}
		acct.AddUnbonding(act.Validator, act.Amount)
	case actRedelegate:
		if _, err := chain.Redelegate(acct.Name, act.SrcValidator, act.DstValidator, act.Amount); err != nil {
			return err
		}
		acct.AddRedelegation(act.SrcValidator, act.DstValidator, act.Amount)
	case actWithdrawAddr:
		if _, err := chain.SetWithdrawAddress(acct.Name, act.Peer); err != nil {
			return err
		}
		acct.AddWithdrawAddress(act.Peer)
	case actAuthzGrant:
		if _, err := chain.GrantAuthzSend(acct.Name, act.Peer); err != nil {
			return err
		}
		acct.AddAuthzGrant(act.Peer, common.BankSendMsgType)
	case actFeegrant:
		if _, err := chain.GrantFeegrant(acct.Name, act.Peer, act.Amount); err != nil {
			return err
		}
		acct.AddFeegrant(act.Peer, act.Amount)
	case actBankSend:
		txHash, err := chain.BankSend(acct.Name, act.Peer, act.Amount)
		if err != nil {
			return err
		}
		acct.AddBankSend(common.BankSendActivity{To: act.Peer, Amount: act.Amount, TxHash: txHash})
	default:
		return fmt.Errorf("unknown activity kind %v", act.Kind)
	}
	return nil
}

// runActivityWorkers runs each account's planned activities, with accounts
// processed concurrently up to `parallelism` workers and each account's own
// activities run sequentially (one signer, one sequence). Per-activity failures
// are logged and skipped; one bad activity never aborts the account or the run.
func runActivityWorkers(chain activityChain, accounts []*AccountRecord, planFor func(*AccountRecord) []plannedActivity, parallelism int) {
	if parallelism < 1 {
		parallelism = 1
	}
	sem := make(chan struct{}, parallelism)
	var wg sync.WaitGroup
	for _, acct := range accounts {
		wg.Add(1)
		go func(acct *AccountRecord) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			for _, act := range planFor(acct) {
				if err := executeActivity(chain, acct, act); err != nil {
					log.Printf("  WARN: %s %s failed: %v", acct.Name, act.Kind, err)
				}
			}
		}(acct)
	}
	wg.Wait()
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
