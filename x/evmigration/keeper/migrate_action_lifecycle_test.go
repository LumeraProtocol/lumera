package keeper_test

import (
	"errors"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	evmigrationtypes "github.com/LumeraProtocol/lumera/x/evmigration/types"
)

func TestMigrateActions_LifecycleMatrix(t *testing.T) {
	tests := []struct {
		name             string
		state            actiontypes.ActionState
		wantWrite        bool
		wantCreatorMoved bool
		wantSNMoved      bool
	}{
		{"pending", actiontypes.ActionStatePending, true, true, true},
		{"processing", actiontypes.ActionStateProcessing, true, true, true},
		{"done", actiontypes.ActionStateDone, true, true, false},
		{"approved", actiontypes.ActionStateApproved, false, false, false},
		{"rejected", actiontypes.ActionStateRejected, false, false, false},
		{"failed", actiontypes.ActionStateFailed, false, false, false},
		{"expired", actiontypes.ActionStateExpired, false, false, false},
		{"unspecified", actiontypes.ActionStateUnspecified, false, false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := initMockFixture(t)
			legacy, destination, other := testAccAddr(), testAccAddr(), testAccAddr()
			indexedCreator := &actiontypes.Action{ActionID: "matrix", Creator: legacy.String()}
			indexedSN := &actiontypes.Action{ActionID: "matrix", SuperNodes: []string{legacy.String()}}
			canonical := &actiontypes.Action{
				ActionID: "matrix", Creator: legacy.String(), State: tc.state,
				ActionType: actiontypes.ActionTypeCascade, Metadata: []byte{1, 2, 3},
				Price: "7ulume", ExpirationTime: 99, BlockHeight: 42,
				SuperNodes: []string{legacy.String(), other.String()}, FileSizeKbs: 123,
				AppPubkey: []byte{4, 5, 6},
			}
			before := *canonical
			before.Metadata = append([]byte(nil), canonical.Metadata...)
			before.SuperNodes = append([]string(nil), canonical.SuperNodes...)
			before.AppPubkey = append([]byte(nil), canonical.AppPubkey...)

			f.actionKeeper.EXPECT().GetActionsByCreator(gomock.Any(), legacy.String()).Return([]*actiontypes.Action{indexedCreator}, nil)
			f.actionKeeper.EXPECT().GetActionsBySuperNode(gomock.Any(), legacy.String()).Return([]*actiontypes.Action{indexedSN}, nil)
			f.actionKeeper.EXPECT().GetActionByID(gomock.Any(), "matrix").Return(canonical, true).Times(1)
			if tc.wantWrite {
				f.actionKeeper.EXPECT().SetAction(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, got *actiontypes.Action) error {
					require.Equal(t, before.ActionID, got.ActionID)
					require.Equal(t, before.ActionType, got.ActionType)
					require.Equal(t, before.Metadata, got.Metadata)
					require.Equal(t, before.Price, got.Price)
					require.Equal(t, before.ExpirationTime, got.ExpirationTime)
					require.Equal(t, before.State, got.State)
					require.Equal(t, before.BlockHeight, got.BlockHeight)
					require.Equal(t, before.FileSizeKbs, got.FileSizeKbs)
					require.Equal(t, before.AppPubkey, got.AppPubkey)
					if tc.wantCreatorMoved {
						require.Equal(t, destination.String(), got.Creator)
					} else {
						require.Equal(t, legacy.String(), got.Creator)
					}
					if tc.wantSNMoved {
						require.Equal(t, destination.String(), got.SuperNodes[0])
					} else {
						require.Equal(t, legacy.String(), got.SuperNodes[0])
					}
					// Prove the writable value does not alias canonical store/index data.
					got.Metadata[0], got.AppPubkey[0], got.SuperNodes[1] = 9, 9, "mutated"
					return nil
				})
			}

			require.NoError(t, f.keeper.MigrateActions(f.ctx, legacy, destination))
			require.Equal(t, before, *canonical, "canonical value must remain immutable")
		})
	}
}

