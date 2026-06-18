# gen-activity Multisig Accounts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract the generic multisig key + signing ceremony from `devnet/tests/evmigration` into the shared `devnet/tests/common` package, refactor evmigration to delegate to it, and use it in `gen-activity` to create, fund, and exercise K-of-N multisig accounts (2-of-3 via `-num-multisig23-accounts`, 3-of-5 via `-num-multisig35-accounts`).

**Architecture:** A new `common.Multisig` wraps a `*common.ChainCLI` plus an injectable exec seam. Pure argument-builder methods (unit-tested) encode the exact CLI flags (`--nosort`, `--multisig`, `--sign-mode amino-json`, first-K signature files); thin methods compose them with the exec seam and `ChainCLI` queries to run the ceremony. evmigration keeps its mnemonic-based member-key generation and delegates only the composite-key creation and the sign/broadcast ceremony. gen-activity adds schema-v2 registry records with a `MultisigInfo` field and integrates generation/funding/exercise into `run()`.

**Tech Stack:** Go, `gen` module (`devnet/go.mod`), `common.ChainCLI`. Companion to `gen-activity-config-wizard-plan.md` (which defines the `-num-multisig23/35` flags this plan implements behavior for).

**Prerequisite:** The config/wizard plan's Task 2 (Config fields `NumMultisig23`/`NumMultisig35`) and Task 3 (flag registration) should be done first. If executing this plan standalone, add those two `Config` fields and their flags first (see that plan's Tasks 2–3).

---

## File Structure

New:
- `devnet/tests/common/multisig.go` — `Multisig` type, arg builders, ceremony methods.
- `devnet/tests/common/multisig_test.go` — arg-builder + orchestration tests (fake exec seam).

Modified:
- `devnet/tests/evmigration/multisig.go` — `ensureMultisigCompositeKey`, `registerMultisigPubKey`, `signAndBroadcastMultisigTx`, `buildUnsignedMultisigBankSendTx`, `buildUnsignedMultisigDelegateTx` delegate to `common.Multisig`.
- `devnet/tests/gen-activity/registry.go` — schema v2, `MultisigInfo`, generalized name allocation.
- `devnet/tests/gen-activity/config.go` — `multisigPlan()` helper (counts → kinds).
- `devnet/tests/gen-activity/chain.go` — multisig member/ceremony wiring on top of `common.Multisig`.
- `devnet/tests/gen-activity/main.go` — generate/fund/exercise multisig accounts in `run()`.
- `docs/design/gen-activity-design.md` — document multisig accounts.

---

## Task 1: Multisig arg builders in `common`

**Files:**
- Create: `devnet/tests/common/multisig.go`
- Test: `devnet/tests/common/multisig_test.go`

- [ ] **Step 1: Write the failing test**

Create `devnet/tests/common/multisig_test.go`:

```go
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
	args := m.genBankSendArgs("msig", "lumera1msig", "lumera1peer", "5ulume", 7, 3)
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
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd devnet && go test ./tests/common/ -run 'TestKeyAddArgs|TestSignArgs|TestMultisignArgs|TestGenBankSendArgs|TestBroadcastArgs' -v
```

Expected: FAIL — `undefined: NewMultisig`.

- [ ] **Step 3: Write the implementation**

Create `devnet/tests/common/multisig.go`:

```go
package common

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Multisig provides the generic K-of-N multisig key + signing ceremony shared by
// the evmigration and gen-activity tools. It is built on a *ChainCLI for
// connection config and queries; the command execution is behind an injectable
// seam (exec) so the ceremony orchestration is unit-testable with a fake.
//
// The ceremony is:
//
//	keys add --multisig --nosort        (composite key, key-type agnostic)
//	tx ... --generate-only              (unsigned tx with the multisig as sender)
//	tx sign --multisig --sign-mode amino-json   (× K members)
//	tx multisign                        (combine the K signatures)
//	tx broadcast --broadcast-mode sync  (+ wait for inclusion)
//
// --nosort keeps members in caller order so legacy/new sides agree on index
// ordering; amino-json is required for offline multisig signing.
type Multisig struct {
	CLI  *ChainCLI
	exec func(bin string, args ...string) (string, error)
}

// NewMultisig builds a Multisig over the given ChainCLI with the real command
// executor.
func NewMultisig(cli *ChainCLI) *Multisig {
	return &Multisig{
		CLI: cli,
		exec: func(bin string, args ...string) (string, error) {
			out, err := exec.Command(bin, args...).CombinedOutput()
			return strings.TrimSpace(string(out)), err
		},
	}
}

func (m *Multisig) homeArgs() []string {
	if m.CLI.Home == "" {
		return nil
	}
	return []string{"--home", m.CLI.Home}
}

func (m *Multisig) nodeArgs() []string {
	if m.CLI.RPC == "" {
		return nil
	}
	return []string{"--node", m.CLI.RPC}
}

func (m *Multisig) keyring() string {
	if m.CLI.KeyringBackend == "" {
		return "test"
	}
	return m.CLI.KeyringBackend
}

// keyAddArgs builds `keys add <name> --multisig <m1,m2,..> --multisig-threshold K
// --nosort`. keys add is a pure keyring op (no --node/--chain-id).
func (m *Multisig) keyAddArgs(name string, members []string, threshold int) []string {
	args := []string{
		"keys", "add", name,
		"--multisig", strings.Join(members, ","),
		"--multisig-threshold", strconv.Itoa(threshold),
		"--nosort",
		"--keyring-backend", m.keyring(),
	}
	return append(args, m.homeArgs()...)
}

