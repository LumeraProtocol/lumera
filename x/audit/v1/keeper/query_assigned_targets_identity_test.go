package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAssignedTargetIdentityResolutionUnmigratedAndOrdered(t *testing.T) {
	f := initFixture(t)
	a := testAddress(t, f, []byte{1, 2, 3, 4})
	b := testAddress(t, f, []byte{5, 6, 7, 8})

	got, err := f.keeper.ResolveAccountIdentityMappings(f.ctx, []string{b, a})
	require.NoError(t, err)
	require.Equal(t, []types.AccountIdentityMapping{
		{LogicalAccount: b, CurrentAccount: b},
		{LogicalAccount: a, CurrentAccount: a},
	}, got)
}

func TestAssignedTargetIdentityResolutionFollowsTwoHopLineage(t *testing.T) {
	f := initFixture(t)
	old := testAddress(t, f, []byte{11, 12, 13, 14})
	mid := testAddress(t, f, []byte{21, 22, 23, 24})
	current := testAddress(t, f, []byte{31, 32, 33, 34})
	other := testAddress(t, f, []byte{41, 42, 43, 44})

	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{
		SourceAccount: old, DestinationAccount: mid, EffectiveEpoch: 2,
	}))
	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{
		SourceAccount: mid, DestinationAccount: current, EffectiveEpoch: 3,
	}))

	got, err := f.keeper.ResolveAccountIdentityMappings(f.ctx, []string{old, other})
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, []types.AccountIdentityMapping{
		{LogicalAccount: old, CurrentAccount: current},
		{LogicalAccount: other, CurrentAccount: other},
	}, got)
}

func TestAssignedTargetIdentityResolutionFailsClosed(t *testing.T) {
	t.Run("malformed lineage", func(t *testing.T) {
		f := initFixture(t)
		account := testAddress(t, f, []byte{51, 52, 53, 54})
		f.ctx.KVStore(f.storeKey).Set(types.AccountTransitionForwardKey(account), []byte("not protobuf"))

		got, err := f.keeper.ResolveAccountIdentityMappings(f.ctx, []string{account})
		require.ErrorContains(t, err, "malformed account transition")
		require.Nil(t, got)
	})

	t.Run("cyclic lineage", func(t *testing.T) {
		f := initFixture(t)
		a := testAddress(t, f, []byte{61, 62, 63, 64})
		b := testAddress(t, f, []byte{71, 72, 73, 74})
		store := f.ctx.KVStore(f.storeKey)
		ab, err := proto.Marshal(&types.AccountTransition{SourceAccount: a, DestinationAccount: b, EffectiveEpoch: 2})
		require.NoError(t, err)
		ba, err := proto.Marshal(&types.AccountTransition{SourceAccount: b, DestinationAccount: a, EffectiveEpoch: 3})
		require.NoError(t, err)
		store.Set(types.AccountTransitionForwardKey(a), ab)
		store.Set(types.AccountTransitionReverseKey(b), ab)
		store.Set(types.AccountTransitionForwardKey(b), ba)
		store.Set(types.AccountTransitionReverseKey(a), ba)

		got, err := f.keeper.ResolveAccountIdentityMappings(f.ctx, []string{a})
		require.ErrorContains(t, err, "account transition cycle")
		require.Nil(t, got)
	})
}

func TestAssignedTargetsResponseLegacyWireFieldsRemainCompatible(t *testing.T) {
	legacyFieldsOnly := &types.QueryAssignedTargetsResponse{
		EpochId:                 7,
		EpochStartHeight:        123,
		RequiredOpenPorts:       []uint32{4444, 5555},
		TargetSupernodeAccounts: []string{"logical-a", "logical-b"},
	}
	wire, err := proto.Marshal(legacyFieldsOnly)
	require.NoError(t, err)

	var decoded types.QueryAssignedTargetsResponse
	require.NoError(t, proto.Unmarshal(wire, &decoded))
	require.Equal(t, legacyFieldsOnly.EpochId, decoded.EpochId)
	require.Equal(t, legacyFieldsOnly.EpochStartHeight, decoded.EpochStartHeight)
	require.Equal(t, legacyFieldsOnly.RequiredOpenPorts, decoded.RequiredOpenPorts)
	require.Equal(t, legacyFieldsOnly.TargetSupernodeAccounts, decoded.TargetSupernodeAccounts)
	require.Empty(t, decoded.ReporterSupernodeAccount)
	require.Empty(t, decoded.TargetAccountMappings)
}

