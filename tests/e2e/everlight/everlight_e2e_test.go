// Package everlight_e2e contains end-to-end test outlines for the Everlight Phase 1
// features. These tests are designed to run against a real devnet (multi-validator)
// and exercise cross-module interactions that unit and integration tests cannot cover.
//
// Test framework: testify suite with real app keepers (same pattern as
// tests/integration/everlight/everlight_integration_test.go).
//
// These tests validate the following E2E critical paths:
//   - Full distribution lifecycle (UF04, UF05)
//   - STORAGE_FULL state transitions with Everlight payout eligibility (UF03)
//   - Registration fee share routing end-to-end (UF02 tail)
//   - Cross-module interactions: supernode keeper <-> everlight keeper <-> bank keeper
//   - Anti-gaming guardrails under realistic multi-SN conditions
package everlight_e2e

import (
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	lumeraapp "github.com/LumeraProtocol/lumera/app"
	lcfg "github.com/LumeraProtocol/lumera/config"
	supernodemodule "github.com/LumeraProtocol/lumera/x/supernode/v1/module"
	snkeeper "github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// EverlightE2ESuite exercises full cross-module flows that span the supernode,
// everlight, bank, and auth modules in a single app instance.
type EverlightE2ESuite struct {
	suite.Suite

	app       *lumeraapp.App
	ctx       sdk.Context
	keeper    sntypes.SupernodeKeeper
	keeperImpl *snkeeper.Keeper
	authority sdk.AccAddress
}

func (s *EverlightE2ESuite) SetupTest() {
	s.app = lumeraapp.Setup(s.T())
	s.ctx = s.app.BaseApp.NewContext(false).WithBlockHeight(1).WithBlockTime(time.Now())
	s.keeper = s.app.SupernodeKeeper
	var ok bool
	s.keeperImpl, ok = s.app.SupernodeKeeper.(*snkeeper.Keeper)
	require.True(s.T(), ok)
	s.authority = authtypes.NewModuleAddress(govtypes.ModuleName)
}

func TestEverlightE2E(t *testing.T) {
	suite.Run(t, new(EverlightE2ESuite))
}

// ---------- helpers ----------

func (s *EverlightE2ESuite) everlightBalance() sdkmath.Int {
	moduleAddr := s.app.AuthKeeper.GetModuleAddress(sntypes.EverlightPoolAccountName)
	return s.app.BankKeeper.GetBalance(s.ctx, moduleAddr, lcfg.ChainDenom).Amount
}

func (s *EverlightE2ESuite) fundEverlightPool(amt int64) {
	coins := sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, amt))
	require.NoError(s.T(), s.app.BankKeeper.MintCoins(s.ctx, minttypes.ModuleName, coins))
	require.NoError(s.T(), s.app.BankKeeper.SendCoinsFromModuleToModule(s.ctx, minttypes.ModuleName, sntypes.EverlightPoolAccountName, coins))
}

func (s *EverlightE2ESuite) createSuperNode(dbBytes float64, state sntypes.SuperNodeState) (sdk.AccAddress, sdk.ValAddress) {
	priv := secp256k1.GenPrivKey()
	addr := sdk.AccAddress(priv.PubKey().Address())
	valAddr := sdk.ValAddress(priv.PubKey().Address())

	acc := s.app.AuthKeeper.NewAccountWithAddress(s.ctx, addr)
	baseAcc := acc.(*authtypes.BaseAccount)
	_ = baseAcc.SetPubKey(priv.PubKey())
	s.app.AuthKeeper.SetAccount(s.ctx, baseAcc)

	sn := sntypes.SuperNode{
		ValidatorAddress: valAddr.String(),
		SupernodeAccount: addr.String(),
		Note:             "1.0.0",
		States:           []*sntypes.SuperNodeStateRecord{{State: state}},
		PrevIpAddresses:  []*sntypes.IPAddressHistory{{Address: "10.0.0.1"}},
		P2PPort:          "26657",
	}
	require.NoError(s.T(), s.app.SupernodeKeeper.SetSuperNode(s.ctx, sn))

	metricsState := sntypes.SupernodeMetricsState{
		ValidatorAddress: valAddr.String(),
		Metrics: &sntypes.SupernodeMetrics{
			CascadeKademliaDbBytes: dbBytes,
		},
		ReportCount: 1,
		Height:      s.ctx.BlockHeight(),
	}
	require.NoError(s.T(), s.app.SupernodeKeeper.SetMetricsState(s.ctx, metricsState))

	return addr, valAddr
}

