package simulation

import (
	"encoding/json"
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/cosmos/cosmos-sdk/x/simulation"
)

func SimulateMsgSubmitEvidence(
	ak types.AuthKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
	txGen client.TxConfig,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		subjectAccount, _ := simtypes.RandomAcc(r, accs)

		metadataBz, _ := json.Marshal(types.ExpirationEvidenceMetadata{
			ExpirationHeight: uint64(ctx.BlockHeight()),
			Reason:           "simulation",
		})

		msg := &types.MsgSubmitEvidence{
			Creator:        simAccount.Address.String(),
			SubjectAddress: subjectAccount.Address.String(),
			EvidenceType:   types.EvidenceTypeActionExpired,
			ActionId:       "sim-action-id",
			Metadata:       string(metadataBz),
		}

		return simulation.GenAndDeliverTxWithRandFees(simulation.OperationInput{
			R:             r,
			App:           app,
			TxGen:         txGen,
			Cdc:           (*codec.ProtoCodec)(nil),
			Msg:           msg,
			Context:       ctx,
			SimAccount:    simAccount,
			AccountKeeper: ak,
			Bankkeeper:    bk,
			ModuleName:    types.ModuleName,
		})
	}
}
