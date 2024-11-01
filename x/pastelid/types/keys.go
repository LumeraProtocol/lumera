package types

const (
	// ModuleName defines the module name
	ModuleName = "pastelid"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_pastelid"
)

var (
	ParamsKey = []byte("p_pastelid")
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}
