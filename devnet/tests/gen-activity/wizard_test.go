package main

import (
	"strings"
	"testing"
)

func TestMenuItemsRenderCurrentValues(t *testing.T) {
	c := &Config{
		FundingKey:    "faucet",
		NumAccounts:   10,
		NumMultisig23: 2,
		Parallelism:   5,
		Actions:       true,
		AccountsPath:  "accounts.json",
	}
	items := menuItems(c)

	joined := strings.Join(items, "\n")
	if !strings.Contains(joined, menuRun) {
		t.Errorf("menu missing run entry %q:\n%s", menuRun, joined)
	}
	if !strings.Contains(joined, menuQuit) {
		t.Errorf("menu missing quit entry %q:\n%s", menuQuit, joined)
	}
	if !strings.Contains(joined, "faucet") || !strings.Contains(joined, "10") {
		t.Errorf("menu missing current values:\n%s", joined)
	}
}

func TestEditSettingUpdatesConfig(t *testing.T) {
	c := &Config{FundingKey: "old", NumAccounts: 1, Actions: false}
	p := &fakePrompter{
		inputs:   map[string]string{"funding-key": "newfunder", "num-accounts": "7"},
		confirms: map[string]bool{"actions": true},
	}

	if err := editSetting(c, settingFundingKey, p); err != nil {
		t.Fatalf("edit funding-key: %v", err)
	}
	if c.FundingKey != "newfunder" {
		t.Errorf("FundingKey = %q, want newfunder", c.FundingKey)
	}

	if err := editSetting(c, settingNumAccounts, p); err != nil {
		t.Fatalf("edit num-accounts: %v", err)
	}
	if c.NumAccounts != 7 {
		t.Errorf("NumAccounts = %d, want 7", c.NumAccounts)
	}

	if err := editSetting(c, settingActions, p); err != nil {
		t.Fatalf("edit actions: %v", err)
	}
	if !c.Actions {
		t.Errorf("Actions = %v, want true", c.Actions)
	}
}

func TestEditSettingRejectsBadInt(t *testing.T) {
	c := &Config{NumAccounts: 3}
	p := &fakePrompter{inputs: map[string]string{"num-accounts": "notanumber"}}
	if err := editSetting(c, settingNumAccounts, p); err == nil {
		t.Error("expected error for non-integer num-accounts input")
	}
	if c.NumAccounts != 3 {
		t.Errorf("NumAccounts mutated to %d on bad input, want unchanged 3", c.NumAccounts)
	}
}

func TestEditSettingModeMapping(t *testing.T) {
	cases := []struct {
		choice           string
		addAccounts      bool
		activityExisting bool
	}{
		{"fresh", false, false},
		{"add-accounts", true, false},
		{"activity-existing", false, true},
	}
	for _, tc := range cases {
		c := &Config{}
		p := &fakePrompter{selectQueue: []string{tc.choice}}
		if err := editSetting(c, settingMode, p); err != nil {
			t.Fatalf("edit mode %q: %v", tc.choice, err)
		}
		if c.AddAccounts != tc.addAccounts || c.ActivityExisting != tc.activityExisting {
			t.Errorf("mode %q -> AddAccounts=%v ActivityExisting=%v, want %v/%v",
				tc.choice, c.AddAccounts, c.ActivityExisting, tc.addAccounts, tc.activityExisting)
		}
	}
}

func TestRunWizardManualChainNoConfig(t *testing.T) {
	cfg := &Config{
		Bin: "lumerad", KeyringBackend: "test", FundingKey: "faucet",
		AccountsPath: "accounts.json", MaxAccountAmount: "10000000ulume",
		ActionStates: "pending,done,approved", MaxActionsPerRun: 3,
		FundingBatchSize: 10, Parallelism: 5,
	}
	// fc == nil: runWizard takes the manual-entry branch (promptManualChain),
	// then the menu loop exits immediately on Quit.
	p := &fakePrompter{
		selectQueue: []string{menuQuit},
		inputs: map[string]string{
			"rpc":      "tcp://manual:26657",
			"grpc":     "manual:9090",
			"chain-id": "lumera-manual-1",
		},
	}
	if err := runWizard(cfg, nil, nil, p, func(*Config) error { return nil }); err != nil {
		t.Fatalf("runWizard: %v", err)
	}
	if cfg.RPC != "tcp://manual:26657" || cfg.GRPC != "manual:9090" || cfg.ChainID != "lumera-manual-1" {
		t.Errorf("manual chain not applied: rpc=%q grpc=%q chain-id=%q", cfg.RPC, cfg.GRPC, cfg.ChainID)
	}
}

// fakePrompter scripts wizard answers keyed by setting key. selectQueue is a
// FIFO of answers for SelectOne calls.
type fakePrompter struct {
	selectQueue []string
	inputs      map[string]string
	confirms    map[string]bool
}

func (f *fakePrompter) SelectOne(label string, options []string, def string) (string, error) {
	if len(f.selectQueue) == 0 {
		return def, nil
	}
	ans := f.selectQueue[0]
	f.selectQueue = f.selectQueue[1:]
	return ans, nil
}

func (f *fakePrompter) Input(key, label, def string) (string, error) {
	if v, ok := f.inputs[key]; ok {
		return v, nil
	}
	return def, nil
}

func (f *fakePrompter) Confirm(key, label string, def bool) (bool, error) {
	if v, ok := f.confirms[key]; ok {
		return v, nil
	}
	return def, nil
}

