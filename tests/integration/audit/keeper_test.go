package integration_test

import (
	"testing"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	sdktestutil "github.com/cosmos/cosmos-sdk/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/LumeraProtocol/lumera/testutil/cryptotestutils"
	auditkeeper "github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	auditmodule "github.com/LumeraProtocol/lumera/x/audit/v1/module"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	supernodemocks "github.com/LumeraProtocol/lumera/x/supernode/v1/mocks"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// KeeperIntegrationSuite tests the audit keeper with the production codec and a
// real IAVL-backed KV store (identical to what the live app uses).
//
// Integration value over unit tests:
//   - Real protobuf codec for all encode/decode paths.
//   - Real IAVL store (key ordering, iteration, pagination).
//   - Multiple keeper operations share one store within each test method.
//
// The supernode keeper is mocked because setting up real staking validators is
// out of scope for this level of testing.
type KeeperIntegrationSuite struct {
	suite.Suite

	ctx    sdk.Context
	keeper auditkeeper.Keeper
	snMock *supernodemocks.MockSupernodeKeeper
	ctrl   *gomock.Controller
}

func TestAuditKeeperIntegrationSuite(t *testing.T) {
	suite.Run(t, new(KeeperIntegrationSuite))
}

// SetupTest creates a fresh keeper + mock per test method using a real IAVL store.
func (s *KeeperIntegrationSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.snMock = supernodemocks.NewMockSupernodeKeeper(s.ctrl)

	encCfg := moduletestutil.MakeTestEncodingConfig(auditmodule.AppModuleBasic{})
	addrCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	transientKey := storetypes.NewTransientStoreKey("transient_integration_audit")
	storeService := runtime.NewKVStoreService(storeKey)
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)

	// DefaultContextWithDB provides a real on-disk (or mem-mapped) IAVL store.
	dbCtx := sdktestutil.DefaultContextWithDB(s.T(), storeKey, transientKey)
	s.ctx = dbCtx.Ctx.
		WithBlockHeight(1).
		WithEventManager(sdk.NewEventManager())

	s.keeper = auditkeeper.NewKeeper(
		encCfg.Codec,
		addrCodec,
		storeService,
		log.NewNopLogger(),
		authority,
		s.snMock,
	)
	require.NoError(s.T(), s.keeper.SetParams(s.ctx, types.DefaultParams()))
}

func (s *KeeperIntegrationSuite) TearDownTest() {
	s.ctrl.Finish()
}

// ── helpers ──────────────────────────────────────────────────────────────────

func (s *KeeperIntegrationSuite) freshNode() (sntypes.SuperNode, sdk.AccAddress, sdk.ValAddress) {
	s.T().Helper()
	_, acc, val := cryptotestutils.SupernodeAddresses()
	sn := sntypes.SuperNode{
		SupernodeAccount: acc.String(),
		ValidatorAddress: sdk.ValAddress(val).String(),
	}
	return sn, acc, val
}

func (s *KeeperIntegrationSuite) setSuspicion(account string, score, epoch int64) {
	s.T().Helper()
	require.NoError(s.T(), s.keeper.SetNodeSuspicionState(s.ctx, types.NodeSuspicionState{
		SupernodeAccount: account,
		SuspicionScore:   score,
		LastUpdatedEpoch: uint64(epoch),
		// Preset enforcement matrix predicate fields so postpone decisions work:
		// (ClassA >= 1 AND total >= 2) is the normal-postpone predicate.
		ClassACountWindow: 1,
		ClassBCountWindow: 1,
		// Preset clean passes for recovery gate (requires CleanPassCount >= params.StorageTruthRecoveryCleanPassCount).
		CleanPassCount: 5,
	}))
}

func (s *KeeperIntegrationSuite) setDeterioration(ticketID string, score int64, epoch uint64) {
	s.T().Helper()
	require.NoError(s.T(), s.keeper.SetTicketDeteriorationState(s.ctx, types.TicketDeteriorationState{
		TicketId:           ticketID,
		DeteriorationScore: score,
		LastUpdatedEpoch:   epoch,
		// Preset heal eligibility predicate: RecentFailureEpochCount >= 2 satisfies the
		// heal scheduling eligibility check (§12 predicates).
		RecentFailureEpochCount: 2,
	}))
}

