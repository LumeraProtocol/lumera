package types

const (
	// ModuleName defines the module name
	ModuleName = "action"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_action"
)

var (
	ParamsKey = []byte("p_action")
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}