// signArgs builds `tx sign <file> --from <member> --multisig <addr>` with offline
// account-number/sequence and amino-json sign mode.
func (m *Multisig) signArgs(file, fromKey, multisigAddr string, accNum, seq uint64) []string {
	args := []string{
		"tx", "sign", file,
		"--from", fromKey,
		"--multisig", multisigAddr,
		"--keyring-backend", m.keyring(),
		"--chain-id", m.CLI.ChainID,
		"--account-number", strconv.FormatUint(accNum, 10),
		"--sequence", strconv.FormatUint(seq, 10),
		"--sign-mode", "amino-json",
		"--output", "json",
	}
	args = append(args, m.nodeArgs()...)
	return append(args, m.homeArgs()...)
}

// multisignArgs builds `tx multisign <file> <name> <sig1> <sig2> ...`.
func (m *Multisig) multisignArgs(file, name string, sigFiles []string) []string {
	args := []string{"tx", "multisign", file, name}
	args = append(args, sigFiles...)
	args = append(args,
		"--keyring-backend", m.keyring(),
		"--chain-id", m.CLI.ChainID,
		"--output", "json",
	)
	args = append(args, m.nodeArgs()...)
	return append(args, m.homeArgs()...)
}

// broadcastArgs builds `tx broadcast <signedFile> --broadcast-mode sync`.
func (m *Multisig) broadcastArgs(signedFile string) []string {
	args := []string{
		"tx", "broadcast", signedFile,
		"--broadcast-mode", "sync",
		"--output", "json",
	}
	return append(args, m.nodeArgs()...)
}

// genBankSendArgs builds a generate-only `tx bank send` from the multisig.
func (m *Multisig) genBankSendArgs(name, fromAddr, toAddr, amount string, accNum, seq uint64) []string {
	return m.genTxArgs(name, accNum, seq, "bank", "send", fromAddr, toAddr, amount)
}

// genDelegateArgs builds a generate-only `tx staking delegate` from the multisig.
func (m *Multisig) genDelegateArgs(name, valoper, amount string, accNum, seq uint64) []string {
	return m.genTxArgs(name, accNum, seq, "staking", "delegate", valoper, amount)
}

// genTxArgs builds a generate-only tx with the multisig name as --from and the
// supplied module/positional args.
func (m *Multisig) genTxArgs(name string, accNum, seq uint64, module, action string, positional ...string) []string {
	args := []string{"tx", module, action}
	args = append(args, positional...)
	args = append(args,
		"--from", name,
		"--keyring-backend", m.keyring(),
		"--chain-id", m.CLI.ChainID,
		"--account-number", strconv.FormatUint(accNum, 10),
		"--sequence", strconv.FormatUint(seq, 10),
		"--gas", m.gas(),
		"--gas-prices", m.CLI.GasPrices,
		"--generate-only",
		"--output", "json",
	)
	args = append(args, m.nodeArgs()...)
	return append(args, m.homeArgs()...)
}

func (m *Multisig) gas() string {
	if m.CLI.Gas == "" {
		return "auto"
	}
	return m.CLI.Gas
}

// extractTxHash pulls "txhash":"..." from a broadcast/query JSON response.
func extractTxHash(out string) string {
	const marker = `"txhash":"`
	idx := strings.Index(out, marker)
	if idx < 0 {
		return ""
	}
	rest := out[idx+len(marker):]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}

// unused guard to keep fmt imported for ceremony methods added in Task 2.
var _ = fmt.Sprintf
```

> Implementation note: the `var _ = fmt.Sprintf` guard exists only so this file compiles in isolation after Task 1 (no `fmt` use yet). Task 2 adds real `fmt` usage and you delete this guard then.

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd devnet && go test ./tests/common/ -run 'TestKeyAddArgs|TestSignArgs|TestMultisignArgs|TestGenBankSendArgs|TestBroadcastArgs' -v
```

Expected: PASS for all five.

- [ ] **Step 5: Commit**

```bash
git add devnet/tests/common/multisig.go devnet/tests/common/multisig_test.go
git commit -m "feat(common): multisig CLI arg builders (keys add/sign/multisign/broadcast)"
```

---

## Task 2: Multisig ceremony methods (composite create + sign/broadcast)

**Files:**
- Modify: `devnet/tests/common/multisig.go`
- Test: `devnet/tests/common/multisig_test.go`

- [ ] **Step 1: Write the failing test**

Add to `devnet/tests/common/multisig_test.go`:

```go
// fakeExec records command invocations and returns canned outputs keyed by a
// substring match of the joined args (first match wins).
type fakeExec struct {
	calls    [][]string
	canned   []cannedResponse
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
	// Exactly K=2 sign calls (members s1, s2), then multisign, then broadcast.
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
}

func TestSignAndBroadcastRejectsTooFewMembers(t *testing.T) {
	m := testMultisig()
	m.exec = (&fakeExec{}).run
	_, err := m.SignAndBroadcastFile("/tmp/u.json", "msig", "lumera1msig", []string{"s1"}, 2, 7, 3)
	if err == nil {
		t.Error("expected error when members < threshold")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd devnet && go test ./tests/common/ -run TestSignAndBroadcast -v
```

Expected: FAIL — `m.SignAndBroadcastFile undefined`.

- [ ] **Step 3: Write the implementation**

In `devnet/tests/common/multisig.go`, delete the `var _ = fmt.Sprintf` guard and add (note `os` and `time` imports are needed):

