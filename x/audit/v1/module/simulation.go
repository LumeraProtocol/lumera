package audit

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	auditsimulation "github.com/LumeraProtocol/lumera/x/audit/v1/simulation"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// GenerateGenesisState creates a randomized GenState of the module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	accs := make([]string, len(simState.Accounts))
	for i, acc := range simState.Accounts {
		accs[i] = acc.Address.String()
	}
	auditGenesis := types.GenesisState{
		Params: types.DefaultParams(),
	}
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&auditGenesis)
}

// RegisterStoreDecoder registers a decoder.
func (am AppModule) RegisterStoreDecoder(_ simtypes.StoreDecoderRegistry) {}

// WeightedOperations returns the all the gov module operations with their respective weights.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	operations := make([]simtypes.WeightedOperation, 0)
	const (
		opWeightMsgSubmitEvidence          = "op_weight_msg_audit"
		defaultWeightMsgSubmitEvidence int = 100
	)

	var weightMsgSubmitEvidence int
	simState.AppParams.GetOrGenerate(opWeightMsgSubmitEvidence, &weightMsgSubmitEvidence, nil,
		func(_ *rand.Rand) {
			weightMsgSubmitEvidence = defaultWeightMsgSubmitEvidence
		},
	)
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgSubmitEvidence,
		auditsimulation.SimulateMsgSubmitEvidence(am.authKeeper, am.bankKeeper, am.keeper, simState.TxConfig),
	))

	return operations
}

// ProposalMsgs returns msgs used for governance proposals for simulations.
func (am AppModule) ProposalMsgs(simState module.SimulationState) []simtypes.WeightedProposalMsg {
	return []simtypes.WeightedProposalMsg{}
}
