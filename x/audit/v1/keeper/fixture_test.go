package keeper_test

import (
	"testing"

	"cosmossdk.io/core/address"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"go.uber.org/mock/gomock"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	auditmodule "github.com/LumeraProtocol/lumera/x/audit/v1/module"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	supernodemocks "github.com/LumeraProtocol/lumera/x/supernode/v1/mocks"
)

type fixture struct {
	ctx          sdk.Context
	keeper       keeper.Keeper
	addressCodec address.Codec

	supernodeKeeper *supernodemocks.MockSupernodeKeeper
}

func initFixture(t *testing.T) *fixture {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	encCfg := moduletestutil.MakeTestEncodingConfig(auditmodule.AppModuleBasic{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx

	authority := authtypes.NewModuleAddress(govtypes.ModuleName)

	snKeeper := supernodemocks.NewMockSupernodeKeeper(ctrl)

	k := keeper.NewKeeper(
		encCfg.Codec,
		addressCodec,
		storeService,
		log.NewNopLogger(),
		authority,
		snKeeper,
	)

	if err := k.SetParams(ctx, types.DefaultParams()); err != nil {
		t.Fatalf("failed to set params: %v", err)
	}

	return &fixture{
		ctx:             ctx,
		keeper:          k,
		addressCodec:    addressCodec,
		supernodeKeeper: snKeeper,
	}
}
