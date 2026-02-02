package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (k msgServer) SubmitEvidence(ctx context.Context, msg *types.MsgSubmitEvidence) (*types.MsgSubmitEvidenceResponse, error) {
	if msg == nil {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "empty request")
	}

	evidenceID, err := k.Keeper.CreateEvidence(ctx, msg.Creator, msg.SubjectAddress, msg.ActionId, msg.EvidenceType, msg.Metadata)
	if err != nil {
		return nil, err
	}

	return &types.MsgSubmitEvidenceResponse{EvidenceId: evidenceID}, nil
}
