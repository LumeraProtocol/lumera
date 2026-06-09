package common

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAddKeyWithStylePassesExplicitKeyStyleFlags(t *testing.T) {
	scriptPath, argsPath := recordingLumerad(t)

	t.Run("evm style passes coin type and eth algo", func(t *testing.T) {
		cli := &ChainCLI{Bin: scriptPath, KeyringBackend: "test"}
		if _, err := cli.AddKeyWithStyle("alice", KeyStyleEVM); err != nil {
			t.Fatalf("AddKeyWithStyle error: %v", err)
		}
		args := recordedArgs(t, argsPath)
		assertContainsArgs(t, args, "--coin-type", "60")
		assertContainsArgs(t, args, "--algo", "eth_secp256k1")
	})

	t.Run("legacy style passes coin type without eth algo", func(t *testing.T) {
		cli := &ChainCLI{Bin: scriptPath, KeyringBackend: "test"}
		if _, err := cli.AddKeyWithStyle("bob", KeyStyleLegacy); err != nil {
			t.Fatalf("AddKeyWithStyle error: %v", err)
		}
		args := recordedArgs(t, argsPath)
		assertContainsArgs(t, args, "--coin-type", "118")
		if hasArgValue(args, "--algo", "eth_secp256k1") {
			t.Fatalf("legacy key command unexpectedly used eth algo: %v", args)
		}
	})
}

func TestParseSyncBroadcastHandlesQueryTxResponse(t *testing.T) {
	out := `{
		"tx_response": {
			"txhash": "ABC123",
			"code": 7,
			"raw_log": "failed in deliver"
		}
	}`

	txHash, code, rawLog, ok := parseSyncBroadcast(out)
	if !ok {
		t.Fatal("parseSyncBroadcast did not recognize query tx response")
	}
	if txHash != "ABC123" || code != 7 || rawLog != "failed in deliver" {
		t.Fatalf("got hash=%q code=%d raw_log=%q", txHash, code, rawLog)
	}
}

func TestSendBankNoWaitRejectsUnparseableBroadcastOutput(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   string
	}{
		{"missing JSON", "broadcast accepted maybe", "no JSON tx response"},
		{"malformed JSON", `{"txhash":}`, "parse tx response JSON"},
		{"unexpected JSON", `{"message":"not a tx response"}`, "tx response missing txhash"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cli := &ChainCLI{Bin: staticLumerad(t, tc.output, 0), ChainID: "chain", RPC: "tcp://localhost:26657"}
			_, err := cli.SendBankNoWait("funder", 1, 2, "lumera1to", "10ulume")
			if err == nil {
				t.Fatal("SendBankNoWait error = nil, want parse error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("SendBankNoWait error = %q, want substring %q", err, tc.want)
			}
		})
	}
}

func TestSubmitTxRejectsUnparseableBroadcastOutput(t *testing.T) {
	cli := &ChainCLI{Bin: staticLumerad(t, "not json", 0), ChainID: "chain", RPC: "tcp://localhost:26657"}

	_, err := cli.SubmitTx("tx", "bank", "send", "from", "to", "10ulume", "--from", "from")
	if err == nil {
		t.Fatal("SubmitTx error = nil, want parse error")
	}
	if !strings.Contains(err.Error(), "no JSON tx response") {
		t.Fatalf("SubmitTx error = %q, want missing JSON error", err)
	}
}

func TestWaitForNextBlockReturnsInitialHeightError(t *testing.T) {
	cli := &ChainCLI{Bin: staticLumerad(t, "rpc unavailable", 1), ChainID: "chain", RPC: "tcp://localhost:26657"}

	start := time.Now()
	err := cli.WaitForNextBlock(time.Second)
	if err == nil {
		t.Fatal("WaitForNextBlock error = nil, want height query error")
	}
	if !strings.Contains(err.Error(), "get starting height") {
		t.Fatalf("WaitForNextBlock error = %q, want starting height context", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("WaitForNextBlock took %s, want immediate failure", elapsed)
	}
}

func recordingLumerad(t *testing.T) (scriptPath, argsPath string) {
	t.Helper()

	dir := t.TempDir()
	argsPath = filepath.Join(dir, "args.txt")
	scriptPath = filepath.Join(dir, "lumerad")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + shellQuote(argsPath) + "\n" +
		"printf '%s\\n' '{\"address\":\"lumera1test\",\"pubkey\":\"testpub\",\"mnemonic\":\"test mnemonic\"}'\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake lumerad: %v", err)
	}
	return scriptPath, argsPath
}

func staticLumerad(t *testing.T, output string, exitCode int) string {
	t.Helper()

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "lumerad")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' " + shellQuote(output) + "\n" +
		"exit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake lumerad: %v", err)
	}
	return scriptPath
}

func recordedArgs(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read recorded args: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

func assertContainsArgs(t *testing.T, args []string, flag, value string) {
	t.Helper()
	if !hasArgValue(args, flag, value) {
		t.Fatalf("args %v missing %s %s", args, flag, value)
	}
}

func hasArgValue(args []string, flag, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
