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

func readStatusRegistryMnemonic(name string) string {
	accounts, err := loadStatusRegistryAccounts()
	if err != nil {
		log.Printf("  WARN: cannot read account registry %s: %v", statusRegistryFile(), err)
		return ""
	}
	for _, account := range accounts {
		if account.Name == name {
			return strings.TrimSpace(account.Mnemonic)
		}
	}
	log.Printf("  WARN: account %q not found in status registry %s", name, statusRegistryFile())
	return ""
}

// appendStatusRegistryAccount adds a {name, address, mnemonic} entry to the
// shared status registry if it isn't already present. Idempotent by name.
func appendStatusRegistryAccount(name, address, mnemonic string) {
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
	for _, account := range accounts {
		if fmtName, _ := account["name"].(string); fmtName == name {
			return
		}
	}
	accounts = append(accounts, map[string]any{
		"name":     name,
		"address":  address,
		"mnemonic": mnemonic,
	})
	encoded, err := json.MarshalIndent(accounts, "", "  ")
	if err != nil {
		log.Printf("  WARN: cannot encode updated account registry %s: %v", registryFile, err)
		return
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(registryFile, encoded, 0o644); err != nil {
		log.Printf("  WARN: failed to append to account registry %s: %v", registryFile, err)
		return
	}
	log.Printf("  appended %s to account registry %s", name, registryFile)
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
		log.Printf("  WARN: account %q not found in status registry %s", name, registryFile)
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
