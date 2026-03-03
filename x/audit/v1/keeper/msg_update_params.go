package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (m msgServer) UpdateParams(ctx context.Context, req *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if req == nil {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "empty request")
	}

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

	// Params are changed only by genesis or by this governance message. Epoch cadence is an
	// especially sensitive invariant because it defines height->epoch mapping used throughout
	// the module (anchoring, gating, enforcement, pruning).
	// Epoch cadence is a global invariant. Do not allow epoch math changes via governance,
	// since it would re-map heights->epoch_id and break deterministic off-chain behavior.
	current := m.GetParams(ctx).WithDefaults()
	if params.EpochLengthBlocks != current.EpochLengthBlocks {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "epoch_length_blocks is immutable after genesis")
	}
	if params.EpochZeroHeight != current.EpochZeroHeight {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "epoch_zero_height is immutable after genesis")
	}

	if err := m.SetParams(ctx, params); err != nil {
		return nil, err
	}

	return &types.MsgUpdateParamsResponse{}, nil
}
