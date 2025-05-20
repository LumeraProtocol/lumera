package keeper

import (
	"context"
	errorsmod "cosmossdk.io/errors"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	"time"

	"github.com/LumeraProtocol/lumera/x/claim/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k msgServer) DelayedClaim(goCtx context.Context, msg *types.MsgDelayedClaim) (*types.MsgDelayedClaimResponse, error) {

	msgClaim := types.MsgClaim{
		OldAddress: msg.OldAddress,
		NewAddress: msg.NewAddress,
		PubKey:     msg.PubKey,
		Signature:  msg.Signature,
	}

	delayMonth := int(msg.Tier * 6)
	_, err := k.processClaim(goCtx, &msgClaim, k.CreateDelayedAccount, types.EventTypeDelayedClaimProcessed, delayMonth)
	if err != nil {
		return nil, err
	}

	return &types.MsgDelayedClaimResponse{}, nil
}

func (k msgServer) CreateDelayedAccount(ctx context.Context, blockTime time.Time, destAddr sdk.AccAddress, balance sdk.Coins, delayMonth int) (int64, error) {
	acc := k.accountKeeper.GetAccount(ctx, destAddr)
	if acc != nil {
		return -1, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "can't create delayed account - account already exists")
	}

	// endTime is the current time plus X months
	endTime := blockTime.AddDate(0, delayMonth, 0).Unix()

	baseAccount := authtypes.NewBaseAccountWithAddress(destAddr)
	baseAccount = k.accountKeeper.NewAccount(ctx, baseAccount).(*authtypes.BaseAccount)
	baseVestingAccount, err := vestingypes.NewBaseVestingAccount(baseAccount, balance, endTime)
	if err != nil {
		return -1, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, err.Error())
	}

	vestingAccount := vestingypes.NewDelayedVestingAccountRaw(baseVestingAccount)
	k.accountKeeper.SetAccount(ctx, vestingAccount)
	return endTime, nil
}