```go
// CreateMultisigKey creates (or reuses) a K-of-N composite key over the given
// member key names. Rerun-safe: an existing composite is returned as-is.
func (m *Multisig) CreateMultisigKey(name string, members []string, threshold int) (string, error) {
	if threshold < 1 {
		return "", fmt.Errorf("invalid multisig threshold %d", threshold)
	}
	if len(members) < threshold {
		return "", fmt.Errorf("multisig %s has %d members, need >= threshold %d", name, len(members), threshold)
	}
	if m.CLI.HasKey(name) {
		return m.CLI.ShowAddress(name)
	}
	out, err := m.exec(m.CLI.Bin, m.keyAddArgs(name, members, threshold)...)
	if err != nil {
		return "", fmt.Errorf("keys add multisig %s: %s: %w", name, out, err)
	}
	return m.CLI.ShowAddress(name)
}

// SignAndBroadcastFile collects `threshold` member signatures over an unsigned
// tx file, combines them with `tx multisign`, broadcasts the result (sync), and
// waits for inclusion. Returns the tx hash. Temp signature files are removed.
func (m *Multisig) SignAndBroadcastFile(unsignedFile, name, multisigAddr string, members []string, threshold int, accNum, seq uint64) (string, error) {
	if len(members) < threshold {
		return "", fmt.Errorf("multisig %s has %d members, need >= %d signers", name, len(members), threshold)
	}

	sigFiles := make([]string, threshold)
	for i := range sigFiles {
		f, err := os.CreateTemp("", fmt.Sprintf("multisig-sig%d-*.json", i+1))
		if err != nil {
			return "", fmt.Errorf("create temp sig file: %w", err)
		}
		_ = f.Close()
		sigFiles[i] = f.Name()
		defer os.Remove(sigFiles[i]) //nolint:gocritic // deferred cleanup of all sig files
	}

	for i, member := range members[:threshold] {
		out, err := m.exec(m.CLI.Bin, m.signArgs(unsignedFile, member, multisigAddr, accNum, seq)...)
		if err != nil {
			return "", fmt.Errorf("sign with %s: %s: %w", member, out, err)
		}
		if err := os.WriteFile(sigFiles[i], []byte(out), 0o600); err != nil {
			return "", fmt.Errorf("write sig %s: %w", sigFiles[i], err)
		}
	}

	signedFile, err := os.CreateTemp("", "multisig-signed-*.json")
	if err != nil {
		return "", fmt.Errorf("create temp signed file: %w", err)
	}
	_ = signedFile.Close()
	defer os.Remove(signedFile.Name())

	msignOut, err := m.exec(m.CLI.Bin, m.multisignArgs(unsignedFile, name, sigFiles)...)
	if err != nil {
		return "", fmt.Errorf("multisign: %s: %w", msignOut, err)
	}
	if err := os.WriteFile(signedFile.Name(), []byte(msignOut), 0o600); err != nil {
		return "", fmt.Errorf("write signed tx: %w", err)
	}

	bcastOut, err := m.exec(m.CLI.Bin, m.broadcastArgs(signedFile.Name())...)
	if err != nil {
		return "", fmt.Errorf("broadcast: %s: %w", bcastOut, err)
	}
	txHash := extractTxHash(bcastOut)
	if txHash != "" {
		if err := m.CLI.WaitForTxInclusion(txHash, 45*time.Second); err != nil {
			return txHash, err
		}
	}
	return txHash, nil
}

// BuildUnsignedToFile generates an unsigned tx (generate-only) using args and
// writes it to outFile, for the caller to feed into SignAndBroadcastFile.
func (m *Multisig) BuildUnsignedToFile(outFile string, args []string) error {
	out, err := m.exec(m.CLI.Bin, args...)
	if err != nil {
		return fmt.Errorf("generate unsigned tx: %s: %w", out, err)
	}
	return os.WriteFile(outFile, []byte(out), 0o600)
}
```

Update the import block of `multisig.go` to:

```go
import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd devnet && go test ./tests/common/ -run 'TestSignAndBroadcast|TestKeyAddArgs|TestSignArgs|TestMultisignArgs|TestGenBankSendArgs|TestBroadcastArgs' -v
```

Expected: PASS for all.

- [ ] **Step 5: Commit**

```bash
git add devnet/tests/common/multisig.go devnet/tests/common/multisig_test.go
git commit -m "feat(common): multisig composite-create + sign/multisign/broadcast ceremony"
```

---

## Task 3: Refactor evmigration to delegate to `common.Multisig`

**Files:**
- Modify: `devnet/tests/evmigration/multisig.go`

This task changes implementations only; behavior must be identical. There is no new test — the gate is that evmigration's existing tests stay green and the package builds.

- [ ] **Step 1: Add a helper that builds a common.Multisig from the evmigration globals**

In `devnet/tests/evmigration/multisig.go`, add near the top (after the imports):

```go
// commonMultisig builds a *common.Multisig from the evmigration flag globals so
// the generic ceremony is shared with gen-activity. Gas is fixed (matches the
// values used across this tool) so generate-only/offline signing works.
func commonMultisig() *common.Multisig {
	cli := &common.ChainCLI{
		Bin:            *flagBin,
		ChainID:        *flagChainID,
		RPC:            *flagRPC,
		Home:           *flagHome,
		KeyringBackend: "test",
		Gas:            *flagGas,
		GasPrices:      *flagGasPrices,
	}
	return common.NewMultisig(cli)
}
```

