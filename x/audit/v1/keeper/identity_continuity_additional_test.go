package keeper_test

import (
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func addContinuityResultFacts(t *testing.T, f *fixture, epoch uint64, reporter string, failures, passes int) {
	t.Helper()
	for i := 0; i < failures+passes; i++ {
		class := types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_HASH_MISMATCH
		if i >= failures {
			class = types.StorageProofResultClass_STORAGE_PROOF_RESULT_CLASS_PASS
		}
		require.NoError(t, keeper.SetStorageTruthReporterResultForTest(f.keeper, f.ctx, epoch, reporter, &types.StorageProofResult{
			TicketId: fmt.Sprintf("continuity-%d-%s-%d", epoch, reporter, i), TargetSupernodeAccount: fmt.Sprintf("target-%d", i), ResultClass: class,
		}))
	}
}

func TestAccountTransitionHealOpBlockersAndBound(t *testing.T) {
	for _, tc := range []struct {
		name string
		op   func(source, other string) types.HealOp
		want string
	}{
		{"source healer", func(source, other string) types.HealOp {
			return types.HealOp{HealOpId: 1, HealerSupernodeAccount: source, VerifierSupernodeAccounts: []string{other}, Status: types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED}
		}, "source is healer"},
		{"source verifier", func(source, other string) types.HealOp {
			return types.HealOp{HealOpId: 1, HealerSupernodeAccount: other, VerifierSupernodeAccounts: []string{source}, Status: types.HealOpStatus_HEAL_OP_STATUS_IN_PROGRESS}
		}, "source is verifier"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := initFixture(t)
			source := testAddress(t, f, []byte{1, 2, 3, 40})
			destination := testAddress(t, f, []byte{5, 6, 7, 80})
			other := testAddress(t, f, []byte{9, 10, 11, 120})
			require.NoError(t, f.keeper.SetHealOp(f.ctx, tc.op(source, other)))
			_, err := buildAccountTransitionPlan(t, f, types.AccountTransition{SourceAccount: source, DestinationAccount: destination, EffectiveEpoch: 2})
			require.ErrorContains(t, err, tc.want)
		})
	}

	t.Run("final operation allowed", func(t *testing.T) {
		f := initFixture(t)
		source := testAddress(t, f, []byte{1, 3, 5, 7})
		destination := testAddress(t, f, []byte{2, 4, 6, 8})
		require.NoError(t, f.keeper.SetHealOp(f.ctx, types.HealOp{HealOpId: 1, HealerSupernodeAccount: source, Status: types.HealOpStatus_HEAL_OP_STATUS_VERIFIED}))
		_, err := buildAccountTransitionPlan(t, f, types.AccountTransition{SourceAccount: source, DestinationAccount: destination, EffectiveEpoch: 2})
		require.NoError(t, err)
	})

	for _, extra := range []int{0, 1} {
		name := "exact cap"
		if extra == 1 {
			name = "cap plus one"
		}
		t.Run(name, func(t *testing.T) {
			f := initFixture(t)
			for i := 0; i < types.MaxIdentityTransitionHealOps+extra; i++ {
				require.NoError(t, f.keeper.SetHealOp(f.ctx, types.HealOp{HealOpId: uint64(i + 1), Status: types.HealOpStatus_HEAL_OP_STATUS_VERIFIED}))
			}
			source := testAddress(t, f, []byte{13, 14, 15, 16})
			destination := testAddress(t, f, []byte{17, 18, 19, 20})
			_, err := buildAccountTransitionPlan(t, f, types.AccountTransition{SourceAccount: source, DestinationAccount: destination, EffectiveEpoch: 2})
			if extra == 0 {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "heal operations exceed")
			}
		})
	}
}