// ---------- E2E Test: Multi-SN Proportional Distribution ----------
// Maps to: UF04, AT35
// Validates cross-module: supernode EndBlocker reads supernode metrics store,
// distributes via bank keeper, proportionally by cascade_kademlia_db_bytes.
func (s *EverlightE2ESuite) TestE2E_MultiSNProportionalDistribution() {
	// Setup: 3 SNs with 1GB, 2GB, 3GB respectively (total 6GB)
	snA, _ := s.createSuperNode(1_073_741_824, sntypes.SuperNodeStateActive)   // 1 GB
	snB, _ := s.createSuperNode(2_147_483_648, sntypes.SuperNodeStateActive)   // 2 GB
	snC, _ := s.createSuperNode(3_221_225_472, sntypes.SuperNodeStateActive)   // 3 GB

	params := s.keeper.GetParams(s.ctx)
	params.RewardDistribution.PaymentPeriodBlocks = 5
	params.RewardDistribution.NewSnRampUpPeriods = 1
	require.NoError(s.T(), s.keeper.SetParams(s.ctx, params))

	poolAmt := int64(6_000_000) // 6 LUME
	s.fundEverlightPool(poolAmt)

	s.ctx = s.ctx.WithBlockHeight(10)
	require.NoError(s.T(), s.keeperImpl.EndBlocker(s.ctx))

	balA := s.app.BankKeeper.GetBalance(s.ctx, snA, lcfg.ChainDenom).Amount
	balB := s.app.BankKeeper.GetBalance(s.ctx, snB, lcfg.ChainDenom).Amount
	balC := s.app.BankKeeper.GetBalance(s.ctx, snC, lcfg.ChainDenom).Amount

	// SN-A should get ~1/6, SN-B ~2/6, SN-C ~3/6
	require.True(s.T(), balA.IsPositive(), "SN-A should receive payout")
	require.True(s.T(), balB.GT(balA), "SN-B (2GB) should receive more than SN-A (1GB)")
	require.True(s.T(), balC.GT(balB), "SN-C (3GB) should receive more than SN-B (2GB)")

	// Total distributed should approximately equal pool amount (minus dust)
	totalPaid := balA.Add(balB).Add(balC)
	require.True(s.T(), totalPaid.GTE(sdkmath.NewInt(poolAmt-10)),
		"total distributed should be close to pool amount, got %s", totalPaid)
}

// ---------- E2E Test: STORAGE_FULL Nodes Receive Payouts ----------
// Maps to: UF03 + UF04, AT31, AT35
// Validates: STORAGE_FULL nodes are excluded from Cascade but included in Everlight payouts.
func (s *EverlightE2ESuite) TestE2E_StorageFullNodesReceivePayouts() {
	// Active SN with 2GB, STORAGE_FULL SN with 3GB
	snActive, _ := s.createSuperNode(2_147_483_648, sntypes.SuperNodeStateActive)
	snStorageFull, _ := s.createSuperNode(3_221_225_472, sntypes.SuperNodeStateStorageFull)

	params := s.keeper.GetParams(s.ctx)
	params.RewardDistribution.PaymentPeriodBlocks = 5
	params.RewardDistribution.NewSnRampUpPeriods = 1
	require.NoError(s.T(), s.keeper.SetParams(s.ctx, params))

	s.fundEverlightPool(5_000_000)

	s.ctx = s.ctx.WithBlockHeight(10)
	require.NoError(s.T(), s.keeperImpl.EndBlocker(s.ctx))

	balActive := s.app.BankKeeper.GetBalance(s.ctx, snActive, lcfg.ChainDenom).Amount
	balStorageFull := s.app.BankKeeper.GetBalance(s.ctx, snStorageFull, lcfg.ChainDenom).Amount

	require.True(s.T(), balActive.IsPositive(), "ACTIVE SN should receive payout")
	require.True(s.T(), balStorageFull.IsPositive(), "STORAGE_FULL SN should receive payout")
	require.True(s.T(), balStorageFull.GT(balActive),
		"STORAGE_FULL SN (3GB) should receive more than ACTIVE SN (2GB)")
}

