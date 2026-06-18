package common

import (
	"strings"
	"testing"
)

func TestProofFlowArgBuilders(t *testing.T) {
	t.Run("generate-proof-payload", func(t *testing.T) {
		args := generateProofPayloadArgs("lumera1legacy", "claim", "/tmp/proof.json", "evm-ms-signer-1,evm-ms-signer-2", 2, "lumera-devnet-1", "test", "/root/.lumera")
		joined := strings.Join(args, " ")
		for _, want := range []string{
			"tx evmigration generate-proof-payload",
			"--legacy lumera1legacy",
			"--kind claim",
			"--out /tmp/proof.json",
			"--new-sub-pub-keys evm-ms-signer-1,evm-ms-signer-2",
			"--new-threshold 2",
			// --chain-id is REQUIRED: without it the payload is built with the
			// wrong chain id (the bech32 prefix "lumera"), so every proof
			// signature fails on-chain verification.
			"--chain-id lumera-devnet-1",
			"--keyring-backend test",
			"--home /root/.lumera",
		} {
			if !strings.Contains(joined, want) {
				t.Errorf("generate args missing %q in:\n%s", want, joined)
			}
		}
	})

	t.Run("sign-proof signs both sides", func(t *testing.T) {
		args := signProofArgs("/tmp/proof.json", "ms-signer-1", "evm-ms-signer-1", "/tmp/partial-1.json", "lumera-devnet-1", "test", "")
		joined := strings.Join(args, " ")
		for _, want := range []string{
			"tx evmigration sign-proof /tmp/proof.json",
			"--from ms-signer-1",
			"--new-key evm-ms-signer-1",
			"--out /tmp/partial-1.json",
			"--chain-id lumera-devnet-1",
		} {
			if !strings.Contains(joined, want) {
				t.Errorf("sign args missing %q in:\n%s", want, joined)
			}
		}
		if strings.Contains(joined, "--home") {
			t.Errorf("did not expect --home when home empty:\n%s", joined)
		}
	})

	t.Run("combine-proof lists all partials", func(t *testing.T) {
		args := combineProofArgs([]string{"/tmp/p1.json", "/tmp/p2.json"}, "/tmp/tx.json")
		joined := strings.Join(args, " ")
		for _, want := range []string{
			"tx evmigration combine-proof /tmp/p1.json /tmp/p2.json",
			"--out /tmp/tx.json",
		} {
			if !strings.Contains(joined, want) {
				t.Errorf("combine args missing %q in:\n%s", want, joined)
			}
		}
	})

	t.Run("submit-proof", func(t *testing.T) {
		args := submitProofArgs("/tmp/tx.json", "lumera-devnet-1", "tcp://localhost:26657", "test")
		joined := strings.Join(args, " ")
		for _, want := range []string{
			"tx evmigration submit-proof /tmp/tx.json",
			"--chain-id lumera-devnet-1",
			"--node tcp://localhost:26657",
			"--keyring-backend test",
			"--yes",
		} {
			if !strings.Contains(joined, want) {
				t.Errorf("submit args missing %q in:\n%s", want, joined)
			}
		}
	})
}

func TestEVMMultisigMemberNames(t *testing.T) {
	got := evmMultisigMemberNames("gen-msig35-0001", 5)
	want := []string{
		"evm-gen-msig35-0001-signer-1", "evm-gen-msig35-0001-signer-2", "evm-gen-msig35-0001-signer-3",
		"evm-gen-msig35-0001-signer-4", "evm-gen-msig35-0001-signer-5",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d names, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("name[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if name := evmMultisigCompositeName("gen-msig35-0001"); name != "evm-gen-msig35-0001" {
		t.Errorf("composite name = %q, want evm-gen-msig35-0001", name)
	}
}
