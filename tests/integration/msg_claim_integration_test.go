package integration_test

import (
	"testing"
	"time"

	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"

	lcfg "github.com/LumeraProtocol/lumera/config"
	"github.com/LumeraProtocol/lumera/x/claim/keeper"
	"github.com/LumeraProtocol/lumera/x/claim/types"
)

type MsgClaimIntegrationTestSuite struct {
	suite.Suite
	keeper      keeper.Keeper
	ctx         sdk.Context
	validPubKey string
	validSig    string
	oldAddress  string
	newAddress  string
	msgServer   types.MsgServer
}

func (s *MsgClaimIntegrationTestSuite) SetupTest() {
	k, ctx := keepertest.ClaimKeeper(s.T(), "")
	s.keeper = k
	s.ctx = ctx
	s.msgServer = keeper.NewMsgServerImpl(k)

	// Set up valid test data
	s.validPubKey = "038685010ec7ce724c1f83ba333564135feadf70eade12036546f20b95ce276a12"
	s.validSig = "1f4926307e7d94f5290058f8836429963431b3f7fc091567f1621a510d25cbcb71240f9c4488d9cafa16c8c3457d07c263afcfe4c77e0081a4e75bb618e99e1cd3"
	s.oldAddress = "Ptko7ZkiQXyT9Db45GtsewpnhnRRwpHkHBc"
	s.newAddress = "lumera1zvnc27832srgxa207y5hu2agy83wazfzurufyp"

	// Set up module parameters
	params := types.DefaultParams()
	params.EnableClaims = true
	params.MaxClaimsPerBlock = 10
	params.ClaimEndTime = time.Now().Add(time.Hour).Unix()
	s.Require().NoError(s.keeper.SetParams(ctx, params))
}

func (s *MsgClaimIntegrationTestSuite) TestClaimIntegration() {

	testAmount := int64(1_000_000) // Amount to be claimed in test cases
	validFee := sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, int64(1_000)))
	testCases := []struct {
		name      string
		setup     func()
		msg       *types.MsgClaim
		expErr    bool
		errString string
	}{
		{
			name: "successful claim",
			setup: func() {
				// Create and store claim record
				claimRecord := types.ClaimRecord{
					OldAddress: s.oldAddress,
					Balance:    sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, testAmount)),
					Claimed:    false,
				}
				err := s.keeper.SetClaimRecord(s.ctx, claimRecord)
				s.Require().NoError(err)

				// Fund module account
				err = s.keeper.GetBankKeeper().MintCoins(s.ctx, types.ModuleName, claimRecord.Balance)
				s.Require().NoError(err)
			},
			msg: &types.MsgClaim{
				OldAddress: "Ptko7ZkiQXyT9Db45GtsewpnhnRRwpHkHBc",
				NewAddress: "lumera1zvnc27832srgxa207y5hu2agy83wazfzurufyp",
				PubKey:     "038685010ec7ce724c1f83ba333564135feadf70eade12036546f20b95ce276a12",
				Signature:  "1f4926307e7d94f5290058f8836429963431b3f7fc091567f1621a510d25cbcb71240f9c4488d9cafa16c8c3457d07c263afcfe4c77e0081a4e75bb618e99e1cd3",
			},
			expErr: false,
		},
		{
			name: "claim already processed",
			setup: func() {
				// Create and store claimed record
				claimRecord := types.ClaimRecord{
					OldAddress: s.oldAddress,
					Balance:    sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, testAmount)),
					Claimed:    true,
					ClaimTime:  time.Now().Add(-15).Unix(),
				}
				err := s.keeper.SetClaimRecord(s.ctx, claimRecord)
				s.Require().NoError(err)
			},
			msg: &types.MsgClaim{
				OldAddress: s.oldAddress,
				NewAddress: s.newAddress,
				PubKey:     s.validPubKey,
				Signature:  s.validSig,
			},
			expErr:    true,
			errString: "claim already claimed",
		},
		{
			name:  "claim not found",
			setup: func() {}, // No setup needed - claim record doesn't exist
			msg: &types.MsgClaim{
				OldAddress: "NonExistentAddress",
				NewAddress: s.newAddress,
				PubKey:     s.validPubKey,
				Signature:  s.validSig,
			},
			expErr:    true,
			errString: "claim not found",
		},
		{
			name: "claims disabled",
			setup: func() {
				params := types.DefaultParams()
				params.EnableClaims = false
				s.Require().NoError(s.keeper.SetParams(s.ctx, params))

				// Add a claim record to ensure the check happens before record lookup
				claimRecord := types.ClaimRecord{
					OldAddress: s.oldAddress,
					Balance:    sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, testAmount)),
					Claimed:    false,
				}
				err := s.keeper.SetClaimRecord(s.ctx, claimRecord)
				s.Require().NoError(err)
			},
			msg: &types.MsgClaim{
				OldAddress: s.oldAddress,
				NewAddress: s.newAddress,
				PubKey:     s.validPubKey,
				Signature:  s.validSig,
			},
			expErr:    true,
			errString: "claim is disabled",
		},
		{
			name: "invalid signature",
			setup: func() {
				claimRecord := types.ClaimRecord{
					OldAddress: s.oldAddress,
					Balance:    sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, testAmount)),
					Claimed:    false,
				}
				err := s.keeper.SetClaimRecord(s.ctx, claimRecord)
				s.Require().NoError(err)
			},
			msg: &types.MsgClaim{
				OldAddress: s.oldAddress,
				NewAddress: s.newAddress,
				PubKey:     s.validPubKey,
				Signature:  "invalid_signature",
			},
			expErr:    true,
			errString: "invalid signature",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // Reset state for each test case
			if tc.setup != nil {
				tc.setup()
			}

			// Execute claim

			// Set fee in context
			s.ctx = s.ctx.WithValue(types.ClaimTxFee, validFee)
			resp, err := s.msgServer.Claim(s.ctx, tc.msg)

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errString)
				s.Require().Nil(resp)

				// For error cases, verify record state only if it's expected to exist
				if tc.name != "claim not found" {
					record, found, err := s.keeper.GetClaimRecord(s.ctx, tc.msg.OldAddress)
					s.Require().NoError(err)
					s.Require().True(found)
					if tc.name == "claim already processed" {
						s.Require().True(record.Claimed)
					} else {
						s.Require().False(record.Claimed)
					}
				}
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(resp)

				// Verify claim record is updated
				record, found, err := s.keeper.GetClaimRecord(s.ctx, tc.msg.OldAddress)
				s.Require().NoError(err)
				s.Require().True(found)
				s.Require().True(record.Claimed)
				s.Require().NotEqual(time.Time{}, record.ClaimTime)

				// Verify events were emitted
				events := s.ctx.EventManager().Events()
				s.Require().NotEmpty(events)

				found = false
				for _, event := range events {
					if event.Type == types.EventTypeClaimProcessed {
						found = true
						break
					}
				}
				s.Require().True(found, "claim_processed event not found")
			}
		})
	}
}

func TestMsgClaimIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(MsgClaimIntegrationTestSuite))
}
