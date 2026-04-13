package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (m msgServer) SubmitStorageRecheckEvidence(_ context.Context, req *types.MsgSubmitStorageRecheckEvidence) (*types.MsgSubmitStorageRecheckEvidenceResponse, error) {
	if req == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "empty request")
	}
	if req.Creator == "" {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "creator is required")
	}
	return nil, errorsmod.Wrap(types.ErrNotImplemented, "SubmitStorageRecheckEvidence is introduced in storage-truth foundation and implemented in a later PR")
}

func (m msgServer) ClaimHealComplete(_ context.Context, req *types.MsgClaimHealComplete) (*types.MsgClaimHealCompleteResponse, error) {
	if req == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "empty request")
	}
	if req.Creator == "" {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "creator is required")
	}
	return nil, errorsmod.Wrap(types.ErrNotImplemented, "ClaimHealComplete is introduced in storage-truth foundation and implemented in a later PR")
}

func (m msgServer) SubmitHealVerification(_ context.Context, req *types.MsgSubmitHealVerification) (*types.MsgSubmitHealVerificationResponse, error) {
	if req == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "empty request")
	}
	if req.Creator == "" {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "creator is required")
	}
	return nil, errorsmod.Wrap(types.ErrNotImplemented, "SubmitHealVerification is introduced in storage-truth foundation and implemented in a later PR")
}
