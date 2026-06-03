package main

import (
	"bytes"
	"flag"
	"strings"
	"testing"
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
