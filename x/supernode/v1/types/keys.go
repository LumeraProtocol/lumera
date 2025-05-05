package types

import sdk "github.com/cosmos/cosmos-sdk/types"

const (
	// ModuleName defines the module name
	ModuleName = "supernode"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_supernode"
)

var (
	ParamsKey = []byte("p_supernode")

	// SuperNodeKey prefix for storing SuperNode entities
	SuperNodeKey = []byte("sn_") // prefix 'sn_' for supernode storage
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}

// GetSupernodeKey returns the store key to retrieve a SuperNode by validator address
func GetSupernodeKey(valAddr sdk.ValAddress) []byte {
	return append(SuperNodeKey, valAddr.Bytes()...)
}
