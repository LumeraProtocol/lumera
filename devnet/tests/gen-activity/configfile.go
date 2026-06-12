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
	NumMultisig23    *int    `toml:"num-multisig23-accounts"`
	NumMultisig35    *int    `toml:"num-multisig35-accounts"`
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

// ChainNames returns the configured chain names in sorted order (stable wizard
// menu ordering).
func (fc *FileConfig) ChainNames() []string {
	names := make([]string, 0, len(fc.Chains))
	for name := range fc.Chains {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
