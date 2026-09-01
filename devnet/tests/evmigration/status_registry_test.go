package main

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestStatusRegistry points *flagFile at a temp accounts file so
// statusRegistryFile() resolves to <tmp>/accounts.json, then writes the given
// entries there. Restores the flag on cleanup.
func writeTestStatusRegistry(t *testing.T, accounts []statusRegistryAccount) string {
	t.Helper()
	dir := t.TempDir()
	prev := *flagFile
	*flagFile = filepath.Join(dir, "accounts-devnet.json")
	t.Cleanup(func() { *flagFile = prev })

	registryFile := filepath.Join(dir, "accounts.json")
	data, err := json.Marshal(accounts)
	if err != nil {
		t.Fatalf("marshal registry: %v", err)
	}
	if err := os.WriteFile(registryFile, data, 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	return registryFile
}

// captureLog redirects the standard logger to a buffer for the duration of
// the test and returns it.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func TestUpdateStatusRegistryAddressUpdatesTrackedAccount(t *testing.T) {
	registryFile := writeTestStatusRegistry(t, []statusRegistryAccount{
		{Name: "governance_key", Address: "lumera1old", Mnemonic: "m"},
	})

	updateStatusRegistryAddress("governance_key", "lumera1new")

	data, err := os.ReadFile(registryFile)
	if err != nil {
		t.Fatalf("read registry: %v", err)
	}
	var accounts []statusRegistryAccount
	if err := json.Unmarshal(data, &accounts); err != nil {
		t.Fatalf("parse registry: %v", err)
	}
	if len(accounts) != 1 || accounts[0].Address != "lumera1new" {
		t.Fatalf("expected governance_key address updated to lumera1new, got %+v", accounts)
	}
}

// Generated pre-evm-* fixtures are tracked in accounts-devnet.json, never in
// the per-host status registry; skipping them must not spam WARN logs.
func TestUpdateStatusRegistryAddressSilentlySkipsUntrackedAccount(t *testing.T) {
	registryFile := writeTestStatusRegistry(t, []statusRegistryAccount{
		{Name: "governance_key", Address: "lumera1old", Mnemonic: "m"},
	})
	before, err := os.ReadFile(registryFile)
	if err != nil {
		t.Fatalf("read registry: %v", err)
	}
	buf := captureLog(t)

	updateStatusRegistryAddress("pre-evm-val5-003", "lumera1new")

	if out := buf.String(); strings.Contains(out, "WARN") {
		t.Fatalf("expected no WARN for untracked account, got log output: %q", out)
	}
	after, err := os.ReadFile(registryFile)
	if err != nil {
		t.Fatalf("read registry: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("registry file changed for untracked account:\nbefore: %s\nafter: %s", before, after)
	}
}

func TestLookupStatusRegistryMnemonicFound(t *testing.T) {
	writeTestStatusRegistry(t, []statusRegistryAccount{
		{Name: "sncli-account", Address: "lumera1abc", Mnemonic: " word1 word2 "},
	})

	mnemonic, found := lookupStatusRegistryMnemonic("sncli-account")
	if !found || mnemonic != "word1 word2" {
		t.Fatalf("lookupStatusRegistryMnemonic = (%q, %v), want (\"word1 word2\", true)", mnemonic, found)
	}
}

// Infrastructure-key probes check hosts that legitimately don't have the key
// (e.g. governance_key on a secondary validator); the lookup must stay silent.
func TestLookupStatusRegistryMnemonicNotFoundIsSilent(t *testing.T) {
	writeTestStatusRegistry(t, []statusRegistryAccount{
		{Name: "supernova_validator_5_key", Address: "lumera1abc", Mnemonic: "m"},
	})
	buf := captureLog(t)

	mnemonic, found := lookupStatusRegistryMnemonic("governance_key")
	if found || mnemonic != "" {
		t.Fatalf("lookupStatusRegistryMnemonic = (%q, %v), want (\"\", false)", mnemonic, found)
	}
	if out := buf.String(); strings.Contains(out, "WARN") {
		t.Fatalf("expected no WARN for absent probe candidate, got log output: %q", out)
	}
}
