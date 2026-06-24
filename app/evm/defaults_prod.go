//go:build !test
// +build !test

package evm

import (
	"testing"

	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	lcfg "github.com/LumeraProtocol/lumera/config"
)

// SetKeeperDefaults configures the EVM keeper's default coin info for production.
// This ensures RPC queries that arrive before the first PreBlock/InitGenesis don't
// cause nil pointer dereferences when accessing EVM coin info.
//
// In test binaries compiled without -tags=test, cosmos/evm's SetDefaultEvmCoinInfo
// and setTestingEVMCoinInfo share the same global variable, so this would conflict
// with Configure() in InitGenesis.
func SetKeeperDefaults(k *evmkeeper.Keeper) {
	if testing.Testing() {
		panicTestTagRequired()
	}
	k.WithDefaultEvmCoinInfo(evmtypes.EvmCoinInfo{
		Denom:         lcfg.ChainDenom,
		ExtendedDenom: lcfg.ChainEVMExtendedDenom,
		DisplayDenom:  lcfg.ChainDisplayDenom,
		Decimals:      evmtypes.SixDecimals.Uint32(),
	})
}
