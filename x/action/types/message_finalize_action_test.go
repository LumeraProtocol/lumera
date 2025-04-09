package types

import (
	"testing"

	"github.com/LumeraProtocol/lumera/testutil/cryptotestutils"
	"github.com/stretchr/testify/require"
)

func TestMsgFinalizeAction_ValidateBasic(t *testing.T) {
	validCreator := cryptotestutils.AccAddress()
	validActionId := "action123"

	// Create valid metadata for SENSE action type
	validSenseMetadataStr := `{
		"dd_and_fingerprints_ids": ["id1", "id2", "id3"],
		"signatures": "valid-signatures"
	}`

	// Create valid metadata for CASCADE action type
	validCascadeMetadataStr := `{
		"rq_ids_ids": ["id1", "id2", "id3"],
		"rq_ids_oti": "b3RpX2RhdGE="
	}`

	// Create invalid SENSE metadata (missing required fields)
	invalidSenseMetadataStr := `{
		"signatures": "valid-signatures"
	}`

	// Create another invalid SENSE metadata (missing different required field)
	invalidSenseMetadataStr2 := `{
		"dd_and_fingerprints_ids": ["id1", "id2", "id3"]
	}`

	// Create invalid CASCADE metadata (missing required fields)
	invalidCascadeMetadataStr := `{
		"rq_ids_ids": ["id1", "id2", "id3"]
	}`

	// Create another invalid CASCADE metadata (missing different required field)
	invalidCascadeMetadataStr2 := `{
		"rq_ids_oti": "b3RpX2RhdGE="
	}`

	tests := []struct {
		name string
		msg  MsgFinalizeAction
		err  error
	}{
		// Valid test cases
		{
			name: "valid SENSE action finalization",
			msg: MsgFinalizeAction{
				Creator:    validCreator,
				ActionId:   validActionId,
				ActionType: "SENSE",
				Metadata:   validSenseMetadataStr,
			},
			err: nil,
		},
		{
			name: "valid CASCADE action finalization",
			msg: MsgFinalizeAction{
				Creator:    validCreator,
				ActionId:   validActionId,
				ActionType: "CASCADE",
				Metadata:   validCascadeMetadataStr,
			},
			err: nil,
		},
		// Case insensitivity test
		{
			name: "valid with lowercase action type",
			msg: MsgFinalizeAction{
				Creator:    validCreator,
				ActionId:   validActionId,
				ActionType: "sense",
				Metadata:   validSenseMetadataStr,
			},
			err: nil,
		},

		// Test cases for creator address validation
		{
			name: "invalid creator address",
			msg: MsgFinalizeAction{
				Creator: "invalid_address",
			},
			err: ErrInvalidAddress,
		},

		// Test cases for action ID validation
		{
			name: "invalid action ID (empty)",
			msg: MsgFinalizeAction{
				Creator:    validCreator,
				ActionId:   "",
				ActionType: "SENSE",
				Metadata:   validSenseMetadataStr,
			},
			err: ErrInvalidID,
		},

		// Test cases for action type validation
		{
			name: "empty action type",
			msg: MsgFinalizeAction{
				Creator:  validCreator,
				ActionId: validActionId,
			},
			err: ErrInvalidActionType,
		},
		{
			name: "invalid action type",
			msg: MsgFinalizeAction{
				Creator:    validCreator,
				ActionId:   validActionId,
				ActionType: "UNKNOWN_TYPE",
			},
			err: ErrInvalidActionType,
		},
		{
			name: "unspecified action type",
			msg: MsgFinalizeAction{
				Creator:    validCreator,
				ActionId:   validActionId,
				ActionType: "UNSPECIFIED",
			},
			err: ErrInvalidActionType,
		},

		// Test cases for metadata validation
		{
			name: "empty metadata - SENSE",
			msg: MsgFinalizeAction{
				Creator:    validCreator,
				ActionId:   validActionId,
				ActionType: "SENSE",
				Metadata:   "",
			},
			err: ErrInvalidMetadata,
		},
		{
			name: "invalid metadata - SENSE missing dd_and_fingerprints_ids",
			msg: MsgFinalizeAction{
				Creator:    validCreator,
				ActionId:   validActionId,
				ActionType: "SENSE",
				Metadata:   invalidSenseMetadataStr,
			},
			err: ErrInvalidMetadata,
		},
		{
			name: "invalid metadata - SENSE missing signatures",
			msg: MsgFinalizeAction{
				Creator:    validCreator,
				ActionId:   validActionId,
				ActionType: "SENSE",
				Metadata:   invalidSenseMetadataStr2,
			},
			err: ErrInvalidMetadata,
		},
		{
			name: "empty metadata - CASCADE",
			msg: MsgFinalizeAction{
				Creator:    validCreator,
				ActionId:   validActionId,
				ActionType: "CASCADE",
				Metadata:   "",
			},
			err: ErrInvalidMetadata,
		},
		{
			name: "invalid metadata - CASCADE missing rq_ids_oti",
			msg: MsgFinalizeAction{
				Creator:    validCreator,
				ActionId:   validActionId,
				ActionType: "CASCADE",
				Metadata:   invalidCascadeMetadataStr,
			},
			err: ErrInvalidMetadata,
		},
		{
			name: "invalid metadata - CASCADE missing rq_ids_ids",
			msg: MsgFinalizeAction{
				Creator:    validCreator,
				ActionId:   validActionId,
				ActionType: "CASCADE",
				Metadata:   invalidCascadeMetadataStr2,
			},
			err: ErrInvalidMetadata,
		},
		{
			name: "invalid JSON metadata",
			msg: MsgFinalizeAction{
				Creator:    validCreator,
				ActionId:   validActionId,
				ActionType: "SENSE",
				Metadata:   "{invalid json",
			},
			err: ErrInvalidMetadata,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateBasic()
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}
