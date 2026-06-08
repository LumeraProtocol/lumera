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

func TestDetectKeyStyleFallsBackToLegacy(t *testing.T) {
	got := detectKeyStyle(filepath.Join(t.TempDir(), "missing-lumerad"), "v1.11.0")
	if got != common.KeyStyleLegacy {
		t.Fatalf("detectKeyStyle fallback = %+v, want legacy", got)
	}
}