// ── Test 1: multi-epoch score accumulation ────────────────────────────────────

// TestMultiEpochScoreAccumulation verifies that multiple node suspicion states
// are independently stored and retrieved from the real KV store.  Each node's
// score must survive a round-trip through the codec and not bleed into other keys.
func (s *KeeperIntegrationSuite) TestMultiEpochScoreAccumulation() {
	nodes := make([]sntypes.SuperNode, 5)
	for i := range nodes {
		sn, _, _ := s.freshNode()
		nodes[i] = sn
		s.setSuspicion(sn.SupernodeAccount, int64((i+1)*20), int64(i))
	}

	// All five nodes must be retrievable with independent scores.
	for i, sn := range nodes {
		state, found := s.keeper.GetNodeSuspicionState(s.ctx, sn.SupernodeAccount)
		require.True(s.T(), found, "node %d suspicion state not found", i)
		require.Equal(s.T(), int64((i+1)*20), state.SuspicionScore, "node %d score mismatch", i)
		require.Equal(s.T(), uint64(i), state.LastUpdatedEpoch)
	}
}

// ── Test 2: stored score is unchanged; decay is on-the-fly ────────────────────

// TestScoreDecayPreservesStoredValue confirms that the underlying stored score
// is NOT mutated by epoch-end enforcement.  Decay is applied transiently during
// band calculation and does not overwrite the store.
func (s *KeeperIntegrationSuite) TestScoreDecayPreservesStoredValue() {
	sn, _, valAddr := s.freshNode()

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SOFT
	params.StorageTruthNodeSuspicionThresholdPostpone = 50
	params.StorageTruthNodeSuspicionThresholdWatch = 20
	// Exponential decay factor 920 (0.92/epoch). Score=100, epoch 5:
	// decayTowardZero(100, 920, 5) = 100→92→84→77→70→64 ≥ postpone(50) → still postpones.
	params.StorageTruthNodeSuspicionDecayPerEpoch = 920
	params.ConsecutiveEpochsToPostpone = 99

	// Score set at epoch 0.
	s.setSuspicion(sn.SupernodeAccount, 100, 0)

	// At epoch 5 the decayed score would be 100 * 0.92^5 ≈ 64 ≥ threshold 50 → postpone.
	s.snMock.EXPECT().GetAllSuperNodes(gomock.AssignableToTypeOf(s.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{sn}, nil)
	s.snMock.EXPECT().GetAllSuperNodes(gomock.AssignableToTypeOf(s.ctx), sntypes.SuperNodeStatePostponed).
		Return([]sntypes.SuperNode{}, nil)
	s.snMock.EXPECT().SetSuperNodePostponed(gomock.AssignableToTypeOf(s.ctx), sdk.ValAddress(valAddr), "audit_storage_truth_suspicion").
		Return(nil).Times(1)

	require.NoError(s.T(), s.keeper.EnforceEpochEnd(s.ctx, 5, params))

	// Stored score must still be 100, not the decayed value.
	stateAfter, found := s.keeper.GetNodeSuspicionState(s.ctx, sn.SupernodeAccount)
	require.True(s.T(), found)
	require.Equal(s.T(), int64(100), stateAfter.SuspicionScore,
		"EnforceEpochEnd must not mutate the stored suspicion score")
	require.Equal(s.T(), uint64(0), stateAfter.LastUpdatedEpoch,
		"EnforceEpochEnd must not update LastUpdatedEpoch in the store")
}

// ── Test 3: heal op max cap ───────────────────────────────────────────────────

// TestHealOpMaxCapEnforced verifies that ProcessStorageTruthHealOpsAtEpochEnd
// respects StorageTruthMaxSelfHealOpsPerEpoch.  Given 5 eligible tickets and a
// cap of 3, only the 3 highest-score tickets receive heal ops.
func (s *KeeperIntegrationSuite) TestHealOpMaxCapEnforced() {
	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SHADOW
	params.StorageTruthTicketDeteriorationHealThreshold = 10
	params.StorageTruthMaxSelfHealOpsPerEpoch = 3
	require.NoError(s.T(), s.keeper.SetParams(s.ctx, params))

	tickets := []struct {
		id    string
		score int64
	}{
		{"ticket-alpha", 90},
		{"ticket-beta", 80},
		{"ticket-gamma", 70},
		{"ticket-delta", 60},
		{"ticket-epsilon", 50},
	}
	for _, tc := range tickets {
		s.setDeterioration(tc.id, tc.score, 0)
	}

	// Need active supernodes for assignment.
	_, acc1, _ := cryptotestutils.SupernodeAddresses()
	_, acc2, _ := cryptotestutils.SupernodeAddresses()
	_, acc3, _ := cryptotestutils.SupernodeAddresses()
	s.snMock.EXPECT().
		GetAllSuperNodes(gomock.AssignableToTypeOf(s.ctx), sntypes.SuperNodeStateActive).
		Return([]sntypes.SuperNode{
			{SupernodeAccount: acc1.String()},
			{SupernodeAccount: acc2.String()},
			{SupernodeAccount: acc3.String()},
		}, nil).AnyTimes()

	s.keeper.SetNextHealOpID(s.ctx, 1)
	require.NoError(s.T(), s.keeper.ProcessStorageTruthHealOpsAtEpochEnd(s.ctx, 0, params))

	healOps, err := s.keeper.GetAllHealOps(s.ctx)
	require.NoError(s.T(), err)
	require.Len(s.T(), healOps, 3, "should schedule exactly 3 heal ops (cap enforced)")

	// The 3 highest-score tickets should be scheduled.
	scheduled := make(map[string]bool)
	for _, op := range healOps {
		scheduled[op.TicketId] = true
		require.Equal(s.T(), types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED, op.Status)
	}
	require.True(s.T(), scheduled["ticket-alpha"])
	require.True(s.T(), scheduled["ticket-beta"])
	require.True(s.T(), scheduled["ticket-gamma"])
	require.False(s.T(), scheduled["ticket-delta"], "lower-score ticket should NOT be scheduled")
	require.False(s.T(), scheduled["ticket-epsilon"], "lower-score ticket should NOT be scheduled")
}

// ── Test 4: heal op expiry releases ticket ────────────────────────────────────

// TestHealOpExpiryReleasesTicket verifies that when ProcessStorageTruthHealOpsAtEpochEnd
// runs past a heal op's deadline, the op transitions to EXPIRED and the
// ActiveHealOpId is cleared from the ticket deterioration state.
func (s *KeeperIntegrationSuite) TestHealOpExpiryReleasesTicket() {
	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())

	params := types.DefaultParams()
	params.StorageTruthMaxSelfHealOpsPerEpoch = 0 // only expiry logic runs
	require.NoError(s.T(), s.keeper.SetParams(s.ctx, params))

	expiredOp := types.HealOp{
		HealOpId:               42,
		TicketId:               "ticket-to-expire",
		ScheduledEpochId:       1,
		HealerSupernodeAccount: "sn-healer-x",
		Status:                 types.HealOpStatus_HEAL_OP_STATUS_HEALER_REPORTED,
		DeadlineEpochId:        3,
		CreatedHeight:          1,
		UpdatedHeight:          1,
	}
	require.NoError(s.T(), s.keeper.SetHealOp(s.ctx, expiredOp))
	require.NoError(s.T(), s.keeper.SetTicketDeteriorationState(s.ctx, types.TicketDeteriorationState{
		TicketId:           "ticket-to-expire",
		DeteriorationScore: 60,
		ActiveHealOpId:     42,
	}))

	// Current epoch = 3 → heal op with deadline 3 should expire.
	require.NoError(s.T(), s.keeper.ProcessStorageTruthHealOpsAtEpochEnd(s.ctx, 3, params))

	op, found := s.keeper.GetHealOp(s.ctx, 42)
	require.True(s.T(), found)
	require.Equal(s.T(), types.HealOpStatus_HEAL_OP_STATUS_EXPIRED, op.Status,
		"heal op should be EXPIRED after deadline passes")

	ticketState, found := s.keeper.GetTicketDeteriorationState(s.ctx, "ticket-to-expire")
	require.True(s.T(), found)
	require.Equal(s.T(), uint64(0), ticketState.ActiveHealOpId,
		"ActiveHealOpId must be cleared when heal op expires")
}

