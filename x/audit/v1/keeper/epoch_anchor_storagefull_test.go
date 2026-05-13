package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCreateEpochAnchor_IncludesStorageFullInActiveAndTargets(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(100)

	active := sntypes.SuperNode{SupernodeAccount: sdk.AccAddress([]byte("active_account________")).String(), ValidatorAddress: sdk.ValAddress([]byte("active_val_addr______")).String()}
	storageFull := sntypes.SuperNode{SupernodeAccount: sdk.AccAddress([]byte("storage_full_account_")).String(), ValidatorAddress: sdk.ValAddress([]byte("storage_full_val____")).String()}
	postponed := sntypes.SuperNode{SupernodeAccount: sdk.AccAddress([]byte("postponed_account____")).String(), ValidatorAddress: sdk.ValAddress([]byte("postponed_val_addr___")).String()}

	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive, sntypes.SuperNodeStateStorageFull).
		Return([]sntypes.SuperNode{active, storageFull}, nil).
		Times(1)
	f.supernodeKeeper.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(f.ctx), sntypes.SuperNodeStateActive, sntypes.SuperNodeStateStorageFull, sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{active, storageFull, postponed}, nil).
		Times(1)

	err := f.keeper.CreateEpochAnchorIfNeeded(f.ctx, 3, 100, 199, types.DefaultParams())
	require.NoError(t, err)

	anchor, found := f.keeper.GetEpochAnchor(f.ctx, 3)
	require.True(t, found)
	require.Contains(t, anchor.ActiveSupernodeAccounts, active.SupernodeAccount)
	require.Contains(t, anchor.ActiveSupernodeAccounts, storageFull.SupernodeAccount)
	require.NotContains(t, anchor.ActiveSupernodeAccounts, postponed.SupernodeAccount)

	require.Contains(t, anchor.TargetSupernodeAccounts, active.SupernodeAccount)
	require.Contains(t, anchor.TargetSupernodeAccounts, storageFull.SupernodeAccount)
	require.Contains(t, anchor.TargetSupernodeAccounts, postponed.SupernodeAccount)
}
