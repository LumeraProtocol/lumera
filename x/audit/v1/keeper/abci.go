package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

func (k Keeper) BeginBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx).WithDefaults()

	origin := k.getOrInitWindowOriginHeight(sdkCtx)
	currentWindowID := k.windowIDAtHeight(origin, params, sdkCtx.BlockHeight())
	windowStart := k.windowStartHeight(origin, params, currentWindowID)

	// Only create the snapshot exactly at the window start height.
	if sdkCtx.BlockHeight() != windowStart {
		return nil
	}

	return k.CreateWindowSnapshotIfNeeded(sdkCtx, currentWindowID, params)
}

func (k Keeper) EndBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx).WithDefaults()

	origin := k.getOrInitWindowOriginHeight(sdkCtx)
	height := sdkCtx.BlockHeight()
	grace := int64(params.MissingReportGraceBlocks)
	if height < origin+grace+1 {
		return nil
	}

	candidateHeight := height - grace - 1
	if candidateHeight < origin {
		return nil
	}

	windowToEnforce := k.windowIDAtHeight(origin, params, candidateHeight)
	enforceHeight := k.windowEndHeight(origin, params, windowToEnforce) + grace + 1
	if height != enforceHeight {
		return nil
	}

	snap, found := k.GetWindowSnapshot(sdkCtx, windowToEnforce)
	if !found {
		// Snapshot missing means we cannot deterministically decide who was required to report.
		return nil
	}

	for _, sender := range snap.Senders {
		if k.HasReport(sdkCtx, windowToEnforce, sender) {
			continue
		}

		if err := k.postponeSupernodeForMissingReport(sdkCtx, sender, windowToEnforce); err != nil {
			return err
		}

		status, ok := k.GetAuditStatus(sdkCtx, sender)
		if !ok {
			status = types.AuditStatus{
				ValidatorAddress: sender,
			}
		}
		status.Compliant = false
		status.Reasons = []string{"missing_report"}
		if err := k.SetAuditStatus(sdkCtx, status); err != nil {
			return err
		}
	}

	return nil
}

func (k Keeper) postponeSupernodeForMissingReport(ctx sdk.Context, validatorAddress string, windowID uint64) error {
	valAddr, err := sdk.ValAddressFromBech32(validatorAddress)
	if err != nil {
		return err
	}

	sn, found := k.supernodeKeeper.QuerySuperNode(ctx, valAddr)
	if !found {
		return nil
	}
	if len(sn.States) == 0 {
		return nil
	}

	last := sn.States[len(sn.States)-1]
	if last.State == sntypes.SuperNodeStatePostponed {
		return nil
	}

	sn.States = append(sn.States, &sntypes.SuperNodeStateRecord{
		State:  sntypes.SuperNodeStatePostponed,
		Height: ctx.BlockHeight(),
	})

	if err := k.supernodeKeeper.SetSuperNode(ctx, sn); err != nil {
		return err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			sntypes.EventTypeSupernodePostponed,
			sdk.NewAttribute(sntypes.AttributeKeyValidatorAddress, sn.ValidatorAddress),
			sdk.NewAttribute(sntypes.AttributeKeyOldState, last.State.String()),
			sdk.NewAttribute(sntypes.AttributeKeyReason, fmt.Sprintf("audit missing report (window_id=%d)", windowID)),
		),
	)

	return nil
}

