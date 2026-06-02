package main

import (
	"os"
	"path/filepath"
	"testing"

	"gen/tests/common"
)

func newRec(name, addr string) *AccountRecord {
	return &AccountRecord{AccountIdentity: common.AccountIdentity{Name: name, Address: addr}}
}

func TestRegistryUpsertAndRoundTrip(t *testing.T) {
	reg := NewRegistry("lumera-devnet-1", "funder", "lumera1funder", "evm", "2026-06-02T00:00:00Z")

	reg.UpsertAccount(newRec("gen-0001", "lumera1a"))

	// Upsert with the same name+address must update in place, not append.
	updated := newRec("gen-0001", "lumera1a")
	updated.Funded = true
	reg.UpsertAccount(updated)
	if len(reg.Accounts) != 1 {
		t.Fatalf("accounts after in-place upsert = %d, want 1", len(reg.Accounts))
	}
	if !reg.Accounts[0].Funded {
		t.Error("expected in-place upsert to update Funded")
	}

	// A different account appends.
	reg.UpsertAccount(newRec("gen-0002", "lumera1b"))
	if len(reg.Accounts) != 2 {
		t.Fatalf("accounts after second upsert = %d, want 2", len(reg.Accounts))
	}

	// Save then reload yields an equivalent registry.
	path := filepath.Join(t.TempDir(), "accounts.json")
	if err := reg.Save(path, "2026-06-02T01:00:00Z"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if got.ChainID != "lumera-devnet-1" || got.FunderKey != "funder" || got.KeyStyle != "evm" {
		t.Errorf("reloaded envelope mismatch: %+v", got)
	}
	if got.UpdatedAt != "2026-06-02T01:00:00Z" {
		t.Errorf("UpdatedAt = %q, want the Save timestamp", got.UpdatedAt)
	}
	if len(got.Accounts) != 2 {
		t.Errorf("reloaded accounts = %d, want 2", len(got.Accounts))
	}
}

func TestLoadRegistryRejectsUnparseable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")
	if err := os.WriteFile(path, []byte("not json{"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if _, err := LoadRegistry(path); err == nil {
		t.Error("expected error loading unparseable registry, got nil")
	}
}

func TestLoadRegistryRejectsWrongSchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")
	// This is valid JSON and resembles an evmigration registry envelope, but it
	// is not a gen-activity registry and must not be silently overwritten.
	data := []byte(`{"chain_id":"lumera-devnet-1","funder":"validator","accounts":[]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if _, err := LoadRegistry(path); err == nil {
		t.Error("expected error loading registry without schema_version, got nil")
	}

	data = []byte(`{"schema_version":2,"chain_id":"lumera-devnet-1","accounts":[]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if _, err := LoadRegistry(path); err == nil {
		t.Error("expected error loading unsupported schema_version, got nil")
	}
}

func TestLoadRegistryMissingFileIsDistinguishable(t *testing.T) {
	_, err := LoadRegistry(filepath.Join(t.TempDir(), "absent.json"))
	if !os.IsNotExist(err) {
		t.Errorf("expected os.IsNotExist error for missing registry, got %v", err)
	}
}

func TestAllocateNames(t *testing.T) {
	reg := NewRegistry("c", "f", "addr", "evm", "t0")

	names := reg.AllocateNames("gen", 3)
	want := []string{"gen-0001", "gen-0002", "gen-0003"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names = %v, want %v", names, want)
		}
	}

	for _, n := range names {
		reg.UpsertAccount(newRec(n, n))
	}

	// Subsequent allocation continues past the highest existing index.
	more := reg.AllocateNames("gen", 2)
	wantMore := []string{"gen-0004", "gen-0005"}
	for i := range wantMore {
		if more[i] != wantMore[i] {
			t.Fatalf("more = %v, want %v", more, wantMore)
		}
	}
}
