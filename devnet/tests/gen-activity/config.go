package main

import (
	"fmt"
	"time"

	"gen/tests/common"
)

// Config holds the parsed command-line configuration for the activity
// generator.
type Config struct {
	// Connection / runtime.
	Bin            string
	RPC            string
	GRPC           string
	ChainID        string
	Home           string
	KeyringBackend string
	EVMCutoverVer  string

	// Config file + chain selection + mode.
	ConfigPath string
	Chain      string
	Wizard     bool

	// Funding signer.
	FundingKey string

	// Registry and account generation.
	AccountsPath     string
	NumAccounts      int
	MaxAccountAmount string
	AccountPrefix    string

	// Multisig account generation. Behavior implemented in the multisig plan;
	// validated here so the flag/wizard surface is complete.
	NumMultisig23 int // 2-of-3 multisig accounts
	NumMultisig35 int // 3-of-5 multisig accounts

	VestingPercent     int // percent of regular accounts created as vesting (0-100)
	NumPermanentLocked int // dedicated PermanentLocked accounts

	// Rerun modes.
	AddAccounts      bool
	ActivityExisting bool

	// Actions.
	Actions                bool
	RequireActions         bool
	MaxActionsPerRun       int
	ActionStates           string
	ActionReadinessTimeout time.Duration

	// Execution.
	FundingBatchSize int
	Parallelism      int
	DryRun           bool

	// Parsed/validated values, populated by Validate.
	maxAmount    common.Coin
	actionStates []common.ActionState
}

// Validate checks the configuration and populates parsed values (maxAmount,
// actionStates). It returns the first error encountered.
func (c *Config) Validate() error {
	if c.Bin == "" {
		return fmt.Errorf("-bin is required")
	}
	if c.ChainID == "" {
		return fmt.Errorf("-chain-id is required")
	}
	if c.FundingKey == "" {
		return fmt.Errorf("-funding-key is required")
	}
	if c.AccountsPath == "" {
		return fmt.Errorf("-accounts is required")
	}
	if c.NumAccounts < 0 {
		return fmt.Errorf("-num-accounts must be >= 0, got %d", c.NumAccounts)
	}
	if c.Parallelism < 1 {
		return fmt.Errorf("-parallelism must be >= 1, got %d", c.Parallelism)
	}
	if c.FundingBatchSize < 1 {
		return fmt.Errorf("-funding-batch-size must be >= 1, got %d", c.FundingBatchSize)
	}
	if c.MaxActionsPerRun < 0 {
		return fmt.Errorf("-max-actions-per-run must be >= 0, got %d", c.MaxActionsPerRun)
	}
	if c.NumMultisig23 < 0 {
		return fmt.Errorf("-num-multisig23-accounts must be >= 0, got %d", c.NumMultisig23)
	}
	if c.NumMultisig35 < 0 {
		return fmt.Errorf("-num-multisig35-accounts must be >= 0, got %d", c.NumMultisig35)
	}
	if c.VestingPercent < 0 || c.VestingPercent > 100 {
		return fmt.Errorf("-vesting-percent must be in [0,100], got %d", c.VestingPercent)
	}
	if c.NumPermanentLocked < 0 {
		return fmt.Errorf("-num-permanent-locked-accounts must be >= 0, got %d", c.NumPermanentLocked)
	}

	coin, err := common.ValidateMaxAccountAmount(c.MaxAccountAmount)
	if err != nil {
		return err
	}
	c.maxAmount = coin

	if c.Actions {
		states, err := common.ParseActionStates(c.ActionStates)
		if err != nil {
			return err
		}
		c.actionStates = states
	}
	return nil
}
