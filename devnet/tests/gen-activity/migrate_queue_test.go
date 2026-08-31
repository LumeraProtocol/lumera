package main

import (
	"testing"

	"gen/tests/common"
)

func TestMigrationKindOf(t *testing.T) {
	single := legacyRec("gen-0001", "lumera1a", "seed")
	if got := migrationKindOf(single); got != migrationKindSingleSig {
		t.Errorf("single-sig kind = %q, want %q", got, migrationKindSingleSig)
	}

	ms := &AccountRecord{AccountIdentity: common.AccountIdentity{Name: "ms", Address: "lumera1ms", KeyStyle: "legacy"}}
	ms.Multisig = &MultisigInfo{MemberNames: []string{"a", "b", "c"}, Threshold: 2, Signers: 3}
	if got := migrationKindOf(ms); got != migrationKindMultisig {
		t.Errorf("multisig kind = %q, want %q", got, migrationKindMultisig)
	}
}

func TestMigrationAccountType(t *testing.T) {
	reg := legacyRec("gen-0001", "lumera1a", "seed")
	if got := migrationAccountType(reg); got != "regular" {
		t.Errorf("account type = %q, want regular", got)
	}

	vest := legacyRec("gen-0002", "lumera1b", "seed")
	vest.Vesting = &VestingInfo{Type: "continuous", LockedAmount: "100ulume"}
	if got := migrationAccountType(vest); got != "continuous" {
		t.Errorf("vesting account type = %q, want continuous", got)
	}

	ms := &AccountRecord{AccountIdentity: common.AccountIdentity{Name: "ms", KeyStyle: "legacy"}}
	ms.Multisig = &MultisigInfo{MemberNames: []string{"a", "b", "c"}, Threshold: 2, Signers: 3}
	if got := migrationAccountType(ms); got != "multisig-2-of-3" {
		t.Errorf("multisig account type = %q, want multisig-2-of-3", got)
	}
}

func TestBuildMigrationQueue(t *testing.T) {
	reg := NewRegistry("lumera-devnet-1", "funder", "lumera1funder", "legacy", "2026-06-16T00:00:00Z")
	reg.UpsertAccount(legacyRec("gen-0001", "lumera1a", "seed"))
	ms := &AccountRecord{AccountIdentity: common.AccountIdentity{Name: "gen-msig-0001", Address: "lumera1ms", Mnemonic: "", KeyStyle: "legacy"}}
	ms.Multisig = &MultisigInfo{MemberNames: []string{"a", "b", "c"}, Threshold: 2, Signers: 3}
	reg.UpsertAccount(ms)

	q := buildMigrationQueue(reg)
	if len(q) != 2 {
		t.Fatalf("queue length = %d, want 2", len(q))
	}
	if q[0].Index != 1 || q[1].Index != 2 {
		t.Errorf("indices = %d,%d, want 1,2", q[0].Index, q[1].Index)
	}
	if q[0].CorrelationID != "migrate gen-0001 #1" {
		t.Errorf("correlation id = %q, want 'migrate gen-0001 #1'", q[0].CorrelationID)
	}
	if q[0].Kind != migrationKindSingleSig || q[1].Kind != migrationKindMultisig {
		t.Errorf("kinds = %q,%q", q[0].Kind, q[1].Kind)
	}
}
