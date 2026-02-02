package keeper_test

import (
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/stretchr/testify/require"
)

func TestSubmitEvidenceAndQueries(t *testing.T) {
	f := initFixture(t)

	ms := keeper.NewMsgServerImpl(f.keeper)
	qs := keeper.NewQueryServerImpl(f.keeper)

	reporter, err := f.addressCodec.BytesToString([]byte("reporter_address_20b"))
	require.NoError(t, err)
	subject, err := f.addressCodec.BytesToString([]byte("subject_address_20bb"))
	require.NoError(t, err)

	resp, err := ms.SubmitEvidence(f.ctx, &types.MsgSubmitEvidence{
		Creator:        reporter,
		SubjectAddress: subject,
		EvidenceType:   types.EvidenceType_EVIDENCE_TYPE_ACTION_EXPIRED,
		ActionId:       "action-123",
		Metadata:       `{"expiration_height":123,"reason":"unit test"}`,
	})
	require.NoError(t, err)
	require.Equal(t, uint64(1), resp.EvidenceId)

	gotByID, err := qs.EvidenceById(f.ctx, &types.QueryEvidenceByIdRequest{EvidenceId: resp.EvidenceId})
	require.NoError(t, err)
	require.Equal(t, resp.EvidenceId, gotByID.Evidence.EvidenceId)
	require.Equal(t, subject, gotByID.Evidence.SubjectAddress)
	require.Equal(t, reporter, gotByID.Evidence.ReporterAddress)
	require.Equal(t, "action-123", gotByID.Evidence.ActionId)
	require.Equal(t, types.EvidenceType_EVIDENCE_TYPE_ACTION_EXPIRED, gotByID.Evidence.EvidenceType)
	require.NotEmpty(t, gotByID.Evidence.Metadata)

	gotBySubject, err := qs.EvidenceBySubject(f.ctx, &types.QueryEvidenceBySubjectRequest{SubjectAddress: subject})
	require.NoError(t, err)
	require.Len(t, gotBySubject.Evidence, 1)
	require.Equal(t, resp.EvidenceId, gotBySubject.Evidence[0].EvidenceId)

	gotByAction, err := qs.EvidenceByAction(f.ctx, &types.QueryEvidenceByActionRequest{ActionId: "action-123"})
	require.NoError(t, err)
	require.Len(t, gotByAction.Evidence, 1)
	require.Equal(t, resp.EvidenceId, gotByAction.Evidence[0].EvidenceId)
}
