package common

import (
	"testing"
)

func testMultisig() *Multisig {
	cli := &ChainCLI{
		Bin: "lumerad", ChainID: "lumera-devnet-1", RPC: "tcp://localhost:26657",
		Home: "/home/u/.lumera", KeyringBackend: "test",
		Gas: "500000", GasPrices: "0.025ulume",
	}
	return NewMultisig(cli)
}

func contains(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestKeyAddArgsUsesNosortAndThreshold(t *testing.T) {
	m := testMultisig()
	args := m.keyAddArgs("msig", []string{"s1", "s2", "s3"}, 2)
	if !contains(args, "--nosort") {
		t.Errorf("keys add multisig must pass --nosort: %v", args)
	}
	if !contains(args, "--multisig") || !contains(args, "s1,s2,s3") {
		t.Errorf("keys add must pass --multisig members joined: %v", args)
	}
	if !contains(args, "--multisig-threshold") || !contains(args, "2") {
		t.Errorf("keys add must pass --multisig-threshold 2: %v", args)
	}
	if !contains(args, "--home") || !contains(args, "/home/u/.lumera") {
		t.Errorf("keys add must pass --home when set: %v", args)
	}
}

func TestSignArgsUsesAminoJSONAndMultisig(t *testing.T) {
	m := testMultisig()
	args := m.signArgs("/tmp/unsigned.json", "s1", "lumera1msig", 7, 3)
	if !contains(args, "--sign-mode") || !contains(args, "amino-json") {
		t.Errorf("sign must use amino-json: %v", args)
	}
	if !contains(args, "--multisig") || !contains(args, "lumera1msig") {
		t.Errorf("sign must reference the multisig address: %v", args)
	}
	if !contains(args, "--from") || !contains(args, "s1") {
		t.Errorf("sign must set --from member: %v", args)
	}
	if !contains(args, "--account-number") || !contains(args, "7") ||
		!contains(args, "--sequence") || !contains(args, "3") {
		t.Errorf("sign must pass offline account-number/sequence: %v", args)
	}
}

func TestMultisignArgsConsumesSigFiles(t *testing.T) {
	m := testMultisig()
	args := m.multisignArgs("/tmp/unsigned.json", "msig", []string{"/tmp/s1.json", "/tmp/s2.json"})
	for _, want := range []string{"multisign", "/tmp/unsigned.json", "msig", "/tmp/s1.json", "/tmp/s2.json"} {
		if !contains(args, want) {
			t.Errorf("multisign args missing %q: %v", want, args)
		}
	}
}

func TestGenBankSendArgsIsGenerateOnly(t *testing.T) {
	m := testMultisig()
	args := m.GenBankSendArgs("msig", "lumera1msig", "lumera1peer", "5ulume", 7, 3)
	if !contains(args, "--generate-only") {
		t.Errorf("unsigned tx must be generate-only: %v", args)
	}
	for _, want := range []string{"send", "lumera1msig", "lumera1peer", "5ulume"} {
		if !contains(args, want) {
			t.Errorf("bank send args missing %q: %v", want, args)
		}
	}
}

func TestBroadcastArgsSyncMode(t *testing.T) {
	m := testMultisig()
	args := m.broadcastArgs("/tmp/signed.json")
	if !contains(args, "broadcast") || !contains(args, "/tmp/signed.json") {
		t.Errorf("broadcast args missing file: %v", args)
	}
	if !contains(args, "--broadcast-mode") || !contains(args, "sync") {
		t.Errorf("broadcast must be sync mode: %v", args)
	}
}
