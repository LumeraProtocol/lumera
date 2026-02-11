package types

import "encoding/binary"

const (
	ModuleName = "audit"

	StoreKey    = ModuleName
	MemStoreKey = "mem_audit"
)

var (
	ParamsKey = []byte("p_audit")

	// Key layout notes
	//
	// This module uses human-readable ASCII prefixes plus fixed-width binary epoch IDs.
	// - Prefixes are short and unambiguous when iterating by prefix.
	// - epoch_id is encoded as 8-byte big-endian so lexicographic ordering matches numeric ordering.
	// - supernode accounts are stored as their bech32 string bytes.
	//
	// Epoch anchors:
	// - Each epoch stores an EpochAnchor under "ea/<epoch_id>".
	// - The anchor is intended to be an immutable per-epoch source-of-truth for:
	//   - deterministic seed
	//   - frozen eligible sets at epoch start
	//
	// Formats:
	// - EpochAnchorKey:          "ea/" + u64be(epoch_id)
	// - EpochParamsSnapshotKey:  "eps/" + u64be(epoch_id)
	// - ReportKey:               "r/"  + u64be(epoch_id) + reporter_supernode_account
	// - ReportIndexKey:          "ri/" + reporter_supernode_account + "/" + u64be(epoch_id)
	// - StorageChallengeReportIndexKey: "sc/" + supernode_account + "/" + u64be(epoch_id) + "/" + reporter_supernode_account
	// - HostReportIndexKey:             "hr/" + reporter_supernode_account + "/" + u64be(epoch_id)
	//
	// Examples (shown as pseudo strings; the u64be bytes will appear as non-printable in raw dumps):
	// - EpochAnchorKey(1)          => "ea/" + u64be(1)
	// - ReportKey(1, "<reporter>") => "r/"  + u64be(1) + "<reporter>"
	epochAnchorPrefix = []byte("ea/")
	// epochParamsSnapshotPrefix stores a per-epoch snapshot of assignment/gating-related params.
	// Format: "eps/" + u64be(epoch_id)
	epochParamsSnapshotPrefix = []byte("eps/")
	reportPrefix      = []byte("r/")

	reportIndexPrefix = []byte("ri/")

	// storageChallengeReportIndexPrefix indexes reports that include a storage-challenge observation for a given supernode.
	// Format: "sc/" + supernode_account + "/" + u64be(epoch_id) + "/" + reporter_supernode_account
	storageChallengeReportIndexPrefix = []byte("sc/")

	// hostReportIndexPrefix indexes all submitted reports (for listing host reports across epochs for a reporter).
	// Format: "hr/" + reporter_supernode_account + "/" + u64be(epoch_id)
	hostReportIndexPrefix = []byte("hr/")

	// Evidence:
	// - NextEvidenceIDKey: "ev/next_id" -> 8 bytes u64be(next_evidence_id)
	// - EvidenceKey: "ev/r/" + u64be(evidence_id) -> Evidence bytes
	// - EvidenceBySubjectIndexKey: "ev/s/" + subject_address + "/" + u64be(evidence_id) -> empty
	// - EvidenceByActionIndexKey:  "ev/a/" + action_id + 0x00 + u64be(evidence_id) -> empty
	//
	// Evidence epoch counts (epoch-scoped aggregates used for postponement/recovery):
	// - EvidenceEpochCountKey: "eve/" + u64be(epoch_id) + "/" + subject_address + "/" + u32be(evidence_type) -> 8 bytes u64be(count)
	//
	// Action finalization postponement state:
	// - ActionFinalizationPostponementKey: "ap/af/" + supernode_account -> 8 bytes u64be(postponed_at_epoch_id)
	nextEvidenceIDKey        = []byte("ev/next_id")
	evidenceRecordPrefix     = []byte("ev/r/")
	evidenceBySubjectPrefix  = []byte("ev/s/")
	evidenceByActionIDPrefix = []byte("ev/a/")

	evidenceEpochCountPrefix = []byte("eve/")

	actionFinalizationPostponementPrefix = []byte("ap/af/")
)

// EpochAnchorKey returns the store key for the EpochAnchor identified by epochID.
func EpochAnchorKey(epochID uint64) []byte {
	key := make([]byte, 0, len(epochAnchorPrefix)+8) // "ea/" + u64be(epoch_id)
	key = append(key, epochAnchorPrefix...)
	key = binary.BigEndian.AppendUint64(key, epochID)
	return key
}

func EpochAnchorPrefix() []byte {
	return epochAnchorPrefix
}

// EpochParamsSnapshotKey returns the store key for the per-epoch params snapshot identified by epochID.
func EpochParamsSnapshotKey(epochID uint64) []byte {
	key := make([]byte, 0, len(epochParamsSnapshotPrefix)+8) // "eps/" + u64be(epoch_id)
	key = append(key, epochParamsSnapshotPrefix...)
	key = binary.BigEndian.AppendUint64(key, epochID)
	return key
}

func EpochParamsSnapshotPrefix() []byte {
	return epochParamsSnapshotPrefix
}

