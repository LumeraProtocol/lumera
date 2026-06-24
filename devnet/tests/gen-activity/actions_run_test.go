package main

import (
	"fmt"
	"testing"
	"time"

	"gen/tests/common"
)

type fakeGate struct {
	ready  bool
	called int
}

func (g *fakeGate) WaitForReadySupernodes([]string, time.Duration) bool {
	g.called++
	return g.ready
}

type fakeCreator struct {
	calls   int
	failAt  int // 1-based call number to fail at; 0 = never
	failErr error
	created []string
}

func (c *fakeCreator) CreateAction(acct *AccountRecord, state common.ActionState, idx int) (common.ActionActivity, error) {
	c.calls++
	if c.failAt != 0 && c.calls == c.failAt {
		return common.ActionActivity{}, c.failErr
	}
	id := fmt.Sprintf("%s-%d", acct.Name, idx)
	c.created = append(c.created, id)
	return common.ActionActivity{ActionID: id, State: string(state), CreatedViaSDK: true}, nil
}

var (
	_ supernodeGate = (*fakeGate)(nil)
	_ actionCreator = (*fakeCreator)(nil)
)

func actionAccounts() []*AccountRecord  { return []*AccountRecord{acct("a"), acct("b")} }
func pendingOnly() []common.ActionState { return []common.ActionState{common.ActionStatePending} }

func TestGenerateActionsHappyPath(t *testing.T) {
	gate := &fakeGate{ready: true}
	creator := &fakeCreator{}
	accounts := actionAccounts()

	err := generateActions(creator, gate, accounts, []string{"val"}, pendingOnly(), 4, false, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creator.calls != 4 {
		t.Errorf("creator calls = %d, want 4", creator.calls)
	}
	total := 0
	for _, a := range accounts {
		total += len(a.Actions)
	}
	if total != 4 {
		t.Errorf("recorded actions = %d, want 4", total)
	}
}

func TestGenerateActionsNotReadyNonFatal(t *testing.T) {
	gate := &fakeGate{ready: false}
	creator := &fakeCreator{}
	err := generateActions(creator, gate, actionAccounts(), []string{"val"}, pendingOnly(), 4, false, time.Second)
	if err != nil {
		t.Fatalf("expected nil (non-fatal skip), got %v", err)
	}
	if creator.calls != 0 {
		t.Errorf("creator called %d times despite no ready supernodes", creator.calls)
	}
}

func TestGenerateActionsNotReadyFatalWithRequire(t *testing.T) {
	gate := &fakeGate{ready: false}
	creator := &fakeCreator{}
	err := generateActions(creator, gate, actionAccounts(), []string{"val"}, pendingOnly(), 4, true, time.Second)
	if err == nil {
		t.Fatal("expected error with -require-actions and no ready supernodes")
	}
	if creator.calls != 0 {
		t.Errorf("creator called despite not ready")
	}
}

func TestGenerateActionsCreateErrorNonFatalSkips(t *testing.T) {
	gate := &fakeGate{ready: true}
	creator := &fakeCreator{failAt: 2, failErr: fmt.Errorf("supernode flake")}
	accounts := actionAccounts()

	err := generateActions(creator, gate, accounts, []string{"val"}, pendingOnly(), 3, false, time.Second)
	if err != nil {
		t.Fatalf("expected nil (non-fatal), got %v", err)
	}
	total := 0
	for _, a := range accounts {
		total += len(a.Actions)
	}
	// 3 attempts, 1 failed -> 2 recorded.
	if total != 2 {
		t.Errorf("recorded actions = %d, want 2 (one creation failed)", total)
	}
}

func TestGenerateActionsCreateErrorFatalWithRequire(t *testing.T) {
	gate := &fakeGate{ready: true}
	creator := &fakeCreator{failAt: 1, failErr: fmt.Errorf("supernode flake")}
	err := generateActions(creator, gate, actionAccounts(), []string{"val"}, pendingOnly(), 3, true, time.Second)
	if err == nil {
		t.Fatal("expected error with -require-actions when a creation fails")
	}
}

func TestGenerateActionsNothingToDoSkipsGate(t *testing.T) {
	gate := &fakeGate{ready: true}
	creator := &fakeCreator{}
	// maxActions 0 -> no work, gate not consulted.
	if err := generateActions(creator, gate, actionAccounts(), []string{"val"}, pendingOnly(), 0, false, time.Second); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gate.called != 0 || creator.calls != 0 {
		t.Errorf("expected no gate/creator calls for maxActions=0, got gate=%d creator=%d", gate.called, creator.calls)
	}
}
