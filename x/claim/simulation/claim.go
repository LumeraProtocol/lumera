package simulation

import (
	"fmt"
	"math/rand"

	"encoding/hex"

	claimcrypto "github.com/LumeraProtocol/lumera/x/claim/keeper/crypto"
	"github.com/LumeraProtocol/lumera/x/claim/keeper"
	"github.com/LumeraProtocol/lumera/x/claim/types"
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
		privKey, pubKey := claimcrypto.GenerateKeyPair()
		pubKeyHex := hex.EncodeToString(pubKey.Key)
		oldAddress, err := claimcrypto.GetAddressFromPubKey(pubKeyHex)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgClaim, "failed to generate claim address"), nil, err
		}

		testAmount := int64(1_000_000) // Amount to be claimed in the test case

		// Create claim record if it doesn't exist
		_, found, err := k.GetClaimRecord(ctx, oldAddress)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgClaim, "failed to get claim record"), nil, err
		}

		if !found {
			claimRecord := types.ClaimRecord{
				OldAddress: oldAddress,
				Balance:    sdk.NewCoins(sdk.NewInt64Coin(types.DefaultClaimsDenom, testAmount)),
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
		signature, err := claimcrypto.SignMessage(privKey, message)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgClaim, "failed to sign message"), nil, err
		}

		msg := types.MsgClaim{
			OldAddress: oldAddress,
			NewAddress: simAccount.Address.String(),
			PubKey:     pubKeyHex,
			Signature:  signature,
		}

		// Check if we've hit the claims per block limit
		claimsInBlock, err := k.GetBlockClaimCount(ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgClaim, "failed to get block claim count"), nil, err
		}
		if claimsInBlock >= params.MaxClaimsPerBlock {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgClaim, "max claims per block reached"), nil, nil
		}

		// Execute the message
		msgServer := keeper.NewMsgServerImpl(k)
		_, err = msgServer.Claim(ctx, &msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, TypeMsgClaim, err.Error()), nil, err
		}

		return simtypes.NewOperationMsg(&msg, true, "success"), nil, nil
	}
}