func TestRunWizardEditsThenRuns(t *testing.T) {
	fc, err := LoadFileConfig(writeTempTOML(t, sampleTOML))
	if err != nil {
		t.Fatalf("LoadFileConfig: %v", err)
	}
	cfg := &Config{
		Bin: "lumerad", KeyringBackend: "test",
		MaxAccountAmount: "10000000ulume", ActionStates: "pending,done,approved",
		MaxActionsPerRun: 3, FundingBatchSize: 10, Parallelism: 5,
	}

	// Script: pick chain "devnet", edit funding-key, then Run.
	p := &fakePrompter{
		selectQueue: []string{
			"devnet",                           // chain picker
			menuKeyFromItem(menuItems(cfg)[0]), // first menu choice -> funding-key setting line
			menuRun,                            // then run
		},
		inputs: map[string]string{"funding-key": "wizfunder"},
	}

	var ran *Config
	runner := func(c *Config) error { ran = c; return nil }

	if err := runWizard(cfg, fc, nil, p, runner); err != nil {
		t.Fatalf("runWizard: %v", err)
	}
	if ran == nil {
		t.Fatal("runner was not invoked on Run")
	}
	if ran.ChainID != "lumera-devnet-1" {
		t.Errorf("ChainID = %q, want lumera-devnet-1 (chain applied)", ran.ChainID)
	}
	if ran.FundingKey != "wizfunder" {
		t.Errorf("FundingKey = %q, want wizfunder (edited)", ran.FundingKey)
	}
}

func TestRunWizardPreservesCLIOverridesOnReseed(t *testing.T) {
	fc, err := LoadFileConfig(writeTempTOML(t, sampleTOML))
	if err != nil {
		t.Fatalf("LoadFileConfig: %v", err)
	}
	// Simulate `-funding-key cliuser -accounts cli-accounts.json -w`: resolveConfig
	// already layered the file onto cfg honoring these flags, so cfg holds the CLI
	// values and setFlags marks them as explicitly set.
	cfg := &Config{
		Bin: "lumerad", KeyringBackend: "test",
		FundingKey: "cliuser", AccountsPath: "cli-accounts.json",
		MaxAccountAmount: "10000000ulume", ActionStates: "pending,done,approved",
		MaxActionsPerRun: 3, FundingBatchSize: 10, Parallelism: 5,
	}
	setFlags := map[string]bool{"funding-key": true, "accounts": true}

	// Pick the devnet chain (which re-seeds [common]+[chains.devnet]), then Quit.
	// [common] sets funding-key=faucet and [chains.devnet] sets accounts=...; both
	// must NOT clobber the explicit CLI overrides.
	p := &fakePrompter{selectQueue: []string{"devnet", menuQuit}}

	if err := runWizard(cfg, fc, setFlags, p, func(*Config) error { return nil }); err != nil {
		t.Fatalf("runWizard: %v", err)
	}
	if cfg.FundingKey != "cliuser" {
		t.Errorf("FundingKey = %q, want cliuser (CLI override must survive reseed)", cfg.FundingKey)
	}
	if cfg.AccountsPath != "cli-accounts.json" {
		t.Errorf("AccountsPath = %q, want cli-accounts.json (CLI override must survive reseed)", cfg.AccountsPath)
	}
	// Non-overridden chain values should still be seeded from the chosen chain.
	if cfg.ChainID != "lumera-devnet-1" {
		t.Errorf("ChainID = %q, want lumera-devnet-1 (chain seeded)", cfg.ChainID)
	}
}

func TestRunWizardQuitDoesNotRun(t *testing.T) {
	cfg := &Config{Bin: "lumerad"}
	p := &fakePrompter{selectQueue: []string{menuQuit}}
	called := false
	runner := func(*Config) error { called = true; return nil }

	if err := runWizard(cfg, nil, nil, p, runner); err != nil {
		t.Fatalf("runWizard: %v", err)
	}
	if called {
		t.Error("runner must not be called when user quits")
	}
}

func TestEditSettingVesting(t *testing.T) {
	c := &Config{}
	p := &fakePrompter{inputs: map[string]string{
		"vesting-percent":               "25",
		"num-permanent-locked-accounts": "3",
	}}
	if err := editSetting(c, settingVestingPercent, p); err != nil {
		t.Fatalf("edit vesting-percent: %v", err)
	}
	if c.VestingPercent != 25 {
		t.Errorf("VestingPercent = %d, want 25", c.VestingPercent)
	}
	if err := editSetting(c, settingNumPermLocked, p); err != nil {
		t.Fatalf("edit num-permanent-locked: %v", err)
	}
	if c.NumPermanentLocked != 3 {
		t.Errorf("NumPermanentLocked = %d, want 3", c.NumPermanentLocked)
	}
}

func TestMenuItemsIncludeVesting(t *testing.T) {
	items := menuItems(&Config{})
	for _, want := range []string{settingVestingPercent, settingNumPermLocked} {
		found := false
		for _, it := range items {
			if menuKeyFromItem(it) == want {
				found = true
			}
		}
		if !found {
			t.Errorf("menu missing setting %q", want)
		}
	}
}

func TestChainSummaryShowsConnectionAndDivider(t *testing.T) {
	c := &Config{Chain: "devnet", ChainID: "lumera-devnet-1", RPC: "tcp://localhost:26657", GRPC: "localhost:9090"}
	out := chainSummary(c)
	for _, want := range []string{"devnet", "lumera-devnet-1", "tcp://localhost:26657", "localhost:9090", "----------"} {
		if !strings.Contains(out, want) {
			t.Errorf("chainSummary missing %q:\n%s", want, out)
		}
	}
}
