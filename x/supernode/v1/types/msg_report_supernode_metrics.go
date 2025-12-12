package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ValidateBasic implements sdk.Msg.
func (m MsgReportSupernodeMetrics) ValidateBasic() error {
	if m.ValidatorAddress == "" {
		return fmt.Errorf("validator address cannot be empty")
	}
	if _, err := sdk.ValAddressFromBech32(m.ValidatorAddress); err != nil {
		return fmt.Errorf("invalid validator address: %w", err)
	}
	if m.SupernodeAccount == "" {
		return fmt.Errorf("supernode account cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(m.SupernodeAccount); err != nil {
		return fmt.Errorf("invalid supernode account: %w", err)
	}
	// Structural validation only; semantic metrics checks (including
	// completeness and threshold enforcement) are performed in the
	// keeper's evaluateCompliance function.
	return nil
}

// GetSigners requires the supernode account to submit the report.
func (m MsgReportSupernodeMetrics) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.SupernodeAccount)
	return []sdk.AccAddress{addr}
}
