package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

func (m msgServer) SubmitAuditReport(ctx context.Context, req *types.MsgSubmitAuditReport) (*types.MsgSubmitAuditReportResponse, error) {
	if req == nil {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "empty request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := m.GetParams(ctx).WithDefaults()
	origin := m.getOrInitWindowOriginHeight(sdkCtx)

	// Validate window_id acceptance: allow submitting for a window until end.
	windowStart := m.windowStartHeight(origin, params, req.WindowId)
	windowEnd := m.windowEndHeight(origin, params, req.WindowId)
	if sdkCtx.BlockHeight() < windowStart || sdkCtx.BlockHeight() > windowEnd {
		return nil, errorsmod.Wrapf(types.ErrInvalidWindowID, "window_id not accepted at height %d", sdkCtx.BlockHeight())
	}

	_, found, err := m.supernodeKeeper.GetSuperNodeByAccount(sdkCtx, req.SupernodeAccount)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errorsmod.Wrap(types.ErrReporterNotFound, "unknown supernode_account")
	}

	reporterAccount := req.SupernodeAccount
	if m.HasReport(sdkCtx, req.WindowId, reporterAccount) {
		return nil, errorsmod.Wrap(types.ErrDuplicateReport, "report already submitted for this window")
	}

	report := types.AuditReport{
		SupernodeAccount: reporterAccount,
		WindowId:         req.WindowId,
		ReportHeight:     sdkCtx.BlockHeight(),
		SelfReport:       req.SelfReport,
		PeerObservations: req.PeerObservations,
	}

	if err := m.SetReport(sdkCtx, report); err != nil {
		return nil, err
	}
	m.SetReportIndex(sdkCtx, req.WindowId, reporterAccount)
	m.SetSelfReportIndex(sdkCtx, req.WindowId, reporterAccount)

	seenSupernodes := make(map[string]struct{}, len(req.PeerObservations))
	for _, obs := range req.PeerObservations {
		supernodeAccount := obs.TargetSupernodeAccount
		if supernodeAccount == "" {
			continue
		}
		if _, seen := seenSupernodes[supernodeAccount]; seen {
			continue
		}
		seenSupernodes[supernodeAccount] = struct{}{}
		m.SetSupernodeReportIndex(sdkCtx, supernodeAccount, req.WindowId, reporterAccount)
	}

	return &types.MsgSubmitAuditReportResponse{}, nil
}
