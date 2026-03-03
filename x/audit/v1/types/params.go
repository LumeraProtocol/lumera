package types

import (
	"fmt"
	"math"
	"sort"

	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

var _ paramtypes.ParamSet = (*Params)(nil)

var (
	KeyEpochLengthBlocks                = []byte("EpochLengthBlocks")
	KeyEpochZeroHeight                  = []byte("EpochZeroHeight")
	KeyPeerQuorumReports                = []byte("PeerQuorumReports")
	KeyMinProbeTargetsPerEpoch          = []byte("MinProbeTargetsPerEpoch")
	KeyMaxProbeTargetsPerEpoch          = []byte("MaxProbeTargetsPerEpoch")
	KeyRequiredOpenPorts                = []byte("RequiredOpenPorts")
	KeyMinCpuFreePercent                = []byte("MinCpuFreePercent")
	KeyMinMemFreePercent                = []byte("MinMemFreePercent")
	KeyMinDiskFreePercent               = []byte("MinDiskFreePercent")
	KeyConsecutiveEpochsToPostpone      = []byte("ConsecutiveEpochsToPostpone")
	KeyKeepLastEpochEntries             = []byte("KeepLastEpochEntries")
	KeyPeerPortPostponeThresholdPercent = []byte("PeerPortPostponeThresholdPercent")

	KeyActionFinalizationSignatureFailureEvidencesPerEpoch = []byte("ActionFinalizationSignatureFailureEvidencesPerEpoch")
	KeyActionFinalizationSignatureFailureConsecutiveEpochs = []byte("ActionFinalizationSignatureFailureConsecutiveEpochs")
	KeyActionFinalizationNotInTop10EvidencesPerEpoch       = []byte("ActionFinalizationNotInTop10EvidencesPerEpoch")
	KeyActionFinalizationNotInTop10ConsecutiveEpochs       = []byte("ActionFinalizationNotInTop10ConsecutiveEpochs")
	KeyActionFinalizationRecoveryEpochs                    = []byte("ActionFinalizationRecoveryEpochs")
	KeyActionFinalizationRecoveryMaxTotalBadEvidences      = []byte("ActionFinalizationRecoveryMaxTotalBadEvidences")

	KeyScEnabled             = []byte("ScEnabled")
	KeyScChallengersPerEpoch = []byte("ScChallengersPerEpoch")
)

var (
	DefaultEpochLengthBlocks = uint64(400)
	// DefaultEpochZeroHeight is a placeholder used for genesis-based initialization.
	// For module activation on an already-running chain, the upgrade handler must set
	// epoch_zero_height dynamically to the upgrade block height.
	DefaultEpochZeroHeight = uint64(1)

	// DefaultPeerQuorumReports is the desired average number of peer observations per target per epoch.
	// This indirectly drives how many targets each prober must observe in an epoch.
	DefaultPeerQuorumReports = uint32(3)

	// DefaultMinProbeTargetsPerEpoch clamps the per-prober target assignment to a minimum.
	DefaultMinProbeTargetsPerEpoch = uint32(3)

	// DefaultMaxProbeTargetsPerEpoch clamps the per-prober target assignment to a maximum.
	DefaultMaxProbeTargetsPerEpoch = uint32(5)

	// DefaultRequiredOpenPorts is the ordered list of ports that probers must check and report for assigned targets.
	// The index in peer observations corresponds to the index in this list.
	DefaultRequiredOpenPorts = []uint32{4444, 4445, 8002}

	// DefaultMin*FreePercent are minimum required free-capacity thresholds derived from self reports.
	// A value of 0 disables that resource-based postponement.
	DefaultMinCpuFreePercent  = uint32(0)
	DefaultMinMemFreePercent  = uint32(0)
	DefaultMinDiskFreePercent = uint32(0)

	// DefaultConsecutiveEpochsToPostpone is the lookback window for postponement decisions that rely on
	// consecutive-epoch streaks (missing reports and peer port closures).
	DefaultConsecutiveEpochsToPostpone = uint32(1)

	// DefaultKeepLastEpochEntries is the retention window for epoch-scoped state (anchors, reports, indices,
	// evidence epoch counters). It must be >= the maximum configured lookback windows.
	DefaultKeepLastEpochEntries = uint64(200)

	// DefaultPeerPortPostponeThresholdPercent is the percentage of peer reporters that must report a required
	// port as CLOSED to treat that port as CLOSED for the epoch. 100 means unanimous.
	DefaultPeerPortPostponeThresholdPercent = uint32(100)

	// DefaultActionFinalization* are action-finalization evidence-based postponement thresholds.
	DefaultActionFinalizationSignatureFailureEvidencesPerEpoch = uint32(1)
	DefaultActionFinalizationSignatureFailureConsecutiveEpochs = uint32(1)
	DefaultActionFinalizationNotInTop10EvidencesPerEpoch       = uint32(1)
	DefaultActionFinalizationNotInTop10ConsecutiveEpochs       = uint32(1)

	// DefaultActionFinalizationRecovery* define the action-finalization recovery window.
	DefaultActionFinalizationRecoveryEpochs               = uint32(1)
	DefaultActionFinalizationRecoveryMaxTotalBadEvidences = uint32(1)

	// DefaultScEnabled is the storage challenge feature gate (supernode-side execution and chain-side evidence validation).
	DefaultScEnabled = true

	// DefaultScChallengersPerEpoch is the number of challengers selected per epoch from the anchored ACTIVE set.
	// A value of 0 means "auto" (implementation-defined default).
	DefaultScChallengersPerEpoch = uint32(0) // 0 means auto
)

// Params notes
//
// Params are initialized from genesis and may later be updated by governance via
// `MsgUpdateParams` (with some fields immutable; see keeper/msg_update_params.go).
//
// - epoch_length_blocks: fixed-length epoch size in blocks (immutable after genesis).
// - epoch_zero_height: reference height at which epoch_id=0 starts (immutable after genesis).
// - peer_quorum_reports: desired number of peer observations per receiver (drives per-epoch target count).
// - min/max_probe_targets_per_epoch: clamps the computed target count to a safe range.
// - required_open_ports: ports every peer observation must cover (ordering matters; see keeper enforcement).
// - min_*_free_percent: minimum required free capacity from self report (0 disables).
// - consecutive_epochs_to_postpone: lookback window for missing-report/peer-port postponement decisions.
// - peer_port_postpone_threshold_percent: percent of peers that must report CLOSED to treat a port as CLOSED.
// - keep_last_epoch_entries: how many epochs of epoch-scoped state to keep (pruning at epoch end).
// - action_finalization_*: postponement + recovery windows for action-finalization evidence types.

func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

func NewParams(
	epochLengthBlocks uint64,
	epochZeroHeight uint64,
	peerQuorumReports uint32,
	minProbeTargetsPerEpoch uint32,
	maxProbeTargetsPerEpoch uint32,
	requiredOpenPorts []uint32,
	minCpuFreePercent uint32,
	minMemFreePercent uint32,
	minDiskFreePercent uint32,
	consecutiveEpochsToPostpone uint32,
	keepLastEpochEntries uint64,
	peerPortPostponeThresholdPercent uint32,
	actionFinalizationSignatureFailureEvidencesPerEpoch uint32,
	actionFinalizationSignatureFailureConsecutiveEpochs uint32,
	actionFinalizationNotInTop10EvidencesPerEpoch uint32,
	actionFinalizationNotInTop10ConsecutiveEpochs uint32,
	actionFinalizationRecoveryEpochs uint32,
	actionFinalizationRecoveryMaxTotalBadEvidences uint32,
	scEnabled bool,
	scChallengersPerEpoch uint32,
) Params {
	return Params{
		EpochLengthBlocks:                epochLengthBlocks,
		EpochZeroHeight:                  epochZeroHeight,
		PeerQuorumReports:                peerQuorumReports,
		MinProbeTargetsPerEpoch:          minProbeTargetsPerEpoch,
		MaxProbeTargetsPerEpoch:          maxProbeTargetsPerEpoch,
		RequiredOpenPorts:                requiredOpenPorts,
		MinCpuFreePercent:                minCpuFreePercent,
		MinMemFreePercent:                minMemFreePercent,
		MinDiskFreePercent:               minDiskFreePercent,
		ConsecutiveEpochsToPostpone:      consecutiveEpochsToPostpone,
		KeepLastEpochEntries:             keepLastEpochEntries,
		PeerPortPostponeThresholdPercent: peerPortPostponeThresholdPercent,

		ActionFinalizationSignatureFailureEvidencesPerEpoch: actionFinalizationSignatureFailureEvidencesPerEpoch,
		ActionFinalizationSignatureFailureConsecutiveEpochs: actionFinalizationSignatureFailureConsecutiveEpochs,
		ActionFinalizationNotInTop10EvidencesPerEpoch:       actionFinalizationNotInTop10EvidencesPerEpoch,
		ActionFinalizationNotInTop10ConsecutiveEpochs:       actionFinalizationNotInTop10ConsecutiveEpochs,
		ActionFinalizationRecoveryEpochs:                    actionFinalizationRecoveryEpochs,
		ActionFinalizationRecoveryMaxTotalBadEvidences:      actionFinalizationRecoveryMaxTotalBadEvidences,

		ScEnabled:             scEnabled,
		ScChallengersPerEpoch: scChallengersPerEpoch,
	}
}

func DefaultParams() Params {
	return NewParams(
		DefaultEpochLengthBlocks,
		DefaultEpochZeroHeight,
		DefaultPeerQuorumReports,
		DefaultMinProbeTargetsPerEpoch,
		DefaultMaxProbeTargetsPerEpoch,
		append([]uint32(nil), DefaultRequiredOpenPorts...),
		DefaultMinCpuFreePercent,
		DefaultMinMemFreePercent,
		DefaultMinDiskFreePercent,
		DefaultConsecutiveEpochsToPostpone,
		DefaultKeepLastEpochEntries,
		DefaultPeerPortPostponeThresholdPercent,
		DefaultActionFinalizationSignatureFailureEvidencesPerEpoch,
		DefaultActionFinalizationSignatureFailureConsecutiveEpochs,
		DefaultActionFinalizationNotInTop10EvidencesPerEpoch,
		DefaultActionFinalizationNotInTop10ConsecutiveEpochs,
		DefaultActionFinalizationRecoveryEpochs,
		DefaultActionFinalizationRecoveryMaxTotalBadEvidences,
		DefaultScEnabled,
		DefaultScChallengersPerEpoch,
	)
}

func (p Params) WithDefaults() Params {
	if p.EpochLengthBlocks == 0 {
		p.EpochLengthBlocks = DefaultEpochLengthBlocks
	}
	if p.EpochZeroHeight == 0 {
		p.EpochZeroHeight = DefaultEpochZeroHeight
	}
	if p.PeerQuorumReports == 0 {
		p.PeerQuorumReports = DefaultPeerQuorumReports
	}
	if p.MinProbeTargetsPerEpoch == 0 {
		p.MinProbeTargetsPerEpoch = DefaultMinProbeTargetsPerEpoch
	}
	if p.MaxProbeTargetsPerEpoch == 0 {
		p.MaxProbeTargetsPerEpoch = DefaultMaxProbeTargetsPerEpoch
	}
	if len(p.RequiredOpenPorts) == 0 {
		p.RequiredOpenPorts = append([]uint32(nil), DefaultRequiredOpenPorts...)
	}
	if p.ConsecutiveEpochsToPostpone == 0 {
		p.ConsecutiveEpochsToPostpone = DefaultConsecutiveEpochsToPostpone
	}
	if p.KeepLastEpochEntries == 0 {
		p.KeepLastEpochEntries = DefaultKeepLastEpochEntries
	}
	if p.PeerPortPostponeThresholdPercent == 0 {
		p.PeerPortPostponeThresholdPercent = DefaultPeerPortPostponeThresholdPercent
	}
	if p.ActionFinalizationSignatureFailureEvidencesPerEpoch == 0 {
		p.ActionFinalizationSignatureFailureEvidencesPerEpoch = DefaultActionFinalizationSignatureFailureEvidencesPerEpoch
	}
	if p.ActionFinalizationSignatureFailureConsecutiveEpochs == 0 {
		p.ActionFinalizationSignatureFailureConsecutiveEpochs = DefaultActionFinalizationSignatureFailureConsecutiveEpochs
	}
	if p.ActionFinalizationNotInTop10EvidencesPerEpoch == 0 {
		p.ActionFinalizationNotInTop10EvidencesPerEpoch = DefaultActionFinalizationNotInTop10EvidencesPerEpoch
	}
	if p.ActionFinalizationNotInTop10ConsecutiveEpochs == 0 {
		p.ActionFinalizationNotInTop10ConsecutiveEpochs = DefaultActionFinalizationNotInTop10ConsecutiveEpochs
	}
	if p.ActionFinalizationRecoveryEpochs == 0 {
		p.ActionFinalizationRecoveryEpochs = DefaultActionFinalizationRecoveryEpochs
	}
	if p.ActionFinalizationRecoveryMaxTotalBadEvidences == 0 {
		p.ActionFinalizationRecoveryMaxTotalBadEvidences = DefaultActionFinalizationRecoveryMaxTotalBadEvidences
	}

	return p
}

func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(KeyEpochLengthBlocks, &p.EpochLengthBlocks, validateUint64),
		paramtypes.NewParamSetPair(KeyEpochZeroHeight, &p.EpochZeroHeight, validateUint64),
		paramtypes.NewParamSetPair(KeyPeerQuorumReports, &p.PeerQuorumReports, validateUint32),
		paramtypes.NewParamSetPair(KeyMinProbeTargetsPerEpoch, &p.MinProbeTargetsPerEpoch, validateUint32),
		paramtypes.NewParamSetPair(KeyMaxProbeTargetsPerEpoch, &p.MaxProbeTargetsPerEpoch, validateUint32),
		paramtypes.NewParamSetPair(KeyRequiredOpenPorts, &p.RequiredOpenPorts, validateUint32Slice),
		paramtypes.NewParamSetPair(KeyMinCpuFreePercent, &p.MinCpuFreePercent, validateUint32),
		paramtypes.NewParamSetPair(KeyMinMemFreePercent, &p.MinMemFreePercent, validateUint32),
		paramtypes.NewParamSetPair(KeyMinDiskFreePercent, &p.MinDiskFreePercent, validateUint32),
		paramtypes.NewParamSetPair(KeyConsecutiveEpochsToPostpone, &p.ConsecutiveEpochsToPostpone, validateUint32),
		paramtypes.NewParamSetPair(KeyKeepLastEpochEntries, &p.KeepLastEpochEntries, validateUint64),
		paramtypes.NewParamSetPair(KeyPeerPortPostponeThresholdPercent, &p.PeerPortPostponeThresholdPercent, validateUint32),

		paramtypes.NewParamSetPair(KeyActionFinalizationSignatureFailureEvidencesPerEpoch, &p.ActionFinalizationSignatureFailureEvidencesPerEpoch, validateUint32),
		paramtypes.NewParamSetPair(KeyActionFinalizationSignatureFailureConsecutiveEpochs, &p.ActionFinalizationSignatureFailureConsecutiveEpochs, validateUint32),
		paramtypes.NewParamSetPair(KeyActionFinalizationNotInTop10EvidencesPerEpoch, &p.ActionFinalizationNotInTop10EvidencesPerEpoch, validateUint32),
		paramtypes.NewParamSetPair(KeyActionFinalizationNotInTop10ConsecutiveEpochs, &p.ActionFinalizationNotInTop10ConsecutiveEpochs, validateUint32),
		paramtypes.NewParamSetPair(KeyActionFinalizationRecoveryEpochs, &p.ActionFinalizationRecoveryEpochs, validateUint32),
		paramtypes.NewParamSetPair(KeyActionFinalizationRecoveryMaxTotalBadEvidences, &p.ActionFinalizationRecoveryMaxTotalBadEvidences, validateUint32),

		paramtypes.NewParamSetPair(KeyScEnabled, &p.ScEnabled, validateBool),
		paramtypes.NewParamSetPair(KeyScChallengersPerEpoch, &p.ScChallengersPerEpoch, validateUint32),
	}
}