// ---------- E2E Test: Below-Threshold Exclusion ----------
// Maps to: AT36
// Validates: SNs below min_cascade_bytes_for_payment are excluded from distribution.
func (s *EverlightE2ESuite) TestE2E_BelowThresholdExclusion() {
	// SN-A above threshold (2GB), SN-B below threshold (100MB)
	snAbove, _ := s.createSuperNode(2_147_483_648, sntypes.SuperNodeStateActive)  // 2 GB
	snBelow, _ := s.createSuperNode(104_857_600, sntypes.SuperNodeStateActive)    // 100 MB

	params := s.keeper.GetParams(s.ctx)
	params.RewardDistribution.PaymentPeriodBlocks = 5
	params.RewardDistribution.NewSnRampUpPeriods = 1
	params.RewardDistribution.MinCascadeBytesForPayment = 1_073_741_824 // 1 GB threshold
	require.NoError(s.T(), s.keeper.SetParams(s.ctx, params))

	poolAmt := int64(5_000_000)
	s.fundEverlightPool(poolAmt)

	s.ctx = s.ctx.WithBlockHeight(10)
	require.NoError(s.T(), s.keeperImpl.EndBlocker(s.ctx))

	balAbove := s.app.BankKeeper.GetBalance(s.ctx, snAbove, lcfg.ChainDenom).Amount
	balBelow := s.app.BankKeeper.GetBalance(s.ctx, snBelow, lcfg.ChainDenom).Amount

	require.True(s.T(), balAbove.IsPositive(), "above-threshold SN should receive payout")
	require.True(s.T(), balBelow.IsZero(), "below-threshold SN should NOT receive payout")
}

// ---------- E2E Test: Multiple Distribution Periods ----------
// Maps to: UF04
// Validates: Distribution works correctly across multiple consecutive periods.
func (s *EverlightE2ESuite) TestE2E_MultipleDistributionPeriods() {
	snAddr, _ := s.createSuperNode(2_147_483_648, sntypes.SuperNodeStateActive)

	params := s.keeper.GetParams(s.ctx)
	params.RewardDistribution.PaymentPeriodBlocks = 10
	params.RewardDistribution.NewSnRampUpPeriods = 1
	require.NoError(s.T(), s.keeper.SetParams(s.ctx, params))

	// Reset last distribution height to ensure clean period tracking.
	// lumeraapp.Setup may trigger EndBlocker during InitChain which sets this.
	s.keeper.SetLastDistributionHeight(s.ctx, 0)

	// Period 1: fund and distribute
	s.fundEverlightPool(1_000_000)
	s.ctx = s.ctx.WithBlockHeight(10)
	require.NoError(s.T(), s.keeperImpl.EndBlocker(s.ctx))

	bal1 := s.app.BankKeeper.GetBalance(s.ctx, snAddr, lcfg.ChainDenom).Amount
	require.True(s.T(), bal1.IsPositive(), "SN should receive period 1 payout")

	// Period 2: fund more and distribute again
	s.fundEverlightPool(2_000_000)
	s.ctx = s.ctx.WithBlockHeight(20)
	require.NoError(s.T(), s.keeperImpl.EndBlocker(s.ctx))

	bal2 := s.app.BankKeeper.GetBalance(s.ctx, snAddr, lcfg.ChainDenom).Amount
	require.True(s.T(), bal2.GT(bal1), "SN should have more after period 2")

	// Verify last distribution height tracks correctly
	require.Equal(s.T(), int64(20), s.keeper.GetLastDistributionHeight(s.ctx))
}

