package main

import "testing"

func hasSetting(items []string, key string) bool {
	for _, it := range items {
		if menuKeyFromItem(it) == key {
			return true
		}
	}
	return false
}

func TestVisibleSettingsMigrateModeHidesGenerationFields(t *testing.T) {
	c := &Config{Mode: ModeMigrate}
	items := menuItems(c)

	// Migrate mode keeps only registry/connection/execution-relevant knobs.
	for _, want := range []string{settingMode, settingAccountsPath, settingParallelism, settingDryRun} {
		if !hasSetting(items, want) {
			t.Errorf("migrate menu missing expected setting %q", want)
		}
	}
	// Generation/funding settings are irrelevant in migrate mode and must be hidden.
	for _, hidden := range []string{
		settingFundingKey, settingNumAccounts, settingNumMultisig23, settingNumMultisig35,
		settingActions, settingFundingBatch, settingMaxAmount, settingVestingPercent, settingNumPermLocked,
	} {
		if hasSetting(items, hidden) {
			t.Errorf("migrate menu should hide setting %q", hidden)
		}
	}
}

func TestVisibleSettingsFreshModeShowsGenerationFields(t *testing.T) {
	c := &Config{} // fresh
	items := menuItems(c)
	for _, want := range []string{settingFundingKey, settingNumAccounts, settingMaxAmount, settingVestingPercent} {
		if !hasSetting(items, want) {
			t.Errorf("fresh menu missing expected setting %q", want)
		}
	}
}

func TestEditSettingModeMigrate(t *testing.T) {
	c := &Config{}
	p := &fakePrompter{selectQueue: []string{"migrate"}}
	if err := editSetting(c, settingMode, p); err != nil {
		t.Fatalf("edit mode migrate: %v", err)
	}
	if c.Mode != ModeMigrate {
		t.Errorf("Mode = %q, want %q", c.Mode, ModeMigrate)
	}
	if c.AddAccounts || c.ActivityExisting {
		t.Errorf("legacy bools must be cleared when selecting migrate: add=%v activity=%v", c.AddAccounts, c.ActivityExisting)
	}
	if modeLabel(c) != ModeMigrate {
		t.Errorf("modeLabel = %q, want %q", modeLabel(c), ModeMigrate)
	}
}
