package main

import (
	"testing"

	"gen/tests/common"
)

// sendCall captures one SendFromFunder invocation.
type sendCall struct {
	accNum, seq uint64
	to, amount  string
}

// fakeFundingChain is a controllable fundingChain for testing the batcher.
type fakeFundingChain struct {
	accNum uint64
	// seqByAttempt supplies the funder's start sequence per FunderAccountSequence call.
	seqByAttempt []uint64
	seqCalls     int
	// fundedAfterAttempt[addr] = the attempt index (1-based) at which the address
	// becomes funded. 0 means never.
	fundedAfterAttempt map[string]int
	attempt            int

	sends     []sendCall
	waitCalls int
}

func (f *fakeFundingChain) FunderAccountSequence() (uint64, uint64, error) {
	seq := f.seqByAttempt[min(f.seqCalls, len(f.seqByAttempt)-1)]
	f.seqCalls++
	f.attempt++
	return f.accNum, seq, nil
}

func (f *fakeFundingChain) SendFromFunder(accNum, seq uint64, to, amount string) error {
	f.sends = append(f.sends, sendCall{accNum, seq, to, amount})
	return nil
}

func (f *fakeFundingChain) WaitForNextBlock() error {
	f.waitCalls++
	return nil
}

func (f *fakeFundingChain) IsFunded(addr string) (bool, error) {
	n, ok := f.fundedAfterAttempt[addr]
	return ok && n != 0 && f.attempt >= n, nil
}

func fixedAmount(*AccountRecord) string { return "100ulume" }

func targets(addrs ...string) []*AccountRecord {
	var recs []*AccountRecord
	for _, a := range addrs {
		recs = append(recs, &AccountRecord{AccountIdentity: common.AccountIdentity{Name: a, Address: a}})
	}
	return recs
}

func TestFundAccountsHappyPath(t *testing.T) {
	chain := &fakeFundingChain{
		accNum:             10,
		seqByAttempt:       []uint64{5},
		fundedAfterAttempt: map[string]int{"a": 1, "b": 1, "c": 1},
	}
	recs := targets("a", "b", "c")

	funded, err := FundAccounts(chain, recs, fixedAmount, 10, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if funded != 3 {
		t.Errorf("funded = %d, want 3", funded)
	}
	// Sequences must be contiguous from the funder start sequence.
	wantSeqs := []uint64{5, 6, 7}
	for i, c := range chain.sends {
		if c.accNum != 10 || c.seq != wantSeqs[i] {
			t.Errorf("send[%d] = accNum %d seq %d, want 10/%d", i, c.accNum, c.seq, wantSeqs[i])
		}
		if c.amount != "100ulume" {
			t.Errorf("send[%d] amount = %q, want 100ulume", i, c.amount)
		}
	}
	for _, r := range recs {
		if !r.Funded || !r.HasBalance {
			t.Errorf("account %s not marked funded", r.Name)
		}
	}
}

func TestFundAccountsRetriesUnconfirmedWithRefreshedSequence(t *testing.T) {
	// All three confirm only on the second attempt; the funder sequence advances
	// between attempts and the retry must use the refreshed value.
	chain := &fakeFundingChain{
		accNum:             10,
		seqByAttempt:       []uint64{5, 8},
		fundedAfterAttempt: map[string]int{"a": 2, "b": 2, "c": 2},
	}
	recs := targets("a", "b", "c")

	funded, err := FundAccounts(chain, recs, fixedAmount, 10, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if funded != 3 {
		t.Errorf("funded = %d, want 3", funded)
	}
	// 3 sends in attempt 1 (seq 5,6,7) + 3 in attempt 2 (seq 8,9,10).
	if len(chain.sends) != 6 {
		t.Fatalf("sends = %d, want 6", len(chain.sends))
	}
	if chain.sends[3].seq != 8 || chain.sends[5].seq != 10 {
		t.Errorf("retry sequences = %d..%d, want 8..10", chain.sends[3].seq, chain.sends[5].seq)
	}
}

func TestFundAccountsGivesUpAfterMaxAttempts(t *testing.T) {
	// "b" never funds.
	chain := &fakeFundingChain{
		accNum:             10,
		seqByAttempt:       []uint64{5, 8, 11},
		fundedAfterAttempt: map[string]int{"a": 1, "b": 0, "c": 1},
	}
	recs := targets("a", "b", "c")

	funded, err := FundAccounts(chain, recs, fixedAmount, 10, 2)
	if err == nil {
		t.Fatal("expected error for unfundable account, got nil")
	}
	if funded != 2 {
		t.Errorf("funded = %d, want 2", funded)
	}
	for _, r := range recs {
		if r.Name == "b" && r.Funded {
			t.Error("account b should not be marked funded")
		}
	}
}

func TestFundAccountsNoTargets(t *testing.T) {
	chain := &fakeFundingChain{accNum: 10, seqByAttempt: []uint64{5}}
	funded, err := FundAccounts(chain, nil, fixedAmount, 10, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if funded != 0 {
		t.Errorf("funded = %d, want 0", funded)
	}
	if len(chain.sends) != 0 || chain.seqCalls != 0 {
		t.Error("expected no chain interaction for empty targets")
	}
}

// Ensure the fake satisfies the interface the batcher depends on.
var _ fundingChain = (*fakeFundingChain)(nil)
