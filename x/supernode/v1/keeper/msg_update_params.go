package keeper

import (
	"context"

	types2 "github.com/LumeraProtocol/lumera/x/action/v1/types"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
<<<<<<<< HEAD:x/action/v1/keeper/msg_update_params.go
========

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
>>>>>>>> f8437a0 (IBC & Wasm upgrade):x/supernode/v1/keeper/msg_update_params.go
)

func (k msgServer) UpdateParams(goCtx context.Context, req *types2.MsgUpdateParams) (*types2.MsgUpdateParamsResponse, error) {
	if k.GetAuthority() != req.Authority {
		return nil, errorsmod.Wrapf(types2.ErrInvalidSigner, "invalid authority; expected %s, got %s", k.GetAuthority(), req.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if err := k.SetParams(ctx, req.Params); err != nil {
		return nil, err
	}

	return &types2.MsgUpdateParamsResponse{}, nil
}
