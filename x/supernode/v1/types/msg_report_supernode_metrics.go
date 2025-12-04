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
	if len(m.Metrics) == 0 {
		return fmt.Errorf("metrics cannot be empty")
	}
	return nil
}

// GetSigners allows either the validator operator or supernode account to submit the report.
func (m MsgReportSupernodeMetrics) GetSigners() []sdk.AccAddress {
	// Prefer validator operator as signer when valid; fall back to supernode account.
	if valAddr, err := sdk.ValAddressFromBech32(m.ValidatorAddress); err == nil {
		return []sdk.AccAddress{sdk.AccAddress(valAddr)}
	}
	addr, _ := sdk.AccAddressFromBech32(m.SupernodeAccount)
	return []sdk.AccAddress{addr}
}
