package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type statusRegistryAccount struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	Mnemonic string `json:"mnemonic"`
}

func statusRegistryFile() string {
	return filepath.Join(filepath.Dir(*flagFile), "accounts.json")
}

func loadStatusRegistryAccounts() ([]statusRegistryAccount, error) {
	data, err := os.ReadFile(statusRegistryFile())
	if err != nil {
		return nil, err
	}
	var accounts []statusRegistryAccount
	if err := json.Unmarshal(data, &accounts); err != nil {
		return nil, err
	}
	return accounts, nil
}

// lookupStatusRegistryMnemonic reports whether `name` is tracked in the
// status registry, without logging when it isn't. Absence is a normal outcome
// when probing infrastructure-key candidates that don't apply to this host
// (e.g. governance_key on a secondary validator).
func lookupStatusRegistryMnemonic(name string) (string, bool) {
	accounts, err := loadStatusRegistryAccounts()
	if err != nil {
		log.Printf("  WARN: cannot read account registry %s: %v", statusRegistryFile(), err)
		return "", false
	}
	for _, account := range accounts {
		if account.Name == name {
			return strings.TrimSpace(account.Mnemonic), true
		}
	}
	return "", false
}

// readStatusRegistryMnemonic is the lookup for accounts that are expected to
// be registered (validator keys); it warns when the entry is missing.
func readStatusRegistryMnemonic(name string) string {
	mnemonic, found := lookupStatusRegistryMnemonic(name)
	if !found {
		log.Printf("  WARN: account %q not found in status registry %s", name, statusRegistryFile())
	}
	return mnemonic
}

func updateStatusRegistryAddress(name, newAddr string) {
	registryFile := statusRegistryFile()
	data, err := os.ReadFile(registryFile)
	if err != nil {
		log.Printf("  WARN: cannot read account registry %s: %v", registryFile, err)
		return
	}

	var accounts []map[string]any
	if err := json.Unmarshal(data, &accounts); err != nil {
		log.Printf("  WARN: cannot parse account registry %s: %v", registryFile, err)
		return
	}

	updated := false
	for _, account := range accounts {
		if fmtName, _ := account["name"].(string); fmtName == name {
			account["address"] = newAddr
			updated = true
			break
		}
	}
	if !updated {
		// Not tracked: the registry only holds infrastructure keys (validator,
		// governance, funders); generated pre-evm-* fixtures live solely in
		// accounts-devnet.json, so skipping them silently is the normal case.
		return
	}

	encoded, err := json.MarshalIndent(accounts, "", "  ")
	if err != nil {
		log.Printf("  WARN: cannot encode updated account registry %s: %v", registryFile, err)
		return
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(registryFile, encoded, 0o644); err != nil {
		log.Printf("  WARN: failed to update account registry %s: %v", registryFile, err)
		return
	}
	log.Printf("  updated account registry address for %s in %s", name, registryFile)
}

func updateStatusRegistryMigratedAccount(name, newAddr string) {
	updateStatusRegistryAddress(name, newAddr)
}
