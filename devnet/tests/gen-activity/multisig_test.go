package main

import (
	"reflect"
	"testing"
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