func TestMigrateActions_CreatorOnlyUsesCanonicalValue(t *testing.T) {
	f := initMockFixture(t)
	legacy, destination := testAccAddr(), testAccAddr()
	indexCopy := &actiontypes.Action{ActionID: "creator-only", Creator: legacy.String(), Metadata: []byte("stale-copy")}
	canonical := &actiontypes.Action{ActionID: "creator-only", Creator: legacy.String(), State: actiontypes.ActionStatePending, Metadata: []byte("canonical")}

	f.actionKeeper.EXPECT().GetActionsByCreator(gomock.Any(), legacy.String()).Return([]*actiontypes.Action{indexCopy}, nil)
	f.actionKeeper.EXPECT().GetActionsBySuperNode(gomock.Any(), legacy.String()).Return(nil, nil)
	f.actionKeeper.EXPECT().GetActionByID(gomock.Any(), "creator-only").Return(canonical, true).Times(1)
	f.actionKeeper.EXPECT().SetAction(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, got *actiontypes.Action) error {
		require.Equal(t, []byte("canonical"), got.Metadata)
		require.Equal(t, destination.String(), got.Creator)
		return nil
	})

	require.NoError(t, f.keeper.MigrateActions(f.ctx, legacy, destination))
}

func TestMigrateActions_FailsClosedOnBadIndexOrCanonicalData(t *testing.T) {
	t.Run("nil index row", func(t *testing.T) {
		f := initMockFixture(t)
		legacy, destination := testAccAddr(), testAccAddr()
		f.actionKeeper.EXPECT().GetActionsByCreator(gomock.Any(), legacy.String()).Return([]*actiontypes.Action{nil}, nil)
		f.actionKeeper.EXPECT().GetActionsBySuperNode(gomock.Any(), legacy.String()).Return(nil, nil)
		require.ErrorContains(t, f.keeper.MigrateActions(f.ctx, legacy, destination), "nil action")
	})

	t.Run("missing canonical row", func(t *testing.T) {
		f := initMockFixture(t)
		legacy, destination := testAccAddr(), testAccAddr()
		indexed := &actiontypes.Action{ActionID: "missing", Creator: legacy.String()}
		f.actionKeeper.EXPECT().GetActionsByCreator(gomock.Any(), legacy.String()).Return([]*actiontypes.Action{indexed}, nil)
		f.actionKeeper.EXPECT().GetActionsBySuperNode(gomock.Any(), legacy.String()).Return(nil, nil)
		f.actionKeeper.EXPECT().GetActionByID(gomock.Any(), "missing").Return(nil, false)
		require.ErrorContains(t, f.keeper.MigrateActions(f.ctx, legacy, destination), "not found")
	})

	t.Run("stale creator index conflicts with canonical", func(t *testing.T) {
		f := initMockFixture(t)
		legacy, destination, other := testAccAddr(), testAccAddr(), testAccAddr()
		indexed := &actiontypes.Action{ActionID: "stale", Creator: legacy.String()}
		canonical := &actiontypes.Action{ActionID: "stale", Creator: other.String(), State: actiontypes.ActionStatePending}
		f.actionKeeper.EXPECT().GetActionsByCreator(gomock.Any(), legacy.String()).Return([]*actiontypes.Action{indexed}, nil)
		f.actionKeeper.EXPECT().GetActionsBySuperNode(gomock.Any(), legacy.String()).Return(nil, nil)
		f.actionKeeper.EXPECT().GetActionByID(gomock.Any(), "stale").Return(canonical, true)
		require.ErrorContains(t, f.keeper.MigrateActions(f.ctx, legacy, destination), "conflicts")
	})

	t.Run("duplicate legacy supernodes", func(t *testing.T) {
		f := initMockFixture(t)
		legacy, destination := testAccAddr(), testAccAddr()
		indexed := &actiontypes.Action{ActionID: "duplicate", SuperNodes: []string{legacy.String()}}
		canonical := &actiontypes.Action{ActionID: "duplicate", State: actiontypes.ActionStatePending, SuperNodes: []string{legacy.String(), legacy.String()}}
		f.actionKeeper.EXPECT().GetActionsByCreator(gomock.Any(), legacy.String()).Return(nil, nil)
		f.actionKeeper.EXPECT().GetActionsBySuperNode(gomock.Any(), legacy.String()).Return([]*actiontypes.Action{indexed}, nil)
		f.actionKeeper.EXPECT().GetActionByID(gomock.Any(), "duplicate").Return(canonical, true)
		require.ErrorContains(t, f.keeper.MigrateActions(f.ctx, legacy, destination), "duplicate legacy")
	})
}