// ── Test 5: genesis round-trip ────────────────────────────────────────────────

// TestGenesisRoundTrip verifies that ExportGenesis captures all storage-truth
// state and InitGenesis restores it faithfully in a fresh store.
func (s *KeeperIntegrationSuite) TestGenesisRoundTrip() {
	_, acc1, _ := cryptotestutils.SupernodeAddresses()
	_, acc2, _ := cryptotestutils.SupernodeAddresses()

	// Set diverse state.
	s.setSuspicion(acc1.String(), 75, 3)
	s.setSuspicion(acc2.String(), 30, 1)
	s.setDeterioration("ticket-genesis-1", 55, 2)
	s.setDeterioration("ticket-genesis-2", 22, 0)
	require.NoError(s.T(), s.keeper.SetHealOp(s.ctx, types.HealOp{
		HealOpId:               99,
		TicketId:               "ticket-genesis-1",
		ScheduledEpochId:       3,
		HealerSupernodeAccount: acc1.String(),
		Status:                 types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED,
		DeadlineEpochId:        10,
		CreatedHeight:          300,
		UpdatedHeight:          300,
	}))
	s.keeper.SetNextHealOpID(s.ctx, 100)

	// Export.
	gs, err := s.keeper.ExportGenesis(s.ctx)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), gs)

	// Import into a fresh keeper backed by a separate IAVL store.
	freshKey := storetypes.NewKVStoreKey("audit_genesis_test")
	freshTransient := storetypes.NewTransientStoreKey("transient_genesis_test")
	// Per 119-F8: ValidateScoreStatesGenesis rejects LastUpdatedEpoch > currentEpoch.
	// Highest LastUpdatedEpoch in this test is 3; epoch 3 starts at block 1201 (400 blocks/epoch).
	freshCtx := sdktestutil.DefaultContextWithDB(s.T(), freshKey, freshTransient).Ctx.
		WithBlockHeight(1201).
		WithEventManager(sdk.NewEventManager())

	encCfg := moduletestutil.MakeTestEncodingConfig(auditmodule.AppModuleBasic{})
	addrCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)
	freshKeeper := auditkeeper.NewKeeper(
		encCfg.Codec,
		addrCodec,
		runtime.NewKVStoreService(freshKey),
		log.NewNopLogger(),
		authority,
		s.snMock,
	)
	require.NoError(s.T(), freshKeeper.InitGenesis(freshCtx, *gs))

	// Verify node suspicion states.
	state1, found := freshKeeper.GetNodeSuspicionState(freshCtx, acc1.String())
	require.True(s.T(), found)
	require.Equal(s.T(), int64(75), state1.SuspicionScore)

	state2, found := freshKeeper.GetNodeSuspicionState(freshCtx, acc2.String())
	require.True(s.T(), found)
	require.Equal(s.T(), int64(30), state2.SuspicionScore)

	// Verify ticket deterioration states.
	tdState1, found := freshKeeper.GetTicketDeteriorationState(freshCtx, "ticket-genesis-1")
	require.True(s.T(), found)
	require.Equal(s.T(), int64(55), tdState1.DeteriorationScore)

	tdState2, found := freshKeeper.GetTicketDeteriorationState(freshCtx, "ticket-genesis-2")
	require.True(s.T(), found)
	require.Equal(s.T(), int64(22), tdState2.DeteriorationScore)

	// Verify heal op.
	healOp, found := freshKeeper.GetHealOp(freshCtx, 99)
	require.True(s.T(), found)
	require.Equal(s.T(), "ticket-genesis-1", healOp.TicketId)
	require.Equal(s.T(), types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED, healOp.Status)
}

