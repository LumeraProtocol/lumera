package main

import (
	"path/filepath"
	"testing"

	"gen/tests/common"
)

func legacyRec(name, addr, mnemonic string) *AccountRecord {
	return &AccountRecord{AccountIdentity: common.AccountIdentity{
		Name: name, Address: addr, Mnemonic: mnemonic, KeyStyle: "legacy",
	}}
}

func TestMigrationInfoRoundTrip(t *testing.T) {
	reg := NewRegistry("lumera-devnet-1", "funder", "lumera1funder", "legacy", "2026-06-16T00:00:00Z")
	rec := legacyRec("gen-0001", "lumera1a", "twelve words mnemonic")
	rec.Migration = &MigrationInfo{
		NewName:    "gen-0001-evm",
		NewAddress: "lumera1new",
		TxHash:     "ABC123",
		Height:     64566,
		MigratedAt: "2026-06-16T18:25:04Z",
		Status:     MigrationStatusMigrated,
	}
	reg.UpsertAccount(rec)

	path := filepath.Join(t.TempDir(), "accounts.json")
	if err := reg.Save(path, "2026-06-16T01:00:00Z"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(got.Accounts) != 1 || got.Accounts[0].Migration == nil {
		t.Fatalf("migration metadata not persisted: %+v", got.Accounts)
	}
	m := got.Accounts[0].Migration
	if m.NewAddress != "lumera1new" || m.Status != MigrationStatusMigrated || m.Height != 64566 {
		t.Errorf("migration metadata mismatch after reload: %+v", m)
	}
}

func TestMultisigMembersRoundTrip(t *testing.T) {
	reg := NewRegistry("lumera-devnet-1", "funder", "lumera1funder", "legacy", "t0")
	rec := &AccountRecord{AccountIdentity: common.AccountIdentity{Name: "gen-msig23-0001", Address: "lumera1ms", KeyStyle: "legacy"}}
	rec.Multisig = &MultisigInfo{
		MemberNames: []string{"gen-msig23-0001-signer-1", "gen-msig23-0001-signer-2", "gen-msig23-0001-signer-3"},
		Members: []MultisigMember{
			{Name: "gen-msig23-0001-signer-1", Address: "lumera1m1", Mnemonic: "seed one"},
			{Name: "gen-msig23-0001-signer-2", Address: "lumera1m2", Mnemonic: "seed two"},
			{Name: "gen-msig23-0001-signer-3", Address: "lumera1m3", Mnemonic: "seed three"},
		},
		Threshold: 2, Signers: 3,
	}
	reg.UpsertAccount(rec)

	path := filepath.Join(t.TempDir(), "accounts.json")
	if err := reg.Save(path, "t1"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	m := got.Accounts[0].Multisig
	if m == nil || len(m.Members) != 3 {
		t.Fatalf("members not persisted: %+v", m)
	}
	if m.Members[0].Mnemonic != "seed one" || m.Members[2].Name != "gen-msig23-0001-signer-3" {
		t.Errorf("member material wrong after reload: %+v", m.Members)
	}
}

func TestMultisigMembersMaterial(t *testing.T) {
	t.Run("prefers Members with mnemonics", func(t *testing.T) {
		ms := &MultisigInfo{
			MemberNames: []string{"a", "b"},
			Members:     []MultisigMember{{Name: "a", Mnemonic: "ma"}, {Name: "b", Mnemonic: "mb"}},
			Threshold:   2, Signers: 2,
		}
		got := multisigMembers(ms)
		if len(got) != 2 || got[0].Name != "a" || got[0].Mnemonic != "ma" {
			t.Errorf("material = %+v, want names+mnemonics from Members", got)
		}
	})
	t.Run("falls back to MemberNames without mnemonics", func(t *testing.T) {
		ms := &MultisigInfo{MemberNames: []string{"a", "b", "c"}, Threshold: 2, Signers: 3}
		got := multisigMembers(ms)
		if len(got) != 3 || got[1].Name != "b" || got[1].Mnemonic != "" {
			t.Errorf("material = %+v, want names from MemberNames with empty mnemonics", got)
		}
	})
}

func TestMigrationEligible(t *testing.T) {
	t.Run("legacy single-sig with mnemonic is eligible", func(t *testing.T) {
		if !migrationEligible(legacyRec("gen-0001", "lumera1a", "seed words")) {
			t.Error("expected legacy single-sig with mnemonic to be eligible")
		}
	})

	t.Run("already migrated is not eligible", func(t *testing.T) {
		r := legacyRec("gen-0001", "lumera1a", "seed words")
		r.Migration = &MigrationInfo{Status: MigrationStatusMigrated, NewAddress: "lumera1new"}
		if migrationEligible(r) {
			t.Error("expected already-migrated record to be ineligible")
		}
	})

	t.Run("already_migrated status is not eligible", func(t *testing.T) {
		r := legacyRec("gen-0001", "lumera1a", "seed words")
		r.Migration = &MigrationInfo{Status: MigrationStatusAlreadyMigrated, NewAddress: "lumera1new"}
		if migrationEligible(r) {
			t.Error("expected already_migrated record to be ineligible")
		}
	})

	t.Run("previously failed record is still eligible (retryable)", func(t *testing.T) {
		r := legacyRec("gen-0001", "lumera1a", "seed words")
		r.Migration = &MigrationInfo{Status: MigrationStatusFailed, Error: "boom"}
		if !migrationEligible(r) {
			t.Error("expected failed record to be retryable/eligible")
		}
	})

	t.Run("no mnemonic and not multisig is not eligible", func(t *testing.T) {
		r := &AccountRecord{AccountIdentity: common.AccountIdentity{Name: "x", Address: "lumera1a", KeyStyle: "legacy"}}
		if migrationEligible(r) {
			t.Error("expected record with no signing material to be ineligible")
		}
	})

	t.Run("multisig without top-level mnemonic is eligible", func(t *testing.T) {
		r := &AccountRecord{AccountIdentity: common.AccountIdentity{Name: "ms", Address: "lumera1ms", KeyStyle: "legacy"}}
		r.Multisig = &MultisigInfo{MemberNames: []string{"ms-signer-1", "ms-signer-2", "ms-signer-3"}, Threshold: 2, Signers: 3}
		if !migrationEligible(r) {
			t.Error("expected legacy multisig to be eligible")
		}
	})

	t.Run("non-legacy key style is not eligible", func(t *testing.T) {
		r := legacyRec("gen-0001", "lumera1a", "seed words")
		r.KeyStyle = "evm"
		if migrationEligible(r) {
			t.Error("expected EVM-style record to be ineligible")
		}
	})
}

func TestSelectMigrationCandidates(t *testing.T) {
	reg := NewRegistry("lumera-devnet-1", "funder", "lumera1funder", "legacy", "2026-06-16T00:00:00Z")
	reg.UpsertAccount(legacyRec("gen-0001", "lumera1a", "seed")) // eligible
	migrated := legacyRec("gen-0002", "lumera1b", "seed")        // not eligible (migrated)
	migrated.Migration = &MigrationInfo{Status: MigrationStatusMigrated}
	reg.UpsertAccount(migrated)
	reg.UpsertAccount(legacyRec("gen-0003", "lumera1c", "seed")) // eligible
	evm := legacyRec("gen-0004", "lumera1d", "seed")             // not eligible (evm)
	evm.KeyStyle = "evm"
	reg.UpsertAccount(evm)

	cands := selectMigrationCandidates(reg)
	if len(cands) != 2 {
		t.Fatalf("candidates = %d, want 2", len(cands))
	}
	if cands[0].Name != "gen-0001" || cands[1].Name != "gen-0003" {
		t.Errorf("candidate order/identity wrong: %s, %s", cands[0].Name, cands[1].Name)
	}
}
