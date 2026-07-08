package common

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
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

func TestImportKeyWithStyleDoesNotReenterCommandSlot(t *testing.T) {
	t.Setenv("LUMERA_CLI_PARALLELISM", "1")
	resetCommandSlotsForTest()
	t.Cleanup(resetCommandSlotsForTest)

	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args.txt")
	bin := filepath.Join(dir, "lumerad")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" >> " + shellQuote(argsPath) + "\n" +
		"if [ \"$1\" = keys ] && [ \"$2\" = add ]; then\n" +
		"  cat >/dev/null\n" +
		"  printf '%s\\n' '{\"name\":\"alice\",\"address\":\"lumera1recovered\",\"pubkey\":\"pub\"}'\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = keys ] && [ \"$2\" = show ]; then\n" +
		"  printf '%s\\n' 'unexpected nested keys show' >&2\n" +
		"  exit 2\n" +
		"fi\n" +
		"exit 1\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake lumerad: %v", err)
	}

	var (
		addr string
		err  error
	)
	done := make(chan struct{})
	go func() {
		defer close(done)
		addr, err = (&ChainCLI{Bin: bin, KeyringBackend: "test"}).ImportKeyWithStyle("alice", "seed words", KeyStyleEVM)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ImportKeyWithStyle hung; likely re-entered the one-slot CLI semaphore")
	}
	if err != nil {
		t.Fatalf("ImportKeyWithStyle returned error: %v", err)
	}
	if addr != "lumera1recovered" {
		t.Fatalf("address = %q, want lumera1recovered", addr)
	}
	if got := strings.TrimSpace(string(mustReadFile(t, argsPath))); strings.Contains(got, "keys\nshow") {
		t.Fatalf("ImportKeyWithStyle should not call nested keys show while holding the command slot; args:\n%s", got)
	}
}

func resetCommandSlotsForTest() {
	commandSlotState.once = sync.Once{}
	commandSlotState.ch = nil
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}