// ── Test 6: recovery across epochs ───────────────────────────────────────────

// TestRecoveryAcrossEpochs confirms that a storage-truth postponed node recovers
// only when the decayed score drops below the watch threshold, and NOT before.
func (s *KeeperIntegrationSuite) TestRecoveryAcrossEpochs() {
	sn, _, valAddr := s.freshNode()

	params := types.DefaultParams()
	params.StorageTruthEnforcementMode = types.StorageTruthEnforcementMode_STORAGE_TRUTH_ENFORCEMENT_MODE_SOFT
	params.StorageTruthNodeSuspicionThresholdWatch = 20
	params.StorageTruthNodeSuspicionThresholdProbation = 35
	params.StorageTruthNodeSuspicionThresholdPostpone = 50
	// Exponential decay factor 700 (0.7/epoch).
	// Score=80, epoch 2: 80*0.7^2 = 80*0.49 ≈ 39 > watch(20) → no recovery.
	// Score=80, epoch 5: 80*0.7^5 = 80*0.168 ≈ 13 < watch(20) → recovery.
	params.StorageTruthNodeSuspicionDecayPerEpoch = 700
	params.StorageTruthRecoveryCleanPassCount = 3
	params.ConsecutiveEpochsToPostpone = 99

	// Score = 80 at epoch 0 → postponed at epoch 0.
	// setSuspicion presets ClassACountWindow=1, ClassBCountWindow=1, CleanPassCount=5.
	s.setSuspicion(sn.SupernodeAccount, 80, 0)

	// Epoch 0 end: score 80 > threshold 50, predicates met → postpone.
	s.snMock.EXPECT().GetAllSuperNodes(gomock.AssignableToTypeOf(s.ctx), sntypes.SuperNodeStateActive).Return([]sntypes.SuperNode{sn}, nil)
	s.snMock.EXPECT().GetAllSuperNodes(gomock.AssignableToTypeOf(s.ctx), sntypes.SuperNodeStatePostponed).Return([]sntypes.SuperNode{}, nil)
	s.snMock.EXPECT().SetSuperNodePostponed(gomock.AssignableToTypeOf(s.ctx), sdk.ValAddress(valAddr), "audit_storage_truth_suspicion").Return(nil)
	require.NoError(s.T(), s.keeper.EnforceEpochEnd(s.ctx, 0, params))

	// Per 121-F8: recovery uses delta since postponement (CleanPassCount - CleanPassCountAtPostpone),
	// not the cumulative count. Postponement snapshotted CleanPassCountAtPostpone=5. Simulate 3
	// clean epochs earned between epoch 0 and epoch 5 by incrementing CleanPassCount to 8.
	{
		st, found := s.keeper.GetNodeSuspicionState(s.ctx, sn.SupernodeAccount)
		require.True(s.T(), found)
		st.CleanPassCount += 3
		require.NoError(s.T(), s.keeper.SetNodeSuspicionState(s.ctx, st))
	}

	// Epoch 2 end: decayed score = 80 * 0.7^2 ≈ 39 > watch(20) → still postponed, no recovery.
	s.snMock.EXPECT().GetAllSuperNodes(gomock.AssignableToTypeOf(s.ctx), sntypes.SuperNodeStateActive).Return([]sntypes.SuperNode{}, nil)
	s.snMock.EXPECT().GetAllSuperNodes(gomock.AssignableToTypeOf(s.ctx), sntypes.SuperNodeStatePostponed).Return([]sntypes.SuperNode{sn}, nil)
	s.snMock.EXPECT().RecoverSuperNodeFromPostponed(gomock.Any(), gomock.Any()).Times(0)
	require.NoError(s.T(), s.keeper.EnforceEpochEnd(s.ctx, 2, params))

	// Epoch 5 end: decayed score = 80 * 0.7^5 ≈ 13 < watch(20), cleanPassDelta=3 >= required=3 → recovery.
	s.snMock.EXPECT().GetAllSuperNodes(gomock.AssignableToTypeOf(s.ctx), sntypes.SuperNodeStateActive).Return([]sntypes.SuperNode{}, nil)
	s.snMock.EXPECT().GetAllSuperNodes(gomock.AssignableToTypeOf(s.ctx), sntypes.SuperNodeStatePostponed).Return([]sntypes.SuperNode{sn}, nil)
	s.snMock.EXPECT().RecoverSuperNodeFromPostponed(gomock.AssignableToTypeOf(s.ctx), sdk.ValAddress(valAddr)).Return(nil).Times(1)
	require.NoError(s.T(), s.keeper.EnforceEpochEnd(s.ctx, 5, params))
}

