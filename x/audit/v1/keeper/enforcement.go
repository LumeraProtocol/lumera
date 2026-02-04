package keeper

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// EnforceWindowEnd evaluates the completed window and updates supernode states accordingly.
// It does not re-check peer assignment gating; that must be enforced at MsgSubmitAuditReport time.
func (k Keeper) EnforceWindowEnd(ctx sdk.Context, windowID uint64, params types.Params) error {
	params = params.WithDefaults()

	active, err := k.supernodeKeeper.GetAllSuperNodes(ctx, sntypes.SuperNodeStateActive)
	if err != nil {
		return err
	}
	postponed, err := k.supernodeKeeper.GetAllSuperNodes(ctx, sntypes.SuperNodeStatePostponed)
	if err != nil {
		return err
	}

	// Postpone ACTIVE supernodes that fail criteria.
	for _, sn := range active {
		if sn.SupernodeAccount == "" {
			continue
		}

		shouldPostpone, reason, err := k.shouldPostponeAtWindowEnd(ctx, sn.SupernodeAccount, windowID, params)
		if err != nil {
			return err
		}
		if !shouldPostpone {
			continue
		}

		if err := k.setSupernodePostponed(ctx, sn, reason); err != nil {
			return err
		}
	}

	// Recover POSTPONED supernodes that meet recovery criteria.
	for _, sn := range postponed {
		if sn.SupernodeAccount == "" {
			continue
		}

		shouldRecover, err := k.shouldRecoverAtWindowEnd(ctx, sn.SupernodeAccount, windowID, params)
		if err != nil {
			return err
		}
		if !shouldRecover {
			continue
		}

		if err := k.recoverSupernodeActive(ctx, sn); err != nil {
			return err
		}
	}

	return nil
}

func (k Keeper) shouldPostponeAtWindowEnd(ctx sdk.Context, supernodeAccount string, windowID uint64, params types.Params) (bool, string, error) {
	// Missing-report based postponement.
	consecutive := params.ConsecutiveWindowsToPostpone
	if consecutive == 0 {
		consecutive = 1
	}
	if k.missingReportsForConsecutiveWindows(ctx, supernodeAccount, windowID, consecutive) {
		return true, "audit_missing_reports", nil
	}

	// Self host-metrics-based postponement (if a self report exists and violates minimums).
	if ok, err := k.selfHostViolatesMinimums(ctx, supernodeAccount, windowID, params); err != nil {
		return false, "", err
	} else if ok {
		return true, "audit_host_requirements", nil
	}

	// Peer ports-based postponement.
	requiredPortsLen := len(params.RequiredOpenPorts)
	if requiredPortsLen == 0 {
		return false, "", nil
	}

	if consecutive > uint32(windowID+1) {
		// Not enough history on-chain to satisfy the consecutive rule.
		return false, "", nil
	}

	for portIndex := 0; portIndex < requiredPortsLen; portIndex++ {
		streak := uint32(0)
		for offset := uint32(0); offset < consecutive; offset++ {
			w := windowID - uint64(offset)
			closed, err := k.peersPortStateMeetsThreshold(ctx, supernodeAccount, w, portIndex, types.PortState_PORT_STATE_CLOSED, params.PeerPortPostponeThresholdPercent)
			if err != nil {
				return false, "", err
			}
			if !closed {
				break
			}
			streak++
		}
		if streak == consecutive {
			return true, "audit_peer_ports", nil
		}
	}

	return false, "", nil
}

func (k Keeper) shouldRecoverAtWindowEnd(ctx sdk.Context, supernodeAccount string, windowID uint64, params types.Params) (bool, error) {
	// Need one compliant self report.
	selfCompliant, err := k.selfHostCompliant(ctx, supernodeAccount, windowID, params)
	if err != nil || !selfCompliant {
		return false, err
	}

	// Need at least one compliant peer report that shows all required ports OPEN.
	requiredPortsLen := len(params.RequiredOpenPorts)
	if requiredPortsLen == 0 {
		return true, nil
	}

	peers, err := k.peerReportersForTargetWindow(ctx, supernodeAccount, windowID)
	if err != nil {
		return false, err
	}
	if len(peers) == 0 {
		return false, nil
	}

	// Recovery requires at least one peer report that shows all required ports OPEN for this supernode in this window.
	for _, reporter := range peers {
		r, found := k.GetReport(ctx, windowID, reporter)
		if !found {
			continue
		}

		var obs *types.AuditPeerObservation
		for i := range r.PeerObservations {
			if r.PeerObservations[i] != nil && r.PeerObservations[i].TargetSupernodeAccount == supernodeAccount {
				obs = r.PeerObservations[i]
				break
			}
		}
		if obs == nil {
			continue
		}
		if len(obs.PortStates) != requiredPortsLen {
			continue
		}

		allOpen := true
		for portIndex := 0; portIndex < requiredPortsLen; portIndex++ {
			if obs.PortStates[portIndex] != types.PortState_PORT_STATE_OPEN {
				allOpen = false
				break
			}
		}
		if allOpen {
			return true, nil
		}
	}

	return false, nil
}

func (k Keeper) selfHostViolatesMinimums(ctx sdk.Context, supernodeAccount string, windowID uint64, params types.Params) (bool, error) {
	r, found := k.GetReport(ctx, windowID, supernodeAccount)
	if !found {
		return false, nil
	}

	// If any known metric is below minimum free%, postpone.
	if violatesMinFree(r.SelfReport.CpuUsagePercent, params.MinCpuFreePercent) {
		return true, nil
	}
	if violatesMinFree(r.SelfReport.MemUsagePercent, params.MinMemFreePercent) {
		return true, nil
	}
	if violatesMinFree(r.SelfReport.DiskUsagePercent, params.MinDiskFreePercent) {
		return true, nil
	}

	return false, nil
}

