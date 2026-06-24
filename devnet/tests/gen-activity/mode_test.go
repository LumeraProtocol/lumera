package main

import "testing"

func TestResolveMode(t *testing.T) {
	t.Run("nothing set defaults to fresh", func(t *testing.T) {
		c := Config{}
		got, err := c.resolveMode()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != ModeFresh {
			t.Errorf("resolveMode() = %q, want %q", got, ModeFresh)
		}
	})

	t.Run("add-accounts bool implies add-accounts mode", func(t *testing.T) {
		c := Config{AddAccounts: true}
		got, err := c.resolveMode()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != ModeAddAccounts {
			t.Errorf("resolveMode() = %q, want %q", got, ModeAddAccounts)
		}
	})

	t.Run("activity-existing bool implies activity-existing mode", func(t *testing.T) {
		c := Config{ActivityExisting: true}
		got, err := c.resolveMode()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != ModeActivityExisting {
			t.Errorf("resolveMode() = %q, want %q", got, ModeActivityExisting)
		}
	})

	t.Run("explicit migrate mode resolves to migrate", func(t *testing.T) {
		c := Config{Mode: ModeMigrate}
		got, err := c.resolveMode()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != ModeMigrate {
			t.Errorf("resolveMode() = %q, want %q", got, ModeMigrate)
		}
	})

	t.Run("explicit mode matching the bool is not a conflict", func(t *testing.T) {
		c := Config{Mode: ModeAddAccounts, AddAccounts: true}
		got, err := c.resolveMode()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != ModeAddAccounts {
			t.Errorf("resolveMode() = %q, want %q", got, ModeAddAccounts)
		}
	})

	t.Run("migrate mode conflicts with add-accounts bool", func(t *testing.T) {
		c := Config{Mode: ModeMigrate, AddAccounts: true}
		if _, err := c.resolveMode(); err == nil {
			t.Error("expected conflict error for migrate mode + add-accounts bool")
		}
	})

	t.Run("both legacy bools are mutually exclusive", func(t *testing.T) {
		c := Config{AddAccounts: true, ActivityExisting: true}
		if _, err := c.resolveMode(); err == nil {
			t.Error("expected error when both add-accounts and activity-existing set")
		}
	})

	t.Run("unknown mode string fails", func(t *testing.T) {
		c := Config{Mode: "bogus"}
		if _, err := c.resolveMode(); err == nil {
			t.Error("expected error for unknown mode")
		}
	})
}

func TestConfigValidateMigrateMode(t *testing.T) {
	migrateConfig := func() Config {
		return Config{
			Bin:          "lumerad",
			ChainID:      "lumera-devnet-1",
			AccountsPath: "accounts.json",
			Mode:         ModeMigrate,
			Parallelism:  5,
		}
	}

	t.Run("migrate mode does not require funding key or generation fields", func(t *testing.T) {
		c := migrateConfig()
		if err := c.Validate(); err != nil {
			t.Fatalf("unexpected error in migrate mode: %v", err)
		}
		if c.resolvedMode != ModeMigrate {
			t.Errorf("resolvedMode = %q, want %q", c.resolvedMode, ModeMigrate)
		}
	})

	t.Run("migrate mode still requires accounts path", func(t *testing.T) {
		c := migrateConfig()
		c.AccountsPath = ""
		if err := c.Validate(); err == nil {
			t.Error("expected error for missing accounts path in migrate mode")
		}
	})

	t.Run("migrate mode still requires chain-id", func(t *testing.T) {
		c := migrateConfig()
		c.ChainID = ""
		if err := c.Validate(); err == nil {
			t.Error("expected error for missing chain-id in migrate mode")
		}
	})

	t.Run("migrate mode still requires parallelism >= 1", func(t *testing.T) {
		c := migrateConfig()
		c.Parallelism = 0
		if err := c.Validate(); err == nil {
			t.Error("expected error for parallelism < 1 in migrate mode")
		}
	})
}
