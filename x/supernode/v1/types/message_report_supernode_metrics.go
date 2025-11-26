package types

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ sdk.Msg = &MsgReportSupernodeMetrics{}

// MsgReportSupernodeMetrics reports health metrics for a supernode.
type MsgReportSupernodeMetrics struct {
	Creator          string             `json:"creator"`
	ValidatorAddress string             `json:"validatorAddress"`
	Metrics          map[string]float64 `json:"metrics"`
	Version          string             `json:"version"`
	ReportedHeight   int64              `json:"reported_height"`
}

// MsgReportSupernodeMetricsResponse returns any compliance issues discovered.
type MsgReportSupernodeMetricsResponse struct {
	Issues []string `json:"issues"`
}

func NewMsgReportSupernodeMetrics(creator, validatorAddress, version string, metrics map[string]float64, reportedHeight int64) *MsgReportSupernodeMetrics {
	return &MsgReportSupernodeMetrics{
		Creator:          creator,
		ValidatorAddress: validatorAddress,
		Metrics:          metrics,
		Version:          version,
		ReportedHeight:   reportedHeight,
	}
}

func (msg *MsgReportSupernodeMetrics) Route() string { return RouterKey }

func (msg *MsgReportSupernodeMetrics) Type() string { return "report_supernode_metrics" }

func (msg *MsgReportSupernodeMetrics) GetSigners() []sdk.AccAddress {
	creator, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{creator}
}

func (msg *MsgReportSupernodeMetrics) ValidateBasic() error {
	if msg == nil {
		return fmt.Errorf("msg cannot be nil")
	}

	if _, err := sdk.AccAddressFromBech32(msg.Creator); err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}

	if _, err := sdk.ValAddressFromBech32(msg.ValidatorAddress); err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator operator address (%s)", err)
	}

	if len(msg.Metrics) == 0 {
		return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "metrics cannot be empty")
	}

	return nil
}
