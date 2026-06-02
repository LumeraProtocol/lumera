package main

import (
	"fmt"

	"gen/tests/common"
)

// cliActivityChain implements activityChain by issuing lumerad tx subcommands
// through a common.ChainCLI. Each call signs online with the account's own key,
// so within a single account txs run sequentially (one signer, one sequence);
// the worker pool runs different accounts concurrently.
type cliActivityChain struct {
	cli *common.ChainCLI
}

func (a *cliActivityChain) Delegate(fromKey, valoper, amount string) (string, error) {
	return a.cli.SubmitTx("tx", "staking", "delegate", valoper, amount, "--from", fromKey)
}

func (a *cliActivityChain) Unbond(fromKey, valoper, amount string) (string, error) {
	return a.cli.SubmitTx("tx", "staking", "unbond", valoper, amount, "--from", fromKey)
}

func (a *cliActivityChain) Redelegate(fromKey, src, dst, amount string) (string, error) {
	return a.cli.SubmitTx("tx", "staking", "redelegate", src, dst, amount, "--from", fromKey)
}

func (a *cliActivityChain) SetWithdrawAddress(fromKey, withdrawAddr string) (string, error) {
	return a.cli.SubmitTx("tx", "distribution", "set-withdraw-addr", withdrawAddr, "--from", fromKey)
}

func (a *cliActivityChain) GrantAuthzSend(fromKey, grantee string) (string, error) {
	return a.cli.SubmitTx("tx", "authz", "grant", grantee, "generic",
		"--msg-type", common.BankSendMsgType, "--from", fromKey)
}

func (a *cliActivityChain) GrantFeegrant(fromKey, grantee, spendLimit string) (string, error) {
	granterAddr, err := a.cli.ShowAddress(fromKey)
	if err != nil {
		return "", fmt.Errorf("resolve feegrant granter %s: %w", fromKey, err)
	}
	return a.cli.SubmitTx("tx", "feegrant", "grant", granterAddr, grantee,
		"--spend-limit", spendLimit, "--from", fromKey)
}

func (a *cliActivityChain) BankSend(fromKey, to, amount string) (string, error) {
	return a.cli.SubmitTx("tx", "bank", "send", fromKey, to, amount, "--from", fromKey)
}

var _ activityChain = (*cliActivityChain)(nil)
