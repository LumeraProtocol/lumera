package keeper

import (
	"context"
	"fmt"
	"strings"

	"cosmossdk.io/collections"
	"cosmossdk.io/errors"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	gogoproto "github.com/cosmos/gogoproto/proto"
	"github.com/cosmos/gogoproto/jsonpb"
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
		return 0, errors.Wrap(types.ErrInvalidReporter, err.Error())
	}

	if _, err := k.addressCodec.StringToBytes(subjectAddress); err != nil {
		return 0, errors.Wrap(types.ErrInvalidSubject, err.Error())
	}

	if evidenceType == types.EvidenceType_EVIDENCE_TYPE_UNSPECIFIED {
		return 0, types.ErrInvalidEvidenceType
	}

	metadataJSON = strings.TrimSpace(metadataJSON)
	if metadataJSON == "" {
		return 0, types.ErrInvalidMetadata
	}

	if actionID == "" {
		// For the initial supported evidence types (action expiration/finalization), action id is required.
		switch evidenceType {
		case types.EvidenceType_EVIDENCE_TYPE_ACTION_EXPIRED, types.EvidenceType_EVIDENCE_TYPE_ACTION_WRONG_FINALIZATION:
			return 0, types.ErrInvalidActionID
		}
	}

	metadataBytes, err := marshalEvidenceMetadataJSON(evidenceType, metadataJSON)
	if err != nil {
		return 0, errors.Wrap(types.ErrInvalidMetadata, err.Error())
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	reportedHeight := uint64(sdkCtx.BlockHeight())

	evidenceID, err := k.EvidenceID.Next(ctx)
	if err != nil {
		return 0, err
	}

	ev := types.Evidence{
		EvidenceId:      evidenceID,
		SubjectAddress:  subjectAddress,
		ReporterAddress: reporterAddress,
		ActionId:        actionID,
		EvidenceType:    evidenceType,
		Metadata:        metadataBytes,
		ReportedHeight:  reportedHeight,
	}

	if err := k.Evidences.Set(ctx, evidenceID, ev); err != nil {
		return 0, err
	}

	if err := k.BySubject.Set(ctx, collections.Join(subjectAddress, evidenceID)); err != nil {
		return 0, err
	}
	if actionID != "" {
		if err := k.ByActionID.Set(ctx, collections.Join(actionID, evidenceID)); err != nil {
			return 0, err
		}
	}

	return evidenceID, nil
}

func marshalEvidenceMetadataJSON(evidenceType types.EvidenceType, metadataJSON string) ([]byte, error) {
	u := &jsonpb.Unmarshaler{}

	switch evidenceType {
	case types.EvidenceType_EVIDENCE_TYPE_ACTION_EXPIRED:
		var m types.ExpirationEvidenceMetadata
		if err := u.Unmarshal(strings.NewReader(metadataJSON), &m); err != nil {
			return nil, fmt.Errorf("unmarshal ExpirationEvidenceMetadata: %w", err)
		}
		return gogoproto.Marshal(&m)

	case types.EvidenceType_EVIDENCE_TYPE_ACTION_WRONG_FINALIZATION:
		var m types.FinalizationEvidenceMetadata
		if err := u.Unmarshal(strings.NewReader(metadataJSON), &m); err != nil {
			return nil, fmt.Errorf("unmarshal FinalizationEvidenceMetadata: %w", err)
		}
		if strings.TrimSpace(m.AttemptedFinalizerAddress) == "" {
			return nil, fmt.Errorf("attempted_finalizer_address is required")
		}
		return gogoproto.Marshal(&m)

	default:
		return nil, fmt.Errorf("unsupported evidence_type: %s", evidenceType.String())
	}
}