func TestAssignedTargetsReturnsEpochLogicalReporterAndCurrentTargets(t *testing.T) {
	f := initFixture(t)
	reporterLogical := testAddress(t, f, []byte{81, 82, 83, 84})
	reporterCurrent := testAddress(t, f, []byte{85, 86, 87, 88})
	targetLogical := testAddress(t, f, []byte{91, 92, 93, 94})
	targetCurrent := testAddress(t, f, []byte{95, 96, 97, 98})

	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{
		SourceAccount: reporterLogical, DestinationAccount: reporterCurrent, EffectiveEpoch: 2,
	}))
	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{
		SourceAccount: targetLogical, DestinationAccount: targetCurrent, EffectiveEpoch: 2,
	}))
	require.NoError(t, f.keeper.SetEpochAnchor(f.ctx, types.EpochAnchor{
		EpochId:                 1,
		Seed:                    []byte("01234567890123456789012345678901"),
		ActiveSupernodeAccounts: []string{reporterLogical, targetLogical},
		TargetSupernodeAccounts: []string{reporterLogical, targetLogical},
	}))
	require.NoError(t, f.keeper.SetEpochAnchor(f.ctx, types.EpochAnchor{
		EpochId:                 2,
		Seed:                    []byte("01234567890123456789012345678902"),
		ActiveSupernodeAccounts: []string{reporterCurrent, targetCurrent},
		TargetSupernodeAccounts: []string{reporterCurrent, targetCurrent},
	}))
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporterCurrent).
		Return(sntypes.SuperNode{SupernodeAccount: reporterCurrent}, true, nil).
		Times(2)

	response, err := keeper.NewQueryServerImpl(f.keeper).AssignedTargets(f.ctx, &types.QueryAssignedTargetsRequest{
		SupernodeAccount: reporterCurrent,
		EpochId:          1,
		FilterByEpochId:  true,
	})
	require.NoError(t, err)
	require.Equal(t, reporterLogical, response.ReporterSupernodeAccount)
	require.Equal(t, response.TargetSupernodeAccounts, logicalAccounts(response.TargetAccountMappings))
	require.Len(t, response.TargetAccountMappings, 1)
	require.Equal(t, targetLogical, response.TargetAccountMappings[0].LogicalAccount)
	require.Equal(t, targetCurrent, response.TargetAccountMappings[0].CurrentAccount)

	nextEpochResponse, err := keeper.NewQueryServerImpl(f.keeper).AssignedTargets(f.ctx, &types.QueryAssignedTargetsRequest{
		SupernodeAccount: reporterCurrent,
		EpochId:          2,
		FilterByEpochId:  true,
	})
	require.NoError(t, err)
	require.Equal(t, reporterCurrent, nextEpochResponse.ReporterSupernodeAccount)
	require.Equal(t, nextEpochResponse.TargetSupernodeAccounts, logicalAccounts(nextEpochResponse.TargetAccountMappings))
	require.Len(t, nextEpochResponse.TargetAccountMappings, 1)
	require.Equal(t, targetCurrent, nextEpochResponse.TargetAccountMappings[0].LogicalAccount)
	require.Equal(t, targetCurrent, nextEpochResponse.TargetAccountMappings[0].CurrentAccount)
}

func TestAssignedTargetIdentityResolutionRejectsBrokenMirrorIndexes(t *testing.T) {
	for _, tt := range brokenMirrorIndexCases() {
		t.Run(tt.name, func(t *testing.T) {
			f := initFixture(t)
			account := tt.arrange(t, f)

			got, err := f.keeper.ResolveAccountIdentityMappings(f.ctx, []string{account})
			require.ErrorContains(t, err, "forward and reverse indexes disagree")
			require.Nil(t, got)
		})
	}
}

func TestAssignedTargetsPreservesMixedTargetOrderAndCardinality(t *testing.T) {
	f := initFixture(t)
	reporter := testAddress(t, f, []byte{141, 142, 143, 144})
	migratedLogical := testAddress(t, f, []byte{145, 146, 147, 148})
	migratedCurrent := testAddress(t, f, []byte{151, 152, 153, 154})
	unmigrated := testAddress(t, f, []byte{155, 156, 157, 158})
	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_UNSPECIFIED
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{
		SourceAccount: migratedLogical, DestinationAccount: migratedCurrent, EffectiveEpoch: 2,
	}))
	require.NoError(t, f.keeper.SetEpochAnchor(f.ctx, types.EpochAnchor{
		EpochId:                 1,
		Seed:                    []byte("01234567890123456789012345678903"),
		ActiveSupernodeAccounts: []string{reporter, migratedLogical, unmigrated},
		TargetSupernodeAccounts: []string{reporter, migratedLogical, unmigrated},
	}))
	f.supernodeKeeper.EXPECT().
		GetSuperNodeByAccount(gomock.Any(), reporter).
		Return(sntypes.SuperNode{SupernodeAccount: reporter}, true, nil)

	response, err := keeper.NewQueryServerImpl(f.keeper).AssignedTargets(f.ctx, &types.QueryAssignedTargetsRequest{
		SupernodeAccount: reporter,
		EpochId:          1,
		FilterByEpochId:  true,
	})
	require.NoError(t, err)
	require.Len(t, response.TargetSupernodeAccounts, 2)
	require.Len(t, response.TargetAccountMappings, len(response.TargetSupernodeAccounts))
	require.Equal(t, response.TargetSupernodeAccounts, logicalAccounts(response.TargetAccountMappings))

	currentByLogical := make(map[string]string, len(response.TargetAccountMappings))
	for _, mapping := range response.TargetAccountMappings {
		currentByLogical[mapping.LogicalAccount] = mapping.CurrentAccount
	}
	require.Equal(t, migratedCurrent, currentByLogical[migratedLogical])
	require.Equal(t, unmigrated, currentByLogical[unmigrated])
}

