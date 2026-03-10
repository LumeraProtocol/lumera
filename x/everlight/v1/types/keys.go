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
)
