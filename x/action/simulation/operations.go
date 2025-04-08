package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/codec"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	"github.com/LumeraProtocol/lumera/x/action/keeper"
	"github.com/LumeraProtocol/lumera/x/action/types"
)

// WeightedOperations returns all the operations from the module with their respective weights
func WeightedOperations(
	appParams simtypes.AppParams,
	cdc codec.JSONCodec,
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simulation.WeightedOperations {
	var (
		weightMsgRequestAction int
		//weightMsgFinalizeAction   int
		//weightMsgApproveAction    int
		//weightActionExpirationSim int
		//weightActionFailureSim    int
		//weightProcessingStateSim  int
		//weightFeeDistributionSim  int
	)

	appParams.GetOrGenerate(
		"action",
		&weightMsgRequestAction,
		nil,
		func(r *rand.Rand) { weightMsgRequestAction = 100 },
	)

	//appParams.GetOrGenerate(
	//	"action",
	//	&weightMsgFinalizeAction,
	//	nil,
	//	func(r *rand.Rand) { weightMsgFinalizeAction = 50 },
	//)
	//
	//appParams.GetOrGenerate(
	//	"action",
	//	&weightMsgApproveAction,
	//	nil,
	//	func(r *rand.Rand) { weightMsgApproveAction = 50 },
	//)
	//
	//appParams.GetOrGenerate(
	//	"action",
	//	&weightActionExpirationSim,
	//	nil,
	//	func(r *rand.Rand) { weightActionExpirationSim = 25 }, // Lower weight for state transition simulation
	//)
	//
	//appParams.GetOrGenerate(
	//	"action",
	//	&weightActionFailureSim,
	//	nil,
	//	func(r *rand.Rand) { weightActionFailureSim = 25 }, // Equal weight for action failure simulation
	//)
	//
	//appParams.GetOrGenerate(
	//	"action",
	//	&weightProcessingStateSim,
	//	nil,
	//	func(r *rand.Rand) { weightProcessingStateSim = 25 }, // Equal weight for processing state simulation
	//)
	//
	//appParams.GetOrGenerate(
	//	"action",
	//	&weightFeeDistributionSim,
	//	nil,
	//	func(r *rand.Rand) { weightFeeDistributionSim = 25 }, // Equal weight for fee distribution simulation
	//)

	operations := simulation.WeightedOperations{
		simulation.NewWeightedOperation(
			weightMsgRequestAction,
			SimulateMsgRequestActionSuccessSense(ak, bk, k),
		),
		simulation.NewWeightedOperation(
			weightMsgRequestAction, // Using the same weight for CASCADE simulation
			SimulateMsgRequestActionSuccessCascade(ak, bk, k),
		),
		simulation.NewWeightedOperation(
			weightMsgRequestAction, // Using the same weight for Invalid Metadata simulation
			SimulateMsgRequestActionInvalidMetadata(ak, bk, k),
		),
		simulation.NewWeightedOperation(
			weightMsgRequestAction, // Using the same weight for Insufficient Funds simulation
			SimulateMsgRequestActionInsufficientFunds(ak, bk, k),
		),
		simulation.NewWeightedOperation(
			weightMsgRequestAction, // Using the same weight for Permission simulation
			SimulateMsgRequestActionPermission(ak, bk, k),
		),
		//simulation.NewWeightedOperation(
		//	weightMsgFinalizeAction,
		//	SimulateMsgFinalizeAction_Success_Sense(ak, bk, k),
		//),
		//simulation.NewWeightedOperation(
		//	weightMsgFinalizeAction, // Using the same weight for CASCADE finalization
		//	SimulateMsgFinalizeAction_Success_Cascade(ak, bk, k),
		//),
		//simulation.NewWeightedOperation(
		//	weightMsgFinalizeAction, // Using the same weight for Invalid ID finalization
		//	SimulateMsgFinalizeAction_Invalid_ID(ak, bk, k),
		//),
		//simulation.NewWeightedOperation(
		//	weightMsgFinalizeAction, // Using the same weight for Invalid State finalization
		//	SimulateMsgFinalizeAction_InvalidState(ak, bk, k),
		//),
		//simulation.NewWeightedOperation(
		//	weightMsgFinalizeAction, // Using the same weight for Unauthorized finalization
		//	SimulateMsgFinalizeAction_Unauthorized(ak, bk, k),
		//),
		//simulation.NewWeightedOperation(
		//	weightMsgFinalizeAction, // Using the same weight for SenseConsensus simulation
		//	SimulateMsgFinalizeAction_SenseConsensus(ak, bk, k),
		//),
		//simulation.NewWeightedOperation(
		//	weightMsgFinalizeAction, // Using the same weight for MetadataValidation simulation
		//	SimulateMsgFinalizeAction_MetadataValidation(ak, bk, k),
		//),
		//simulation.NewWeightedOperation(
		//	weightMsgApproveAction,
		//	SimulateMsgApproveAction_Success(ak, bk, k),
		//),
		//simulation.NewWeightedOperation(
		//	weightMsgApproveAction, // Using the same weight for Invalid ID approval simulation
		//	SimulateMsgApproveAction_Invalid_ID(ak, bk, k),
		//),
		//simulation.NewWeightedOperation(
		//	weightMsgApproveAction, // Using the same weight for Invalid State approval simulation
		//	SimulateMsgApproveAction_InvalidState(ak, bk, k),
		//),
		//simulation.NewWeightedOperation(
		//	weightMsgApproveAction, // Using the same weight for Unauthorized approval simulation
		//	SimulateMsgApproveAction_Unauthorized(ak, bk, k),
		//),
		//simulation.NewWeightedOperation(
		//	weightMsgApproveAction, // Using the same weight for SignatureValidation simulation
		//	SimulateMsgApproveAction_SignatureValidation(ak, bk, k),
		//),
		//simulation.NewWeightedOperation(
		//	weightActionExpirationSim, // Lower weight for state transition simulation
		//	SimulateActionExpiration(ak, bk, k),
		//),
		//simulation.NewWeightedOperation(
		//	weightActionFailureSim, // Equal weight for action failure simulation
		//	SimulateActionFailure(ak, bk, k),
		//),
		//simulation.NewWeightedOperation(
		//	weightProcessingStateSim, // Equal weight for processing state simulation
		//	SimulateProcessingState(ak, bk, k),
		//),
		//simulation.NewWeightedOperation(
		//	weightFeeDistributionSim, // Equal weight for fee distribution simulation
		//	SimulateFeeDistribution_Success(ak, bk, k),
		//),
		//simulation.NewWeightedOperation(
		//	weightFeeDistributionSim, // Equal weight for multiple supernodes fee distribution simulation
		//	SimulateFeeDistribution_MultipleSuperNodes(ak, bk, k),
		//),
	}

	return operations
}
