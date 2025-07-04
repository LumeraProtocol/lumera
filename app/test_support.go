package app

import (
	"github.com/cosmos/cosmos-sdk/baseapp"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	ibcporttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
)

// GetBaseApp returns the base application.
func (app *App) GetBaseApp() *baseapp.BaseApp {
	return app.BaseApp
}

// GetBankKeeper returns the bank keeper.
func (app *App) GetBankKeeper() bankkeeper.Keeper {
	return app.BankKeeper
}

// GetStakingKeeper returns the staking keeper.
func (app *App) GetStakingKeeper() *stakingkeeper.Keeper {
	return app.StakingKeeper
}

// GetAuthKeeper returns the auth keeper.
func (app *App) GetAuthKeeper() authkeeper.AccountKeeper {
	return app.AuthKeeper
}

// GetWasmKeeper returns the Wasm keeper.
func (app *App) GetWasmKeeper() *wasmkeeper.Keeper {
	return app.WasmKeeper
}

// GetIBCRouter returns the IBC router.
func (app *App) GetIBCRouter() *ibcporttypes.Router {
	return app.ibcRouter
}