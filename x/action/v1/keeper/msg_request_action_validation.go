package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"github.com/LumeraProtocol/lumera/x/action/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	icatypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/types"
)

var creatorAccountCtxKey = struct{}{}

type creatorAccountInfo struct {
	account   sdk.AccountI
	isICA     bool
	owner     string // ICA account_owner; empty otherwise
	appPubkey []byte
}

func (k Keeper) getCreatorAccountInfo(ctx context.Context, msg *types.MsgRequestAction) (*creatorAccountInfo, error) {
	creatorAddrBz, err := k.addressCodec.StringToBytes(msg.Creator)
	if err != nil {
		return nil, errorsmod.Wrapf(types.ErrInvalidAddress, "invalid creator address: %s", err)
	}

	creatorAcct := k.authKeeper.GetAccount(ctx, sdk.AccAddress(creatorAddrBz))
	if creatorAcct == nil {
		return nil, errorsmod.Wrapf(types.ErrInvalidAddress, "creator account not found: %s", msg.Creator)
	}

	ica, creatorIsICA := creatorAcct.(*icatypes.InterchainAccount)
	info := &creatorAccountInfo{
		account:   creatorAcct,
		isICA:     creatorIsICA,
		appPubkey: msg.AppPubkey,
	}
	if creatorIsICA {
		info.owner = ica.AccountOwner
	}

	return info, nil
}

func (info *creatorAccountInfo) validateAppPubKey() error {
	// ICA accounts have no auth module pubkey and can't sign metadata using the account key.
	// When the creator is an ICA account, callers must provide an application-level pubkey.
	// For non-ICA creators, app_pubkey must be empty to avoid ambiguity about the signing scheme.
	if info.isICA {
		if len(info.appPubkey) == 0 {
			return errorsmod.Wrap(types.ErrInvalidAppPubKey, "app_pubkey is required for interchain account creators")
		}
	} else {
		if len(info.appPubkey) != 0 {
			return errorsmod.Wrap(types.ErrInvalidAppPubKey, "app_pubkey must be empty for non-interchain account creators")
		}
	}

	return nil
}
