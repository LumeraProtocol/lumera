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
		weightMsgRequestAction    int
		weightMsgFinalizeAction   int
		weightMsgApproveAction    int
		weightActionExpirationSim int
	)

	appParams.GetOrGenerate(
		"action",
		&weightMsgRequestAction,
		nil,
		func(r *rand.Rand) { weightMsgRequestAction = 100 },
	)

	appParams.GetOrGenerate(
		"action",
		&weightMsgFinalizeAction,
		nil,
		func(r *rand.Rand) { weightMsgFinalizeAction = 50 },
	)

	appParams.GetOrGenerate(
		"action",
		&weightMsgApproveAction,
		nil,
		func(r *rand.Rand) { weightMsgApproveAction = 50 },
	)

	appParams.GetOrGenerate(
		"action",
		&weightActionExpirationSim,
		nil,
		func(r *rand.Rand) { weightActionExpirationSim = 25 }, // Lower weight for state transition simulation
	)

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
		simulation.NewWeightedOperation(
			weightMsgFinalizeAction,
			SimulateMsgFinalizeActionSuccessSense(ak, bk, k),
		),
		simulation.NewWeightedOperation(
			weightMsgFinalizeAction, // Using the same weight for CASCADE finalization
			SimulateMsgFinalizeActionSuccessCascade(ak, bk, k),
		),
		simulation.NewWeightedOperation(
			weightMsgFinalizeAction, // Using the same weight for Invalid ID finalization
			SimulateMsgFinalizeActionInvalidID(ak, bk, k),
		),
		simulation.NewWeightedOperation(
			weightMsgFinalizeAction, // Using the same weight for Invalid State finalization
			SimulateMsgFinalizeActionInvalidState(ak, bk, k),
		),
		simulation.NewWeightedOperation(
			weightMsgFinalizeAction, // Using the same weight for Unauthorized finalization
			SimulateMsgFinalizeActionUnauthorized(ak, bk, k),
		),
		simulation.NewWeightedOperation(
			weightMsgFinalizeAction, // Using the same weight for SenseConsensus simulation
			SimulateMsgFinalizeActionSenseConsensus(ak, bk, k),
		),
		simulation.NewWeightedOperation(
			weightMsgFinalizeAction, // Using the same weight for MetadataValidation simulation
			SimulateMsgFinalizeActionMetadataValidation(ak, bk, k),
		),
		simulation.NewWeightedOperation(
			weightMsgApproveAction,
			SimulateMsgApproveActionSuccess(ak, bk, k),
		),
		simulation.NewWeightedOperation(
			weightMsgApproveAction, // Using the same weight for Invalid ID approval simulation
			SimulateMsgApproveActionInvalidID(ak, bk, k),
		),
		simulation.NewWeightedOperation(
			weightMsgApproveAction, // Using the same weight for Invalid State approval simulation
			SimulateMsgApproveActionInvalidState(ak, bk, k),
		),
		simulation.NewWeightedOperation(
			weightMsgApproveAction, // Using the same weight for Unauthorized approval simulation
			SimulateMsgApproveActionUnauthorized(ak, bk, k),
		),
		simulation.NewWeightedOperation(
			weightActionExpirationSim, // Lower weight for state transition simulation
			SimulateActionExpiration(ak, bk, k),
		),
	}

	return operations
}
