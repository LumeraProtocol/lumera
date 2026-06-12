package main

import (
	"fmt"
	"strconv"
)

// prompter abstracts the interactive prompts so the wizard's control flow is
// unit-testable with a scripted fake. The production implementation
// (surveyPrompter) is backed by github.com/AlecAivazis/survey/v2. Input/Confirm
// take a stable `key` (the setting key) plus a human label so tests can script
// answers without matching on display text.
type prompter interface {
	SelectOne(label string, options []string, def string) (string, error)
	Input(key, label, def string) (string, error)
	Confirm(key, label string, def bool) (bool, error)
}

// Setting keys: stable identifiers for editable settings.
const (
	settingFundingKey    = "funding-key"
	settingMode          = "mode"
	settingNumAccounts   = "num-accounts"
	settingNumMultisig23 = "num-multisig23-accounts"
	settingNumMultisig35 = "num-multisig35-accounts"
	settingAccountsPath  = "accounts"
	settingParallelism   = "parallelism"
	settingActions       = "actions"
	settingFundingBatch  = "funding-batch-size"
	settingMaxAmount     = "max-account-amount"
	settingDryRun        = "dry-run"
)

// Menu sentinels.
const (
	menuRun  = "▶ Run now"
	menuQuit = "✗ Quit"
)

// editableSettings is the ordered list of setting keys shown in the menu.
var editableSettings = []string{
	settingFundingKey,
	settingMode,
	settingNumAccounts,
	settingNumMultisig23,
	settingNumMultisig35,
	settingAccountsPath,
	settingParallelism,
	settingActions,
	settingFundingBatch,
	settingMaxAmount,
	settingDryRun,
}

// settingValue returns the current value of a setting as a display string.
func settingValue(c *Config, key string) string {
	switch key {
	case settingFundingKey:
		return c.FundingKey
	case settingMode:
		return modeLabel(c)
	case settingNumAccounts:
		return strconv.Itoa(c.NumAccounts)
	case settingNumMultisig23:
		return strconv.Itoa(c.NumMultisig23)
	case settingNumMultisig35:
		return strconv.Itoa(c.NumMultisig35)
	case settingAccountsPath:
		return c.AccountsPath
	case settingParallelism:
		return strconv.Itoa(c.Parallelism)
	case settingActions:
		return strconv.FormatBool(c.Actions)
	case settingFundingBatch:
		return strconv.Itoa(c.FundingBatchSize)
	case settingMaxAmount:
		return c.MaxAccountAmount
	case settingDryRun:
		return strconv.FormatBool(c.DryRun)
	default:
		return ""
	}
}

// modeLabel renders the current run mode derived from the AddAccounts /
// ActivityExisting flags.
func modeLabel(c *Config) string {
	switch {
	case c.AddAccounts:
		return "add-accounts"
	case c.ActivityExisting:
		return "activity-existing"
	default:
		return "fresh"
	}
}

// menuItems renders the settings menu: one entry per editable setting (with its
// current value) followed by the Run and Quit sentinels.
func menuItems(c *Config) []string {
	items := make([]string, 0, len(editableSettings)+2)
	for _, key := range editableSettings {
		items = append(items, fmt.Sprintf("%-24s %s", key, settingValue(c, key)))
	}
	items = append(items, menuRun, menuQuit)
	return items
}

// menuKeyFromItem maps a rendered menu line back to its setting key (or the
// sentinel itself for Run/Quit).
func menuKeyFromItem(item string) string {
	if item == menuRun || item == menuQuit {
		return item
	}
	for i := 0; i < len(item); i++ {
		if item[i] == ' ' {
			return item[:i]
		}
	}
	return item
}

// editSetting prompts for a new value for one setting and applies it to c. A
// parse error on a numeric field is returned and leaves c unchanged.
func editSetting(c *Config, key string, p prompter) error {
	switch key {
	case settingFundingKey:
		v, err := p.Input(key, "Funder key name", c.FundingKey)
		if err != nil {
			return err
		}
		c.FundingKey = v
	case settingMode:
		v, err := p.SelectOne("Run mode", []string{"fresh", "add-accounts", "activity-existing"}, modeLabel(c))
		if err != nil {
			return err
		}
		c.AddAccounts = v == "add-accounts"
		c.ActivityExisting = v == "activity-existing"
	case settingAccountsPath:
		v, err := p.Input(key, "Accounts registry path", c.AccountsPath)
		if err != nil {
			return err
		}
		c.AccountsPath = v
	case settingMaxAmount:
		v, err := p.Input(key, "Max per-account amount", c.MaxAccountAmount)
		if err != nil {
			return err
		}
		c.MaxAccountAmount = v
	case settingActions:
		v, err := p.Confirm(key, "Include CASCADE action activity?", c.Actions)
		if err != nil {
			return err
		}
		c.Actions = v
	case settingDryRun:
		v, err := p.Confirm(key, "Dry run (plan only, no txs)?", c.DryRun)
		if err != nil {
			return err
		}
		c.DryRun = v
	case settingNumAccounts:
		return editInt(key,"Number of accounts", &c.NumAccounts, p)
	case settingNumMultisig23:
		return editInt(key,"Number of 2-of-3 multisig accounts", &c.NumMultisig23, p)
	case settingNumMultisig35:
		return editInt(key,"Number of 3-of-5 multisig accounts", &c.NumMultisig35, p)
	case settingParallelism:
		return editInt(key,"Parallelism", &c.Parallelism, p)
	case settingFundingBatch:
		return editInt(key,"Funding batch size", &c.FundingBatchSize, p)
	default:
		return fmt.Errorf("unknown setting %q", key)
	}
	return nil
}

// editInt prompts for an integer setting and applies it only on a clean parse.
func editInt(key, label string, dst *int, p prompter) error {
	v, err := p.Input(key, label, strconv.Itoa(*dst))
	if err != nil {
		return err
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("%s: %q is not an integer", key, v)
	}
	*dst = n
	return nil
}
