package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gen/tests/common"
)

// fakeActivityChain records calls and can be told to fail a specific kind.
type fakeActivityChain struct {
	mu       sync.Mutex
	calls    []string
	failKind activityKind
	fail     bool

	inFlight    int32
	maxInFlight int32
	delay       time.Duration
}

func (f *fakeActivityChain) track(name string) func() {
	cur := atomic.AddInt32(&f.inFlight, 1)
	for {
		old := atomic.LoadInt32(&f.maxInFlight)
		if cur <= old || atomic.CompareAndSwapInt32(&f.maxInFlight, old, cur) {
			break
		}
	}
	f.mu.Lock()
	f.calls = append(f.calls, name)
	f.mu.Unlock()
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	return func() { atomic.AddInt32(&f.inFlight, -1) }
}

func (f *fakeActivityChain) maybeFail(k activityKind) error {
	if f.fail && f.failKind == k {
		return fmt.Errorf("simulated failure for %v", k)
	}
	return nil
}

func (f *fakeActivityChain) Delegate(from, val, amt string) (string, error) {
	defer f.track("delegate")()
	return "tx", f.maybeFail(actDelegate)
}
func (f *fakeActivityChain) Unbond(from, val, amt string) (string, error) {
	defer f.track("unbond")()
	return "tx", f.maybeFail(actUnbond)
}
func (f *fakeActivityChain) Redelegate(from, src, dst, amt string) (string, error) {
	defer f.track("redelegate")()
	return "tx", f.maybeFail(actRedelegate)
}
func (f *fakeActivityChain) SetWithdrawAddress(from, addr string) (string, error) {
	defer f.track("withdraw")()
	return "tx", f.maybeFail(actWithdrawAddr)
}
func (f *fakeActivityChain) GrantAuthzSend(from, grantee string) (string, error) {
	defer f.track("authz")()
	return "tx", f.maybeFail(actAuthzGrant)
}
func (f *fakeActivityChain) GrantFeegrant(from, grantee, lim string) (string, error) {
	defer f.track("feegrant")()
	return "tx", f.maybeFail(actFeegrant)
}
func (f *fakeActivityChain) BankSend(from, to, amt string) (string, error) {
	defer f.track("banksend")()
	return "txhash-bank", f.maybeFail(actBankSend)
}

var _ activityChain = (*fakeActivityChain)(nil)

func acct(name string) *AccountRecord {
	return &AccountRecord{AccountIdentity: common.AccountIdentity{Name: name, Address: "addr-" + name}}
}

func TestExecuteActivityRecordsEachKind(t *testing.T) {
	chain := &fakeActivityChain{}
	a := acct("u1")

	acts := []plannedActivity{
		{Kind: actDelegate, Validator: "valA", Amount: "10ulume"},
		{Kind: actUnbond, Validator: "valA", Amount: "5ulume"},
		{Kind: actRedelegate, SrcValidator: "valA", DstValidator: "valB", Amount: "3ulume"},
		{Kind: actWithdrawAddr, Peer: "addr-u2"},
		{Kind: actAuthzGrant, Peer: "addr-u2"},
		{Kind: actFeegrant, Peer: "addr-u2", Amount: "2ulume"},
		{Kind: actBankSend, Peer: "addr-u2", Amount: "1ulume"},
	}
	for _, act := range acts {
		if err := executeActivity(chain, a, act); err != nil {
			t.Fatalf("executeActivity(%v): %v", act.Kind, err)
		}
	}

	if len(a.Delegations) != 1 || a.Delegations[0].Validator != "valA" {
		t.Errorf("delegations = %v", a.Delegations)
	}
	if len(a.Unbondings) != 1 || len(a.Redelegations) != 1 {
		t.Errorf("unbondings=%v redelegations=%v", a.Unbondings, a.Redelegations)
	}
	if len(a.WithdrawAddresses) != 1 || len(a.AuthzGrants) != 1 || len(a.Feegrants) != 1 {
		t.Errorf("withdraw/authz/feegrant not all recorded")
	}
	if len(a.BankSends) != 1 || a.BankSends[0].TxHash != "txhash-bank" {
		t.Errorf("bank send not recorded with tx hash: %v", a.BankSends)
	}
}

func TestExecuteActivityErrorIsNotRecorded(t *testing.T) {
	chain := &fakeActivityChain{fail: true, failKind: actDelegate}
	a := acct("u1")
	err := executeActivity(chain, a, plannedActivity{Kind: actDelegate, Validator: "valA", Amount: "10ulume"})
	if err == nil {
		t.Fatal("expected error from failing chain")
	}
	if len(a.Delegations) != 0 {
		t.Errorf("failed delegation must not be recorded, got %v", a.Delegations)
	}
}

func TestRunActivityWorkersRespectsParallelismAndCompletes(t *testing.T) {
	chain := &fakeActivityChain{delay: 2 * time.Millisecond}
	var accounts []*AccountRecord
	for i := range 6 {
		accounts = append(accounts, acct(fmt.Sprintf("u%d", i)))
	}
	planFor := func(*AccountRecord) []plannedActivity {
		return []plannedActivity{{Kind: actBankSend, Peer: "addr-x", Amount: "1ulume"}}
	}

	const parallelism = 2
	runActivityWorkers(chain, accounts, planFor, parallelism)

	if got := atomic.LoadInt32(&chain.maxInFlight); got > parallelism {
		t.Errorf("max concurrent workers = %d, exceeds parallelism %d", got, parallelism)
	}
	if len(chain.calls) != len(accounts) {
		t.Errorf("calls = %d, want %d (one per account)", len(chain.calls), len(accounts))
	}
	for _, a := range accounts {
		if len(a.BankSends) != 1 {
			t.Errorf("account %s did not record its bank send", a.Name)
		}
	}
}

func TestReconcileReceivedGrants(t *testing.T) {
	granter := acct("g")
	granter.Address = "addr-g"
	grantee := acct("e")
	grantee.Address = "addr-e"
	// g granted authz and feegrant to e.
	granter.AddAuthzGrant("addr-e", common.BankSendMsgType)
	granter.AddFeegrant("addr-e", "5ulume")

	reconcileReceivedGrants([]*AccountRecord{granter, grantee})

	if len(grantee.AuthzAsGrantee) != 1 || grantee.AuthzAsGrantee[0].Granter != "addr-g" {
		t.Errorf("grantee authz-received not reconciled: %v", grantee.AuthzAsGrantee)
	}
	if len(grantee.FeegrantsReceived) != 1 || grantee.FeegrantsReceived[0].Granter != "addr-g" {
		t.Errorf("grantee feegrant-received not reconciled: %v", grantee.FeegrantsReceived)
	}
}
