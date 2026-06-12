package common

import (
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
