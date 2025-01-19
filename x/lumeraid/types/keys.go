package types

const (
	// ModuleName defines the module name
	ModuleName = "lumeraid"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_lumeraid"
)

var (
	ParamsKey = []byte("p_lumeraid")
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}