// ── Test 7: many tickets pagination ──────────────────────────────────────────

// TestGetAllHealOpsPagination verifies that GetAllHealOps returns all heal ops
// stored across many different ticket IDs (tests real KV scan ordering).
func (s *KeeperIntegrationSuite) TestGetAllHealOpsPagination() {
	const numOps = 10
	s.keeper.SetNextHealOpID(s.ctx, 1)

	for i := 0; i < numOps; i++ {
		require.NoError(s.T(), s.keeper.SetHealOp(s.ctx, types.HealOp{
			HealOpId:               uint64(i + 1),
			TicketId:               "ticket-paginate-" + string(rune('A'+i)),
			ScheduledEpochId:       uint64(i),
			HealerSupernodeAccount: "sn-healer",
			Status:                 types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED,
			DeadlineEpochId:        uint64(i + 5),
			CreatedHeight:          uint64(i + 1),
			UpdatedHeight:          uint64(i + 1),
		}))
	}

	ops, err := s.keeper.GetAllHealOps(s.ctx)
	require.NoError(s.T(), err)
	require.Len(s.T(), ops, numOps, "all %d heal ops should be returned", numOps)

	// Each op should have a unique ID.
	seen := make(map[uint64]bool)
	for _, op := range ops {
		require.False(s.T(), seen[op.HealOpId], "duplicate heal op ID %d", op.HealOpId)
		seen[op.HealOpId] = true
	}
}

