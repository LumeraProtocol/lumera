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

// TestLoadFileConfigPointerDistinguishesZeroFromAbsent guards the load-bearing
// pointer-field behavior the config layering relies on: a key explicitly set to
// its zero value must yield a non-nil pointer, while an absent key stays nil.
func TestLoadFileConfigPointerDistinguishesZeroFromAbsent(t *testing.T) {
	fc, err := LoadFileConfig(writeTempTOML(t, "[common]\nparallelism = 0\n"))
	if err != nil {
		t.Fatalf("LoadFileConfig: %v", err)
	}
	if fc.Common.Parallelism == nil {
		t.Fatal("explicit parallelism=0 must yield a non-nil pointer (not absent)")
	}
	if *fc.Common.Parallelism != 0 {
		t.Errorf("parallelism = %d, want 0", *fc.Common.Parallelism)
	}
	if fc.Common.Home != nil {
		t.Errorf("absent home must stay nil, got %q", *fc.Common.Home)
	}
}

func TestLoadFileConfigRejectsUnknownKey(t *testing.T) {
	_, err := LoadFileConfig(writeTempTOML(t, "[common]\nbogus-key = 1\n"))
	if err == nil {
		t.Error("expected error for unknown TOML key (strict decoding)")
	}
}

func TestApplyFileConfigPrecedence(t *testing.T) {
	fc, err := LoadFileConfig(writeTempTOML(t, sampleTOML))
	if err != nil {
		t.Fatalf("LoadFileConfig: %v", err)
	}

	t.Run("common then chain overlay onto unset fields", func(t *testing.T) {
		c := &Config{}
		setFlags := map[string]bool{} // nothing set explicitly
		if err := ApplyFileConfig(c, fc, "devnet", setFlags); err != nil {
			t.Fatalf("ApplyFileConfig: %v", err)
		}
		if c.FundingKey != "faucet" { // from [common]
			t.Errorf("FundingKey = %q, want faucet", c.FundingKey)
		}
		if c.Parallelism != 8 { // from [common]
			t.Errorf("Parallelism = %d, want 8", c.Parallelism)
		}
		if c.ChainID != "lumera-devnet-1" { // from [chains.devnet]
			t.Errorf("ChainID = %q, want lumera-devnet-1", c.ChainID)
		}
	})

	t.Run("explicit flags win over config", func(t *testing.T) {
		c := &Config{FundingKey: "cli-funder", Parallelism: 99}
		setFlags := map[string]bool{"funding-key": true, "parallelism": true}
		if err := ApplyFileConfig(c, fc, "devnet", setFlags); err != nil {
			t.Fatalf("ApplyFileConfig: %v", err)
		}
		if c.FundingKey != "cli-funder" {
			t.Errorf("FundingKey = %q, want cli-funder (flag wins)", c.FundingKey)
		}
		if c.Parallelism != 99 {
			t.Errorf("Parallelism = %d, want 99 (flag wins)", c.Parallelism)
		}
		if c.ChainID != "lumera-devnet-1" { // not set as flag → config applies
			t.Errorf("ChainID = %q, want lumera-devnet-1", c.ChainID)
		}
	})

	t.Run("unknown chain errors", func(t *testing.T) {
		c := &Config{}
		err := ApplyFileConfig(c, fc, "nosuchchain", map[string]bool{})
		if err == nil {
			t.Error("expected error for unknown chain name")
		}
	})

	t.Run("empty chain applies only common", func(t *testing.T) {
		c := &Config{}
		if err := ApplyFileConfig(c, fc, "", map[string]bool{}); err != nil {
			t.Fatalf("ApplyFileConfig: %v", err)
		}
		if c.FundingKey != "faucet" {
			t.Errorf("FundingKey = %q, want faucet (common)", c.FundingKey)
		}
		if c.ChainID != "" {
			t.Errorf("ChainID = %q, want empty (no chain selected)", c.ChainID)
		}
	})

	t.Run("chain section overrides common for an overlapping key", func(t *testing.T) {
		overlap, err := LoadFileConfig(writeTempTOML(t, "[common]\nfunding-key = \"common-funder\"\n[chains.devnet]\nfunding-key = \"chain-funder\"\n"))
		if err != nil {
			t.Fatalf("LoadFileConfig: %v", err)
		}
		c := &Config{}
		if err := ApplyFileConfig(c, overlap, "devnet", map[string]bool{}); err != nil {
			t.Fatalf("ApplyFileConfig: %v", err)
		}
		if c.FundingKey != "chain-funder" {
			t.Errorf("FundingKey = %q, want chain-funder (chain layer must override common)", c.FundingKey)
		}
	})
}