func TestAccountTransitionMovesEverySingletonAndRewritesEmbeddedAccounts(t *testing.T) {
	f := initFixture(t)
	source := testAddress(t, f, []byte{21, 22, 23, 24})
	destination := testAddress(t, f, []byte{25, 26, 27, 28})
	store := f.ctx.KVStore(f.storeKey)
	store.Set(types.ActionFinalizationPostponementKey(source), binary.BigEndian.AppendUint64(nil, 11))
	store.Set(types.StorageTruthPostponementKey(source), binary.BigEndian.AppendUint64(nil, 12))
	store.Set(types.StorageTruthPostponementStrongKey(source), []byte{1})
	f.keeper.SetNodeSuspicionState(f.ctx, types.NodeSuspicionState{SupernodeAccount: source, SuspicionScore: 13})
	f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{ReporterSupernodeAccount: source, ReliabilityScore: 14})

	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: source, DestinationAccount: destination, EffectiveEpoch: 2}))
	for _, pair := range [][2][]byte{
		{types.ActionFinalizationPostponementKey(source), types.ActionFinalizationPostponementKey(destination)},
		{types.StorageTruthPostponementKey(source), types.StorageTruthPostponementKey(destination)},
		{types.StorageTruthPostponementStrongKey(source), types.StorageTruthPostponementStrongKey(destination)},
		{types.NodeSuspicionStateKey(source), types.NodeSuspicionStateKey(destination)},
		{types.ReporterReliabilityStateKey(source), types.ReporterReliabilityStateKey(destination)},
	} {
		require.False(t, store.Has(pair[0]))
		require.True(t, store.Has(pair[1]))
	}
	node, found := f.keeper.GetNodeSuspicionState(f.ctx, destination)
	require.True(t, found)
	require.Equal(t, destination, node.SupernodeAccount)
	reporter, found := f.keeper.GetReporterReliabilityState(f.ctx, destination)
	require.True(t, found)
	require.Equal(t, destination, reporter.ReporterSupernodeAccount)
}

func TestAccountTransitionCanonicalEndpointsAndGenesisValidation(t *testing.T) {
	f := initFixture(t)
	source := testAddress(t, f, []byte{31, 32, 33, 34})
	destination := testAddress(t, f, []byte{35, 36, 37, 38})
	_, err := buildAccountTransitionPlan(t, f, types.AccountTransition{SourceAccount: strings.ToUpper(source), DestinationAccount: destination, EffectiveEpoch: 2})
	require.ErrorContains(t, err, "noncanonical")

	genesis := types.DefaultGenesis()
	genesis.AccountTransitions = []types.AccountTransition{{SourceAccount: source, DestinationAccount: strings.ToUpper(destination), EffectiveEpoch: 2}}
	require.NoError(t, genesis.Validate(), "types validation is structural")
	require.ErrorContains(t, f.keeper.InitGenesis(f.ctx, *genesis), "noncanonical")
}

func TestAccountTransitionExactMaxAndGetAllNoOffByOne(t *testing.T) {
	f := initFixture(t)
	accounts := make([]string, types.MaxAccountTransitions+2)
	for i := range accounts {
		bz := make([]byte, 4)
		binary.BigEndian.PutUint32(bz, uint32(i+1))
		accounts[i] = testAddress(t, f, bz)
	}
	for i := 0; i < types.MaxAccountTransitions; i++ {
		require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: accounts[i], DestinationAccount: accounts[i+1], EffectiveEpoch: uint64(i + 1)}))
	}
	got, err := f.keeper.CurrentAccount(f.ctx, accounts[0])
	require.NoError(t, err)
	require.Equal(t, accounts[types.MaxAccountTransitions], got)
	all, err := f.keeper.GetAllAccountTransitions(f.ctx)
	require.NoError(t, err)
	require.Len(t, all, types.MaxAccountTransitions)
	err = recordAccountTransition(t, f, types.AccountTransition{SourceAccount: accounts[types.MaxAccountTransitions], DestinationAccount: accounts[types.MaxAccountTransitions+1], EffectiveEpoch: types.MaxAccountTransitions + 1})
	require.ErrorContains(t, err, "exceed limit")
}

