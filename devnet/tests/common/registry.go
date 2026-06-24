package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AccountIdentity is the stable per-account identity shared between the
// migration tooling and the activity generator. Tool-specific account records
// embed it and add their own activity/metadata fields.
type AccountIdentity struct {
	Name      string `json:"name"`
	Mnemonic  string `json:"mnemonic"`
	Address   string `json:"address"`
	PubKeyB64 string `json:"pubkey_b64,omitempty"`
	// KeyStyle records the style this account was created with; it can lag the
	// runtime's current style when a registry is reused across an EVM cutover.
	KeyStyle string `json:"key_style,omitempty"`
}

// AtomicWriteJSON marshals v as indented JSON and writes it to path atomically:
// it writes to a temp file in the same directory, then renames over the target
// so a crash mid-write never leaves a partially written registry.
func AtomicWriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if we fail before the rename succeeds.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// ReadJSON reads and unmarshals JSON from path into v.
func ReadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}
