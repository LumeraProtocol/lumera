package app

import (
	ibcporttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	ibcapi "github.com/cosmos/ibc-go/v10/modules/core/api"
)

const (
	IBCModuleRegisterFnOption   = "ibc_module_register_fn"
	IBCModuleRegisterFnOptionV2 = "ibc_module_register_fn_v2"

	FlagWasmHomeDir = "wasm-homedir"
)

type IBCModuleRegisterFn func(router *ibcporttypes.Router)
type IBCModuleRegisterFnV2 func(router *ibcapi.Router)
