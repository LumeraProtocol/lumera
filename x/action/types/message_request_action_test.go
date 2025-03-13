package types

import (
	"testing"

	"github.com/LumeraProtocol/lumera/testutil/sample"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"
)

func TestMsgRequestAction_ValidateBasic(t *testing.T) {
	validAddress := sample.AccAddress()
	validPrice := "1000ulume"
	validExpTime := "1735689600" // Some future timestamp

	// Create valid metadata for SENSE action type - using JSON string directly to match expected field names
	validSenseMetadataStr := `{
		"data_hash": "hash123",
		"dd_and_fingerprints_ic": 5,
		"collection_id": "collection1"
	}`

	// Create valid metadata for CASCADE action type - using JSON string directly to match expected field names
	validCascadeMetadataStr := `{
		"data_hash": "hash456",
		"file_name": "test.txt",
		"rq_ids_ic": 3,
		"signatures": "valid-signature"
	}`
	// Create invalid CASCADE metadata (missing signatures) - using JSON string directly
	invalidCascadeMetadataSignatureStr := `{
		"data_hash": "hash789",
		"file_name": "test2.txt",
		"rq_ids_ic": 3
	}`

	// Create invalid SENSE metadata (missing required fields) - using JSON string directly
	invalidSenseMetadataStr := `{
		"collection_id": "collection2"
	}`

	tests := []struct {
		name string
		msg  MsgRequestAction
		err  error
	}{
		// Valid test cases
		{
			name: "valid SENSE action request",
			msg: MsgRequestAction{
				Creator:        validAddress,
				ActionType:     "SENSE",
				Metadata:       validSenseMetadataStr,
				Price:          validPrice,
				ExpirationTime: validExpTime,
			},
			err: nil,
		},
		{
			name: "valid CASCADE action request",
			msg: MsgRequestAction{
				Creator:        validAddress,
				ActionType:     "CASCADE",
				Metadata:       validCascadeMetadataStr,
				Price:          validPrice,
				ExpirationTime: validExpTime,
			},
			err: nil,
		},

		// Test cases for creator address validation
		{
			name: "invalid address",
			msg: MsgRequestAction{
				Creator: "invalid_address",
			},
			err: sdkerrors.ErrInvalidAddress,
		},

		// Test cases for action type validation
		{
			name: "empty action type",
			msg: MsgRequestAction{
				Creator:        validAddress,
				Price:          validPrice,
				ExpirationTime: validExpTime,
			},
			err: ErrInvalidActionType,
		},
		{
			name: "invalid action type",
			msg: MsgRequestAction{
				Creator:        validAddress,
				ActionType:     "UNKNOWN_TYPE",
				Price:          validPrice,
				ExpirationTime: validExpTime,
			},
			err: ErrInvalidActionType,
		},
		{
			name: "unspecified action type",
			msg: MsgRequestAction{
				Creator:        validAddress,
				ActionType:     "UNSPECIFIED",
				Price:          validPrice,
				ExpirationTime: validExpTime,
			},
			err: ErrInvalidActionType,
		},

		// Test cases for metadata validation
		{
			name: "empty metadata - SENSE",
			msg: MsgRequestAction{
				Creator:        validAddress,
				ActionType:     "SENSE",
				Price:          validPrice,
				ExpirationTime: validExpTime,
			},
			err: ErrInvalidMetadata,
		},
		{
			name: "invalid metadata - SENSE missing required fields",
			msg: MsgRequestAction{
				Creator:        validAddress,
				ActionType:     "SENSE",
				Metadata:       invalidSenseMetadataStr,
				Price:          validPrice,
				ExpirationTime: validExpTime,
			},
			err: ErrInvalidMetadata,
		},
		{
			name: "empty metadata - CASCADE",
			msg: MsgRequestAction{
				Creator:        validAddress,
				ActionType:     "CASCADE",
				Price:          validPrice,
				ExpirationTime: validExpTime,
			},
			err: ErrInvalidMetadata,
		},
		{
			name: "CASCADE action missing signatures",
			msg: MsgRequestAction{
				Creator:        validAddress,
				ActionType:     "CASCADE",
				Metadata:       invalidCascadeMetadataSignatureStr,
				Price:          validPrice,
				ExpirationTime: validExpTime,
			},
			err: ErrInvalidMetadata,
		},

		// Test cases for price validation
		{
			name: "empty price",
			msg: MsgRequestAction{
				Creator:        validAddress,
				ActionType:     "SENSE",
				Metadata:       validSenseMetadataStr,
				Price:          "",
				ExpirationTime: validExpTime,
			},
			err: ErrInvalidPrice,
		},
		{
			name: "invalid price format",
			msg: MsgRequestAction{
				Creator:        validAddress,
				ActionType:     "SENSE",
				Metadata:       validSenseMetadataStr,
				Price:          "invalid price",
				ExpirationTime: validExpTime,
			},
			err: ErrInvalidPrice,
		},

		// Test cases for expiration time validation
		{
			name: "invalid expiration time format",
			msg: MsgRequestAction{
				Creator:        validAddress,
				ActionType:     "SENSE",
				Metadata:       validSenseMetadataStr,
				Price:          validPrice,
				ExpirationTime: "not a timestamp",
			},
			err: ErrInvalidExpiration,
		},
		{
			name: "zero expiration time - valid",
			msg: MsgRequestAction{
				Creator:        validAddress,
				ActionType:     "SENSE",
				Metadata:       validSenseMetadataStr,
				Price:          validPrice,
				ExpirationTime: "0",
			},
			err: nil,
		},
		{
			name: "negative expiration time",
			msg: MsgRequestAction{
				Creator:        validAddress,
				ActionType:     "SENSE",
				Metadata:       validSenseMetadataStr,
				Price:          validPrice,
				ExpirationTime: "-1",
			},
			err: ErrInvalidExpiration,
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
