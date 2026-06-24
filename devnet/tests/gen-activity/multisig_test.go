package main

import (
	"reflect"
	"testing"

	"gen/tests/common"
)

func TestMultisigPlanFromCounts(t *testing.T) {
	plan := multisigPlan(2, 1)
	want := []multisigSpec{
		{Prefix: "msig23", Threshold: 2, Signers: 3},
		{Prefix: "msig23", Threshold: 2, Signers: 3},
		{Prefix: "msig35", Threshold: 3, Signers: 5},
	}
	if !reflect.DeepEqual(plan, want) {
		t.Errorf("multisigPlan(2,1) = %+v, want %+v", plan, want)
	}
}

func TestMultisigPlanZeroIsEmpty(t *testing.T) {
	if plan := multisigPlan(0, 0); len(plan) != 0 {
		t.Errorf("multisigPlan(0,0) = %+v, want empty", plan)
	}
}

func TestMemberNamesForComposite(t *testing.T) {
	got := memberNames("gen-msig35-0007", 5)
	want := []string{
		"gen-msig35-0007-signer-1", "gen-msig35-0007-signer-2", "gen-msig35-0007-signer-3",
		"gen-msig35-0007-signer-4", "gen-msig35-0007-signer-5",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("memberNames = %v, want %v", got, want)
	}
}

func TestActivityTargetsExcludesMultisig(t *testing.T) {
	reg := NewRegistry("c", "f", "", "legacy", "t")
	reg.Accounts = []*AccountRecord{
		{AccountIdentity: common.AccountIdentity{Name: "reg", Address: "lumera1reg"}, Funded: true},
		{AccountIdentity: common.AccountIdentity{Name: "msig", Address: "lumera1msig"}, Funded: true,
			Multisig: &MultisigInfo{Threshold: 2, Signers: 3}},
	}
	// includeExisting must skip the funded multisig composite (single-sig mix).
	got := activityTargets(reg, nil, true)
	if len(got) != 1 || got[0].Name != "reg" {
		t.Errorf("activityTargets(includeExisting) = %+v, want only the regular account", got)
	}
}

func TestExerciseMultisigRecordsBankSend(t *testing.T) {
	rec := &AccountRecord{
		AccountIdentity: common.AccountIdentity{Name: "gen-msig23-0001", Address: "lumera1msig"},
		Multisig:        &MultisigInfo{MemberNames: []string{"m1", "m2", "m3"}, Threshold: 2, Signers: 3},
		Funded:          true,
	}
	fake := &fakeMultisigExerciser{txHash: "DEAD01"}

	err := exerciseMultisig(fake, rec, "lumera1peer", "5ulume")
	if err != nil {
		t.Fatalf("exerciseMultisig: %v", err)
	}
	if len(rec.BankSends) != 1 || rec.BankSends[0].To != "lumera1peer" {
		t.Fatalf("expected one recorded bank send to peer, got %+v", rec.BankSends)
	}
	if rec.BankSends[0].TxHash != "DEAD01" {
		t.Errorf("recorded tx hash = %q, want DEAD01", rec.BankSends[0].TxHash)
	}
	if !fake.called {
		t.Error("multisig exerciser was not invoked")
	}
}

// fakeMultisigExerciser satisfies multisigExerciser for the test.
type fakeMultisigExerciser struct {
	called bool
	txHash string
}

func (f *fakeMultisigExerciser) MultisigBankSend(rec *AccountRecord, to, amount string) (string, error) {
	f.called = true
	return f.txHash, nil
}
