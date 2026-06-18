package common

import (
	"strings"
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

// fakeExec records command invocations and returns canned outputs keyed by a
// substring match of the joined args (first match wins).
type fakeExec struct {
	calls  [][]string
	canned []cannedResponse
}

type cannedResponse struct {
	match  string
	output string
	err    error
}

func (f *fakeExec) run(bin string, args ...string) (string, error) {
	f.calls = append(f.calls, args)
	joined := strings.Join(args, " ")
	for _, c := range f.canned {
		if strings.Contains(joined, c.match) {
			return c.output, c.err
		}
	}
	return "", nil
}

func (f *fakeExec) calledWith(sub string) bool {
	for _, c := range f.calls {
		if strings.Contains(strings.Join(c, " "), sub) {
			return true
		}
	}
	return false
}

func TestSignAndBroadcastRunsCeremonyInOrder(t *testing.T) {
	m := testMultisig()
	fe := &fakeExec{canned: []cannedResponse{
		{match: "tx sign", output: `{"some":"sig"}`},
		{match: "tx multisign", output: `{"signed":"tx"}`},
		{match: "tx broadcast", output: `{"txhash":"ABC123","code":0}`},
		{match: "query tx ABC123", output: `{"code":0,"txhash":"ABC123"}`},
	}}
	m.exec = fe.run

	txHash, err := m.SignAndBroadcastFile("/tmp/unsigned.json", "msig", "lumera1msig",
		[]string{"s1", "s2", "s3"}, 2, 7, 3)
	if err != nil {
		t.Fatalf("SignAndBroadcastFile: %v", err)
	}
	if txHash != "ABC123" {
		t.Errorf("txHash = %q, want ABC123", txHash)
	}
	if !fe.calledWith("tx sign /tmp/unsigned.json --from s1") {
		t.Error("missing sign with s1")
	}
	if !fe.calledWith("tx sign /tmp/unsigned.json --from s2") {
		t.Error("missing sign with s2")
	}
	if fe.calledWith("--from s3") {
		t.Error("must not sign with the 3rd member when threshold=2")
	}
	if !fe.calledWith("tx multisign") || !fe.calledWith("tx broadcast") {
		t.Error("missing multisign/broadcast step")
	}
	// The inclusion wait must run through the exec seam (not a real node).
	if !fe.calledWith("query tx ABC123") {
		t.Error("inclusion wait did not run through the exec seam (no query tx)")
	}
}

func TestSignAndBroadcastRejectsTooFewMembers(t *testing.T) {
	m := testMultisig()
	m.exec = (&fakeExec{}).run
	_, err := m.SignAndBroadcastFile("/tmp/u.json", "msig", "lumera1msig", []string{"s1"}, 2, 7, 3)
	if err == nil {
		t.Error("expected error when members < threshold")
	}
}

// TestSignAndBroadcastFailsOnRejectedCode verifies a non-zero CheckTx code from
// the sync broadcast is surfaced as an error (not silently polled to timeout).
func TestSignAndBroadcastFailsOnRejectedCode(t *testing.T) {
	m := testMultisig()
	fe := &fakeExec{canned: []cannedResponse{
		{match: "tx sign", output: `{"some":"sig"}`},
		{match: "tx multisign", output: `{"signed":"tx"}`},
		{match: "tx broadcast", output: `{"txhash":"BAD","code":11,"raw_log":"insufficient fee"}`},
	}}
	m.exec = fe.run

	_, err := m.SignAndBroadcastFile("/tmp/unsigned.json", "msig", "lumera1msig",
		[]string{"s1", "s2", "s3"}, 2, 7, 3)
	if err == nil {
		t.Fatal("expected error for rejected broadcast (code=11)")
	}
	if !strings.Contains(err.Error(), "code=11") {
		t.Errorf("error = %q, want it to mention code=11", err)
	}
	// Must fail fast at broadcast, never reaching the inclusion poll.
	if fe.calledWith("query tx") {
		t.Error("must not poll query tx after a rejected broadcast")
	}
}
