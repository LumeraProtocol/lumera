package keeper

import (
	"context"
	"bytes"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/action/v1/types"
)

func (k msgServer) UpdateParams(goCtx context.Context, req *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	authorityBytes, err := k.addressCodec.StringToBytes(req.Authority)
	if err != nil {
		return nil, errorsmod.Wrapf(types.ErrInvalidSigner, "invalid authority address; %s", req.Authority)
	}

	if !bytes.Equal(authorityBytes, k.GetAuthority()) {
		expectedAuthority, err := k.addressCodec.BytesToString(k.GetAuthority())
		if err != nil {
			return nil, errorsmod.Wrapf(types.ErrInvalidSigner, "unable to decode expected authority")
		}

		return nil, errorsmod.Wrapf(types.ErrInvalidSigner, "invalid authority; expected %s, got %s", 
			expectedAuthority, req.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if err := k.SetParams(ctx, req.Params); err != nil {
		return nil, err
	}

	return &types.MsgUpdateParamsResponse{}, nil
}