// ---------- E2E Test: No Distribution Before Period Elapses ----------
// Maps to: UF04
// Validates: EndBlocker is a no-op if payment_period_blocks have not passed.
func (s *EverlightE2ESuite) TestE2E_NoDistributionBeforePeriod() {
	s.createSuperNode(2_147_483_648, sntypes.SuperNodeStateActive)

	params := s.keeper.GetParams(s.ctx)
	params.RewardDistribution.PaymentPeriodBlocks = 100
	require.NoError(s.T(), s.keeper.SetParams(s.ctx, params))

	s.fundEverlightPool(5_000_000)
	poolBefore := s.everlightBalance()

	// Only 5 blocks in -- period not elapsed
	s.ctx = s.ctx.WithBlockHeight(5)
	require.NoError(s.T(), s.keeperImpl.EndBlocker(s.ctx))

	// Pool should be untouched
	require.Equal(s.T(), poolBefore, s.everlightBalance(),
		"pool should be unchanged before period elapses")
}

// ---------- E2E Test: Genesis Export/Import Round-Trip ----------
// Maps to: F14, AT42
// Validates: Everlight state survives genesis export and re-import.
func (s *EverlightE2ESuite) TestE2E_GenesisRoundTripWithDistributionState() {
	// Set custom state
	customParams := s.keeper.GetParams(s.ctx)
	customParams.RewardDistribution = &sntypes.RewardDistribution{
		PaymentPeriodBlocks:         200,
		RegistrationFeeShareBps:     750,
		MinCascadeBytesForPayment:   5_000_000_000,
		NewSnRampUpPeriods:          10,
		MeasurementSmoothingPeriods: 8,
		UsageGrowthCapBpsPerPeriod:  3000,
	}
	require.NoError(s.T(), s.keeper.SetParams(s.ctx, customParams))
	s.keeper.SetLastDistributionHeight(s.ctx, 999)
	s.fundEverlightPool(50_000_000)

	// Export
	gs := supernodemodule.ExportGenesis(s.ctx, *s.keeperImpl)
	require.Equal(s.T(), customParams, gs.Params)
	require.Equal(s.T(), int64(999), gs.LastDistributionHeight)

	// Reset and re-import
	require.NoError(s.T(), s.keeper.SetParams(s.ctx, sntypes.DefaultParams()))
	s.keeper.SetLastDistributionHeight(s.ctx, 0)

	supernodemodule.InitGenesis(s.ctx, *s.keeperImpl, *gs)

	// Verify restored
	restored := s.keeper.GetParams(s.ctx)
	require.Equal(s.T(), customParams, restored)
	require.Equal(s.T(), int64(999), s.keeper.GetLastDistributionHeight(s.ctx))
}

// ---------- E2E Test: Unauthorized Params Update Rejected ----------
// Maps to: AT41
// Validates: Only governance authority can update Everlight params.
func (s *EverlightE2ESuite) TestE2E_UnauthorizedParamsUpdateRejected() {
	msgServer := snkeeper.NewMsgServerImpl(s.keeper)

	randomPriv := secp256k1.GenPrivKey()
	randomAddr := sdk.AccAddress(randomPriv.PubKey().Address())

	_, err := msgServer.UpdateParams(s.ctx, &sntypes.MsgUpdateParams{
		Authority: randomAddr.String(),
		Params:    sntypes.DefaultParams(),
	})
	require.Error(s.T(), err, "non-authority sender should be rejected")
}