func TestSetReportLiveSideEffectUsesCurrentAccount(t *testing.T) {
	f := initFixture(t)
	old := testAddress(t, f, []byte{41, 42, 43, 44})
	current := testAddress(t, f, []byte{45, 46, 47, 48})
	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: old, DestinationAccount: current, EffectiveEpoch: 2}))
	sn := sntypes.SuperNode{SupernodeAccount: current, States: []*sntypes.SuperNodeStateRecord{{State: sntypes.SuperNodeStateActive}}}
	f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), current).Return(sn, true, nil).Times(1)
	f.supernodeKeeper.EXPECT().GetParams(gomock.Any()).Return(sntypes.DefaultParams()).Times(1)
	require.NoError(t, f.keeper.SetReport(f.ctx, types.EpochReport{EpochId: 1, SupernodeAccount: old, HostReport: types.HostReport{DiskUsagePercent: 1}}))
}

func TestSubmitMalformedLineageFailsInsteadOfPanics(t *testing.T) {
	f := initFixture(t)
	f.ctx = f.ctx.WithBlockHeight(1)
	creator := testAddress(t, f, []byte{51, 52, 53, 54})
	seedEpochAnchorForReportTest(t, f, 0, []string{creator}, []string{creator})
	f.ctx.KVStore(f.storeKey).Set(types.AccountTransitionReverseKey(creator), []byte{0xff})
	require.NotPanics(t, func() {
		_, err := keeper.NewMsgServerImpl(f.keeper).SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{Creator: creator, EpochId: 0})
		require.ErrorContains(t, err, "malformed account transition")
	})
}

func TestSubmitSignerContinuityAndLegacyReportDecode(t *testing.T) {
	t.Run("old signer rejected", func(t *testing.T) {
		f := initFixture(t)
		f.ctx = f.ctx.WithBlockHeight(1)
		old := testAddress(t, f, []byte{61, 62, 63, 64})
		current := testAddress(t, f, []byte{65, 66, 67, 68})
		require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: old, DestinationAccount: current, EffectiveEpoch: 1}))
		seedEpochAnchorForReportTest(t, f, 0, []string{old}, []string{old})
		_, err := keeper.NewMsgServerImpl(f.keeper).SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{Creator: old, EpochId: 0})
		require.ErrorIs(t, err, types.ErrInvalidSigner)
	})

	t.Run("unlinked current account rejected", func(t *testing.T) {
		f := initFixture(t)
		f.ctx = f.ctx.WithBlockHeight(1)
		old := testAddress(t, f, []byte{71, 72, 73, 74})
		linkedCurrent := testAddress(t, f, []byte{79, 80, 81, 82})
		unlinked := testAddress(t, f, []byte{75, 76, 77, 78})
		require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: old, DestinationAccount: linkedCurrent, EffectiveEpoch: 1}))
		seedEpochAnchorForReportTest(t, f, 0, []string{old}, []string{old})
		f.supernodeKeeper.EXPECT().GetSuperNodeByAccount(gomock.Any(), unlinked).Return(sntypes.SuperNode{SupernodeAccount: unlinked}, true, nil).Times(1)
		_, err := keeper.NewMsgServerImpl(f.keeper).SubmitEpochReport(f.ctx, &types.MsgSubmitEpochReport{Creator: unlinked, EpochId: 0})
		require.Error(t, err)
	})

	t.Run("historical empty current submitter decodes", func(t *testing.T) {
		f := initFixture(t)
		account := testAddress(t, f, []byte{81, 82, 83, 84})
		require.NoError(t, f.keeper.SetReportRaw(f.ctx, types.EpochReport{EpochId: 4, SupernodeAccount: account}))
		report, found := f.keeper.GetReport(f.ctx, 4, account)
		require.True(t, found)
		require.Empty(t, report.CurrentSubmitter)
	})
}

