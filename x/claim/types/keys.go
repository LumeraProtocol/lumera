package types

const (
	// ModuleName defines the module name
	ModuleName = "claim"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_claim"

	ClaimRecordKey = "ClaimRecord/value/"
	// This is a constant used to store the count of claim records loaded at genesis

	ClaimRecordCountKey = "ClaimRecord/count/"
	// BlockClaimsKey counts the claims on current block

	BlockClaimsKey = "claims_per_block"
)

var (
	ParamsKey = []byte("p_claim")
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}
