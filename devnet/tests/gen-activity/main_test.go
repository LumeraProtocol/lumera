package main

import (
	"bytes"
	"flag"
	"path/filepath"
	"strings"
	"testing"

	"gen/tests/common"
)

func TestHelpUsageIncludesDescription(t *testing.T) {
	var cfg Config
	var out bytes.Buffer
	fs := flag.NewFlagSet("tests-gen-activity", flag.ContinueOnError)
	fs.SetOutput(&out)

	configureFlags(fs, &cfg)
	fs.Usage()

	help := out.String()
	for _, want := range []string{
		"tests-gen-activity generates realistic account activity against a live Lumera devnet chain.",
		"Usage: tests-gen-activity [flags]",
		"-activity-existing",
		"-add-accounts",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help output missing %q:\n%s", want, help)
		}
	}
}

func TestConfigureFlagsRegistersNewFlags(t *testing.T) {
	var cfg Config
	fs := flag.NewFlagSet("tests-gen-activity", flag.ContinueOnError)
	configureFlags(fs, &cfg)

	for _, name := range []string{
		"config", "chain", "wizard", "w",
		"num-multisig23-accounts", "num-multisig35-accounts",
	} {
		if fs.Lookup(name) == nil {
			t.Errorf("flag -%s not registered", name)
		}
	}

	if got := fs.Lookup("config").DefValue; got != "gen-activity-config.toml" {
		t.Errorf("-config default = %q, want gen-activity-config.toml", got)
	}
}

func TestDetectKeyStyleFallsBackToLegacy(t *testing.T) {
	got := detectKeyStyle(filepath.Join(t.TempDir(), "missing-lumerad"), "v1.11.0")
	if got != common.KeyStyleLegacy {
		t.Fatalf("detectKeyStyle fallback = %+v, want legacy", got)
	}
}

func TestResolveConfigAppliesFileAndDetectsWizard(t *testing.T) {
	t.Run("no flags -> wizard mode, common applied", func(t *testing.T) {
		cfg := &Config{}
		fs := flag.NewFlagSet("gen", flag.ContinueOnError)
		configureFlags(fs, cfg)
		if err := fs.Parse([]string{}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		path := writeTempTOML(t, sampleTOML)
		cfg.ConfigPath = path

		wizard, _, err := resolveConfig(cfg, fs)
		if err != nil {
			t.Fatalf("resolveConfig: %v", err)
		}
		if !wizard {
			t.Error("expected wizard=true when no flags passed")
		}
		if cfg.FundingKey != "faucet" {
			t.Errorf("FundingKey = %q, want faucet (from common)", cfg.FundingKey)
		}
	})

	t.Run("flags passed -> command-line mode", func(t *testing.T) {
		cfg := &Config{}
		fs := flag.NewFlagSet("gen", flag.ContinueOnError)
		configureFlags(fs, cfg)
		path := writeTempTOML(t, sampleTOML)
		if err := fs.Parse([]string{"-chain", "devnet", "-config", path}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		wizard, _, err := resolveConfig(cfg, fs)
		if err != nil {
			t.Fatalf("resolveConfig: %v", err)
		}
		if wizard {
			t.Error("expected wizard=false when flags passed")
		}
		if cfg.ChainID != "lumera-devnet-1" {
			t.Errorf("ChainID = %q, want lumera-devnet-1 (chain layered)", cfg.ChainID)
		}
	})

	t.Run("-w forces wizard even with flags", func(t *testing.T) {
		cfg := &Config{}
		fs := flag.NewFlagSet("gen", flag.ContinueOnError)
		configureFlags(fs, cfg)
		path := writeTempTOML(t, sampleTOML)
		if err := fs.Parse([]string{"-w", "-config", path}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		wizard, _, err := resolveConfig(cfg, fs)
		if err != nil {
			t.Fatalf("resolveConfig: %v", err)
		}
		if !wizard {
			t.Error("expected wizard=true when -w passed")
		}
	})

	t.Run("explicit missing -config is an error", func(t *testing.T) {
		cfg := &Config{}
		fs := flag.NewFlagSet("gen", flag.ContinueOnError)
		configureFlags(fs, cfg)
		if err := fs.Parse([]string{"-config", filepath.Join(t.TempDir(), "nope.toml"), "-chain", "devnet"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if _, _, err := resolveConfig(cfg, fs); err == nil {
			t.Error("expected error when explicit -config path is missing")
		}
	})
}