// ── Test 8: reporter reliability state round-trip ────────────────────────────

// TestReporterReliabilityStateRoundTrip verifies that reporter reliability state
// is stored and retrieved correctly in the real KV store.
func (s *KeeperIntegrationSuite) TestReporterReliabilityStateRoundTrip() {
	_, acc1, _ := cryptotestutils.SupernodeAddresses()
	_, acc2, _ := cryptotestutils.SupernodeAddresses()

	state1 := types.ReporterReliabilityState{
		ReporterSupernodeAccount: acc1.String(),
		ReliabilityScore:         42,
		LastUpdatedEpoch:         3,
		ContradictionCount:       2,
	}
	state2 := types.ReporterReliabilityState{
		ReporterSupernodeAccount: acc2.String(),
		ReliabilityScore:         17,
		LastUpdatedEpoch:         1,
	}

	require.NoError(s.T(), s.keeper.SetReporterReliabilityState(s.ctx, state1))
	require.NoError(s.T(), s.keeper.SetReporterReliabilityState(s.ctx, state2))

	got1, found := s.keeper.GetReporterReliabilityState(s.ctx, acc1.String())
	require.True(s.T(), found)
	require.Equal(s.T(), state1.ReliabilityScore, got1.ReliabilityScore)
	require.Equal(s.T(), state1.ContradictionCount, got1.ContradictionCount)

	got2, found := s.keeper.GetReporterReliabilityState(s.ctx, acc2.String())
	require.True(s.T(), found)
	require.Equal(s.T(), state2.ReliabilityScore, got2.ReliabilityScore)
}
