//go:build test
// +build test

package evm

import (
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// ResetGlobalState resets the EVM global configuration (coin info, chain config,
// EIP activators) so that a new app instance can be initialized in the same test
// process without "already set" panics from cosmos/evm's package-level singletons.
func ResetGlobalState() {
	evmtypes.NewEVMConfigurator().ResetTestConfig()
}