// ReportPrefix returns the root prefix for epoch report keys.
//
// Format: "r/" + u64be(epoch_id) + reporter_supernode_account
func ReportPrefix() []byte {
	return reportPrefix
}

// ReportIndexRootPrefix returns the root prefix for report index keys.
//
// Format: "ri/" + reporter_supernode_account + "/" + u64be(epoch_id)
func ReportIndexRootPrefix() []byte {
	return reportIndexPrefix
}

// StorageChallengeReportIndexRootPrefix returns the root prefix for storage-challenge report index keys.
//
// Format: "sc/" + supernode_account + "/" + u64be(epoch_id) + "/" + reporter_supernode_account
func StorageChallengeReportIndexRootPrefix() []byte {
	return storageChallengeReportIndexPrefix
}

// HostReportIndexRootPrefix returns the root prefix for host report index keys.
//
// Format: "hr/" + reporter_supernode_account + "/" + u64be(epoch_id)
func HostReportIndexRootPrefix() []byte {
	return hostReportIndexPrefix
}

// ReportKey returns the store key for the EpochReport identified by (epochID, reporterSupernodeAccount).
func ReportKey(epochID uint64, reporterSupernodeAccount string) []byte {
	key := make([]byte, 0, len(reportPrefix)+8+len(reporterSupernodeAccount)) // "r/" + u64be(epoch_id) + reporter
	key = append(key, reportPrefix...)                                        // "r/"
	key = binary.BigEndian.AppendUint64(key, epochID)                         // u64be(epoch_id)
	key = append(key, reporterSupernodeAccount...)                            // reporter (bech32)
	return key
}

// ReportIndexKey returns the store key for the report index entry identified by (reporterSupernodeAccount, epochID).
// The value is empty; the key exists to allow querying all reports for a reporter without scanning all epochs.
func ReportIndexKey(reporterSupernodeAccount string, epochID uint64) []byte {
	key := make([]byte, 0, len(reportIndexPrefix)+len(reporterSupernodeAccount)+1+8) // "ri/" + reporter + "/" + u64be(epoch_id)
	key = append(key, reportIndexPrefix...)                                          // "ri/"
	key = append(key, reporterSupernodeAccount...)                                   // reporter (bech32)
	key = append(key, '/')                                                           // separator
	key = binary.BigEndian.AppendUint64(key, epochID)                                // u64be(epoch_id)
	return key
}

// ReportIndexPrefix returns the prefix under which report index keys are stored for a reporter.
func ReportIndexPrefix(reporterSupernodeAccount string) []byte {
	key := make([]byte, 0, len(reportIndexPrefix)+len(reporterSupernodeAccount)+1) // "ri/" + reporter + "/"
	key = append(key, reportIndexPrefix...)                                        // "ri/"
	key = append(key, reporterSupernodeAccount...)                                 // reporter (bech32)
	key = append(key, '/')                                                         // separator
	return key
}

// StorageChallengeReportIndexKey returns the store key for an index entry identified by (supernodeAccount, epochID, reporterSupernodeAccount).
// The value is empty; the key exists to allow querying reports about a given supernode without scanning all reports.
func StorageChallengeReportIndexKey(supernodeAccount string, epochID uint64, reporterSupernodeAccount string) []byte {
	key := make([]byte, 0, len(storageChallengeReportIndexPrefix)+len(supernodeAccount)+1+8+1+len(reporterSupernodeAccount)) // "sc/" + supernode + "/" + u64be(epoch_id) + "/" + reporter
	key = append(key, storageChallengeReportIndexPrefix...)                                                                  // "sc/"
	key = append(key, supernodeAccount...)                                                                                   // supernode (bech32)
	key = append(key, '/')                                                                                                   // separator
	key = binary.BigEndian.AppendUint64(key, epochID)                                                                        // u64be(epoch_id)
	key = append(key, '/')                                                                                                   // separator
	key = append(key, reporterSupernodeAccount...)                                                                           // reporter (bech32)
	return key
}

// StorageChallengeReportIndexPrefix returns the prefix under which index keys are stored for a given supernode.
func StorageChallengeReportIndexPrefix(supernodeAccount string) []byte {
	key := make([]byte, 0, len(storageChallengeReportIndexPrefix)+len(supernodeAccount)+1) // "sc/" + supernode + "/"
	key = append(key, storageChallengeReportIndexPrefix...)                                // "sc/"
	key = append(key, supernodeAccount...)                                                 // supernode (bech32)
	key = append(key, '/')                                                                 // separator
	return key
}

// StorageChallengeReportIndexEpochPrefix returns the prefix under which index keys are stored for a given (supernodeAccount, epochID).
func StorageChallengeReportIndexEpochPrefix(supernodeAccount string, epochID uint64) []byte {
	key := make([]byte, 0, len(storageChallengeReportIndexPrefix)+len(supernodeAccount)+1+8+1) // "sc/" + supernode + "/" + u64be(epoch_id) + "/"
	key = append(key, storageChallengeReportIndexPrefix...)                                    // "sc/"
	key = append(key, supernodeAccount...)                                                     // supernode (bech32)
	key = append(key, '/')                                                                     // separator
	key = binary.BigEndian.AppendUint64(key, epochID)                                          // u64be(epoch_id)
	key = append(key, '/')                                                                     // separator
	return key
}

