package common

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
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
			out, err := combinedOutputNoDesktopBus(bin, args...)
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

// GenBankSendArgs builds a generate-only `tx bank send` from the multisig.
func (m *Multisig) GenBankSendArgs(name, fromAddr, toAddr, amount string, accNum, seq uint64) []string {
	return m.genTxArgs(name, accNum, seq, "bank", "send", fromAddr, toAddr, amount)
}

// GenDelegateArgs builds a generate-only `tx staking delegate` from the multisig.
func (m *Multisig) GenDelegateArgs(name, valoper, amount string, accNum, seq uint64) []string {
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

// CreateMultisigKey creates (or reuses) a K-of-N composite key over the given
// member key names. Rerun-safe: an existing composite is returned as-is. Reuse
// is keyed by name only — a stale composite created with different members or
// threshold is reused as-is, so callers that change membership must delete the
// old key first.
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
		defer func(path string) { _ = os.Remove(path) }(sigFiles[i])
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
	defer func() { _ = os.Remove(signedFile.Name()) }()

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
	// A sync broadcast returns exit 0 even when CheckTx rejects the tx, so the
	// response code must be inspected — otherwise a rejected ceremony tx would
	// poll to a misleading timeout instead of surfacing the rejection reason.
	txHash, code, rawLog, ok := parseSyncBroadcast(bcastOut)
	if !ok {
		return "", fmt.Errorf("broadcast: unparseable response: %s", bcastOut)
	}
	if code != 0 {
		return txHash, fmt.Errorf("broadcast rejected code=%d raw_log=%s", code, rawLog)
	}
	if txHash == "" {
		return "", fmt.Errorf("broadcast: missing txhash in response: %s", bcastOut)
	}
	if err := m.waitForTxIncluded(txHash, 45*time.Second); err != nil {
		return txHash, err
	}
	return txHash, nil
}

// waitForTxIncluded polls `query tx <hash>` through the exec seam until the tx
// is committed (code 0) or the timeout elapses. It mirrors
// ChainCLI.WaitForTxInclusion but routes through m.exec so the whole ceremony —
// including the inclusion wait — is testable behind a single injectable seam.
func (m *Multisig) waitForTxIncluded(txHash string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		out, err := m.exec(m.CLI.Bin, m.queryTxArgs(txHash)...)
		if err == nil {
			if _, code, rawLog, ok := parseSyncBroadcast(out); ok {
				if code != 0 {
					return fmt.Errorf("tx %s failed code=%d raw_log=%s", txHash, code, rawLog)
				}
				return nil
			}
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("timeout waiting for tx %s inclusion", txHash)
		}
		time.Sleep(time.Second)
	}
}

// queryTxArgs builds `query tx <hash>` with the standard chain/node/home flags.
func (m *Multisig) queryTxArgs(txHash string) []string {
	args := []string{"query", "tx", txHash, "--chain-id", m.CLI.ChainID, "--output", "json"}
	args = append(args, m.nodeArgs()...)
	return append(args, m.homeArgs()...)
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
