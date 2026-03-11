package types

const (
	// ModuleName defines the module name
	ModuleName = "everlight"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName
)

var (
	// ParamsKey is the key for storing module params
	ParamsKey = []byte("p_everlight")

	// LastDistributionHeightKey is the key for storing the last distribution height
	LastDistributionHeightKey = []byte("ldh")

	// TotalDistributedKey is the key for storing total distributed amounts
	TotalDistributedKey = []byte("td")

	// SNDistStatePrefix is the key prefix for per-SN distribution state
	SNDistStatePrefix = []byte("sn_dist/")
)

// SNDistStateKey returns the store key for a specific supernode's distribution state.
func SNDistStateKey(valAddr string) []byte {
	return append(SNDistStatePrefix, []byte(valAddr)...)
}
