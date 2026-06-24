package main

import (
	"fmt"
	"time"

	"gen/tests/common"
)

// Run modes. The legacy boolean flags (AddAccounts/ActivityExisting) fold into
// these via resolveMode; the unset state means ModeFresh.
const (
	ModeFresh            = "fresh"
	ModeAddAccounts      = "add-accounts"
	ModeActivityExisting = "activity-existing"
	ModeMigrate          = "migrate"
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

	// Mode is the explicit run mode (fresh|add-accounts|activity-existing|
	// migrate). Empty means "derive from the legacy boolean flags below".
	Mode string

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
	resolvedMode string
}

// resolveMode reconciles the explicit Mode field with the legacy boolean flags
// (AddAccounts/ActivityExisting) into a single canonical mode, returning an
// error on any contradiction. The unset state means ModeFresh.
func (c *Config) resolveMode() (string, error) {
	// Mode implied by the legacy boolean flags.
	boolMode := ""
	switch {
	case c.AddAccounts && c.ActivityExisting:
		return "", fmt.Errorf("-add-accounts and -activity-existing are mutually exclusive")
	case c.AddAccounts:
		boolMode = ModeAddAccounts
	case c.ActivityExisting:
		boolMode = ModeActivityExisting
	}

	if c.Mode == "" {
		if boolMode == "" {
			return ModeFresh, nil
		}
		return boolMode, nil
	}

	switch c.Mode {
	case ModeFresh, ModeAddAccounts, ModeActivityExisting, ModeMigrate:
	default:
		return "", fmt.Errorf("invalid -mode %q (want fresh|add-accounts|activity-existing|migrate)", c.Mode)
	}

	// An explicit mode must not contradict a legacy boolean flag.
	if boolMode != "" && boolMode != c.Mode {
		return "", fmt.Errorf("-mode %q conflicts with -%s", c.Mode, boolMode)
	}
	return c.Mode, nil
}

// Validate checks the configuration and populates parsed values (maxAmount,
// actionStates). It returns the first error encountered.
func (c *Config) Validate() error {
	mode, err := c.resolveMode()
	if err != nil {
		return err
	}
	c.resolvedMode = mode

	// Requirements common to every mode.
	if c.Bin == "" {
		return fmt.Errorf("-bin is required")
	}
	if c.ChainID == "" {
		return fmt.Errorf("-chain-id is required")
	}
	if c.AccountsPath == "" {
		return fmt.Errorf("-accounts is required")
	}
	if c.Parallelism < 1 {
		return fmt.Errorf("-parallelism must be >= 1, got %d", c.Parallelism)
	}

	// Migrate mode signs with the legacy accounts and uses evmigration fee
	// handling, so it needs neither a funder key nor any generation/activity
	// settings. Skip those checks.
	if mode == ModeMigrate {
		return nil
	}

	if c.FundingKey == "" {
		return fmt.Errorf("-funding-key is required")
	}
	if c.NumAccounts < 0 {
		return fmt.Errorf("-num-accounts must be >= 0, got %d", c.NumAccounts)
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
