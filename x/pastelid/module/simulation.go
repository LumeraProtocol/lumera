package pastelid

import (
	"math/rand"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	"github.com/pastelnetwork/pastel/testutil/sample"
	pastelidsimulation "github.com/pastelnetwork/pastel/x/pastelid/simulation"
	"github.com/pastelnetwork/pastel/x/pastelid/types"
)

// avoid unused import issue
var (
	_ = pastelidsimulation.FindAccount
	_ = rand.Rand{}
	_ = sample.AccAddress
	_ = sdk.AccAddress{}
	_ = simulation.MsgEntryKind
)

const (
	opWeightMsgCreatePastelId = "op_weight_msg_create_pastel_id"
	// TODO: Determine the simulation weight value
	defaultWeightMsgCreatePastelId int = 50

	// this line is used by starport scaffolding # simapp/module/const
)

// GenerateGenesisState creates a randomized GenState of the module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	accs := make([]string, len(simState.Accounts))
	for i, acc := range simState.Accounts {
		accs[i] = acc.Address.String()
	}
	pastelidGenesis := types.GenesisState{
		Params: types.DefaultParams(),
		// this line is used by starport scaffolding # simapp/module/genesisState
	}
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&pastelidGenesis)
}

// RegisterStoreDecoder registers a decoder.
func (am AppModule) RegisterStoreDecoder(_ simtypes.StoreDecoderRegistry) {}

// WeightedOperations returns the all the gov module operations with their respective weights.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	operations := make([]simtypes.WeightedOperation, 0)

	var weightMsgCreatePastelId int
	simState.AppParams.GetOrGenerate(opWeightMsgCreatePastelId, &weightMsgCreatePastelId, nil,
		func(_ *rand.Rand) {
			weightMsgCreatePastelId = defaultWeightMsgCreatePastelId
		},
	)
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgCreatePastelId,
		pastelidsimulation.SimulateMsgCreatePastelId(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	// this line is used by starport scaffolding # simapp/module/operation

	return operations
}

// ProposalMsgs returns msgs used for governance proposals for simulations.
func (am AppModule) ProposalMsgs(simState module.SimulationState) []simtypes.WeightedProposalMsg {
	return []simtypes.WeightedProposalMsg{
		simulation.NewWeightedProposalMsg(
			opWeightMsgCreatePastelId,
			defaultWeightMsgCreatePastelId,
			func(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account) sdk.Msg {
				pastelidsimulation.SimulateMsgCreatePastelId(am.accountKeeper, am.bankKeeper, am.keeper)
				return nil
			},
		),
		// this line is used by starport scaffolding # simapp/module/OpMsg
	}
}
