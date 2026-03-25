package audit_test

import (
	"bytes"
	"fmt"
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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	auditkeeper "github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	auditmodule "github.com/LumeraProtocol/lumera/x/audit/v1/module"
	audittypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
	supernodemocks "github.com/LumeraProtocol/lumera/x/supernode/v1/mocks"
)

type integrationFixture struct {
	ctx          sdk.Context
	keeper       auditkeeper.Keeper
	addressCodec address.Codec
}

func initIntegrationFixture(t *testing.T) *integrationFixture {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	encCfg := moduletestutil.MakeTestEncodingConfig(auditmodule.AppModuleBasic{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(audittypes.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)
	snKeeper := supernodemocks.NewMockSupernodeKeeper(ctrl)

	k := auditkeeper.NewKeeper(
		encCfg.Codec,
		addressCodec,
		storeService,
		log.NewNopLogger(),
		authority,
		snKeeper,
	)
	require.NoError(t, k.SetParams(ctx, audittypes.DefaultParams()))

	return &integrationFixture{ctx: ctx, keeper: k, addressCodec: addressCodec}
}

func TestSubmitEvidence_CascadeClientFailure_DeterministicMetadataBytes(t *testing.T) {
	f := initIntegrationFixture(t)

	reporter, err := f.addressCodec.BytesToString(bytes.Repeat([]byte{0x11}, 20))
	require.NoError(t, err)
	subject, err := f.addressCodec.BytesToString(bytes.Repeat([]byte{0x22}, 20))
	require.NoError(t, err)

	jsonVariants := []string{
		`{"reporter_component":2,"target_supernode_accounts":["lumera1mfldjaqc7ec5rlh4k58yttv3cd978gzl070zk6"],"details":{"action_id":"123637","error":"download failed: insufficient symbols","iteration":"1","operation":"download","supernode_account":"lumera1mfldjaqc7ec5rlh4k58yttv3cd978gzl070zk6","supernode_endpoint":"18.190.53.108:4444","task_id":"9700ec8a"}}`,
		`{"reporter_component":2,"target_supernode_accounts":["lumera1mfldjaqc7ec5rlh4k58yttv3cd978gzl070zk6"],"details":{"task_id":"9700ec8a","supernode_endpoint":"18.190.53.108:4444","supernode_account":"lumera1mfldjaqc7ec5rlh4k58yttv3cd978gzl070zk6","operation":"download","iteration":"1","error":"download failed: insufficient symbols","action_id":"123637"}}`,
	}

	var first []byte
	for i, metaJSON := range jsonVariants {
		evidenceID, err := f.keeper.CreateEvidence(
			f.ctx,
			reporter,
			subject,
			fmt.Sprintf("action-%d", i),
			audittypes.EvidenceType_EVIDENCE_TYPE_CASCADE_CLIENT_FAILURE,
			metaJSON,
		)
		require.NoError(t, err)

		ev, found := f.keeper.GetEvidence(f.ctx, evidenceID)
		require.True(t, found)
		if i == 0 {
			first = append([]byte(nil), ev.Metadata...)
			continue
		}
		require.Equal(t, first, ev.Metadata)
	}
}
