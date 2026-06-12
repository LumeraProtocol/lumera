package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const sampleTOML = `
[common]
bin = "lumerad"
funding-key = "faucet"
parallelism = 8

[chains.devnet]
rpc = "tcp://localhost:26657"
chain-id = "lumera-devnet-1"
accounts = "accounts-devnet.json"

[chains.testnet]
rpc = "https://rpc.testnet:443"
chain-id = "lumera-testnet-1"
`

func writeTempTOML(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gen-activity-config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write temp toml: %v", err)
	}
	return path
}

func TestLoadFileConfigMissingFileReturnsNil(t *testing.T) {
	fc, err := LoadFileConfig(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}
	if fc != nil {
		t.Fatalf("expected nil FileConfig for missing file, got %+v", fc)
	}
}

func TestLoadFileConfigParsesSections(t *testing.T) {
	fc, err := LoadFileConfig(writeTempTOML(t, sampleTOML))
	if err != nil {
		t.Fatalf("LoadFileConfig: %v", err)
	}
	if fc == nil {
		t.Fatal("expected non-nil FileConfig")
	}
	if fc.Common.FundingKey == nil || *fc.Common.FundingKey != "faucet" {
		t.Errorf("common funding-key not parsed: %+v", fc.Common.FundingKey)
	}
	if fc.Common.Parallelism == nil || *fc.Common.Parallelism != 8 {
		t.Errorf("common parallelism not parsed: %+v", fc.Common.Parallelism)
	}
	dev, ok := fc.Chains["devnet"]
	if !ok {
		t.Fatal("devnet chain section missing")
	}
	if dev.ChainID == nil || *dev.ChainID != "lumera-devnet-1" {
		t.Errorf("devnet chain-id not parsed: %+v", dev.ChainID)
	}
	if want := []string{"devnet", "testnet"}; !reflect.DeepEqual(fc.ChainNames(), want) {
		t.Errorf("ChainNames() = %v, want %v (sorted)", fc.ChainNames(), want)
	}
}

func TestLoadFileConfigRejectsUnknownKey(t *testing.T) {
	_, err := LoadFileConfig(writeTempTOML(t, "[common]\nbogus-key = 1\n"))
	if err == nil {
		t.Error("expected error for unknown TOML key (strict decoding)")
	}
}
