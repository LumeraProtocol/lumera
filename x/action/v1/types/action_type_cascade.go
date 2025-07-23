package types

import (
	"encoding/json"
	"fmt"
	"github.com/LumeraProtocol/lumera/x/action/v1/common"
)

type CascadeValidator struct{}

func init() {
	// Register the Sense validator with multiple string aliases:
	RegisterValidator(&CascadeValidator{},
		"CASCADE",
		"ACTION_TYPE_CASCADE",
	)
}

func (v *CascadeValidator) ActionType() ActionType {
	return ActionTypeCascade
}

func (v *CascadeValidator) ValidateBasic(metadataStr string, msgType common.MessageType) error {
	var metadata CascadeMetadata
	if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
		return fmt.Errorf("failed to unmarshal cascade metadata: %w", err)
	}

	if msgType == common.MsgRequestAction {
		if metadata.DataHash == "" {
			return fmt.Errorf("data_hash is required for cascade metadata")
		}
		if metadata.FileName == "" {
			return fmt.Errorf("file_name is required for cascade metadata")
		}
		if metadata.RqIdsIc == 0 {
			return fmt.Errorf("rq_ids_ic is required for cascade metadata")
		}
		if metadata.Signatures == "" {
			return fmt.Errorf("signatures is required for cascade metadata")
		}
	}
	if msgType == common.MsgFinalizeAction {
		if len(metadata.RqIdsIds) == 0 {
			return fmt.Errorf("rq_ids_ids is required for cascade metadata")
		}
	}
	return nil
}
