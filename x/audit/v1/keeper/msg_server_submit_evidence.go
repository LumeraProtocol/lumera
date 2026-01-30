package keeper

import (
	"context"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (k msgServer) SubmitEvidence(ctx context.Context, msg *types.MsgSubmitEvidence) (*types.MsgSubmitEvidenceResponse, error) {
	evidenceID, err := k.Keeper.CreateEvidence(ctx, msg.Creator, msg.SubjectAddress, msg.ActionId, msg.EvidenceType, msg.Metadata)
	if err != nil {
		return nil, err
	}

	return &types.MsgSubmitEvidenceResponse{EvidenceId: evidenceID}, nil
}
