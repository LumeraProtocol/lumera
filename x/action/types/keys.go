package types

import (
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

const (
	// ModuleName defines the module name
	ModuleName = "action"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_action"

	// ModuleAccountName defines the module account name for fee distribution
	ModuleAccountName = ModuleName
)

var (
	ParamsKey = []byte("p_action")

	ModuleAccountAddress = authtypes.NewModuleAddress(ModuleAccountName)
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}
