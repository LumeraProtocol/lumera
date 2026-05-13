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
		opWeightMsgSubmitEvidence                   = "op_weight_msg_submit_evidence"
		defaultWeightMsgSubmitEvidence          int = 100
		opWeightMsgSubmitStorageRecheckEvidence     = "op_weight_msg_submit_storage_recheck_evidence"
		defaultWeightMsgSubmitStorageRecheck    int = 20
		opWeightMsgClaimHealComplete                = "op_weight_msg_claim_heal_complete"
		defaultWeightMsgClaimHealComplete       int = 15
		opWeightMsgSubmitHealVerification           = "op_weight_msg_submit_heal_verification"
		defaultWeightMsgSubmitHealVerification  int = 15
	)

	var weightMsgSubmitEvidence int
	simState.AppParams.GetOrGenerate(opWeightMsgSubmitEvidence, &weightMsgSubmitEvidence, nil,
		func(_ *rand.Rand) { weightMsgSubmitEvidence = defaultWeightMsgSubmitEvidence },
	)

	var weightMsgStorageRecheck int
	simState.AppParams.GetOrGenerate(opWeightMsgSubmitStorageRecheckEvidence, &weightMsgStorageRecheck, nil,
		func(_ *rand.Rand) { weightMsgStorageRecheck = defaultWeightMsgSubmitStorageRecheck },
	)

	var weightMsgClaimHeal int
	simState.AppParams.GetOrGenerate(opWeightMsgClaimHealComplete, &weightMsgClaimHeal, nil,
		func(_ *rand.Rand) { weightMsgClaimHeal = defaultWeightMsgClaimHealComplete },
	)

	var weightMsgHealVerification int
	simState.AppParams.GetOrGenerate(opWeightMsgSubmitHealVerification, &weightMsgHealVerification, nil,
		func(_ *rand.Rand) { weightMsgHealVerification = defaultWeightMsgSubmitHealVerification },
	)

	operations = append(operations,
		simulation.NewWeightedOperation(
			weightMsgSubmitEvidence,
			auditsimulation.SimulateMsgSubmitEvidence(am.authKeeper, am.bankKeeper, am.keeper, simState.TxConfig),
		),
		simulation.NewWeightedOperation(
			weightMsgStorageRecheck,
			auditsimulation.SimulateMsgSubmitStorageRecheckEvidence(am.authKeeper, am.bankKeeper, am.keeper, simState.TxConfig),
		),
		simulation.NewWeightedOperation(
			weightMsgClaimHeal,
			auditsimulation.SimulateMsgClaimHealComplete(am.authKeeper, am.bankKeeper, am.keeper, simState.TxConfig),
		),
		simulation.NewWeightedOperation(
			weightMsgHealVerification,
			auditsimulation.SimulateMsgSubmitHealVerification(am.authKeeper, am.bankKeeper, am.keeper, simState.TxConfig),
		),
	)

	return operations
}

// ProposalMsgs returns msgs used for governance proposals for simulations.
func (am AppModule) ProposalMsgs(simState module.SimulationState) []simtypes.WeightedProposalMsg {
	return []simtypes.WeightedProposalMsg{}
}
