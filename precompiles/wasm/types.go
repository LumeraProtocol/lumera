package wasm

const (
	// WasmPrecompileAddress is the hex address of the CosmWasm precompile.
	WasmPrecompileAddress = "0x0000000000000000000000000000000000000903"

	// Method names matching the Solidity interface IWasm.sol.
	ExecuteMethod      = "execute"
	QueryMethod        = "query"
	ContractInfoMethod = "contractInfo"
	RawQueryMethod     = "rawQuery"
)
