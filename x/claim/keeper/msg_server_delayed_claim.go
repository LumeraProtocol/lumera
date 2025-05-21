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

func (k msgServer) CreateDelayedAccount(ctx sdk.Context, blockTime time.Time, destAddr sdk.AccAddress, balance sdk.Coins, delayMonth int) (int64, error) {

	// 1. Determine the account that will become the vesting account --------------------------------
	acc := k.accountKeeper.GetAccount(ctx, destAddr)

	// If it already exists it **must** be a plain BaseAccount (e.g. the stub created in ante).
	var baseAccount *authtypes.BaseAccount
	switch a := acc.(type) {
	case nil:
		// No account yet – create a new BaseAccount
		baseAccount = authtypes.NewBaseAccountWithAddress(destAddr)
		baseAccount = k.accountKeeper.NewAccount(ctx, baseAccount).(*authtypes.BaseAccount)

	case *authtypes.BaseAccount:
		// Stub (or manually created) account – we can safely convert it
		baseAccount = a

	default:
		// Any other concrete type (vesting, module, contract, …) is disallowed
		return -1, errorsmod.Wrap(sdkerrors.ErrInvalidRequest,
			"destination address already has a non-base account")
	}

	// 2. Build the vesting account -----------------------------------------------------------------
	// endTime is the current time plus X months
	endTime := blockTime.AddDate(0, delayMonth, 0).Unix()

	baseVestingAccount, err := vestingypes.NewBaseVestingAccount(baseAccount, balance, endTime)
	if err != nil {
		return -1, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, err.Error())
	}

	vestingAccount := vestingypes.NewDelayedVestingAccountRaw(baseVestingAccount)

	// 3. Store (overwriting the stub if it existed) ------------------------------------------------
	k.accountKeeper.SetAccount(ctx, vestingAccount)

	return endTime, nil
}
