//go:build test
// +build test

package evm

import (
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
)

// SetKeeperDefaults is a no-op in test builds. In test mode, cosmos/evm's
// SetDefaultEvmCoinInfo and setTestingEVMCoinInfo share the same global variable,
// so calling WithDefaultEvmCoinInfo would conflict with Configure() in InitGenesis.
// The genesis ordering (evm before precisebank) ensures EVM coin info is available
// when needed.
func SetKeeperDefaults(_ *evmkeeper.Keeper) {}
