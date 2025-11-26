package keeper

import (
	"context"
	"strings"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

func (m msgServer) ReportSupernodeMetrics(goCtx context.Context, msg *types.MsgReportSupernodeMetrics) (*types.MsgReportSupernodeMetricsResponse, error) {
	if msg == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "message cannot be nil")
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	valAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return nil, errorsmod.Wrap(err, "invalid validator address")
	}

	sn, found := m.QuerySuperNode(ctx, valAddr)
	if !found {
		return nil, errorsmod.Wrapf(sdkerrors.ErrNotFound, "supernode for validator %s not found", msg.ValidatorAddress)
	}

	params := m.GetParams(ctx)
	issues := evaluateCompliance(ctx, params, sn, msg.Metrics)
	compliant := len(issues) == 0

	if sn.Metrics == nil {
		sn.Metrics = &types.MetricsAggregate{}
	}
	sn.Metrics.Metrics = msg.Metrics
	sn.Metrics.ReportCount++
	sn.Metrics.Height = ctx.BlockHeight()

	// State transition handling
	if len(sn.States) > 0 {
		lastState := sn.States[len(sn.States)-1].State
		if compliant {
			if lastState == types.SuperNodeStatePostponed {
				target := lastNonPostponedState(sn.States)
				if err := recoverFromPostponed(ctx, m.SupernodeKeeper, &sn, target); err != nil {
					return nil, err
				}
			}
		} else {
			if lastState != types.SuperNodeStatePostponed {
				if err := markPostponed(ctx, m.SupernodeKeeper, &sn, strings.Join(issues, ";")); err != nil {
					return nil, err
				}
			}
		}
	}

	if err := m.SetSuperNode(ctx, sn); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeMetricsReported,
			sdk.NewAttribute(types.AttributeKeyValidatorAddress, msg.ValidatorAddress),
			sdk.NewAttribute(types.AttributeKeySupernodeAccount, msg.SupernodeAccount),
			sdk.NewAttribute(types.AttributeKeyCompliant, boolToString(compliant)),
			sdk.NewAttribute(types.AttributeKeyIssues, strings.Join(issues, ";")),
			sdk.NewAttribute(types.AttributeKeyHeight, stringHeight(ctx.BlockHeight())),
		),
	)

	return &types.MsgReportSupernodeMetricsResponse{Compliant: compliant, Issues: issues}, nil
}

func boolToString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
