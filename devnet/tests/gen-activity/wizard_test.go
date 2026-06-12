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
