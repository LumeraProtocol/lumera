package simulation

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/cometbft/cometbft/crypto/secp256k1"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/pastelnetwork/pastel/x/pastelid/keeper"
	"github.com/pastelnetwork/pastel/x/pastelid/types"
)

func SimulateMsgCreatePastelId(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)

		params := k.GetParams(ctx)
		requiredCoins := sdk.NewCoins(params.PastelIdCreateFee)
		currentBalance := bk.SpendableCoins(ctx, simAccount.Address)

		if !currentBalance.IsAllGTE(requiredCoins) {
			mintCoins := requiredCoins
			err := bk.MintCoins(ctx, types.ModuleName, mintCoins)
			if err != nil {
				return simtypes.NoOpMsg(types.ModuleName, types.TypeMsgCreatePastelId, "failed to mint coins"), nil, err
			}

			err = bk.SendCoinsFromModuleToAccount(ctx, types.ModuleName, simAccount.Address, mintCoins)
			if err != nil {
				fmt.Println("error sending coins from module to account", err.Error())
				return simtypes.NoOpMsg(types.ModuleName, types.TypeMsgCreatePastelId, "failed to transfer minted coins to account"), nil, err
			}
		}

		// Generate a valid PastelID and other fields
		randomPastelId := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
		randomPqKey := simtypes.RandStringOfLength(r, 64)
		randomSignature := simtypes.RandStringOfLength(r, 64)
		randomTimestamp := time.Now().Format(time.RFC3339)

		// Create message
		msg := types.NewMsgCreatePastelId(
			simAccount.Address.String(),
			"simulated-type",
			randomPastelId,
			randomPqKey,
			randomSignature,
			randomTimestamp,
			1,
		)

		spendable := bk.SpendableCoins(ctx, simAccount.Address)
		if !spendable.IsAllGTE(sdk.NewCoins(params.PastelIdCreateFee)) {
			fmt.Println("error creating pastelid", types.ErrInsufficientFunds.Error())
			return simtypes.NoOpMsg(types.ModuleName, types.TypeMsgCreatePastelId, types.ErrInsufficientFunds.Error()), nil, nil
		}

		// Validate PastelID uniqueness
		if k.HasPastelidEntry(ctx, simAccount.Address.String()) {
			fmt.Println("error creating pastelid", types.ErrPastelIDExists.Error())
			return simtypes.NoOpMsg(types.ModuleName, types.TypeMsgCreatePastelId, types.ErrPastelIDExists.Error()), nil, nil
		}

		// Execute the message
		msgServer := keeper.NewMsgServerImpl(k)
		_, err := msgServer.CreatePastelId(sdk.WrapSDKContext(ctx), msg)
		if err != nil {
			fmt.Println("error creating pastelid", err.Error())
			return simtypes.NoOpMsg(types.ModuleName, types.TypeMsgCreatePastelId, err.Error()), nil, err
		}

		return simtypes.NewOperationMsg(msg, true, "successfully created PastelID"), nil, nil
	}
}
