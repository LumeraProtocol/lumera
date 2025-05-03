package types

import (
	"encoding/json"
	"fmt"
	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	"github.com/LumeraProtocol/lumera/x/action/common"
)

type SenseValidator struct{}

func init() {
	// Register the Sense validator with multiple string aliases:
	RegisterValidator(&SenseValidator{},
		"SENSE",
		"ACTION_TYPE_SENSE",
	)
}

func (v *SenseValidator) ActionType() actionapi.ActionType {
	return actionapi.ActionType_ACTION_TYPE_SENSE
}

func (v *SenseValidator) ValidateBasic(metadataStr string, msgType common.MessageType) error {
	var metadata actionapi.SenseMetadata
	if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
		return fmt.Errorf("failed to unmarshal sense metadata: %w", err)
	}

	if msgType == common.MsgRequestAction {
		if metadata.DataHash == "" {
			return fmt.Errorf("data_hash is required for sense metadata")
		}
		if metadata.DdAndFingerprintsIc == 0 {
			return fmt.Errorf("dd_and_fingerprints_ic is required for sense metadata")
		}
	}
	if msgType == common.MsgFinalizeAction {
		if len(metadata.DdAndFingerprintsIds) == 0 {
			return fmt.Errorf("dd_and_fingerprints_ids is required for sense metadata")
		}
		if metadata.Signatures == "" {
			return fmt.Errorf("signatures is required for sense metadata")
		}
	}

	return nil
}
