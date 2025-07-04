package supernode

import (
	"math/rand"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	"github.com/LumeraProtocol/lumera/testutil/cryptotestutils"
	supernodesimulation "github.com/LumeraProtocol/lumera/x/supernode/v1/simulation"
	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// avoid unused import issue
var (
	_ = supernodesimulation.FindAccount
	_ = rand.Rand{}
	_ = cryptotestutils.AccAddress
	_ = sdk.AccAddress{}
	_ = simulation.MsgEntryKind
)

const (
	opWeightMsgRegisterSupernode = "op_weight_msg_register_supernode"
	// TODO: Determine the simulation weight value
	defaultWeightMsgRegisterSupernode int = 100

	opWeightMsgDeregisterSupernode = "op_weight_msg_deregister_supernode"
	// TODO: Determine the simulation weight value
	defaultWeightMsgDeregisterSupernode int = 100

	opWeightMsgStartSupernode = "op_weight_msg_start_supernode"
	// TODO: Determine the simulation weight value
	defaultWeightMsgStartSupernode int = 100

	opWeightMsgStopSupernode = "op_weight_msg_stop_supernode"
	// TODO: Determine the simulation weight value
	defaultWeightMsgStopSupernode int = 100

	opWeightMsgUpdateSupernode = "op_weight_msg_update_supernode"
	// TODO: Determine the simulation weight value
	defaultWeightMsgUpdateSupernode int = 100

	// this line is used by starport scaffolding # simapp/module/const
)

// GenerateGenesisState creates a randomized GenState of the module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	accs := make([]string, len(simState.Accounts))
	for i, acc := range simState.Accounts {
		accs[i] = acc.Address.String()
	}
	supernodeGenesis := types.GenesisState{
		Params: types.DefaultParams(),
		// this line is used by starport scaffolding # simapp/module/genesisState
	}
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&supernodeGenesis)
}

// RegisterStoreDecoder registers a decoder.
func (am AppModule) RegisterStoreDecoder(_ simtypes.StoreDecoderRegistry) {}

// WeightedOperations returns the all the gov module operations with their respective weights.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	operations := make([]simtypes.WeightedOperation, 0)

	var weightMsgRegisterSupernode int
	simState.AppParams.GetOrGenerate(opWeightMsgRegisterSupernode, &weightMsgRegisterSupernode, nil,
		func(_ *rand.Rand) {
			weightMsgRegisterSupernode = defaultWeightMsgRegisterSupernode
		},
	)
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgRegisterSupernode,
		supernodesimulation.SimulateMsgRegisterSupernode(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgDeregisterSupernode int
	simState.AppParams.GetOrGenerate(opWeightMsgDeregisterSupernode, &weightMsgDeregisterSupernode, nil,
		func(_ *rand.Rand) {
			weightMsgDeregisterSupernode = defaultWeightMsgDeregisterSupernode
		},
	)
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgDeregisterSupernode,
		supernodesimulation.SimulateMsgDeregisterSupernode(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgStartSupernode int
	simState.AppParams.GetOrGenerate(opWeightMsgStartSupernode, &weightMsgStartSupernode, nil,
		func(_ *rand.Rand) {
			weightMsgStartSupernode = defaultWeightMsgStartSupernode
		},
	)
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgStartSupernode,
		supernodesimulation.SimulateMsgStartSupernode(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgStopSupernode int
	simState.AppParams.GetOrGenerate(opWeightMsgStopSupernode, &weightMsgStopSupernode, nil,
		func(_ *rand.Rand) {
			weightMsgStopSupernode = defaultWeightMsgStopSupernode
		},
	)
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgStopSupernode,
		supernodesimulation.SimulateMsgStopSupernode(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgUpdateSupernode int
	simState.AppParams.GetOrGenerate(opWeightMsgUpdateSupernode, &weightMsgUpdateSupernode, nil,
		func(_ *rand.Rand) {
			weightMsgUpdateSupernode = defaultWeightMsgUpdateSupernode
		},
	)
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgUpdateSupernode,
		supernodesimulation.SimulateMsgUpdateSupernode(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	// this line is used by starport scaffolding # simapp/module/operation

	return operations
}

// ProposalMsgs returns msgs used for governance proposals for simulations.
func (am AppModule) ProposalMsgs(simState module.SimulationState) []simtypes.WeightedProposalMsg {
	return []simtypes.WeightedProposalMsg{
		simulation.NewWeightedProposalMsg(
			opWeightMsgRegisterSupernode,
			defaultWeightMsgRegisterSupernode,
			func(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account) sdk.Msg {
				supernodesimulation.SimulateMsgRegisterSupernode(am.accountKeeper, am.bankKeeper, am.keeper)
				return nil
			},
		),
		simulation.NewWeightedProposalMsg(
			opWeightMsgDeregisterSupernode,
			defaultWeightMsgDeregisterSupernode,
			func(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account) sdk.Msg {
				supernodesimulation.SimulateMsgDeregisterSupernode(am.accountKeeper, am.bankKeeper, am.keeper)
				return nil
			},
		),
		simulation.NewWeightedProposalMsg(
			opWeightMsgStartSupernode,
			defaultWeightMsgStartSupernode,
			func(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account) sdk.Msg {
				supernodesimulation.SimulateMsgStartSupernode(am.accountKeeper, am.bankKeeper, am.keeper)
				return nil
			},
		),
		simulation.NewWeightedProposalMsg(
			opWeightMsgStopSupernode,
			defaultWeightMsgStopSupernode,
			func(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account) sdk.Msg {
				supernodesimulation.SimulateMsgStopSupernode(am.accountKeeper, am.bankKeeper, am.keeper)
				return nil
			},
		),
		simulation.NewWeightedProposalMsg(
			opWeightMsgUpdateSupernode,
			defaultWeightMsgUpdateSupernode,
			func(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account) sdk.Msg {
				supernodesimulation.SimulateMsgUpdateSupernode(am.accountKeeper, am.bankKeeper, am.keeper)
				return nil
			},
		),
		// this line is used by starport scaffolding # simapp/module/OpMsg
	}
}
