package keeper_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/keeper"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	gogoproto "github.com/cosmos/gogoproto/proto"
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

func TestSubmitEvidenceAndQueries_CascadeClientFailure(t *testing.T) {
	f := initFixture(t)

	ms := keeper.NewMsgServerImpl(f.keeper)
	qs := keeper.NewQueryServerImpl(f.keeper)

	reporter, err := f.addressCodec.BytesToString(bytes.Repeat([]byte{0x11}, 20))
	require.NoError(t, err)
	subject, err := f.addressCodec.BytesToString(bytes.Repeat([]byte{0x22}, 20))
	require.NoError(t, err)
	targetA, err := f.addressCodec.BytesToString(bytes.Repeat([]byte{0x33}, 20))
	require.NoError(t, err)
	targetB, err := f.addressCodec.BytesToString(bytes.Repeat([]byte{0x44}, 20))
	require.NoError(t, err)

	meta := types.CascadeClientFailureEvidenceMetadata{
		ReporterComponent: types.CascadeClientFailureReporterComponent_CASCADE_CLIENT_FAILURE_REPORTER_COMPONENT_SN_API_SERVER,
		TargetSupernodeAccounts: []string{
			targetA,
			targetB,
		},
		Details: map[string]string{
			"error":    "context deadline exceeded while streaming upload",
			"trace_id": "trace-1234",
		},
	}
	metaBz, err := json.Marshal(meta)
	require.NoError(t, err)

	resp, err := ms.SubmitEvidence(f.ctx, &types.MsgSubmitEvidence{
		Creator:        reporter,
		SubjectAddress: subject,
		EvidenceType:   types.EvidenceType_EVIDENCE_TYPE_CASCADE_CLIENT_FAILURE,
		ActionId:       "action-cascade-1",
		Metadata:       string(metaBz),
	})
	require.NoError(t, err)
	require.Equal(t, uint64(1), resp.EvidenceId)

	gotByID, err := qs.EvidenceById(f.ctx, &types.QueryEvidenceByIdRequest{EvidenceId: resp.EvidenceId})
	require.NoError(t, err)
	require.Equal(t, resp.EvidenceId, gotByID.Evidence.EvidenceId)
	require.Equal(t, subject, gotByID.Evidence.SubjectAddress)
	require.Equal(t, reporter, gotByID.Evidence.ReporterAddress)
	require.Equal(t, "action-cascade-1", gotByID.Evidence.ActionId)
	require.Equal(t, types.EvidenceType_EVIDENCE_TYPE_CASCADE_CLIENT_FAILURE, gotByID.Evidence.EvidenceType)

	var gotMeta types.CascadeClientFailureEvidenceMetadata
	err = gogoproto.Unmarshal(gotByID.Evidence.Metadata, &gotMeta)
	require.NoError(t, err)
	require.Equal(t, meta.ReporterComponent, gotMeta.ReporterComponent)
	require.Equal(t, meta.TargetSupernodeAccounts, gotMeta.TargetSupernodeAccounts)
	require.Equal(t, meta.Details, gotMeta.Details)

	gotBySubject, err := qs.EvidenceBySubject(f.ctx, &types.QueryEvidenceBySubjectRequest{SubjectAddress: subject})
	require.NoError(t, err)
	require.Len(t, gotBySubject.Evidence, 1)
	require.Equal(t, resp.EvidenceId, gotBySubject.Evidence[0].EvidenceId)

	gotByAction, err := qs.EvidenceByAction(f.ctx, &types.QueryEvidenceByActionRequest{ActionId: "action-cascade-1"})
	require.NoError(t, err)
	require.Len(t, gotByAction.Evidence, 1)
	require.Equal(t, resp.EvidenceId, gotByAction.Evidence[0].EvidenceId)
}

func TestCreateEvidence_CascadeClientFailure_InvalidMetadata(t *testing.T) {
	f := initFixture(t)

	reporter, err := f.addressCodec.BytesToString(bytes.Repeat([]byte{0x11}, 20))
	require.NoError(t, err)
	subject, err := f.addressCodec.BytesToString(bytes.Repeat([]byte{0x22}, 20))
	require.NoError(t, err)

	_, err = f.keeper.CreateEvidence(
		f.ctx,
		reporter,
		subject,
		"action-cascade-1",
		types.EvidenceType_EVIDENCE_TYPE_CASCADE_CLIENT_FAILURE,
		`{"details":`,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), types.ErrInvalidMetadata.Error())
}
