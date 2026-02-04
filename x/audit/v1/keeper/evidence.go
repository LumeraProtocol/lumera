package keeper

import (
	"context"
	"fmt"
	"strings"

	errorsmod "cosmossdk.io/errors"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/gogoproto/jsonpb"
	gogoproto "github.com/cosmos/gogoproto/proto"
)

func (k Keeper) CreateEvidence(
	ctx context.Context,
	reporterAddress string,
	subjectAddress string,
	actionID string,
	evidenceType types.EvidenceType,
	metadataJSON string,
) (uint64, error) {
	if _, err := k.addressCodec.StringToBytes(reporterAddress); err != nil {
		return 0, errorsmod.Wrap(types.ErrInvalidReporter, err.Error())
	}
	if _, err := k.addressCodec.StringToBytes(subjectAddress); err != nil {
		return 0, errorsmod.Wrap(types.ErrInvalidSubject, err.Error())
	}

	if evidenceType == types.EvidenceType_EVIDENCE_TYPE_UNSPECIFIED {
		return 0, types.ErrInvalidEvidenceType
	}

	switch evidenceType {
	case types.EvidenceType_EVIDENCE_TYPE_ACTION_EXPIRED,
		types.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_SIGNATURE_FAILURE,
		types.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_NOT_IN_TOP_10:
		expectedReporter, err := k.addressCodec.BytesToString(authtypes.NewModuleAddress("action"))
		if err != nil {
			return 0, errorsmod.Wrap(types.ErrInvalidReporter, err.Error())
		}
		if reporterAddress != expectedReporter {
			return 0, errorsmod.Wrap(types.ErrInvalidReporter, "reporter must be the action module account")
		}
	}

	metadataJSON = strings.TrimSpace(metadataJSON)
	if metadataJSON == "" {
		return 0, types.ErrInvalidMetadata
	}

	if actionID == "" {
		// For the initial supported evidence types (action expiration/finalization), action id is required.
		switch evidenceType {
		case types.EvidenceType_EVIDENCE_TYPE_ACTION_EXPIRED,
			types.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_SIGNATURE_FAILURE,
			types.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_NOT_IN_TOP_10:
			return 0, types.ErrInvalidActionID
		}
	}

	metadataBytes, err := marshalEvidenceMetadataJSON(evidenceType, metadataJSON)
	if err != nil {
		return 0, errorsmod.Wrap(types.ErrInvalidMetadata, err.Error())
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	reportedHeight := uint64(sdkCtx.BlockHeight())

	evidenceID := k.GetNextEvidenceID(sdkCtx)
	k.SetNextEvidenceID(sdkCtx, evidenceID+1)

	ev := types.Evidence{
		EvidenceId:      evidenceID,
		SubjectAddress:  subjectAddress,
		ReporterAddress: reporterAddress,
		ActionId:        actionID,
		EvidenceType:    evidenceType,
		Metadata:        metadataBytes,
		ReportedHeight:  reportedHeight,
	}

	if err := k.SetEvidence(sdkCtx, ev); err != nil {
		return 0, err
	}
	k.SetEvidenceBySubjectIndex(sdkCtx, subjectAddress, evidenceID)
	if actionID != "" {
		k.SetEvidenceByActionIndex(sdkCtx, actionID, evidenceID)
	}

	return evidenceID, nil
}

func marshalEvidenceMetadataJSON(evidenceType types.EvidenceType, metadataJSON string) ([]byte, error) {
	u := &jsonpb.Unmarshaler{}

	switch evidenceType {
	case types.EvidenceType_EVIDENCE_TYPE_ACTION_EXPIRED:
		var m types.ActionExpiredEvidenceMetadata
		if err := u.Unmarshal(strings.NewReader(metadataJSON), &m); err != nil {
			return nil, fmt.Errorf("unmarshal ActionExpiredEvidenceMetadata: %w", err)
		}
		return gogoproto.Marshal(&m)

	case types.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_SIGNATURE_FAILURE:
		var m types.ActionFinalizationSignatureFailureEvidenceMetadata
		if err := u.Unmarshal(strings.NewReader(metadataJSON), &m); err != nil {
			return nil, fmt.Errorf("unmarshal ActionFinalizationSignatureFailureEvidenceMetadata: %w", err)
		}
		return gogoproto.Marshal(&m)

	case types.EvidenceType_EVIDENCE_TYPE_ACTION_FINALIZATION_NOT_IN_TOP_10:
		var m types.ActionFinalizationNotInTop10EvidenceMetadata
		if err := u.Unmarshal(strings.NewReader(metadataJSON), &m); err != nil {
			return nil, fmt.Errorf("unmarshal ActionFinalizationNotInTop10EvidenceMetadata: %w", err)
		}
		return gogoproto.Marshal(&m)

	default:
		return nil, fmt.Errorf("unsupported evidence_type: %s", evidenceType.String())
	}
}