func TestEnforcementRecognizesHistoricalReportAcrossTransition(t *testing.T) {
	f := initFixture(t)
	old := testAddress(t, f, []byte{91, 92, 93, 94})
	current := testAddress(t, f, []byte{95, 96, 97, 98})
	validator := sdk.ValAddress([]byte{99, 100, 101, 102}).String()
	require.NoError(t, f.keeper.SetReportRaw(f.ctx, types.EpochReport{EpochId: 1, SupernodeAccount: old}))
	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: old, DestinationAccount: current, EffectiveEpoch: 2}))
	sn := sntypes.SuperNode{SupernodeAccount: current, ValidatorAddress: validator}
	params := types.DefaultParams()
	params.ConsecutiveEpochsToPostpone = 1
	params.RequiredOpenPorts = nil
	f.supernodeKeeper.EXPECT().GetAllSuperNodes(gomock.Any(), sntypes.SuperNodeStateActive).Return([]sntypes.SuperNode{sn}, nil).Times(1)
	f.supernodeKeeper.EXPECT().GetAllSuperNodes(gomock.Any(), sntypes.SuperNodeStatePostponed).Return(nil, nil).Times(1)
	f.supernodeKeeper.EXPECT().SetSuperNodePostponed(gomock.Any(), gomock.Any(), "audit_missing_reports").Times(0)
	require.NoError(t, f.keeper.EnforceEpochEnd(f.ctx, 1, params))
}

func TestReporterDivergenceAndCleanRecoverySpanTransition(t *testing.T) {
	f := initFixture(t)
	old := testAddress(t, f, []byte{111, 112, 113, 114})
	current := testAddress(t, f, []byte{115, 116, 117, 118})
	baseA := testAddress(t, f, []byte{121, 122, 123, 124})
	baseB := testAddress(t, f, []byte{125, 126, 127, 128})
	for _, account := range []string{old, baseA, baseB} {
		require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{ReporterSupernodeAccount: account}))
	}
	addContinuityResultFacts(t, f, 1, old, 4, 1)
	addContinuityResultFacts(t, f, 1, baseA, 1, 4)
	addContinuityResultFacts(t, f, 1, baseB, 1, 4)
	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{SourceAccount: old, DestinationAccount: current, EffectiveEpoch: 2}))
	addContinuityResultFacts(t, f, 2, current, 5, 0)
	addContinuityResultFacts(t, f, 2, baseA, 1, 4)
	addContinuityResultFacts(t, f, 2, baseB, 1, 4)
	params := types.DefaultParams().WithDefaults()
	params.StorageTruthReporterMinReportsForDivergence = 5
	require.NoError(t, f.keeper.ApplyReporterDivergenceAtEpochEnd(f.ctx, 2, params))
	state, found := f.keeper.GetReporterReliabilityState(f.ctx, current)
	require.True(t, found)
	require.Equal(t, int64(8), state.ReliabilityScore)

	addContinuityResultFacts(t, f, 3, current, 0, 5)
	require.NoError(t, f.keeper.ApplyReporterCleanEpochRecoveryAtEpochEnd(f.ctx, 3, params))
	state, found = f.keeper.GetReporterReliabilityState(f.ctx, current)
	require.True(t, found)
	require.Equal(t, int64(3), state.ReliabilityScore, "one epoch of decay followed by the four-point clean recovery")
}

func TestReporterCleanRecoveryDoesNotDoubleApplyDuringTransitionEpoch(t *testing.T) {
	f := initFixture(t)
	old := testAddress(t, f, []byte{131, 132, 133, 134})
	current := testAddress(t, f, []byte{135, 136, 137, 138})
	require.NoError(t, f.keeper.SetReporterReliabilityState(f.ctx, types.ReporterReliabilityState{
		ReporterSupernodeAccount: old,
		ReliabilityScore:         8,
		LastUpdatedEpoch:         1,
	}))
	addContinuityResultFacts(t, f, 1, old, 0, 5)
	require.NoError(t, recordAccountTransition(t, f, types.AccountTransition{
		SourceAccount: old, DestinationAccount: current, EffectiveEpoch: 2,
	}))

	params := types.DefaultParams().WithDefaults()
	require.NoError(t, f.keeper.ApplyReporterCleanEpochRecoveryAtEpochEnd(f.ctx, 1, params))
	state, found := f.keeper.GetReporterReliabilityState(f.ctx, current)
	require.True(t, found)
	require.Equal(t, int64(4), state.ReliabilityScore, "old epoch index and current singleton must collapse to one actor")
}