Add `"gen/tests/common"` to the imports if not already present (it is, per keys.go; confirm in multisig.go's import block and add if missing).

- [ ] **Step 2: Delegate composite-key creation**

Replace the body of `ensureMultisigCompositeKey` (keep its signature) with:

```go
func ensureMultisigCompositeKey(multisigKeyName string, members []string, threshold int) (string, error) {
	addr, err := commonMultisig().CreateMultisigKey(multisigKeyName, members, threshold)
	if err != nil {
		return "", err
	}
	log.Printf("  multisig key %s -> %s", multisigKeyName, addr)
	return addr, nil
}
```

- [ ] **Step 3: Delegate the sign/broadcast helper**

Replace the body of `signAndBroadcastMultisigTx` (keep its signature) with:

```go
func signAndBroadcastMultisigTx(unsignedFile, multisigKeyName, multisigAddr string, members []string) error {
	accNum, seq, err := queryAccountNumberAndSequence(multisigAddr)
	if err != nil {
		return fmt.Errorf("query account number/sequence for %s: %w", multisigAddr, err)
	}
	_, err = commonMultisig().SignAndBroadcastFile(
		unsignedFile, multisigKeyName, multisigAddr, members, defaultMultisigThreshold, accNum, seq,
	)
	return err
}
```

- [ ] **Step 4: Delegate the unsigned-tx builders**

Replace the bodies of `buildUnsignedMultisigBankSendTx` and `buildUnsignedMultisigDelegateTx` (keep signatures) with:

```go
func buildUnsignedMultisigBankSendTx(multisigKeyName, multisigAddr, toAddr, amount, outFile string) error {
	accNum, seq, err := queryAccountNumberAndSequence(multisigAddr)
	if err != nil {
		if waitErr := waitForAccountOnChain(multisigAddr, 30*time.Second); waitErr != nil {
			return fmt.Errorf("wait for multisig account on-chain: %w", waitErr)
		}
		if accNum, seq, err = queryAccountNumberAndSequence(multisigAddr); err != nil {
			return fmt.Errorf("query account number/sequence for %s: %w", multisigAddr, err)
		}
	}
	m := commonMultisig()
	return m.BuildUnsignedToFile(outFile, m.GenBankSendArgs(multisigKeyName, multisigAddr, toAddr, amount, accNum, seq))
}

func buildUnsignedMultisigDelegateTx(multisigKeyName, multisigAddr, validatorAddr, amount, outFile string) error {
	accNum, seq, err := queryAccountNumberAndSequence(multisigAddr)
	if err != nil {
		if waitErr := waitForAccountOnChain(multisigAddr, 30*time.Second); waitErr != nil {
			return fmt.Errorf("wait for multisig account on-chain: %w", waitErr)
		}
		if accNum, seq, err = queryAccountNumberAndSequence(multisigAddr); err != nil {
			return fmt.Errorf("query account number/sequence for %s: %w", multisigAddr, err)
		}
	}
	m := commonMultisig()
	return m.BuildUnsignedToFile(outFile, m.GenDelegateArgs(multisigKeyName, validatorAddr, amount, accNum, seq))
}
```

> Note: `GenBankSendArgs` / `GenDelegateArgs` must be **exported** in common (Task 1 defined them lowercase). In `devnet/tests/common/multisig.go`, rename `genBankSendArgs`→`GenBankSendArgs` and `genDelegateArgs`→`GenDelegateArgs` (and update the Task 1 tests that reference `m.genBankSendArgs` to `m.GenBankSendArgs`). Keep `genTxArgs`, `keyAddArgs`, `signArgs`, `multisignArgs`, `broadcastArgs` lowercase (used only within common). Make this rename now.

- [ ] **Step 5: Keep `registerMultisigPubKey` delegating to the shared signer**

`registerMultisigPubKey` already constructs an unsigned self-send and calls a sign/broadcast path. Leave its self-send construction in place but route its final sign/multisign/broadcast through `signAndBroadcastMultisigTx` (now delegating to common). Concretely, ensure its tail uses `signAndBroadcastMultisigTx(unsignedFile, multisigKeyName, multisigAddr, members)` instead of the inline sign/multisign/broadcast block. If `registerMultisigPubKey` currently inlines those steps, replace that block with the single delegating call. Do not change the 1-ulume self-send semantics.

- [ ] **Step 6: Remove the now-duplicate local `extractTxHash`**

`common.extractTxHash` is unexported (package-internal), so the evmigration `extractTxHash` in `multisig.go` is a separate symbol and still compiles — leave it. (No action; this step is a checkpoint to confirm there is no symbol clash. If the build reports a redeclaration, it means an accidental same-package duplicate; there is none across packages.)

- [ ] **Step 7: Build and run evmigration unit tests**

Run:

```bash
cd devnet && go build ./... && go test ./tests/evmigration/ -v
```

Expected: builds clean; evmigration unit tests pass (same set as before the refactor). If any multisig test requires the `integration`/`test` build tags, run the tagged suite per CLAUDE.md instead:

```bash
cd /home/akobrin/p/lumera && go test -tags='integration test' ./tests/integration/evmigration/... -v
```

Expected: no new failures attributable to this refactor.

- [ ] **Step 8: Commit**

```bash
git add devnet/tests/common/multisig.go devnet/tests/common/multisig_test.go devnet/tests/evmigration/multisig.go
git commit -m "refactor(evmigration): delegate generic multisig ceremony to common.Multisig"
```

---

## Task 4: Registry schema v2 + MultisigInfo + name allocation

**Files:**
- Modify: `devnet/tests/gen-activity/registry.go`
- Test: `devnet/tests/gen-activity/registry_test.go`

- [ ] **Step 1: Write the failing test**

Add to `devnet/tests/gen-activity/registry_test.go`:

```go
func TestRegistrySchemaIsV2(t *testing.T) {
	if schemaVersion != 2 {
		t.Fatalf("schemaVersion = %d, want 2", schemaVersion)
	}
	reg := NewRegistry("lumera-devnet-1", "faucet", "", "legacy", "2026-06-12T00:00:00Z")
	if reg.SchemaVersion != 2 {
		t.Errorf("NewRegistry schema = %d, want 2", reg.SchemaVersion)
	}
}

func TestMultisigRecordRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.json")
	reg := NewRegistry("lumera-devnet-1", "faucet", "", "legacy", "2026-06-12T00:00:00Z")
	reg.UpsertAccount(&AccountRecord{
		AccountIdentity: common.AccountIdentity{Name: "gen-msig23-0001", Address: "lumera1msig"},
		Multisig: &MultisigInfo{
			MemberNames: []string{"gen-msig23-0001-signer-1", "gen-msig23-0001-signer-2", "gen-msig23-0001-signer-3"},
			Threshold:   2,
			Signers:     3,
		},
	})
	if err := reg.Save(path, "2026-06-12T00:00:00Z"); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got.Accounts) != 1 || got.Accounts[0].Multisig == nil {
		t.Fatalf("multisig info not persisted: %+v", got.Accounts)
	}
	if got.Accounts[0].Multisig.Threshold != 2 || got.Accounts[0].Multisig.Signers != 3 {
		t.Errorf("multisig threshold/signers = %d/%d, want 2/3",
			got.Accounts[0].Multisig.Threshold, got.Accounts[0].Multisig.Signers)
	}
}

func TestAllocateNamesPerKindContinuesIndex(t *testing.T) {
	reg := NewRegistry("c", "f", "", "legacy", "t")
	reg.Accounts = []*AccountRecord{
		{AccountIdentity: common.AccountIdentity{Name: "gen-msig23-0001"}},
		{AccountIdentity: common.AccountIdentity{Name: "gen-msig23-0002"}},
	}
	names := reg.AllocateNames("gen-msig23", 2)
	want := []string{"gen-msig23-0003", "gen-msig23-0004"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("AllocateNames = %v, want %v", names, want)
	}
}
```

Add imports `reflect` and (if missing) `path/filepath`, `gen/tests/common` to `registry_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run 'TestRegistrySchemaIsV2|TestMultisigRecordRoundTrip|TestAllocateNamesPerKind' -v
```

Expected: FAIL — `schemaVersion` is 1 / `MultisigInfo` undefined.

- [ ] **Step 3: Write the implementation**

In `devnet/tests/gen-activity/registry.go`:

Change the schema constant:

```go
// schemaVersion identifies the gen-activity registry layout. v2 adds multisig
// account records (AccountRecord.Multisig). v1 files are not supported and must
// be regenerated.
const schemaVersion = 2
```

Add the `MultisigInfo` type and the `Multisig` field on `AccountRecord`:

```go
// MultisigInfo describes a generated K-of-N multisig account: its member key
// names, signing threshold (K), and total signer count (N).
type MultisigInfo struct {
	MemberNames []string `json:"member_names"`
	Threshold   int      `json:"threshold"`
	Signers     int      `json:"signers"`
}
```

In the `AccountRecord` struct, add (after the embedded `common.ActivityLog`):

```go
	Multisig *MultisigInfo `json:"multisig,omitempty"`
```

The existing `LoadRegistry` already rejects any `schema_version` != `schemaVersion`, which now means it accepts only v2 — no change needed there. `AllocateNames` already continues past the highest index matching the given prefix, so it works for `"gen-msig23"` as-is — no change needed.

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run 'TestRegistrySchemaIsV2|TestMultisigRecordRoundTrip|TestAllocateNamesPerKind' -v
```

Expected: PASS for all three.

- [ ] **Step 5: Commit**

```bash
git add devnet/tests/gen-activity/registry.go devnet/tests/gen-activity/registry_test.go
git commit -m "feat(gen-activity): registry schema v2 with multisig account records"
```

---

## Task 5: Multisig generation plan + member/composite creation

**Files:**
- Create: `devnet/tests/gen-activity/multisig.go`
- Test: `devnet/tests/gen-activity/multisig_test.go`

- [ ] **Step 1: Write the failing test**

Create `devnet/tests/gen-activity/multisig_test.go`:

```go
package main

import (
	"reflect"
	"testing"
)

func TestMultisigPlanFromCounts(t *testing.T) {
	plan := multisigPlan(2, 1)
	want := []multisigSpec{
		{Prefix: "msig23", Threshold: 2, Signers: 3},
		{Prefix: "msig23", Threshold: 2, Signers: 3},
		{Prefix: "msig35", Threshold: 3, Signers: 5},
	}
	if !reflect.DeepEqual(plan, want) {
		t.Errorf("multisigPlan(2,1) = %+v, want %+v", plan, want)
	}
}

func TestMultisigPlanZeroIsEmpty(t *testing.T) {
	if plan := multisigPlan(0, 0); len(plan) != 0 {
		t.Errorf("multisigPlan(0,0) = %+v, want empty", plan)
	}
}

func TestMemberNamesForComposite(t *testing.T) {
	got := memberNames("gen-msig35-0007", 5)
	want := []string{
		"gen-msig35-0007-signer-1", "gen-msig35-0007-signer-2", "gen-msig35-0007-signer-3",
		"gen-msig35-0007-signer-4", "gen-msig35-0007-signer-5",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("memberNames = %v, want %v", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run 'TestMultisigPlan|TestMemberNames' -v
```

Expected: FAIL — `undefined: multisigPlan`.

- [ ] **Step 3: Write the implementation**

Create `devnet/tests/gen-activity/multisig.go`:

```go
package main

import (
	"fmt"
	"log"
	"time"

	"gen/tests/common"
)

// multisigSpec describes one multisig account to generate.
type multisigSpec struct {
	Prefix    string // name infix, e.g. "msig23"
	Threshold int    // K
	Signers   int    // N
}

// multisigPlan expands the 2-of-3 and 3-of-5 counts into a flat list of specs.
func multisigPlan(num23, num35 int) []multisigSpec {
	var plan []multisigSpec
	for i := 0; i < num23; i++ {
		plan = append(plan, multisigSpec{Prefix: "msig23", Threshold: 2, Signers: 3})
	}
	for i := 0; i < num35; i++ {
		plan = append(plan, multisigSpec{Prefix: "msig35", Threshold: 3, Signers: 5})
	}
	return plan
}

// memberNames returns deterministic member key names for a composite.
func memberNames(composite string, signers int) []string {
	names := make([]string, signers)
	for i := 0; i < signers; i++ {
		names[i] = fmt.Sprintf("%s-signer-%d", composite, i+1)
	}
	return names
}

// generateMultisigAccounts creates member keys + composite keys for the planned
// multisig accounts, returning new AccountRecords (with Multisig info set). It is
// rerun-safe: existing keys/composites are reused. Member keys use the detected
// key style; the composite is key-type agnostic.
func generateMultisigAccounts(cli *common.ChainCLI, reg *ActivityRegistry, accountPrefix string, specs []multisigSpec, keyStyle common.KeyStyle) []*AccountRecord {
	ms := common.NewMultisig(cli)
	now := time.Now().UTC().Format(time.RFC3339)
	var recs []*AccountRecord

	for _, spec := range specs {
		names := reg.AllocateNames(accountPrefix+"-"+spec.Prefix, 1)
		composite := names[0]
		members := memberNames(composite, spec.Signers)

		if err := ensureMembers(cli, members, keyStyle); err != nil {
			log.Printf("  WARN: multisig %s members: %v", composite, err)
			continue
		}
		addr, err := ms.CreateMultisigKey(composite, members, spec.Threshold)
		if err != nil {
			log.Printf("  WARN: create multisig %s: %v", composite, err)
			continue
		}
		rec := &AccountRecord{
			AccountIdentity: common.AccountIdentity{Name: composite, Address: addr, KeyStyle: keyStyle.Name()},
			Multisig:        &MultisigInfo{MemberNames: members, Threshold: spec.Threshold, Signers: spec.Signers},
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		reg.UpsertAccount(rec)
		recs = append(recs, rec)
		log.Printf("  created %d-of-%d multisig %s (%s)", spec.Threshold, spec.Signers, composite, addr)
	}
	return recs
}

// ensureMembers creates any missing member keys with the detected key style.
func ensureMembers(cli *common.ChainCLI, names []string, keyStyle common.KeyStyle) error {
	for _, name := range names {
		if cli.HasKey(name) {
			continue
		}
		if _, err := cli.AddKeyWithStyle(name, keyStyle); err != nil {
			return fmt.Errorf("add member key %s: %w", name, err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run 'TestMultisigPlan|TestMemberNames' -v
```

Expected: PASS for all three.

- [ ] **Step 5: Commit**

```bash
git add devnet/tests/gen-activity/multisig.go devnet/tests/gen-activity/multisig_test.go
git commit -m "feat(gen-activity): plan + create multisig member/composite keys"
```

---

## Task 6: Wire multisig generation, funding, and exercise into `run()`

**Files:**
- Modify: `devnet/tests/gen-activity/multisig.go`
- Modify: `devnet/tests/gen-activity/main.go`
- Test: `devnet/tests/gen-activity/multisig_test.go`

- [ ] **Step 1: Write the failing test (exercise step is pure-ish: assert the unsigned-tx + ceremony are invoked via a seam)**

Add to `devnet/tests/gen-activity/multisig_test.go`:

```go
func TestExerciseMultisigRecordsBankSend(t *testing.T) {
	rec := &AccountRecord{
		AccountIdentity: common.AccountIdentity{Name: "gen-msig23-0001", Address: "lumera1msig"},
		Multisig:        &MultisigInfo{MemberNames: []string{"m1", "m2", "m3"}, Threshold: 2, Signers: 3},
		Funded:          true,
	}
	fake := &fakeMultisigExerciser{txHash: "DEAD01"}

	err := exerciseMultisig(fake, rec, "lumera1peer", "5ulume")
	if err != nil {
		t.Fatalf("exerciseMultisig: %v", err)
	}
	if len(rec.BankSends) != 1 || rec.BankSends[0].To != "lumera1peer" {
		t.Fatalf("expected one recorded bank send to peer, got %+v", rec.BankSends)
	}
	if rec.BankSends[0].TxHash != "DEAD01" {
		t.Errorf("recorded tx hash = %q, want DEAD01", rec.BankSends[0].TxHash)
	}
	if !fake.called {
		t.Error("multisig exerciser was not invoked")
	}
}

// fakeMultisigExerciser satisfies multisigExerciser for the test.
type fakeMultisigExerciser struct {
	called bool
	txHash string
}

func (f *fakeMultisigExerciser) MultisigBankSend(rec *AccountRecord, to, amount string) (string, error) {
	f.called = true
	return f.txHash, nil
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd devnet && go test ./tests/gen-activity/ -run TestExerciseMultisigRecordsBankSend -v
```

Expected: FAIL — `undefined: exerciseMultisig` / `multisigExerciser`.

- [ ] **Step 3: Write the implementation**

Add to `devnet/tests/gen-activity/multisig.go`:

```go
// multisigExerciser performs one multisign tx for a multisig account. It is an
// interface so the recording logic is testable without a live chain.
type multisigExerciser interface {
	// MultisigBankSend builds, signs (K members), and broadcasts a bank send
	// from the multisig account to `to`, returning the tx hash.
	MultisigBankSend(rec *AccountRecord, to, amount string) (string, error)
}

// exerciseMultisig runs one multisign bank-send for the account and records it.
func exerciseMultisig(ex multisigExerciser, rec *AccountRecord, peer, amount string) error {
	if rec.Multisig == nil {
		return fmt.Errorf("account %s is not a multisig account", rec.Name)
	}
	txHash, err := ex.MultisigBankSend(rec, peer, amount)
	if err != nil {
		return err
	}
	rec.AddBankSend(common.BankSendActivity{To: peer, Amount: amount, TxHash: txHash})
	return nil
}

// cliMultisigExerciser is the production exerciser backed by common.Multisig.
type cliMultisigExerciser struct {
	cli *common.ChainCLI
	ms  *common.Multisig
}

func newCLIMultisigExerciser(cli *common.ChainCLI) *cliMultisigExerciser {
	return &cliMultisigExerciser{cli: cli, ms: common.NewMultisig(cli)}
}

func (e *cliMultisigExerciser) MultisigBankSend(rec *AccountRecord, to, amount string) (string, error) {
	accNum, seq, err := e.cli.AccountNumberAndSequence(rec.Address)
	if err != nil {
		return "", fmt.Errorf("query account number/sequence for %s: %w", rec.Address, err)
	}
	unsigned, err := os.CreateTemp("", "msig-unsigned-*.json")
	if err != nil {
		return "", fmt.Errorf("create temp unsigned file: %w", err)
	}
	_ = unsigned.Close()
	defer os.Remove(unsigned.Name())

	args := e.ms.GenBankSendArgs(rec.Name, rec.Address, to, amount, accNum, seq)
	if err := e.ms.BuildUnsignedToFile(unsigned.Name(), args); err != nil {
		return "", err
	}
	return e.ms.SignAndBroadcastFile(unsigned.Name(), rec.Name, rec.Address,
		rec.Multisig.MemberNames, rec.Multisig.Threshold, accNum, seq)
}
```

Add `"os"` to the import block of `multisig.go`.

- [ ] **Step 4: Integrate into `run()`**

In `devnet/tests/gen-activity/main.go`, inside `run()`, after the existing `generateAccounts` + first `reg.Save` block (right after `log.Printf("generated %d new account(s); registry saved", ...)`), add multisig generation:

```go
	// Generate multisig accounts (2-of-3 / 3-of-5) alongside regular accounts.
	var newMultisig []*AccountRecord
	if specs := multisigPlan(cfg.NumMultisig23, cfg.NumMultisig35); len(specs) > 0 && !cfg.ActivityExisting {
		newMultisig = generateMultisigAccounts(cli, reg, cfg.AccountPrefix, specs, keyStyle)
		if err := reg.Save(cfg.AccountsPath, time.Now().UTC().Format(time.RFC3339)); err != nil {
			return fmt.Errorf("save registry after multisig key generation: %w", err)
		}
		log.Printf("generated %d new multisig account(s)", len(newMultisig))
	}
```

Multisig composites are funded by the existing funding phase because `unfundedTargets(reg)` already returns every `!Funded` record, including the multisig ones just upserted. No change to the funding call is required.

After the funding phase and `reg.Save` (after `log.Printf("funded %d/%d account(s)", ...)` and its save), add the pubkey-registration + exercise step:

```go
	// Register multisig pubkeys on-chain and exercise each with one multisign
	// bank-send to a peer. Non-fatal: a failure logs and continues.
	exerciseMultisigAccounts(cli, reg, newMultisig, cfg)
	if err := reg.Save(cfg.AccountsPath, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("save registry after multisig exercise: %w", err)
	}
```

Add the orchestration helper to `multisig.go`:

```go
// exerciseMultisigAccounts registers each funded multisig composite's pubkey
// on-chain (via the shared ceremony's self-send) and runs one multisign
// bank-send to a regular peer address. Failures are logged, never fatal.
func exerciseMultisigAccounts(cli *common.ChainCLI, reg *ActivityRegistry, newMultisig []*AccountRecord, cfg *Config) {
	targets := newMultisig
	if cfg.ActivityExisting {
		targets = nil
		for _, rec := range reg.Accounts {
			if rec.Multisig != nil && rec.Funded {
				targets = append(targets, rec)
			}
		}
	}
	if len(targets) == 0 {
		return
	}

	peer := firstRegularPeer(reg)
	if peer == "" {
		log.Printf("  no regular peer account to receive multisig sends; skipping multisig exercise")
		return
	}

	ms := common.NewMultisig(cli)
	ex := newCLIMultisigExerciser(cli)
	amount := common.Coin{Amount: 1, Denom: common.ChainDenom}.String()

	for _, rec := range targets {
		if !rec.Funded {
			continue
		}
		// Publish the composite pubkey on-chain via a 1-ulume self-send.
		if err := registerMultisigPubkey(cli, ms, rec); err != nil {
			log.Printf("  WARN: register pubkey for %s: %v", rec.Name, err)
			continue
		}
		if err := exerciseMultisig(ex, rec, peer, amount); err != nil {
			log.Printf("  WARN: exercise multisig %s: %v", rec.Name, err)
		}
	}
}

// registerMultisigPubkey publishes a multisig composite's pubkey on-chain via a
// 1-ulume self-send (required before it can be queried for account number).
func registerMultisigPubkey(cli *common.ChainCLI, ms *common.Multisig, rec *AccountRecord) error {
	accNum, seq, err := cli.AccountNumberAndSequence(rec.Address)
	if err != nil {
		return fmt.Errorf("query account number/sequence for %s: %w", rec.Address, err)
	}
	unsigned, err := os.CreateTemp("", "msig-selfsend-*.json")
	if err != nil {
		return fmt.Errorf("create temp unsigned file: %w", err)
	}
	_ = unsigned.Close()
	defer os.Remove(unsigned.Name())

	args := ms.GenBankSendArgs(rec.Name, rec.Address, rec.Address, "1"+common.ChainDenom, accNum, seq)
	if err := ms.BuildUnsignedToFile(unsigned.Name(), args); err != nil {
		return err
	}
	_, err = ms.SignAndBroadcastFile(unsigned.Name(), rec.Name, rec.Address,
		rec.Multisig.MemberNames, rec.Multisig.Threshold, accNum, seq)
	return err
}

// firstRegularPeer returns the address of the first funded non-multisig account
// to serve as a recipient for multisig sends.
func firstRegularPeer(reg *ActivityRegistry) string {
	for _, rec := range reg.Accounts {
		if rec.Multisig == nil && rec.Funded && rec.Address != "" {
			return rec.Address
		}
	}
	return ""
}
```

- [ ] **Step 5: Run the package tests + build**

Run:

```bash
cd devnet && go build ./... && go test ./tests/gen-activity/ -v
```

Expected: builds clean; all gen-activity tests (including `TestExerciseMultisigRecordsBankSend`) pass.

- [ ] **Step 6: Commit**

```bash
git add devnet/tests/gen-activity/multisig.go devnet/tests/gen-activity/main.go devnet/tests/gen-activity/multisig_test.go
git commit -m "feat(gen-activity): generate, fund, and exercise multisig accounts in run()"
```

---

## Task 7: Verification + documentation

**Files:**
- Modify: `docs/design/gen-activity-design.md`

- [ ] **Step 1: Document multisig accounts**

In `docs/design/gen-activity-design.md`, add a section:

```markdown
## Multisig accounts

`-num-multisig23-accounts` and `-num-multisig35-accounts` generate K-of-N
multisig accounts (2-of-3 and 3-of-5). For each, gen-activity:

1. Creates N member keys (`<composite>-signer-<i>`) using the detected key style
   and a composite key via `keys add --multisig --nosort` (shared
   `common.Multisig.CreateMultisigKey`).
2. Funds the composite from the funder (multisig composites are ordinary funding
   targets).
3. Publishes the composite pubkey on-chain via a 1-ulume self-send.
4. Exercises the account with one multisign bank-send to a regular peer
   (`sign × K → multisign → broadcast`), recorded in the registry.

Records are stored as `AccountRecord.Multisig` (member names, threshold,
signers) under registry schema v2. The generic ceremony lives in
`devnet/tests/common` (`multisig.go`) and is shared with the evmigration tool.
```

- [ ] **Step 2: Full verification sweep**

Run:

```bash
cd devnet && go build ./... && go test ./tests/common/ ./tests/gen-activity/ ./tests/evmigration/ -v
cd /home/akobrin/p/lumera && make lint
```

Expected: all packages build; all listed unit tests pass; `make lint` reports 0 issues.

- [ ] **Step 3: Exercise the evmigration multisig modes (devnet) to confirm the refactor**

Per the spec's verification requirement, run the evmigration multisig modes against a devnet (manual; requires a running devnet + EVM-enabled lumerad):

```bash
cd devnet/tests/evmigration
go run . -mode multisig
go run . -mode multisig-vesting
go run . -mode multisig-validator   # SKIPs cleanly if no multisig validator seeded
```

Expected: `=== MULTISIG MODE: SUCCESS ===` (and the vesting variant); behavior identical to before the refactor.

- [ ] **Step 4: Exercise gen-activity multisig generation (devnet)**

Run (against a running devnet):

```bash
cd devnet/tests/gen-activity
go run . -chain devnet -num-accounts 3 -num-multisig23-accounts 1 -num-multisig35-accounts 1
```

Expected: regular + multisig accounts created and funded; multisig accounts show a recorded `bank_sends` entry in the accounts JSON; run completes without fatal errors.

- [ ] **Step 5: Commit**

```bash
git add docs/design/gen-activity-design.md
git commit -m "docs(gen-activity): document multisig accounts and shared ceremony"
```

---

## Self-Review checklist (for the plan author)

- **Spec coverage:** §3.3 extraction to common → Tasks 1–2; evmigration refactor → Task 3; §3.4 registry v2 + MultisigInfo → Task 4; multisig naming + generation → Task 5; funding (reuses existing `unfundedTargets`) + exercise → Task 6; §4 verification (common/gen-activity/evmigration tests, lint, devnet modes) → Tasks 3, 7; docs → Task 7.
- **Placeholder scan:** the `var _ = fmt.Sprintf` guard in Task 1 and the export-rename note in Task 3 are explicit, with exact final code given (not TODOs).
- **Type consistency:** `MultisigInfo{MemberNames, Threshold, Signers}` is identical across registry.go, multisig.go, and the tests; `common.Multisig` method names (`CreateMultisigKey`, `SignAndBroadcastFile`, `BuildUnsignedToFile`, `GenBankSendArgs`, `GenDelegateArgs`) match between common, evmigration (Task 3), and gen-activity (Tasks 5–6); `multisigExerciser.MultisigBankSend(rec, to, amount)` matches its fake and the `cliMultisigExerciser` impl.
- **Testability tradeoff:** orchestration of the full ceremony is unit-tested in `common` via the injectable `exec` seam (Task 2); the gen-activity `MultisigBankSend` integration and the on-chain steps are covered by the devnet exercises in Task 7 rather than unit tests (they require a live keyring + chain). This is intentional and called out so coverage gaps aren't silent.
```