// HostReportIndexKey returns the store key for an index entry identified by (reporterSupernodeAccount, epochID).
// The value is empty; the key exists to allow listing a reporter's host reports across epochs without scanning all report keys.
func HostReportIndexKey(reporterSupernodeAccount string, epochID uint64) []byte {
	key := make([]byte, 0, len(hostReportIndexPrefix)+len(reporterSupernodeAccount)+1+8) // "hr/" + reporter + "/" + u64be(epoch_id)
	key = append(key, hostReportIndexPrefix...)                                          // "hr/"
	key = append(key, reporterSupernodeAccount...)                                       // reporter (bech32)
	key = append(key, '/')                                                               // separator
	key = binary.BigEndian.AppendUint64(key, epochID)                                    // u64be(epoch_id)
	return key
}

// HostReportIndexPrefix returns the prefix under which host report index keys are stored for a given reporter.
func HostReportIndexPrefix(reporterSupernodeAccount string) []byte {
	key := make([]byte, 0, len(hostReportIndexPrefix)+len(reporterSupernodeAccount)+1) // "hr/" + reporter + "/"
	key = append(key, hostReportIndexPrefix...)                                        // "hr/"
	key = append(key, reporterSupernodeAccount...)                                     // reporter (bech32)
	key = append(key, '/')                                                             // separator
	return key
}

func NextEvidenceIDKey() []byte {
	return nextEvidenceIDKey
}

func EvidenceKey(evidenceID uint64) []byte {
	key := make([]byte, 0, len(evidenceRecordPrefix)+8) // "ev/r/" + u64be(evidence_id)
	key = append(key, evidenceRecordPrefix...)
	key = binary.BigEndian.AppendUint64(key, evidenceID)
	return key
}

func EvidenceRecordPrefix() []byte {
	return evidenceRecordPrefix
}

func EvidenceBySubjectIndexKey(subjectAddress string, evidenceID uint64) []byte {
	key := make([]byte, 0, len(evidenceBySubjectPrefix)+len(subjectAddress)+1+8) // "ev/s/" + subject + "/" + u64be(evidence_id)
	key = append(key, evidenceBySubjectPrefix...)
	key = append(key, subjectAddress...)
	key = append(key, '/')
	key = binary.BigEndian.AppendUint64(key, evidenceID)
	return key
}

func EvidenceBySubjectIndexPrefix(subjectAddress string) []byte {
	key := make([]byte, 0, len(evidenceBySubjectPrefix)+len(subjectAddress)+1) // "ev/s/" + subject + "/"
	key = append(key, evidenceBySubjectPrefix...)
	key = append(key, subjectAddress...)
	key = append(key, '/')
	return key
}

func EvidenceByActionIndexKey(actionID string, evidenceID uint64) []byte {
	key := make([]byte, 0, len(evidenceByActionIDPrefix)+len(actionID)+1+8) // "ev/a/" + action + 0x00 + u64be(evidence_id)
	key = append(key, evidenceByActionIDPrefix...)
	key = append(key, actionID...)
	key = append(key, 0) // delimiter (allows action_id to contain '/')
	key = binary.BigEndian.AppendUint64(key, evidenceID)
	return key
}

func EvidenceByActionIndexPrefix(actionID string) []byte {
	key := make([]byte, 0, len(evidenceByActionIDPrefix)+len(actionID)+1) // "ev/a/" + action + 0x00
	key = append(key, evidenceByActionIDPrefix...)
	key = append(key, actionID...)
	key = append(key, 0) // delimiter
	return key
}

func EvidenceEpochCountKey(epochID uint64, subjectAddress string, evidenceType EvidenceType) []byte {
	key := make([]byte, 0, len(evidenceEpochCountPrefix)+8+1+len(subjectAddress)+1+4) // "eve/" + u64be(epoch_id) + "/" + subject + "/" + u32be(evidence_type)
	key = append(key, evidenceEpochCountPrefix...)
	key = binary.BigEndian.AppendUint64(key, epochID)
	key = append(key, '/')
	key = append(key, subjectAddress...)
	key = append(key, '/')
	key = binary.BigEndian.AppendUint32(key, uint32(evidenceType))
	return key
}

func EvidenceEpochCountPrefix() []byte {
	return evidenceEpochCountPrefix
}

func ActionFinalizationPostponementKey(supernodeAccount string) []byte {
	key := make([]byte, 0, len(actionFinalizationPostponementPrefix)+len(supernodeAccount)) // "ap/af/" + supernode
	key = append(key, actionFinalizationPostponementPrefix...)
	key = append(key, supernodeAccount...)
	return key
}

func ActionFinalizationPostponementPrefix() []byte {
	return actionFinalizationPostponementPrefix
}
