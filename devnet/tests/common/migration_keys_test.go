package common

import (
	"strings"
	"testing"
)

func TestRecoverKeyArgsEVMStyle(t *testing.T) {
	args := recoverKeyArgs("alice-evm", "test", KeyStyleEVM, "")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"keys add alice-evm",
		"--recover",
		"--coin-type 60",
		"--algo eth_secp256k1",
		"--keyring-backend test",
		"--output json",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("recover args missing %q in:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "--home") {
		t.Errorf("did not expect --home when home empty:\n%s", joined)
	}
}

func TestClaimLegacyAccountArgs(t *testing.T) {
	args := claimLegacyAccountArgs("gen-0001", "gen-0001-evm")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"tx evmigration claim-legacy-account gen-0001 gen-0001-evm",
		"--from gen-0001",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("claim args missing %q in:\n%s", want, joined)
		}
	}
}

func TestRecoverKeyArgsLegacyStyleWithHome(t *testing.T) {
	args := recoverKeyArgs("bob", "os", KeyStyleLegacy, "/root/.lumera")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--coin-type 118",
		"--algo secp256k1",
		"--keyring-backend os",
		"--home /root/.lumera",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("recover args missing %q in:\n%s", want, joined)
		}
	}
}