func (p Params) Validate() error {
	p = p.WithDefaults()

	if p.EpochLengthBlocks == 0 {
		return fmt.Errorf("epoch_length_blocks must be > 0")
	}
	if p.EpochZeroHeight == 0 {
		return fmt.Errorf("epoch_zero_height must be > 0")
	}
	// Epoch math currently operates on int64 heights. Guard against values that would overflow
	// when converted (epoch_zero_height and epoch_length_blocks are uint64 on-chain).
	if p.EpochLengthBlocks > uint64(math.MaxInt64) {
		return fmt.Errorf("epoch_length_blocks must be <= %d", int64(math.MaxInt64))
	}
	if p.EpochZeroHeight > uint64(math.MaxInt64) {
		return fmt.Errorf("epoch_zero_height must be <= %d", int64(math.MaxInt64))
	}
	if p.PeerQuorumReports == 0 {
		return fmt.Errorf("peer_quorum_reports must be > 0")
	}
	if p.MinProbeTargetsPerEpoch > p.MaxProbeTargetsPerEpoch {
		return fmt.Errorf("min_probe_targets_per_epoch must be <= max_probe_targets_per_epoch")
	}
	if len(p.RequiredOpenPorts) == 0 {
		return fmt.Errorf("required_open_ports must not be empty")
	}
	if p.MinCpuFreePercent > 100 {
		return fmt.Errorf("min_cpu_free_percent must be <= 100")
	}
	if p.MinMemFreePercent > 100 {
		return fmt.Errorf("min_mem_free_percent must be <= 100")
	}
	if p.MinDiskFreePercent > 100 {
		return fmt.Errorf("min_disk_free_percent must be <= 100")
	}
	if p.ConsecutiveEpochsToPostpone == 0 {
		return fmt.Errorf("consecutive_epochs_to_postpone must be > 0")
	}
	if p.KeepLastEpochEntries == 0 {
		return fmt.Errorf("keep_last_epoch_entries must be > 0")
	}
	// keep_last_epoch_entries must retain enough history to evaluate epoch-end rules that
	// look back across multiple epochs. If history is pruned earlier than these lookbacks,
	// enforcement becomes incorrect (false postponements or disabled postponements).
	{
		requiredHistory := uint64(p.ConsecutiveEpochsToPostpone)
		if v := uint64(p.ActionFinalizationSignatureFailureConsecutiveEpochs); v > requiredHistory {
			requiredHistory = v
		}
		if v := uint64(p.ActionFinalizationNotInTop10ConsecutiveEpochs); v > requiredHistory {
			requiredHistory = v
		}
		if v := uint64(p.ActionFinalizationRecoveryEpochs); v > requiredHistory {
			requiredHistory = v
		}
		if requiredHistory > 0 && p.KeepLastEpochEntries < requiredHistory {
			return fmt.Errorf("keep_last_epoch_entries must be >= max epoch lookback windows (need >= %d)", requiredHistory)
		}
	}
	if p.PeerPortPostponeThresholdPercent == 0 || p.PeerPortPostponeThresholdPercent > 100 {
		return fmt.Errorf("peer_port_postpone_threshold_percent must be within 1..100")
	}
	if p.ActionFinalizationSignatureFailureEvidencesPerEpoch == 0 {
		return fmt.Errorf("action_finalization_signature_failure_evidences_per_epoch must be > 0")
	}
	if p.ActionFinalizationSignatureFailureConsecutiveEpochs == 0 {
		return fmt.Errorf("action_finalization_signature_failure_consecutive_epochs must be > 0")
	}
	if p.ActionFinalizationNotInTop10EvidencesPerEpoch == 0 {
		return fmt.Errorf("action_finalization_not_in_top_10_evidences_per_epoch must be > 0")
	}
	if p.ActionFinalizationNotInTop10ConsecutiveEpochs == 0 {
		return fmt.Errorf("action_finalization_not_in_top_10_consecutive_epochs must be > 0")
	}
	if p.ActionFinalizationRecoveryEpochs == 0 {
		return fmt.Errorf("action_finalization_recovery_epochs must be > 0")
	}
	if p.ActionFinalizationRecoveryMaxTotalBadEvidences == 0 {
		return fmt.Errorf("action_finalization_recovery_max_total_bad_evidences must be > 0")
	}

	ports := append([]uint32(nil), p.RequiredOpenPorts...)
	sort.Slice(ports, func(i, j int) bool { return ports[i] < ports[j] })
	for i := 1; i < len(ports); i++ {
		if ports[i] == ports[i-1] {
			return fmt.Errorf("required_open_ports must be unique")
		}
	}

	return nil
}

func validateUint64(v interface{}) error {
	_, ok := v.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}
	return nil
}

func validateUint32(v interface{}) error {
	_, ok := v.(uint32)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}
	return nil
}

func validateUint32Slice(v interface{}) error {
	_, ok := v.([]uint32)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}
	return nil
}

func validateBool(v interface{}) error {
	_, ok := v.(bool)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}
	return nil
}
