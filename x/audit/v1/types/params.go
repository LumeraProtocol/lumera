package types

import (
	"fmt"
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

	KeyScEnabled                           = []byte("ScEnabled")
	KeyScChallengersPerEpoch               = []byte("ScChallengersPerEpoch")
	KeyScFilesPerChallenger                = []byte("ScFilesPerChallenger")
	KeyScReplicaCount                      = []byte("ScReplicaCount")
	KeyScObserverThreshold                 = []byte("ScObserverThreshold")
	KeyScMinSliceBytes                     = []byte("ScMinSliceBytes")
	KeyScMaxSliceBytes                     = []byte("ScMaxSliceBytes")
	KeyScResponseTimeoutMs                 = []byte("ScResponseTimeoutMs")
	KeyScAffirmationTimeoutMs              = []byte("ScAffirmationTimeoutMs")
	KeyScEvidenceMaxBytes                  = []byte("ScEvidenceMaxBytes")
	KeyScCandidateKeysLookbackEpochs       = []byte("ScCandidateKeysLookbackEpochs")
	KeyScStartJitterMs                     = []byte("ScStartJitterMs")
	KeyScEvidenceSubmitterMustBeChallenger = []byte("ScEvidenceSubmitterMustBeChallenger")
)

var (
	DefaultEpochLengthBlocks                = uint64(400)
	DefaultEpochZeroHeight                  = uint64(1)
	DefaultPeerQuorumReports                = uint32(3)
	DefaultMinProbeTargetsPerEpoch          = uint32(3)
	DefaultMaxProbeTargetsPerEpoch          = uint32(5)
	DefaultRequiredOpenPorts                = []uint32{4444, 4445, 8002}
	DefaultMinCpuFreePercent                = uint32(0)
	DefaultMinMemFreePercent                = uint32(0)
	DefaultMinDiskFreePercent               = uint32(0)
	DefaultConsecutiveEpochsToPostpone      = uint32(1)
	DefaultKeepLastEpochEntries             = uint64(200)
	DefaultPeerPortPostponeThresholdPercent = uint32(100)

	DefaultActionFinalizationSignatureFailureEvidencesPerEpoch = uint32(1)
	DefaultActionFinalizationSignatureFailureConsecutiveEpochs = uint32(1)
	DefaultActionFinalizationNotInTop10EvidencesPerEpoch       = uint32(1)
	DefaultActionFinalizationNotInTop10ConsecutiveEpochs       = uint32(1)
	DefaultActionFinalizationRecoveryEpochs                    = uint32(1)
	DefaultActionFinalizationRecoveryMaxTotalBadEvidences      = uint32(1)

	DefaultScEnabled                           = true
	DefaultScChallengersPerEpoch               = uint32(0) // 0 means auto
	DefaultScFilesPerChallenger                = uint32(2)
	DefaultScReplicaCount                      = uint32(5)
	DefaultScObserverThreshold                 = uint32(2)
	DefaultScMinSliceBytes                     = uint64(1024)
	DefaultScMaxSliceBytes                     = uint64(65536)
	DefaultScResponseTimeoutMs                 = uint64(30000)
	DefaultScAffirmationTimeoutMs              = uint64(30000)
	DefaultScEvidenceMaxBytes                  = uint64(65536)
	DefaultScCandidateKeysLookbackEpochs       = uint32(1)
	DefaultScStartJitterMs                     = uint64(60000)
	DefaultScEvidenceSubmitterMustBeChallenger = true
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
	scFilesPerChallenger uint32,
	scReplicaCount uint32,
	scObserverThreshold uint32,
	scMinSliceBytes uint64,
	scMaxSliceBytes uint64,
	scResponseTimeoutMs uint64,
	scAffirmationTimeoutMs uint64,
	scEvidenceMaxBytes uint64,
	scCandidateKeysLookbackEpochs uint32,
	scStartJitterMs uint64,
	scEvidenceSubmitterMustBeChallenger bool,
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

		ScEnabled:                           scEnabled,
		ScChallengersPerEpoch:               scChallengersPerEpoch,
		ScFilesPerChallenger:                scFilesPerChallenger,
		ScReplicaCount:                      scReplicaCount,
		ScObserverThreshold:                 scObserverThreshold,
		ScMinSliceBytes:                     scMinSliceBytes,
		ScMaxSliceBytes:                     scMaxSliceBytes,
		ScResponseTimeoutMs:                 scResponseTimeoutMs,
		ScAffirmationTimeoutMs:              scAffirmationTimeoutMs,
		ScEvidenceMaxBytes:                  scEvidenceMaxBytes,
		ScCandidateKeysLookbackEpochs:       scCandidateKeysLookbackEpochs,
		ScStartJitterMs:                     scStartJitterMs,
		ScEvidenceSubmitterMustBeChallenger: scEvidenceSubmitterMustBeChallenger,
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
		DefaultScFilesPerChallenger,
		DefaultScReplicaCount,
		DefaultScObserverThreshold,
		DefaultScMinSliceBytes,
		DefaultScMaxSliceBytes,
		DefaultScResponseTimeoutMs,
		DefaultScAffirmationTimeoutMs,
		DefaultScEvidenceMaxBytes,
		DefaultScCandidateKeysLookbackEpochs,
		DefaultScStartJitterMs,
		DefaultScEvidenceSubmitterMustBeChallenger,
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

	if p.ScFilesPerChallenger == 0 {
		p.ScFilesPerChallenger = DefaultScFilesPerChallenger
	}
	if p.ScReplicaCount == 0 {
		p.ScReplicaCount = DefaultScReplicaCount
	}
	if p.ScObserverThreshold == 0 {
		p.ScObserverThreshold = DefaultScObserverThreshold
	}
	if p.ScMinSliceBytes == 0 {
		p.ScMinSliceBytes = DefaultScMinSliceBytes
	}
	if p.ScMaxSliceBytes == 0 {
		p.ScMaxSliceBytes = DefaultScMaxSliceBytes
	}
	if p.ScResponseTimeoutMs == 0 {
		p.ScResponseTimeoutMs = DefaultScResponseTimeoutMs
	}
	if p.ScAffirmationTimeoutMs == 0 {
		p.ScAffirmationTimeoutMs = DefaultScAffirmationTimeoutMs
	}
	if p.ScEvidenceMaxBytes == 0 {
		p.ScEvidenceMaxBytes = DefaultScEvidenceMaxBytes
	}
	if p.ScCandidateKeysLookbackEpochs == 0 {
		p.ScCandidateKeysLookbackEpochs = DefaultScCandidateKeysLookbackEpochs
	}
	if p.ScStartJitterMs == 0 {
		p.ScStartJitterMs = DefaultScStartJitterMs
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
		paramtypes.NewParamSetPair(KeyScFilesPerChallenger, &p.ScFilesPerChallenger, validateUint32),
		paramtypes.NewParamSetPair(KeyScReplicaCount, &p.ScReplicaCount, validateUint32),
		paramtypes.NewParamSetPair(KeyScObserverThreshold, &p.ScObserverThreshold, validateUint32),
		paramtypes.NewParamSetPair(KeyScMinSliceBytes, &p.ScMinSliceBytes, validateUint64),
		paramtypes.NewParamSetPair(KeyScMaxSliceBytes, &p.ScMaxSliceBytes, validateUint64),
		paramtypes.NewParamSetPair(KeyScResponseTimeoutMs, &p.ScResponseTimeoutMs, validateUint64),
		paramtypes.NewParamSetPair(KeyScAffirmationTimeoutMs, &p.ScAffirmationTimeoutMs, validateUint64),
		paramtypes.NewParamSetPair(KeyScEvidenceMaxBytes, &p.ScEvidenceMaxBytes, validateUint64),
		paramtypes.NewParamSetPair(KeyScCandidateKeysLookbackEpochs, &p.ScCandidateKeysLookbackEpochs, validateUint32),
		paramtypes.NewParamSetPair(KeyScStartJitterMs, &p.ScStartJitterMs, validateUint64),
		paramtypes.NewParamSetPair(KeyScEvidenceSubmitterMustBeChallenger, &p.ScEvidenceSubmitterMustBeChallenger, validateBool),
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

	if p.ScEnabled {
		if p.ScFilesPerChallenger == 0 {
			return fmt.Errorf("sc_files_per_challenger must be > 0")
		}
		if p.ScReplicaCount == 0 {
			return fmt.Errorf("sc_replica_count must be > 0")
		}
		if p.ScObserverThreshold == 0 {
			return fmt.Errorf("sc_observer_threshold must be > 0")
		}
		if p.ScObserverThreshold > p.ScReplicaCount {
			return fmt.Errorf("sc_observer_threshold must be <= sc_replica_count")
		}
		if p.ScMinSliceBytes == 0 || p.ScMaxSliceBytes == 0 {
			return fmt.Errorf("sc_min_slice_bytes and sc_max_slice_bytes must be > 0")
		}
		if p.ScMinSliceBytes > p.ScMaxSliceBytes {
			return fmt.Errorf("sc_min_slice_bytes must be <= sc_max_slice_bytes")
		}
		if p.ScResponseTimeoutMs == 0 {
			return fmt.Errorf("sc_response_timeout_ms must be > 0")
		}
		if p.ScAffirmationTimeoutMs == 0 {
			return fmt.Errorf("sc_affirmation_timeout_ms must be > 0")
		}
		if p.ScEvidenceMaxBytes == 0 {
			return fmt.Errorf("sc_evidence_max_bytes must be > 0")
		}
		if p.ScCandidateKeysLookbackEpochs == 0 {
			return fmt.Errorf("sc_candidate_keys_lookback_epochs must be > 0")
		}
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
