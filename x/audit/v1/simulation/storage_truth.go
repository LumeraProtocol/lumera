package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// SimulateMsgSubmitStorageRecheckEvidence is a no-op simulation for
// MsgSubmitStorageRecheckEvidence.
//
// Executing a valid recheck requires:
//   - A live epoch anchor for the chosen epoch_id.
//   - Both creator and challenged_supernode_account registered as active supernodes.
//   - No prior recheck submission for the same (epoch_id, ticket_id, creator) triple.
//
// These preconditions depend on runtime state that the simulation framework does
// not currently provide; a no-op is returned so the operation can be weighted and
// exercised via the ops registry without causing spurious failures.
func SimulateMsgSubmitStorageRecheckEvidence(
	ak types.AuthKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
	txGen client.TxConfig,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		return simtypes.NoOpMsg(
			types.ModuleName,
			sdk.MsgTypeURL(&types.MsgSubmitStorageRecheckEvidence{}),
			"recheck evidence requires a live epoch anchor and two registered supernodes",
		), nil, nil
	}
}

// SimulateMsgClaimHealComplete is a no-op simulation for MsgClaimHealComplete.
//
// Executing a valid claim requires a SCHEDULED heal op assigned to the creator.
// Heal ops are created deterministically at epoch end by the keeper; they are not
// available in the seed state used by simulations.
func SimulateMsgClaimHealComplete(
	ak types.AuthKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
	txGen client.TxConfig,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		return simtypes.NoOpMsg(
			types.ModuleName,
			sdk.MsgTypeURL(&types.MsgClaimHealComplete{}),
			"ClaimHealComplete requires a SCHEDULED heal op assigned to the caller",
		), nil, nil
	}
}

// SimulateMsgSubmitHealVerification is a no-op simulation for MsgSubmitHealVerification.
//
// Executing a valid verification requires a heal op in HEALER_REPORTED status with
// the caller listed as a verifier.  These conditions only exist after a ClaimHealComplete
// has been processed, which depends on a prior heal op being scheduled.
func SimulateMsgSubmitHealVerification(
	ak types.AuthKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
	txGen client.TxConfig,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		return simtypes.NoOpMsg(
			types.ModuleName,
			sdk.MsgTypeURL(&types.MsgSubmitHealVerification{}),
			"SubmitHealVerification requires a heal op in HEALER_REPORTED status",
		), nil, nil
	}
}