func TestAssignedTargetsRejectsBrokenMirrorIndexesWithoutPartialResponse(t *testing.T) {
	for _, tt := range brokenMirrorIndexCases() {
		t.Run(tt.name, func(t *testing.T) {
			f := initFixture(t)
			reporter := testAddress(t, f, []byte{201, 202, 203, 204})
			target := tt.arrange(t, f)
			require.NoError(t, f.keeper.SetEpochAnchor(f.ctx, types.EpochAnchor{
				EpochId:                 1,
				Seed:                    []byte("01234567890123456789012345678904"),
				ActiveSupernodeAccounts: []string{reporter, target},
				TargetSupernodeAccounts: []string{reporter, target},
			}))
			f.supernodeKeeper.EXPECT().
				GetSuperNodeByAccount(gomock.Any(), reporter).
				Return(sntypes.SuperNode{SupernodeAccount: reporter}, true, nil)

			response, err := keeper.NewQueryServerImpl(f.keeper).AssignedTargets(f.ctx, &types.QueryAssignedTargetsRequest{
				SupernodeAccount: reporter,
				EpochId:          1,
				FilterByEpochId:  true,
			})
			require.Nil(t, response)
			require.Equal(t, codes.Internal, status.Code(err))
			require.ErrorContains(t, err, "forward and reverse indexes disagree")
		})
	}
}

type brokenMirrorIndexCase struct {
	name    string
	arrange func(*testing.T, *fixture) string
}

func brokenMirrorIndexCases() []brokenMirrorIndexCase {
	return []brokenMirrorIndexCase{
		{
			name: "orphan forward",
			arrange: func(t *testing.T, f *fixture) string {
				source := testAddress(t, f, []byte{101, 102, 103, 104})
				destination := testAddress(t, f, []byte{105, 106, 107, 108})
				setTransitionIndex(t, f, types.AccountTransitionForwardKey(source), types.AccountTransition{
					SourceAccount: source, DestinationAccount: destination, EffectiveEpoch: 2,
				})
				return source
			},
		},
		{
			name: "orphan reverse",
			arrange: func(t *testing.T, f *fixture) string {
				source := testAddress(t, f, []byte{111, 112, 113, 114})
				destination := testAddress(t, f, []byte{115, 116, 117, 118})
				setTransitionIndex(t, f, types.AccountTransitionReverseKey(destination), types.AccountTransition{
					SourceAccount: source, DestinationAccount: destination, EffectiveEpoch: 2,
				})
				return destination
			},
		},
		{
			name: "mismatched mirror",
			arrange: func(t *testing.T, f *fixture) string {
				source := testAddress(t, f, []byte{121, 122, 123, 124})
				other := testAddress(t, f, []byte{125, 126, 127, 128})
				destination := testAddress(t, f, []byte{131, 132, 133, 134})
				setTransitionIndex(t, f, types.AccountTransitionForwardKey(source), types.AccountTransition{
					SourceAccount: source, DestinationAccount: destination, EffectiveEpoch: 2,
				})
				setTransitionIndex(t, f, types.AccountTransitionReverseKey(destination), types.AccountTransition{
					SourceAccount: other, DestinationAccount: destination, EffectiveEpoch: 2,
				})
				return source
			},
		},
	}
}

func setTransitionIndex(t *testing.T, f *fixture, key []byte, transition types.AccountTransition) {
	t.Helper()
	bz, err := proto.Marshal(&transition)
	require.NoError(t, err)
	f.ctx.KVStore(f.storeKey).Set(key, bz)
}

func logicalAccounts(mappings []types.AccountIdentityMapping) []string {
	accounts := make([]string, len(mappings))
	for i := range mappings {
		accounts[i] = mappings[i].LogicalAccount
	}
	return accounts
}
