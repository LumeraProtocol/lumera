package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (m msgServer) UpdateParams(ctx context.Context, req *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if req == nil {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "empty request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	authority, err := m.addressCodec.BytesToString(m.authority)
	if err != nil {
		return nil, err
	}
	if req.Authority != authority {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "invalid authority")
	}

	params := req.Params.WithDefaults()
	if err := params.Validate(); err != nil {
		return nil, err
	}

	// If reporting_window_blocks changes, schedule the new size to take effect at the next boundary.
	if err := m.scheduleReportingWindowBlocksChangeAtNextBoundary(sdkCtx, m.GetParams(ctx).WithDefaults(), params.ReportingWindowBlocks); err != nil {
		return nil, err
	}

	if err := m.SetParams(ctx, params); err != nil {
		return nil, err
	}

	return &types.MsgUpdateParamsResponse{}, nil
}
