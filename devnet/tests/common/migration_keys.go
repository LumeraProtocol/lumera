package common

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// recoverKeyArgs builds the `keys add --recover` argument list for importing a
// mnemonic under an explicit key style (coin-type + algo). Explicit flags matter
// on an EVM-enabled binary, whose default algo is eth_secp256k1: a legacy
// (coin-118) account would otherwise derive the wrong address.
func recoverKeyArgs(name, keyringBackend string, style KeyStyle, home string) []string {
	args := []string{
		"keys", "add", name,
		"--recover",
		"--coin-type", strconv.FormatUint(uint64(style.CoinType), 10),
		"--algo", style.Algo,
		"--keyring-backend", keyringBackend,
		"--output", "json",
	}
	if home != "" {
		args = append(args, "--home", home)
	}
	return args
}

// ImportKeyWithStyle restores a key from a mnemonic under an explicit key style
// and returns its bech32 address. Use KeyStyleEVM to derive the coin-type-60
// eth_secp256k1 destination key for a migration; use KeyStyleLegacy to restore a
// coin-type-118 secp256k1 legacy key. Reruns should delete a stale key first.
func (c *ChainCLI) ImportKeyWithStyle(name, mnemonic string, style KeyStyle) (string, error) {
	args := recoverKeyArgs(name, c.keyringBackend(), style, c.Home)
	cmd := exec.Command(c.Bin, args...)
	cmd.Stdin = strings.NewReader(mnemonic + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("keys add --recover %s: %s: %w", name, strings.TrimSpace(string(out)), err)
	}
	return c.ShowAddress(name)
}

// claimLegacyAccountArgs builds the `tx evmigration claim-legacy-account` command
// (without gas/broadcast flags, which SubmitTx appends). The legacy key signs.
func claimLegacyAccountArgs(legacyKey, newKey string) []string {
	return []string{
		"tx", "evmigration", "claim-legacy-account", legacyKey, newKey,
		"--from", legacyKey,
	}
}

// ClaimLegacyAccount submits a single-sig MsgClaimLegacyAccount migrating the
// legacy account (legacyKey) to the new EVM-compatible account (newKey), waiting
// for inclusion. Both keys must already exist in the keyring. Returns the tx hash.
func (c *ChainCLI) ClaimLegacyAccount(legacyKey, newKey string) (string, error) {
	return c.SubmitTx(claimLegacyAccountArgs(legacyKey, newKey)...)
}

// DeriveEVMDestinationAddress imports the coin-type-60 eth_secp256k1 key derived
// from the legacy mnemonic under destKeyName and returns its bech32 address.
// This is the migration destination for a single-sig legacy account: the same
// mnemonic deterministically yields a fresh EVM-compatible address.
func (c *ChainCLI) DeriveEVMDestinationAddress(destKeyName, mnemonic string) (string, error) {
	return c.ImportKeyWithStyle(destKeyName, mnemonic, KeyStyleEVM)
}
