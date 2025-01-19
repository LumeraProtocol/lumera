package simulation

import (
	"fmt"
	"math/rand"

	"encoding/hex"

	"cosmossdk.io/math"
	"github.com/LumeraProtocol/lumera/x/claim/keeper"
	"github.com/LumeraProtocol/lumera/x/claim/types"
	"github.com/cometbft/cometbft/crypto/secp256k1"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

const (
	TypeMsgClaim = "claim"
)

func SimulateMsgClaim(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)

		// Skip if claims are disabled
		params := k.GetParams(ctx)
		if !params.EnableClaims {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgClaim, "claims disabled"), nil, nil
		}

		// Generate simulated keys and addresses
		privKey := secp256k1.GenPrivKey()
		pubKeyBytes := privKey.PubKey().Bytes()
		pubKeyHex := hex.EncodeToString(pubKeyBytes)
		oldAddress := sdk.AccAddress(privKey.PubKey().Address()).String()

		// Create claim record if it doesn't exist
		_, found, err := k.GetClaimRecord(ctx, oldAddress)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgClaim, "failed to get claim record"), nil, err
		}

		if !found {
			claimRecord := types.ClaimRecord{
				OldAddress: oldAddress,
				Balance:    sdk.NewCoins(sdk.NewCoin(types.DefaultClaimsDenom, math.NewInt(1000000))),
				Claimed:    false,
			}
			err = k.SetClaimRecord(ctx, claimRecord)
			if err != nil {
				return simtypes.NoOpMsg(types.ModuleName, TypeMsgClaim, "failed to set claim record"), nil, err
			}

			// Mint coins to module account
			err = bk.MintCoins(ctx, types.ModuleName, claimRecord.Balance)
			if err != nil {
				return simtypes.NoOpMsg(types.ModuleName, TypeMsgClaim, "failed to mint coins"), nil, err
			}
		}

		// Generate message signature
		message := fmt.Sprintf("%s.%s.%s", oldAddress, pubKeyHex, simAccount.Address.String())
		signature, err := privKey.Sign([]byte(message))
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgClaim, "failed to sign message"), nil, err
		}

		msg := types.MsgClaim{
			OldAddress: oldAddress,
			NewAddress: simAccount.Address.String(),
			PubKey:     pubKeyHex,
			Signature:  hex.EncodeToString(signature),
		}

		// Check if we've hit the claims per block limit
		claimsInBlock := k.GetBlockClaimCount(ctx)
		if claimsInBlock >= params.MaxClaimsPerBlock {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgClaim, "max claims per block reached"), nil, nil
		}

		// Execute the message
		msgServer := keeper.NewMsgServerImpl(k)
		_, err = msgServer.Claim(sdk.WrapSDKContext(ctx), &msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgClaim, err.Error()), nil, err
		}

		return simtypes.NewOperationMsg(&msg, true, "success"), nil, nil
	}
}
