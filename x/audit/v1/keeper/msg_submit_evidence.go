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

	switch msg.EvidenceType {
	case types.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_SIGNATURE_FAILURE,
		types.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_NOT_IN_TOP_10,
		types.EvidenceType_EVIDENCE_TYPE_ACTION_EXPIRED:
		return nil, errorsmod.Wrap(types.ErrInvalidEvidenceType, "evidence type is reserved for the action module")
	}

	evidenceID, err := k.Keeper.CreateEvidence(ctx, msg.Creator, msg.SubjectAddress, msg.ActionId, msg.EvidenceType, msg.Metadata)
	if err != nil {
		return nil, err
	}

	return &types.MsgSubmitEvidenceResponse{EvidenceId: evidenceID}, nil
}
