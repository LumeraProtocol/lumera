package audit

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/x/simulation"

	auditsimulation "github.com/LumeraProtocol/lumera/x/audit/v1/simulation"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

// GenerateGenesisState creates a default GenState of the module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(types.DefaultGenesis())
}

// RegisterStoreDecoder registers a decoder.
func (AppModule) RegisterStoreDecoder(_ simtypes.StoreDecoderRegistry) {}

// WeightedOperations returns the simulation operations for the module.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	operations := make([]simtypes.WeightedOperation, 0)

	const (
		opWeightMsgSubmitEvidence          = "op_weight_msg_submit_evidence"
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
