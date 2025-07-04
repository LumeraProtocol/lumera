package wasm_test

import (
	"os"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	wasmvmtypes "github.com/CosmWasm/wasmvm/v3/types"

	"github.com/LumeraProtocol/lumera/app"
	"github.com/LumeraProtocol/lumera/tests/ibctesting"
	wasmtest "github.com/LumeraProtocol/lumera/tests/system/wasm"
)

func TestGovVoteByContract(t *testing.T) {
	os.Setenv("SYSTEM_TESTS", "true") // Set SYSTEM_TESTS for this test

	coord := ibctesting.NewCoordinator(t, 1) // One chain setup
	chain := coord.GetChain(ibctesting.GetChainID(1))

	// Instantiate the contract and fund it
	contractAddr := wasmtest.InstantiateReflectContract(t, chain)
	chain.Fund(contractAddr, sdkmath.NewIntFromUint64(1_000_000_000))

	// Delegate a high amount to the contract
	delegateMsg := wasmvmtypes.CosmosMsg{
		Staking: &wasmvmtypes.StakingMsg{
			Delegate: &wasmvmtypes.DelegateMsg{
				Validator: sdk.ValAddress(chain.Vals.Validators[0].Address).String(),
				Amount: wasmvmtypes.Coin{
					Denom:  sdk.DefaultBondDenom,
					Amount: "1000000000",
				},
			},
		},
	}
	wasmtest.MustExecViaReflectContract(t, chain, contractAddr, delegateMsg)

	app := chain.App.(*app.App)

	// Verify the community pool balance
	communityPoolBalance := chain.Balance(app.AuthKeeper.GetModuleAccount(chain.GetContext(), distributiontypes.ModuleName).GetAddress(), sdk.DefaultBondDenom)
	require.False(t, communityPoolBalance.IsZero())

	// Get governance parameters and setup
	gParams, err := app.GovKeeper.Params.Get(chain.GetContext())
	require.NoError(t, err)
	initialDeposit := gParams.MinDeposit
	govAcctAddr := app.GovKeeper.GetGovernanceAccount(chain.GetContext()).GetAddress()

	// Define test cases
	specs := map[string]struct {
		vote    *wasmvmtypes.VoteMsg
		expPass bool
	}{
		"yes": {
			vote: &wasmvmtypes.VoteMsg{
				Option: wasmvmtypes.Yes,
			},
			expPass: true,
		},
		"no": {
			vote: &wasmvmtypes.VoteMsg{
				Option: wasmvmtypes.No,
			},
			expPass: false,
		},
		"abstain": {
			vote: &wasmvmtypes.VoteMsg{
				Option: wasmvmtypes.Abstain,
			},
			expPass: true,
		},
		"no with veto": {
			vote: &wasmvmtypes.VoteMsg{
				Option: wasmvmtypes.NoWithVeto,
			},
			expPass: false,
		},
	}

	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			// Create a unique recipient address
			recipientAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address().Bytes())

			// Submit a new proposal
			payloadMsg := &distributiontypes.MsgCommunityPoolSpend{
				Authority: govAcctAddr.String(),
				Recipient: recipientAddr.String(),
				Amount:    sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.OneInt())),
			}
			msg, err := v1.NewMsgSubmitProposal(
				[]sdk.Msg{payloadMsg},
				initialDeposit,
				chain.SenderAccount.GetAddress().String(),
				"",
				"my proposal",
				"testing",
				false,
			)
			require.NoError(t, err)
			rsp, gotErr := chain.SendMsgs(msg)
			require.NoError(t, gotErr)
			var got v1.MsgSubmitProposalResponse
			chain.UnwrapExecTXResult(rsp, &got)

			propID := got.ProposalId

			// Submit a vote by another delegator
			_, err = chain.SendMsgs(v1.NewMsgVote(chain.SenderAccount.GetAddress(), propID, v1.VoteOption_VOTE_OPTION_YES, ""))
			require.NoError(t, err)

			// Submit a vote by the contract
			spec.vote.ProposalId = propID
			voteMsg := wasmvmtypes.CosmosMsg{
				Gov: &wasmvmtypes.GovMsg{
					Vote: spec.vote,
				},
			}
			wasmtest.MustExecViaReflectContract(t, chain, contractAddr, voteMsg)

			// Validate the proposal execution after the voting period
			proposal, err := app.GovKeeper.Proposals.Get(chain.GetContext(), propID)
			require.NoError(t, err)
			coord.IncrementTimeBy(proposal.VotingEndTime.Sub(chain.GetContext().BlockTime()) + time.Minute)
			coord.CommitBlock(chain)

			// Validate recipient balance updates
			recipientBalance := chain.Balance(recipientAddr, sdk.DefaultBondDenom)
			if !spec.expPass {
				assert.True(t, recipientBalance.IsZero())
				return
			}
			expBalanceAmount := sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.OneInt())
			assert.Equal(t, expBalanceAmount.String(), recipientBalance.String())
		})
	}
}
