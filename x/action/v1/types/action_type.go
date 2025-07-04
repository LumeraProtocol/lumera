package types

import (
	"fmt"
	"github.com/LumeraProtocol/lumera/x/action/v1/common"
	"strings"
)

type ActionTypeValidator interface {
	// ActionType returns the enum constant this validator corresponds to.
	ActionType() ActionType

	// ValidateBasic handles any checks specific to this action type.
	// Return an error if validation fails; otherwise nil.
	ValidateBasic(metadataStr string, msgType common.MessageType) error
}

var validatorRegistry = make(map[string]ActionTypeValidator)

func RegisterValidator(v ActionTypeValidator, aliases ...string) {
	for _, alias := range aliases {
		key := strings.ToUpper(alias)
		validatorRegistry[key] = v
	}
}

func init() {
	// Use init() in the files for each action type to register their validators.
}

func DoActionValidation(metadata string, actionTypeStr string, msgType common.MessageType) error {
	actionType, err := ParseActionType(actionTypeStr)
	if err != nil {
		return err
	}

	validator := validatorRegistry[actionType.String()]
	if validator == nil {
		return fmt.Errorf("no validator registered for action type: %s", actionType.String())
	}

	return validator.ValidateBasic(metadata, msgType)
}

// ParseActionType converts a string action type to ActionType enum in a case-insensitive way
func ParseActionType(actionTypeStr string) (ActionType, error) {
	if actionTypeStr == "" {
		return ActionTypeUnspecified, fmt.Errorf("action type cannot be empty")
	}

	v := validatorRegistry[strings.ToUpper(actionTypeStr)]
	if v == nil {
		return ActionTypeUnspecified, fmt.Errorf("unknown action type: %s", actionTypeStr)
	}
	// If the validator’s ActionType is UNSPECIFIED, treat it as an error
	if v.ActionType() == ActionTypeUnspecified {
		return v.ActionType(), fmt.Errorf("action type cannot be unspecified")
	}
	return v.ActionType(), nil
}
