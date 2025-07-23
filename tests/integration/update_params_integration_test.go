package integration_test

import (
	"testing"
	"time"

	"cosmossdk.io/math"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/stretchr/testify/suite"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/LumeraProtocol/lumera/app"
	claimtypes "github.com/LumeraProtocol/lumera/x/claim/types"
)

type IntegrationTestSuite struct {
	suite.Suite
	app      *app.App
	ctx      sdk.Context
	proposer sdk.AccAddress
	voter    sdk.AccAddress
}

func (s *IntegrationTestSuite) SetupTest() {
	s.app = app.Setup(s.T())
	s.Require().NotNil(s.app, "App initialization failed")

	s.ctx = s.app.BaseApp.NewContext(false)
	s.ctx = s.ctx.WithBlockHeader(tmproto.Header{
		Height:  1,
		Time:    time.Now().UTC(),
		ChainID: "test-chain",
	})

	s.proposer = sdk.AccAddress([]byte("proposer___________"))
	s.voter = sdk.AccAddress([]byte("voter_____________"))

	s.app.AuthKeeper.SetAccount(s.ctx, s.app.AuthKeeper.NewAccountWithAddress(s.ctx, s.proposer))
	s.app.AuthKeeper.SetAccount(s.ctx, s.app.AuthKeeper.NewAccountWithAddress(s.ctx, s.voter))

	proposerCoins := sdk.NewCoins(sdk.NewCoin("stake", math.NewInt(1000000000)))
	voterCoins := sdk.NewCoins(sdk.NewCoin("stake", math.NewInt(1000000000)))

	err := s.app.BankKeeper.MintCoins(s.ctx, minttypes.ModuleName, proposerCoins.Add(voterCoins...))
	s.Require().NoError(err, "Failed to mint coins")

	err = s.app.BankKeeper.SendCoinsFromModuleToAccount(s.ctx, minttypes.ModuleName, s.proposer, proposerCoins)
	s.Require().NoError(err, "Failed to send coins to proposer")
	err = s.app.BankKeeper.SendCoinsFromModuleToAccount(s.ctx, minttypes.ModuleName, s.voter, voterCoins)
	s.Require().NoError(err, "Failed to send coins to voter")

	valPubKey := ed25519.GenPrivKey().PubKey()
	valAddr := sdk.ValAddress(s.voter)
	validator, err := stakingtypes.NewValidator(
		valAddr.String(),
		valPubKey,
		stakingtypes.Description{},
	)
	s.Require().NoError(err, "Failed to create validator")

	validator.Status = stakingtypes.Bonded
	validator.Tokens = math.NewInt(1000000000)
	validator.DelegatorShares = math.LegacyNewDec(1000000000)
	s.app.StakingKeeper.SetValidator(s.ctx, validator)

	delegation := stakingtypes.NewDelegation(s.voter.String(), valAddr.String(), math.LegacyNewDec(1000000000))
	s.app.StakingKeeper.SetDelegation(s.ctx, delegation)

	s.app.StakingKeeper.SetValidatorByPowerIndex(s.ctx, validator)

	govParams := govv1.DefaultParams()
	err = s.app.GovKeeper.Params.Set(s.ctx, govParams)
	s.Require().NoError(err, "Failed to set governance parameters")

	claimParams := claimtypes.DefaultParams()
	s.app.ClaimKeeper.SetParams(s.ctx, claimParams)
}

func (s *IntegrationTestSuite) TestUpdateParamsProposal() {
	initialParams := s.app.ClaimKeeper.GetParams(s.ctx)
	govParams, err := s.app.GovKeeper.Params.Get(s.ctx)
	s.Require().NoError(err, "Failed to get governance parameters")

	newParams := claimtypes.NewParams(
		!initialParams.EnableClaims,
		time.Unix(initialParams.ClaimEndTime, 0).Add(time.Hour).Unix(),
		initialParams.MaxClaimsPerBlock+1,
	)

	paramChangeMsg := &claimtypes.MsgUpdateParams{
		Authority: s.app.GovKeeper.GetAuthority(),
		Params:    newParams,
	}

	proposal, err := s.app.GovKeeper.SubmitProposal(
		s.ctx,
		[]sdk.Msg{paramChangeMsg},
		"ipfs://CID",
		"Update claim parameters",
		"This proposal updates the claim module parameters",
		s.proposer,
		false,
	)
	s.Require().NoError(err, "Failed to submit proposal")

	depositAmount := govParams.MinDeposit
	err = s.app.BankKeeper.SendCoinsFromAccountToModule(s.ctx, s.proposer, govtypes.ModuleName, depositAmount)
	s.Require().NoError(err, "Failed to send deposit")

	votingPeriodStart, err := s.app.GovKeeper.AddDeposit(s.ctx, proposal.Id, s.proposer, depositAmount)
	s.Require().NoError(err, "Failed to add deposit")
	s.Require().True(votingPeriodStart, "Proposal should enter voting period")

	proposal, err = s.app.GovKeeper.Proposals.Get(s.ctx, proposal.Id)
	s.Require().NoError(err, "Failed to get proposal")
	s.Require().Equal(govv1.StatusVotingPeriod, proposal.Status, "Proposal should be in voting period")

	err = s.app.GovKeeper.AddVote(
		s.ctx,
		proposal.Id,
		s.voter,
		govv1.NewNonSplitVoteOption(govv1.OptionYes),
		"",
	)
	s.Require().NoError(err, "Failed to add vote")

	s.ctx = s.ctx.WithBlockTime(proposal.VotingEndTime.Add(time.Hour))
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)

	_, burnDeposits, _, err := s.app.GovKeeper.Tally(s.ctx, proposal)
	s.Require().NoError(err, "Failed to tally votes")

	proposal.Status = govv1.StatusPassed
	err = s.app.GovKeeper.SetProposal(s.ctx, proposal)
	s.Require().NoError(err, "Failed to set proposal status")

	proposal, err = s.app.GovKeeper.Proposals.Get(s.ctx, proposal.Id)
	s.Require().NoError(err, "Failed to get proposal after processing")

	if burnDeposits {
		err = s.app.GovKeeper.DeleteAndBurnDeposits(s.ctx, proposal.Id)
		s.Require().NoError(err, "Failed to burn deposits")
	} else {
		err = s.app.GovKeeper.RefundAndDeleteDeposits(s.ctx, proposal.Id)
		s.Require().NoError(err, "Failed to refund deposits")
	}

	s.Require().Equal(govv1.StatusPassed, proposal.Status, "Proposal should have passed")

	msg := proposal.Messages[0].GetCachedValue().(*claimtypes.MsgUpdateParams)
	s.app.ClaimKeeper.SetParams(s.ctx, msg.Params)
	s.Require().NoError(err, "Failed to update claim parameters")

	updatedParams := s.app.ClaimKeeper.GetParams(s.ctx)
	s.Require().Equal(msg.Params, updatedParams, "Parameters were not updated correctly")
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
