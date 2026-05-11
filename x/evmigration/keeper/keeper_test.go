package keeper_test

import (
	"context"
	"testing"

	"cosmossdk.io/core/address"
	storetypes "cosmossdk.io/store/types"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/LumeraProtocol/lumera/x/evmigration/keeper"
	module "github.com/LumeraProtocol/lumera/x/evmigration/module"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

type fixture struct {
	ctx          context.Context
	keeper       keeper.Keeper
	addressCodec address.Codec
}

func initFixture(t *testing.T) *fixture {
	t.Helper()

	encCfg := moduletestutil.MakeTestEncodingConfig(module.AppModule{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx

	authority := authtypes.NewModuleAddress(types.GovModuleName)

	k := keeper.NewKeeper(
		storeService,
		encCfg.Codec,
		addressCodec,
		authority,
		nil, // accountKeeper
		nil, // bankKeeper
		nil, // stakingKeeper
		nil, // distributionKeeper
		nil, // authzKeeper
		nil, // feegrantKeeper
		nil, // supernodeKeeper
		nil, // actionKeeper
		nil, // claimKeeper
	)

	// Initialize params and counters
	if err := k.Params.Set(ctx, types.DefaultParams()); err != nil {
		t.Fatalf("failed to set params: %v", err)
	}
	if err := k.MigrationCounter.Set(ctx, 0); err != nil {
		t.Fatalf("failed to set migration counter: %v", err)
	}
	if err := k.ValidatorMigrationCounter.Set(ctx, 0); err != nil {
		t.Fatalf("failed to set validator migration counter: %v", err)
	}

	return &fixture{
		ctx:          ctx,
		keeper:       k,
		addressCodec: addressCodec,
	}
}