func (k Keeper) selfHostCompliant(ctx sdk.Context, supernodeAccount string, windowID uint64, params types.Params) (bool, error) {
	r, found := k.GetReport(ctx, windowID, supernodeAccount)
	if !found {
		return false, nil
	}

	if !compliesMinFree(r.SelfReport.CpuUsagePercent, params.MinCpuFreePercent) {
		return false, nil
	}
	if !compliesMinFree(r.SelfReport.MemUsagePercent, params.MinMemFreePercent) {
		return false, nil
	}
	if !compliesMinFree(r.SelfReport.DiskUsagePercent, params.MinDiskFreePercent) {
		return false, nil
	}

	return true, nil
}

func violatesMinFree(usagePercent float64, minFreePercent uint32) bool {
	if minFreePercent == 0 {
		return false
	}
	if usagePercent == 0 {
		// Unknown: skip action.
		return false
	}
	if usagePercent < 0 || usagePercent > 100 {
		return true
	}
	free := 100.0 - usagePercent
	return free < float64(minFreePercent)
}

func compliesMinFree(usagePercent float64, minFreePercent uint32) bool {
	if minFreePercent == 0 {
		return true
	}
	if usagePercent == 0 {
		// Unknown: does not block compliance.
		return true
	}
	if usagePercent < 0 || usagePercent > 100 {
		return false
	}
	free := 100.0 - usagePercent
	return free >= float64(minFreePercent)
}

func (k Keeper) peersPortStateMeetsThreshold(ctx sdk.Context, target string, windowID uint64, portIndex int, desired types.PortState, thresholdPercent uint32) (bool, error) {
	peers, err := k.peerReportersForTargetWindow(ctx, target, windowID)
	if err != nil {
		return false, err
	}
	if len(peers) == 0 {
		return false, nil
	}
	return k.peersPortStateMeetsThresholdWithPeers(ctx, target, windowID, portIndex, desired, thresholdPercent, peers)
}

func (k Keeper) peersPortStateMeetsThresholdWithPeers(ctx sdk.Context, target string, windowID uint64, portIndex int, desired types.PortState, thresholdPercent uint32, peers []string) (bool, error) {
	if len(peers) == 0 {
		return false, nil
	}

	matches := uint64(0)
	for _, reporter := range peers {
		r, found := k.GetReport(ctx, windowID, reporter)
		if !found {
			return false, nil
		}

		var obs *types.AuditPeerObservation
		for i := range r.PeerObservations {
			if r.PeerObservations[i] != nil && r.PeerObservations[i].TargetSupernodeAccount == target {
				obs = r.PeerObservations[i]
				break
			}
		}
		if obs == nil {
			return false, nil
		}
		if portIndex < 0 || portIndex >= len(obs.PortStates) {
			return false, nil
		}
		if obs.PortStates[portIndex] == desired {
			matches++
		}
	}

	// Require a minimum share of matching peer observations.
	total := uint64(len(peers))
	return matches*100 >= uint64(thresholdPercent)*total, nil
}

func (k Keeper) peerReportersForTargetWindow(ctx sdk.Context, target string, windowID uint64) ([]string, error) {
	store := k.kvStore(ctx)
	prefix := types.SupernodeReportIndexWindowPrefix(target, windowID)

	it := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	defer it.Close()

	reporters := make([]string, 0, 8)
	for ; it.Valid(); it.Next() {
		// Key is "<reporter_supernode_account>" under the window-specific prefix.
		key := it.Key()
		if len(key) < len(prefix) {
			return nil, fmt.Errorf("invalid supernode report index key")
		}
		reporter := string(key[len(prefix):])
		if reporter == "" {
			return nil, fmt.Errorf("empty reporter in supernode report index")
		}
		reporters = append(reporters, reporter)
	}
	return reporters, nil
}

func (k Keeper) setSupernodePostponed(ctx sdk.Context, sn sntypes.SuperNode, reason string) error {
	if sn.ValidatorAddress == "" {
		return fmt.Errorf("missing validator address for supernode %q", sn.SupernodeAccount)
	}
	valAddr, err := sdk.ValAddressFromBech32(sn.ValidatorAddress)
	if err != nil {
		return err
	}
	return k.supernodeKeeper.SetSuperNodePostponed(ctx, valAddr, reason)
}

func (k Keeper) recoverSupernodeActive(ctx sdk.Context, sn sntypes.SuperNode) error {
	if sn.ValidatorAddress == "" {
		return fmt.Errorf("missing validator address for supernode %q", sn.SupernodeAccount)
	}
	valAddr, err := sdk.ValAddressFromBech32(sn.ValidatorAddress)
	if err != nil {
		return err
	}
	return k.supernodeKeeper.RecoverSuperNodeFromPostponed(ctx, valAddr)
}

func (k Keeper) missingReportsForConsecutiveWindows(ctx sdk.Context, supernodeAccount string, windowID uint64, consecutive uint32) bool {
	if consecutive == 0 {
		consecutive = 1
	}
	if consecutive > uint32(windowID+1) {
		// Not enough history on-chain to satisfy the consecutive rule.
		return false
	}
	for offset := uint32(0); offset < consecutive; offset++ {
		w := windowID - uint64(offset)
		if k.HasReport(ctx, w, supernodeAccount) {
			return false
		}
	}
	return true
}
