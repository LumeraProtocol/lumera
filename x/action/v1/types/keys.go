package types

import (
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"cosmossdk.io/collections"
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

	// Version defines the current version the IBC module supports
	Version = ModuleName + "-1"

	// PortID is the default port id that module binds to
	PortID = ModuleName
)

var (
	ParamsKey = []byte("p_action")

	// PortKey defines the key to store the port ID in store
	PortKey = collections.NewPrefix("action-port-")

	ModuleAccountAddress = authtypes.NewModuleAddress(ModuleAccountName)
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}
