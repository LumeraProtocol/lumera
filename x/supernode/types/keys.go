package types

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
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}