func TestMigrateActions_ValidatesDestinationCollisionBeforeAnyWrite(t *testing.T) {
	f := initMockFixture(t)
	legacy, destination := testAccAddr(), testAccAddr()
	firstIndex := &actiontypes.Action{ActionID: "first", Creator: legacy.String()}
	secondIndex := &actiontypes.Action{ActionID: "second", SuperNodes: []string{legacy.String()}}
	first := &actiontypes.Action{ActionID: "first", Creator: legacy.String(), State: actiontypes.ActionStatePending}
	second := &actiontypes.Action{ActionID: "second", State: actiontypes.ActionStateProcessing, SuperNodes: []string{legacy.String(), destination.String()}}

	f.actionKeeper.EXPECT().GetActionsByCreator(gomock.Any(), legacy.String()).Return([]*actiontypes.Action{firstIndex}, nil)
	f.actionKeeper.EXPECT().GetActionsBySuperNode(gomock.Any(), legacy.String()).Return([]*actiontypes.Action{secondIndex}, nil)
	f.actionKeeper.EXPECT().GetActionByID(gomock.Any(), "first").Return(first, true)
	f.actionKeeper.EXPECT().GetActionByID(gomock.Any(), "second").Return(second, true)
	// No SetAction expectation: the first prepared update must not be written.
	require.ErrorContains(t, f.keeper.MigrateActions(f.ctx, legacy, destination), "destination supernode")
}

func TestMigrateActions_LateWriteFailureRollsBackCache(t *testing.T) {
	f := initMockFixture(t)
	legacy, destination := testAccAddr(), testAccAddr()
	firstIndex := &actiontypes.Action{ActionID: "first", Creator: legacy.String()}
	secondIndex := &actiontypes.Action{ActionID: "second", Creator: legacy.String()}
	first := &actiontypes.Action{ActionID: "first", Creator: legacy.String(), State: actiontypes.ActionStatePending}
	second := &actiontypes.Action{ActionID: "second", Creator: legacy.String(), State: actiontypes.ActionStatePending}

	f.actionKeeper.EXPECT().GetActionsByCreator(gomock.Any(), legacy.String()).Return([]*actiontypes.Action{firstIndex, secondIndex}, nil)
	f.actionKeeper.EXPECT().GetActionsBySuperNode(gomock.Any(), legacy.String()).Return(nil, nil)
	f.actionKeeper.EXPECT().GetActionByID(gomock.Any(), "first").Return(first, true)
	f.actionKeeper.EXPECT().GetActionByID(gomock.Any(), "second").Return(second, true)
	gomock.InOrder(
		f.actionKeeper.EXPECT().SetAction(gomock.Any(), gomock.Any()).DoAndReturn(func(cacheCtx sdk.Context, _ *actiontypes.Action) error {
			return f.keeper.MigrationRecords.Set(cacheCtx, "rollback-probe", evmigrationtypes.MigrationRecord{
				LegacyAddress: "legacy", NewAddress: "destination",
			})
		}),
		f.actionKeeper.EXPECT().SetAction(gomock.Any(), gomock.Any()).Return(errors.New("late write failure")),
	)

	require.ErrorContains(t, f.keeper.MigrateActions(f.ctx, legacy, destination), "late write failure")
	_, err := f.keeper.MigrationRecords.Get(f.ctx, "rollback-probe")
	require.Error(t, err, "write made through the first SetAction cache context must be discarded")
}
