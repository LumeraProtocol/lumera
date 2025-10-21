package app

import (
	ibcporttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
)

const (
	IBCModuleRegisterFnOption = "ibc_module_register_fn"

	FlagWasmHomeDir           = "wasm-homedir"
)

type IBCModuleRegisterFn func(router *ibcporttypes.Router)
