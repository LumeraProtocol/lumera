package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"

	toml "github.com/pelletier/go-toml/v2"
)

// ChainSection holds the config values for one chain (or the shared [common]
// section). Every field is a pointer so an absent TOML key is distinguishable
// from an explicit zero value; nil means "not set in this section". Keys mirror
// the CLI flag names so config and flags map one-to-one.
type ChainSection struct {
	Bin              *string `toml:"bin"`
	RPC              *string `toml:"rpc"`
	GRPC             *string `toml:"grpc"`
	ChainID          *string `toml:"chain-id"`
	Home             *string `toml:"home"`
	KeyringBackend   *string `toml:"keyring-backend"`
	EVMCutoverVer    *string `toml:"evm-cutover-version"`
	FundingKey       *string `toml:"funding-key"`
	AccountsPath     *string `toml:"accounts"`
	NumAccounts      *int    `toml:"num-accounts"`
	MaxAccountAmount *string `toml:"max-account-amount"`
	AccountPrefix    *string `toml:"account-prefix"`
	Actions          *bool   `toml:"actions"`
	FundingBatchSize *int    `toml:"funding-batch-size"`
	Parallelism      *int    `toml:"parallelism"`
	NumMultisig23      *int    `toml:"num-multisig23-accounts"`
	NumMultisig35      *int    `toml:"num-multisig35-accounts"`
	VestingPercent     *int    `toml:"vesting-percent"`
	NumPermanentLocked *int    `toml:"num-permanent-locked-accounts"`
}

// FileConfig is the parsed gen-activity-config.toml: a shared [common] section
// plus any number of named [chains.<name>] sections.
type FileConfig struct {
	Common ChainSection            `toml:"common"`
	Chains map[string]ChainSection `toml:"chains"`
}

// LoadFileConfig reads and strictly decodes the TOML config at path. A missing
// file returns (nil, nil) so the caller can fall back to pure flag behavior; an
// unparseable file or an unknown key is a hard error.
func LoadFileConfig(path string) (*FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var fc FileConfig
	if err := strictUnmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return &fc, nil
}

// strictUnmarshal decodes TOML with unknown keys rejected so typos in the
// config file surface as errors instead of being silently ignored.
func strictUnmarshal(data []byte, v any) error {
	d := toml.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	return d.Decode(v)
}

// ApplyFileConfig layers a parsed FileConfig onto c following the precedence
// defaults < [common] < [chains.<chain>] < explicit CLI flags. setFlags is the
// set of flag names the user passed on the command line (collected via
// flag.Visit); fields whose flag was set are never overwritten by the config.
// An empty chain applies only [common]; a non-empty chain that is absent from
// the file is an error.
func ApplyFileConfig(c *Config, fc *FileConfig, chain string, setFlags map[string]bool) error {
	if fc == nil {
		return nil
	}
	applyLayer(c, fc.Common, setFlags)
	if chain == "" {
		return nil
	}
	sec, ok := fc.Chains[chain]
	if !ok {
		return fmt.Errorf("chain %q not found in config (available: %v)", chain, fc.ChainNames())
	}
	applyLayer(c, sec, setFlags)
	return nil
}

// applyLayer overlays the non-nil fields of sec onto c, skipping any field
// whose corresponding flag name appears in setFlags (so explicit CLI flags are
// never clobbered). The flag names here MUST match those registered in
// configureFlags.
func applyLayer(c *Config, sec ChainSection, setFlags map[string]bool) {
	str := func(flagName string, src *string, dst *string) {
		if src != nil && !setFlags[flagName] {
			*dst = *src
		}
	}
	intf := func(flagName string, src *int, dst *int) {
		if src != nil && !setFlags[flagName] {
			*dst = *src
		}
	}
	boolf := func(flagName string, src *bool, dst *bool) {
		if src != nil && !setFlags[flagName] {
			*dst = *src
		}
	}

	str("bin", sec.Bin, &c.Bin)
	str("rpc", sec.RPC, &c.RPC)
	str("grpc", sec.GRPC, &c.GRPC)
	str("chain-id", sec.ChainID, &c.ChainID)
	str("home", sec.Home, &c.Home)
	str("keyring-backend", sec.KeyringBackend, &c.KeyringBackend)
	str("evm-cutover-version", sec.EVMCutoverVer, &c.EVMCutoverVer)
	str("funding-key", sec.FundingKey, &c.FundingKey)
	str("accounts", sec.AccountsPath, &c.AccountsPath)
	str("max-account-amount", sec.MaxAccountAmount, &c.MaxAccountAmount)
	str("account-prefix", sec.AccountPrefix, &c.AccountPrefix)
	intf("num-accounts", sec.NumAccounts, &c.NumAccounts)
	intf("funding-batch-size", sec.FundingBatchSize, &c.FundingBatchSize)
	intf("parallelism", sec.Parallelism, &c.Parallelism)
	intf("num-multisig23-accounts", sec.NumMultisig23, &c.NumMultisig23)
	intf("num-multisig35-accounts", sec.NumMultisig35, &c.NumMultisig35)
	intf("vesting-percent", sec.VestingPercent, &c.VestingPercent)
	intf("num-permanent-locked-accounts", sec.NumPermanentLocked, &c.NumPermanentLocked)
	boolf("actions", sec.Actions, &c.Actions)
}

// ChainNames returns the configured chain names in sorted order (stable wizard
// menu ordering).
func (fc *FileConfig) ChainNames() []string {
	if fc == nil {
		return nil
	}
	names := make([]string, 0, len(fc.Chains))
	for name := range fc.Chains {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
