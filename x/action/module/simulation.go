package action

import (
	"math/rand"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	"github.com/LumeraProtocol/lumera/testutil/sample"
	actionsimulation "github.com/LumeraProtocol/lumera/x/action/simulation"
	"github.com/LumeraProtocol/lumera/x/action/types"
)

// avoid unused import issue
var (
	_ = actionsimulation.FindAccount
	_ = rand.Rand{}
	_ = sample.AccAddress
	_ = sdk.AccAddress{}
	_ = simulation.MsgEntryKind
)

const (
	opWeightMsgRequestAction = "op_weight_msg_request_action"
	// TODO: Determine the simulation weight value
	defaultWeightMsgRequestAction int = 100

	opWeightMsgFinalizeAction = "op_weight_msg_finalize_action"
	// TODO: Determine the simulation weight value
	defaultWeightMsgFinalizeAction int = 100

	opWeightMsgApproveAction = "op_weight_msg_approve_action"
	// TODO: Determine the simulation weight value
	defaultWeightMsgApproveAction int = 100

	// this line is used by starport scaffolding # simapp/module/const
)

// GenerateGenesisState creates a randomized GenState of the module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	accs := make([]string, len(simState.Accounts))
	for i, acc := range simState.Accounts {
		accs[i] = acc.Address.String()
	}
	actionGenesis := types.GenesisState{
		Params: types.DefaultParams(),
		// this line is used by starport scaffolding # simapp/module/genesisState
	}
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&actionGenesis)
}

// RegisterStoreDecoder registers a decoder.
func (am AppModule) RegisterStoreDecoder(_ simtypes.StoreDecoderRegistry) {}

// WeightedOperations returns the all the gov module operations with their respective weights.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	// Use the comprehensive weighted operations defined in the action simulation package
	return actionsimulation.WeightedOperations(
		simState.AppParams,
		simState.Cdc,
		am.accountKeeper,
		am.bankKeeper,
		am.keeper,
	)
}

// ProposalMsgs returns msgs used for governance proposals for simulations.
func (am AppModule) ProposalMsgs(simState module.SimulationState) []simtypes.WeightedProposalMsg {
	return []simtypes.WeightedProposalMsg{
		// None of this is possible with governance proposals
		/*		simulation.NewWeightedProposalMsg(
					opWeightMsgRequestAction,
					defaultWeightMsgRequestAction,
					func(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account) sdk.Msg {
						simAccount, _ := simtypes.RandomAcc(r, accs)
						msg := &types.MsgRequestAction{
							Creator:    simAccount.Address.String(),
						}
						return msg
					},
				),
				simulation.NewWeightedProposalMsg(
							opWeightMsgFinalizeAction,
							defaultWeightMsgFinalizeAction,
							func(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account) sdk.Msg {
								simAccount, _ := simtypes.RandomAcc(r, accs)
								msg := &types.MsgFinalizeAction{
									Creator: simAccount.Address.String(),
								}
								return msg
							},
						),
				simulation.NewWeightedProposalMsg(
					opWeightMsgApproveAction,
					defaultWeightMsgApproveAction,
					func(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account) sdk.Msg {
						simAccount, _ := simtypes.RandomAcc(r, accs)
						msg := &types.MsgApproveAction{
							Creator: simAccount.Address.String(),
						}
						return msg
					},
				),*/
		// this line is used by starport scaffolding # simapp/module/OpMsg
	}
}
