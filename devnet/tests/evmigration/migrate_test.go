package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMigratedAccountBaseName(t *testing.T) {
	cases := map[string]string{
		"pre-evm-val1-000":                     "evm-val1-000",
		"pre-evmex-val1-003":                   "evmex-val1-003",
		"evm_test_val1_000":                    "evm-val1-000",
		"evm_testex_val1_004":                  "evmex-val1-004",
		"legacy_000":                           "evm-000",
		"extra_000":                            "evmex-000",
		"prepare-funder-supernova_validator_2": "evm-prepare-funder-supernova-validator-2",
		"custom_name_example":                  "evm-custom-name-example",
	}

	for input, want := range cases {
		got := migratedAccountBaseName(input, true)
		if input == "extra_000" || input == "pre-evmex-val1-003" || input == "evm_testex_val1_004" {
			got = migratedAccountBaseName(input, false)
		}
		if got != want {
			t.Fatalf("migratedAccountBaseName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestUpdateStatusRegistryMigratedAccountKeepsNameAndMnemonic(t *testing.T) {
	dir := t.TempDir()
	accountsPath := filepath.Join(dir, "evmigration-accounts.json")
	registryPath := filepath.Join(dir, "accounts.json")
	oldFlagFile := *flagFile
	*flagFile = accountsPath
	t.Cleanup(func() { *flagFile = oldFlagFile })

	const mnemonic = "fixture mnemonic preserved by registry update"
	const legacyAddress = "lumera1zh6a6hz52plxdmrwe57u0m7wc8knflkl8ku8w9"
	const newAddress = "lumera12yjcrfn3kueqrnjcjfrezkjquven2rxd257qxs"
	if err := os.WriteFile(registryPath, []byte(`[
  {
    "name": "governance_key",
    "address": "`+legacyAddress+`",
    "mnemonic": "`+mnemonic+`",
    "type": "cosmos"
  }
]`), 0o644); err != nil {
		t.Fatalf("write registry fixture: %v", err)
	}

	updateStatusRegistryMigratedAccount("governance_key", newAddress)

	data, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("read registry: %v", err)
	}
	var accounts []statusRegistryAccount
	if err := json.Unmarshal(data, &accounts); err != nil {
		t.Fatalf("parse registry: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if accounts[0].Name != "governance_key" {
		t.Fatalf("name changed: got %q", accounts[0].Name)
	}
	if accounts[0].Address != newAddress {
		t.Fatalf("address = %q, want %q", accounts[0].Address, newAddress)
	}
	if accounts[0].Mnemonic != mnemonic {
		t.Fatalf("mnemonic changed: got %q", accounts[0].Mnemonic)
	}
}
