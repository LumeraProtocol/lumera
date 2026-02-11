package keeper_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	"github.com/stretchr/testify/require"
)

func TestCreateEvidence_StorageChallengeFailure_ChallengerPolicy(t *testing.T) {
	f := initFixture(t)

	params := types.DefaultParams()
	params.ScEnabled = true
	params.ScChallengersPerEpoch = 2
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	challenger1, err := f.addressCodec.BytesToString(bytes.Repeat([]byte{0x11}, 20))
	require.NoError(t, err)
	challenger2, err := f.addressCodec.BytesToString(bytes.Repeat([]byte{0x22}, 20))
	require.NoError(t, err)
	subject, err := f.addressCodec.BytesToString(bytes.Repeat([]byte{0x33}, 20))
	require.NoError(t, err)
	outsider, err := f.addressCodec.BytesToString(bytes.Repeat([]byte{0x44}, 20))
	require.NoError(t, err)

	anchor := types.EpochAnchor{
		EpochId:                 0,
		EpochStartHeight:        1,
		EpochEndHeight:          400,
		EpochLengthBlocks:       params.EpochLengthBlocks,
		Seed:                    bytes.Repeat([]byte{0x99}, 32),
		ActiveSupernodeAccounts: []string{challenger1, challenger2},
		TargetSupernodeAccounts: []string{subject},
	}
	require.NoError(t, f.keeper.SetEpochAnchor(f.ctx, anchor))

	metaOK, err := json.Marshal(types.StorageChallengeFailureEvidenceMetadata{
		EpochId:                    0,
		ChallengerSupernodeAccount: challenger1,
		ChallengedSupernodeAccount: subject,
		ChallengeId:                "deadbeef",
		FileKey:                    "filekey",
		FailureType:                "RECIPIENT_UNREACHABLE",
		TranscriptHash:             "bead",
	})
	require.NoError(t, err)

	_, err = f.keeper.CreateEvidence(
		f.ctx,
		challenger1,
		subject,
		"",
		types.EvidenceType_EVIDENCE_TYPE_STORAGE_CHALLENGE_FAILURE,
		string(metaOK),
	)
	require.NoError(t, err)

	metaBad, err := json.Marshal(types.StorageChallengeFailureEvidenceMetadata{
		EpochId:                    0,
		ChallengerSupernodeAccount: outsider,
		ChallengedSupernodeAccount: subject,
		ChallengeId:                "deadbeef",
		FileKey:                    "filekey",
		FailureType:                "RECIPIENT_UNREACHABLE",
		TranscriptHash:             "bead",
	})
	require.NoError(t, err)

	_, err = f.keeper.CreateEvidence(
		f.ctx,
		outsider,
		subject,
		"",
		types.EvidenceType_EVIDENCE_TYPE_STORAGE_CHALLENGE_FAILURE,
		string(metaBad),
	)
	require.Error(t, err)
}
