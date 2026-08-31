package common

import (
	"fmt"
	"strconv"
)

// VestingType labels the vesting/locked account variants gen-activity creates.
type VestingType string

const (
	VestingContinuous      VestingType = "continuous"
	VestingDelayed         VestingType = "delayed"
	VestingPermanentLocked VestingType = "permanent_locked"
)

// VestingCreateArgs builds `tx vesting create-vesting-account <to> <amount>
// <end-unix> [--delayed] --from <funder>`. Gas/broadcast/keyring/node flags are
// appended by ChainCLI.SubmitTx, so they are intentionally absent here.
func VestingCreateArgs(funderKey, to, amount string, endUnix int64, delayed bool) []string {
	args := []string{
		"tx", "vesting", "create-vesting-account",
		to, amount, strconv.FormatInt(endUnix, 10),
		"--from", funderKey,
	}
	if delayed {
		args = append(args, "--delayed")
	}
	return args
}

// PermanentLockedArgs builds `tx vesting create-permanent-locked-account <to>
// <amount> --from <funder>`.
func PermanentLockedArgs(funderKey, to, amount string) []string {
	return []string{
		"tx", "vesting", "create-permanent-locked-account",
		to, amount,
		"--from", funderKey,
	}
}

// CreateVestingAccount creates a continuous (delayed=false) or delayed/cliff
// (delayed=true) vesting account funded with amount, signed by funderKey. It
// waits for inclusion (SubmitTx semantics) and returns the tx hash.
func (c *ChainCLI) CreateVestingAccount(funderKey, to, amount string, endUnix int64, delayed bool) (string, error) {
	txHash, err := c.SubmitTx(VestingCreateArgs(funderKey, to, amount, endUnix, delayed)...)
	if err != nil {
		return txHash, fmt.Errorf("create vesting account %s: %w", to, err)
	}
	return txHash, nil
}

// CreatePermanentLockedAccount creates a PermanentLockedAccount funded with
// amount, signed by funderKey.
func (c *ChainCLI) CreatePermanentLockedAccount(funderKey, to, amount string) (string, error) {
	txHash, err := c.SubmitTx(PermanentLockedArgs(funderKey, to, amount)...)
	if err != nil {
		return txHash, fmt.Errorf("create permanent-locked account %s: %w", to, err)
	}
	return txHash, nil
}
