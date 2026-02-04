package keeper_test

import (
	"encoding/json"
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"
)

func TestSubmitEvidenceAndQueries(t *testing.T) {
	f := initFixture(t)

	ms := keeper.NewMsgServerImpl(f.keeper)
	qs := keeper.NewQueryServerImpl(f.keeper)

	reporter, err := f.addressCodec.BytesToString(authtypes.NewModuleAddress("action"))
	require.NoError(t, err)
	subject, err := f.addressCodec.BytesToString([]byte("subject_address_20bb"))
	require.NoError(t, err)

	metaBz, err := json.Marshal(types.ActionExpiredEvidenceMetadata{
		Top_10ValidatorAddresses: []string{sdk.ValAddress([]byte("validator_address_20")).String()},
	})
	require.NoError(t, err)

	_, err = ms.SubmitEvidence(f.ctx, &types.MsgSubmitEvidence{
		Creator:        reporter,
		SubjectAddress: subject,
		EvidenceType:   types.EvidenceType_EVIDENCE_TYPE_ACTION_EXPIRED,
		ActionId:       "action-123",
		Metadata:       string(metaBz),
	})
	require.Error(t, err)

	respID, err := f.keeper.CreateEvidence(
		f.ctx,
		reporter,
		subject,
		"action-123",
		types.EvidenceType_EVIDENCE_TYPE_ACTION_EXPIRED,
		string(metaBz),
	)
	require.NoError(t, err)
	require.Equal(t, uint64(1), respID)

	gotByID, err := qs.EvidenceById(f.ctx, &types.QueryEvidenceByIdRequest{EvidenceId: respID})
	require.NoError(t, err)
	require.Equal(t, respID, gotByID.Evidence.EvidenceId)
	require.Equal(t, subject, gotByID.Evidence.SubjectAddress)
	require.Equal(t, reporter, gotByID.Evidence.ReporterAddress)
	require.Equal(t, "action-123", gotByID.Evidence.ActionId)
	require.Equal(t, types.EvidenceType_EVIDENCE_TYPE_ACTION_EXPIRED, gotByID.Evidence.EvidenceType)
	require.NotEmpty(t, gotByID.Evidence.Metadata)

	gotBySubject, err := qs.EvidenceBySubject(f.ctx, &types.QueryEvidenceBySubjectRequest{SubjectAddress: subject})
	require.NoError(t, err)
	require.Len(t, gotBySubject.Evidence, 1)
	require.Equal(t, respID, gotBySubject.Evidence[0].EvidenceId)

	gotByAction, err := qs.EvidenceByAction(f.ctx, &types.QueryEvidenceByActionRequest{ActionId: "action-123"})
	require.NoError(t, err)
	require.Len(t, gotByAction.Evidence, 1)
	require.Equal(t, respID, gotByAction.Evidence[0].EvidenceId)
}
