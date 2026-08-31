package main

import (
	"testing"
	"time"
)

func validConfig() Config {
	return Config{
		Bin:                    "lumerad",
		ChainID:                "lumera-devnet-1",
		FundingKey:             "funder",
		AccountsPath:           "accounts.json",
		NumAccounts:            10,
		MaxAccountAmount:       "10000000ulume",
		KeyringBackend:         "test",
		Actions:                true,
		ActionStates:           "pending,done,approved",
		MaxActionsPerRun:       3,
		ActionReadinessTimeout: 180 * time.Second,
		FundingBatchSize:       10,
		Parallelism:            5,
	}
}

func TestConfigValidate(t *testing.T) {
	t.Run("valid config passes and exposes parsed values", func(t *testing.T) {
		c := validConfig()
		if err := c.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.maxAmount.Amount != 10000000 {
			t.Errorf("parsed max amount = %d, want 10000000", c.maxAmount.Amount)
		}
		if len(c.actionStates) != 3 {
			t.Errorf("parsed action states = %d, want 3", len(c.actionStates))
		}
	})

	t.Run("missing chain-id fails", func(t *testing.T) {
		c := validConfig()
		c.ChainID = ""
		if err := c.Validate(); err == nil {
			t.Error("expected error for empty chain-id")
		}
	})

	t.Run("missing funding key fails", func(t *testing.T) {
		c := validConfig()
		c.FundingKey = ""
		if err := c.Validate(); err == nil {
			t.Error("expected error for empty funding key")
		}
	})

	t.Run("bad max-account-amount fails", func(t *testing.T) {
		c := validConfig()
		c.MaxAccountAmount = "0ulume"
		if err := c.Validate(); err == nil {
			t.Error("expected error for zero max-account-amount")
		}
	})

	t.Run("bad action-states fails when actions enabled", func(t *testing.T) {
		c := validConfig()
		c.ActionStates = "pending,bogus"
		if err := c.Validate(); err == nil {
			t.Error("expected error for bad action states")
		}
	})

	t.Run("bad action-states ignored when actions disabled", func(t *testing.T) {
		c := validConfig()
		c.Actions = false
		c.ActionStates = "bogus"
		if err := c.Validate(); err != nil {
			t.Errorf("unexpected error when actions disabled: %v", err)
		}
	})

	t.Run("parallelism must be at least 1", func(t *testing.T) {
		c := validConfig()
		c.Parallelism = 0
		if err := c.Validate(); err == nil {
			t.Error("expected error for parallelism < 1")
		}
	})

	t.Run("funding batch size must be at least 1", func(t *testing.T) {
		c := validConfig()
		c.FundingBatchSize = 0
		if err := c.Validate(); err == nil {
			t.Error("expected error for funding batch size < 1")
		}
	})

	t.Run("negative num-accounts fails", func(t *testing.T) {
		c := validConfig()
		c.NumAccounts = -1
		if err := c.Validate(); err == nil {
			t.Error("expected error for negative num-accounts")
		}
	})

	t.Run("add-accounts and activity-existing are mutually exclusive", func(t *testing.T) {
		c := validConfig()
		c.AddAccounts = true
		c.ActivityExisting = true
		if err := c.Validate(); err == nil {
			t.Error("expected error when both add-accounts and activity-existing are set")
		}
	})
}

func TestConfigValidateVesting(t *testing.T) {
	t.Run("valid vesting-percent passes", func(t *testing.T) {
		c := validConfig()
		c.VestingPercent = 30
		c.NumPermanentLocked = 2
		if err := c.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("vesting-percent over 100 fails", func(t *testing.T) {
		c := validConfig()
		c.VestingPercent = 101
		if err := c.Validate(); err == nil {
			t.Error("expected error for vesting-percent > 100")
		}
	})
	t.Run("negative vesting-percent fails", func(t *testing.T) {
		c := validConfig()
		c.VestingPercent = -1
		if err := c.Validate(); err == nil {
			t.Error("expected error for negative vesting-percent")
		}
	})
	t.Run("negative num-permanent-locked fails", func(t *testing.T) {
		c := validConfig()
		c.NumPermanentLocked = -1
		if err := c.Validate(); err == nil {
			t.Error("expected error for negative num-permanent-locked-accounts")
		}
	})
}

func TestConfigValidateMultisigCounts(t *testing.T) {
	t.Run("zero multisig counts pass", func(t *testing.T) {
		c := validConfig()
		c.NumMultisig23 = 0
		c.NumMultisig35 = 0
		if err := c.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("negative num-multisig23 fails", func(t *testing.T) {
		c := validConfig()
		c.NumMultisig23 = -1
		if err := c.Validate(); err == nil {
			t.Error("expected error for negative num-multisig23-accounts")
		}
	})

	t.Run("negative num-multisig35 fails", func(t *testing.T) {
		c := validConfig()
		c.NumMultisig35 = -2
		if err := c.Validate(); err == nil {
			t.Error("expected error for negative num-multisig35-accounts")
		}
	})
}
