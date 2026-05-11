package types

import "encoding/binary"

const (
	ModuleName = "audit"

	StoreKey    = ModuleName
	MemStoreKey = "mem_audit"

	// MaxStorageProofResultsPerReport caps the number of storage proof results
	// a reporter may submit in a single epoch report. Per PR #118 / Zee F2.
	MaxStorageProofResultsPerReport = 16
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
	reportPrefix              = []byte("r/")

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

	// Storage-truth postponement state:
	// - StorageTruthPostponementKey: "ap/st/" + supernode_account -> 8 bytes u64be(postponed_at_epoch_id)
	storageTruthPostponementPrefix = []byte("ap/st/")
	// Per F121-F12 — sibling marker recording strong-postpone reason.
	storageTruthPostponementStrongPrefix = []byte("ap/sts/")

	// Storage-truth state:
	// - NodeSuspicionStateKey:          "st/ns/" + supernode_account
	// - ReporterReliabilityStateKey:    "st/rr/" + reporter_supernode_account
	// - TicketDeteriorationStateKey:    "st/td/" + ticket_id
	// - TicketArtifactCountStateKey:    "st/tac/" + ticket_id
	// - HealOpKey:                      "st/ho/" + u64be(heal_op_id)
	// - HealOpByTicketIndexKey:         "st/hot/" + ticket_id + 0x00 + u64be(heal_op_id)
	// - HealOpByStatusIndexKey:         "st/hos/" + u32be(status) + u64be(heal_op_id)
	// - HealOpVerificationKey:          "st/hov/" + u64be(heal_op_id) + "/" + verifier_supernode_account
	// - NextHealOpIDKey:                "st/next_ho_id"
	nodeSuspicionStatePrefix       = []byte("st/ns/")
	reporterReliabilityStatePrefix = []byte("st/rr/")
	ticketDeteriorationStatePrefix = []byte("st/td/")
	ticketArtifactCountStatePrefix = []byte("st/tac/")
	healOpPrefix                   = []byte("st/ho/")
	healOpByTicketIndexPrefix      = []byte("st/hot/")
	healOpByStatusIndexPrefix      = []byte("st/hos/")
	healOpVerificationPrefix       = []byte("st/hov/")
	nextHealOpIDKey                = []byte("st/next_ho_id")

	// Recheck evidence dedup:
	// - RecheckEvidenceKey: "st/rce/" + u64be(epoch_id) + "/" + ticket_id + 0x00 + creator_account
	recheckEvidencePrefix = []byte("st/rce/")

	// Storage-truth fact indexes:
	// - StorageProofTranscriptKey:      "st/spt/" + transcript_hash -> storageProofTranscriptRecord JSON
	// - NodeStorageTruthFailureKey:     "st/nf/" + supernode_account + "/" + u64be(epoch_id) + "/" + ticket_id + 0x00 + reporter_account -> storageTruthNodeFailureRecord JSON
	// - ReporterStorageTruthResultKey:  "st/rrs/" + reporter_account + "/" + u64be(epoch_id) + "/" + ticket_id + 0x00 + target_account -> storageTruthReporterResultRecord JSON
	// - StorageTruthFailedHealKey:      "st/fh/" + supernode_account + "/" + u64be(epoch_id) + "/" + ticket_id -> empty
	storageProofTranscriptPrefix     = []byte("st/spt/")
	nodeStorageTruthFailurePrefix    = []byte("st/nf/")
	reporterStorageTruthResultPrefix = []byte("st/rrs/")
	storageTruthFailedHealPrefix     = []byte("st/fh/")

	// Per 122-Copilot-3/4/5 + 122-F1 — indexed lookup avoids DeliverTx full-table scan.
	//
	// Secondary index: reporter result keyed by (target, epoch, ticketID, reporter).
	// Format: "st/rrs-tt/" + target + "/" + u64be(epoch) + "/" + ticketID + 0x00 + reporter
	reporterResultByTargetPrefix = []byte("st/rrs-tt/")

	// Secondary index: reporter activity keyed by (epoch, reporter).
	// Format: "st/rrs-e/" + u64be(epoch) + "/" + reporter_account -> empty
	reporterResultByEpochPrefix = []byte("st/rrs-e/")

	// Secondary index: transcript keyed by (target, bucket, epoch, transcriptHash).
	// Format: "st/spt-tbe/" + target + "/" + u32be(bucket) + "/" + u64be(epoch) + "/" + transcriptHash
	transcriptByTargetBucketEpochPrefix = []byte("st/spt-tbe/")
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

func NodeSuspicionStateKey(supernodeAccount string) []byte {
	key := make([]byte, 0, len(nodeSuspicionStatePrefix)+len(supernodeAccount))
	key = append(key, nodeSuspicionStatePrefix...)
	key = append(key, supernodeAccount...)
	return key
}

func NodeSuspicionStatePrefix() []byte {
	return nodeSuspicionStatePrefix
}

func ReporterReliabilityStateKey(reporterSupernodeAccount string) []byte {
	key := make([]byte, 0, len(reporterReliabilityStatePrefix)+len(reporterSupernodeAccount))
	key = append(key, reporterReliabilityStatePrefix...)
	key = append(key, reporterSupernodeAccount...)
	return key
}

func ReporterReliabilityStatePrefix() []byte {
	return reporterReliabilityStatePrefix
}

func TicketDeteriorationStateKey(ticketID string) []byte {
	key := make([]byte, 0, len(ticketDeteriorationStatePrefix)+len(ticketID))
	key = append(key, ticketDeteriorationStatePrefix...)
	key = append(key, ticketID...)
	return key
}

func TicketDeteriorationStatePrefix() []byte {
	return ticketDeteriorationStatePrefix
}

func TicketArtifactCountStateKey(ticketID string) []byte {
	key := make([]byte, 0, len(ticketArtifactCountStatePrefix)+len(ticketID))
	key = append(key, ticketArtifactCountStatePrefix...)
	key = append(key, ticketID...)
	return key
}

func TicketArtifactCountStatePrefix() []byte {
	return ticketArtifactCountStatePrefix
}

func HealOpKey(healOpID uint64) []byte {
	key := make([]byte, 0, len(healOpPrefix)+8)
	key = append(key, healOpPrefix...)
	key = binary.BigEndian.AppendUint64(key, healOpID)
	return key
}

func HealOpPrefix() []byte {
	return healOpPrefix
}

func HealOpByTicketIndexKey(ticketID string, healOpID uint64) []byte {
	key := make([]byte, 0, len(healOpByTicketIndexPrefix)+len(ticketID)+1+8) // "st/hot/" + ticket + 0x00 + u64be(heal_op_id)
	key = append(key, healOpByTicketIndexPrefix...)
	key = append(key, ticketID...)
	key = append(key, 0)
	key = binary.BigEndian.AppendUint64(key, healOpID)
	return key
}

func HealOpByTicketIndexPrefix(ticketID string) []byte {
	key := make([]byte, 0, len(healOpByTicketIndexPrefix)+len(ticketID)+1) // "st/hot/" + ticket + 0x00
	key = append(key, healOpByTicketIndexPrefix...)
	key = append(key, ticketID...)
	key = append(key, 0)
	return key
}

func HealOpByStatusIndexKey(status HealOpStatus, healOpID uint64) []byte {
	key := make([]byte, 0, len(healOpByStatusIndexPrefix)+4+8) // "st/hos/" + u32be(status) + u64be(heal_op_id)
	key = append(key, healOpByStatusIndexPrefix...)
	key = binary.BigEndian.AppendUint32(key, uint32(status))
	key = binary.BigEndian.AppendUint64(key, healOpID)
	return key
}

func HealOpByStatusIndexPrefix(status HealOpStatus) []byte {
	key := make([]byte, 0, len(healOpByStatusIndexPrefix)+4) // "st/hos/" + u32be(status)
	key = append(key, healOpByStatusIndexPrefix...)
	key = binary.BigEndian.AppendUint32(key, uint32(status))
	return key
}

func NextHealOpIDKey() []byte {
	return nextHealOpIDKey
}

func HealOpVerificationKey(healOpID uint64, verifierSupernodeAccount string) []byte {
	key := make([]byte, 0, len(healOpVerificationPrefix)+8+1+len(verifierSupernodeAccount)) // "st/hov/" + u64be(heal_op_id) + "/" + verifier
	key = append(key, healOpVerificationPrefix...)
	key = binary.BigEndian.AppendUint64(key, healOpID)
	key = append(key, '/')
	key = append(key, verifierSupernodeAccount...)
	return key
}

func HealOpVerificationPrefix(healOpID uint64) []byte {
	key := make([]byte, 0, len(healOpVerificationPrefix)+8+1) // "st/hov/" + u64be(heal_op_id) + "/"
	key = append(key, healOpVerificationPrefix...)
	key = binary.BigEndian.AppendUint64(key, healOpID)
	key = append(key, '/')
	return key
}

func HealOpVerificationRootPrefix() []byte {
	return healOpVerificationPrefix
}

func StorageTruthPostponementKey(supernodeAccount string) []byte {
	key := make([]byte, 0, len(storageTruthPostponementPrefix)+len(supernodeAccount))
	key = append(key, storageTruthPostponementPrefix...)
	key = append(key, supernodeAccount...)
	return key
}

func StorageTruthPostponementPrefix() []byte {
	return storageTruthPostponementPrefix
}

// StorageTruthPostponementStrongKey marks a postponement record as strong-band
// (F121-F12). Presence of this key indicates the supernode was postponed under
// the strong-postpone band; recovery requires StorageTruthStrongRecoveryCleanPassCount
// rather than StorageTruthRecoveryCleanPassCount.
func StorageTruthPostponementStrongKey(supernodeAccount string) []byte {
	key := make([]byte, 0, len(storageTruthPostponementStrongPrefix)+len(supernodeAccount))
	key = append(key, storageTruthPostponementStrongPrefix...)
	key = append(key, supernodeAccount...)
	return key
}

// RecheckEvidencePrefix returns the root prefix for all recheck-evidence dedup keys.
func RecheckEvidencePrefix() []byte {
	return recheckEvidencePrefix
}

// NodeStorageTruthFailureRootPrefix returns the root prefix for all node-failure facts.
func NodeStorageTruthFailureRootPrefix() []byte {
	return nodeStorageTruthFailurePrefix
}

// StorageTruthFailedHealRootPrefix returns the root prefix for all failed-heal markers.
func StorageTruthFailedHealRootPrefix() []byte {
	return storageTruthFailedHealPrefix
}

// RecheckEvidenceKey returns the dedup key for a recheck evidence submission.
// Format: "st/rce/" + u64be(epoch_id) + "/" + ticket_id + 0x00 + creator_account
func RecheckEvidenceKey(epochID uint64, ticketID string, creatorAccount string) []byte {
	key := make([]byte, 0, len(recheckEvidencePrefix)+8+1+len(ticketID)+1+len(creatorAccount))
	key = append(key, recheckEvidencePrefix...)
	key = binary.BigEndian.AppendUint64(key, epochID)
	key = append(key, '/')
	key = append(key, ticketID...)
	key = append(key, 0) // delimiter allows ticket_id to contain '/'
	key = append(key, creatorAccount...)
	return key
}

func StorageProofTranscriptKey(transcriptHash string) []byte {
	key := make([]byte, 0, len(storageProofTranscriptPrefix)+len(transcriptHash))
	key = append(key, storageProofTranscriptPrefix...)
	key = append(key, transcriptHash...)
	return key
}

func StorageProofTranscriptPrefix() []byte {
	return storageProofTranscriptPrefix
}

func NodeStorageTruthFailureKey(supernodeAccount string, epochID uint64, ticketID string, reporterAccount string) []byte {
	key := make([]byte, 0, len(nodeStorageTruthFailurePrefix)+len(supernodeAccount)+1+8+1+len(ticketID)+1+len(reporterAccount))
	key = append(key, nodeStorageTruthFailurePrefix...)
	key = append(key, supernodeAccount...)
	key = append(key, '/')
	key = binary.BigEndian.AppendUint64(key, epochID)
	key = append(key, '/')
	key = append(key, ticketID...)
	key = append(key, 0)
	key = append(key, reporterAccount...)
	return key
}

func NodeStorageTruthFailurePrefix(supernodeAccount string) []byte {
	key := make([]byte, 0, len(nodeStorageTruthFailurePrefix)+len(supernodeAccount)+1)
	key = append(key, nodeStorageTruthFailurePrefix...)
	key = append(key, supernodeAccount...)
	key = append(key, '/')
	return key
}

func ReporterStorageTruthResultKey(reporterAccount string, epochID uint64, ticketID string, targetAccount string) []byte {
	key := make([]byte, 0, len(reporterStorageTruthResultPrefix)+len(reporterAccount)+1+8+1+len(ticketID)+1+len(targetAccount))
	key = append(key, reporterStorageTruthResultPrefix...)
	key = append(key, reporterAccount...)
	key = append(key, '/')
	key = binary.BigEndian.AppendUint64(key, epochID)
	key = append(key, '/')
	key = append(key, ticketID...)
	key = append(key, 0)
	key = append(key, targetAccount...)
	return key
}

func ReporterStorageTruthResultPrefix(reporterAccount string) []byte {
	key := make([]byte, 0, len(reporterStorageTruthResultPrefix)+len(reporterAccount)+1)
	key = append(key, reporterStorageTruthResultPrefix...)
	key = append(key, reporterAccount...)
	key = append(key, '/')
	return key
}

func ReporterStorageTruthResultRootPrefix() []byte {
	return reporterStorageTruthResultPrefix
}

func ReporterStorageTruthResultByEpochRootPrefix() []byte {
	return reporterResultByEpochPrefix
}

// NodeStorageTruthFailureEpochScanRange returns [start, end) iterator bounds
// for scanning a supernode's failure facts within the inclusive epoch range
// [startEpoch, endEpoch]. Key shape unchanged — start/end built from the
// canonical prefix + u64be(epoch). Per CP-NEW-A-11 residue (bounded scan).
func NodeStorageTruthFailureEpochScanRange(supernodeAccount string, startEpoch, endEpoch uint64) ([]byte, []byte) {
	base := NodeStorageTruthFailurePrefix(supernodeAccount)
	start := make([]byte, 0, len(base)+8)
	start = append(start, base...)
	start = binary.BigEndian.AppendUint64(start, startEpoch)
	end := make([]byte, 0, len(base)+8)
	end = append(end, base...)
	// end is exclusive: u64be(endEpoch+1). Wrap-safe: if endEpoch is MaxUint64,
	// fall back to the unbounded prefix end.
	if endEpoch == ^uint64(0) {
		// No upper bound — emit the prefix-end sentinel.
		return start, prefixEnd(base)
	}
	end = binary.BigEndian.AppendUint64(end, endEpoch+1)
	return start, end
}

// ReporterStorageTruthResultEpochScanRange returns [start, end) iterator
// bounds for scanning a reporter's result facts within the inclusive epoch
// range [startEpoch, endEpoch]. Per CP-NEW-A-11 residue.
func ReporterStorageTruthResultEpochScanRange(reporterAccount string, startEpoch, endEpoch uint64) ([]byte, []byte) {
	base := ReporterStorageTruthResultPrefix(reporterAccount)
	start := make([]byte, 0, len(base)+8)
	start = append(start, base...)
	start = binary.BigEndian.AppendUint64(start, startEpoch)
	end := make([]byte, 0, len(base)+8)
	end = append(end, base...)
	if endEpoch == ^uint64(0) {
		return start, prefixEnd(base)
	}
	end = binary.BigEndian.AppendUint64(end, endEpoch+1)
	return start, end
}

// prefixEnd computes the exclusive end key of a prefix range — i.e. the
// smallest byte string that is strictly greater than every key beginning
// with prefix. Returns nil when prefix is all 0xFF.
func prefixEnd(prefix []byte) []byte {
	end := make([]byte, len(prefix))
	copy(end, prefix)
	for i := len(end) - 1; i >= 0; i-- {
		end[i]++
		if end[i] != 0 {
			return end[:i+1]
		}
	}
	return nil
}

func StorageTruthFailedHealKey(supernodeAccount string, epochID uint64, ticketID string) []byte {
	key := make([]byte, 0, len(storageTruthFailedHealPrefix)+len(supernodeAccount)+1+8+1+len(ticketID))
	key = append(key, storageTruthFailedHealPrefix...)
	key = append(key, supernodeAccount...)
	key = append(key, '/')
	key = binary.BigEndian.AppendUint64(key, epochID)
	key = append(key, '/')
	key = append(key, ticketID...)
	return key
}

func StorageTruthFailedHealPrefix(supernodeAccount string) []byte {
	key := make([]byte, 0, len(storageTruthFailedHealPrefix)+len(supernodeAccount)+1)
	key = append(key, storageTruthFailedHealPrefix...)
	key = append(key, supernodeAccount...)
	key = append(key, '/')
	return key
}

// Per 122-Copilot-3/4/5 + 122-F1 — indexed lookup avoids DeliverTx full-table scan.

// ReporterStorageTruthResultByTargetKey returns the secondary-index key for a reporter result
// keyed by (target, epoch, ticketID, reporter).
// Format: "st/rrs-tt/" + target + "/" + u64be(epoch) + "/" + ticketID + 0x00 + reporter
func ReporterStorageTruthResultByTargetKey(targetAccount string, epochID uint64, ticketID string, reporterAccount string) []byte {
	key := make([]byte, 0, len(reporterResultByTargetPrefix)+len(targetAccount)+1+8+1+len(ticketID)+1+len(reporterAccount))
	key = append(key, reporterResultByTargetPrefix...)
	key = append(key, targetAccount...)
	key = append(key, '/')
	key = binary.BigEndian.AppendUint64(key, epochID)
	key = append(key, '/')
	key = append(key, ticketID...)
	key = append(key, 0)
	key = append(key, reporterAccount...)
	return key
}

// ReporterStorageTruthResultByTargetEpochPrefix returns the prefix for scanning all reporter
// results for a given (target, epoch).
func ReporterStorageTruthResultByTargetEpochPrefix(targetAccount string, epochID uint64) []byte {
	key := make([]byte, 0, len(reporterResultByTargetPrefix)+len(targetAccount)+1+8+1)
	key = append(key, reporterResultByTargetPrefix...)
	key = append(key, targetAccount...)
	key = append(key, '/')
	key = binary.BigEndian.AppendUint64(key, epochID)
	key = append(key, '/')
	return key
}

// ReporterStorageTruthResultByEpochReporterKey returns the secondary-index key
// marking that reporterAccount has at least one reporter-result fact in epochID.
// Format: "st/rrs-e/" + u64be(epoch) + "/" + reporter_account
func ReporterStorageTruthResultByEpochReporterKey(epochID uint64, reporterAccount string) []byte {
	key := make([]byte, 0, len(reporterResultByEpochPrefix)+8+1+len(reporterAccount))
	key = append(key, reporterResultByEpochPrefix...)
	key = binary.BigEndian.AppendUint64(key, epochID)
	key = append(key, '/')
	key = append(key, reporterAccount...)
	return key
}

// ReporterStorageTruthResultByEpochPrefix returns the prefix for scanning
// reporter accounts that have at least one reporter-result fact in epochID.
func ReporterStorageTruthResultByEpochPrefix(epochID uint64) []byte {
	key := make([]byte, 0, len(reporterResultByEpochPrefix)+8+1)
	key = append(key, reporterResultByEpochPrefix...)
	key = binary.BigEndian.AppendUint64(key, epochID)
	key = append(key, '/')
	return key
}

// TranscriptByTargetBucketEpochKey returns the secondary-index key for a transcript
// keyed by (target, bucket, epoch, transcriptHash).
// Format: "st/spt-tbe/" + target + "/" + u32be(bucket) + "/" + u64be(epoch) + "/" + transcriptHash
func TranscriptByTargetBucketEpochKey(targetAccount string, bucketType uint32, epochID uint64, transcriptHash string) []byte {
	key := make([]byte, 0, len(transcriptByTargetBucketEpochPrefix)+len(targetAccount)+1+4+1+8+1+len(transcriptHash))
	key = append(key, transcriptByTargetBucketEpochPrefix...)
	key = append(key, targetAccount...)
	key = append(key, '/')
	key = binary.BigEndian.AppendUint32(key, bucketType)
	key = append(key, '/')
	key = binary.BigEndian.AppendUint64(key, epochID)
	key = append(key, '/')
	key = append(key, transcriptHash...)
	return key
}

// ReporterStorageTruthResultByTargetEpochScanRange returns [start, end)
// iterator bounds for scanning a target's reporter-result secondary index in
// the inclusive epoch range [startEpoch, endEpoch]. It is MaxUint64-safe.
func ReporterStorageTruthResultByTargetEpochScanRange(targetAccount string, startEpoch, endEpoch uint64) ([]byte, []byte) {
	base := make([]byte, 0, len(reporterResultByTargetPrefix)+len(targetAccount)+1)
	base = append(base, reporterResultByTargetPrefix...)
	base = append(base, targetAccount...)
	base = append(base, '/')

	start := make([]byte, 0, len(base)+8)
	start = append(start, base...)
	start = binary.BigEndian.AppendUint64(start, startEpoch)

	if endEpoch == ^uint64(0) {
		return start, prefixEnd(base)
	}
	end := make([]byte, 0, len(base)+8)
	end = append(end, base...)
	end = binary.BigEndian.AppendUint64(end, endEpoch+1)
	return start, end
}

// TranscriptByTargetBucketEpochScanPrefix returns the prefix for epoch-range scanning of
// transcripts for a given (target, bucket). Iterator start/end are derived by callers using
// the u64be-encoded epoch bounds.
func TranscriptByTargetBucketEpochScanPrefix(targetAccount string, bucketType uint32) []byte {
	key := make([]byte, 0, len(transcriptByTargetBucketEpochPrefix)+len(targetAccount)+1+4+1)
	key = append(key, transcriptByTargetBucketEpochPrefix...)
	key = append(key, targetAccount...)
	key = append(key, '/')
	key = binary.BigEndian.AppendUint32(key, bucketType)
	key = append(key, '/')
	return key
}

// TranscriptByTargetBucketEpochScanRange returns [start, end) iterator bounds
// for scanning transcript secondary-index records for a target/bucket in the
// inclusive epoch range [startEpoch, endEpoch]. It is MaxUint64-safe.
func TranscriptByTargetBucketEpochScanRange(targetAccount string, bucketType uint32, startEpoch, endEpoch uint64) ([]byte, []byte) {
	base := TranscriptByTargetBucketEpochScanPrefix(targetAccount, bucketType)
	start := make([]byte, 0, len(base)+8)
	start = append(start, base...)
	start = binary.BigEndian.AppendUint64(start, startEpoch)

	if endEpoch == ^uint64(0) {
		return start, prefixEnd(base)
	}
	end := make([]byte, 0, len(base)+8)
	end = append(end, base...)
	end = binary.BigEndian.AppendUint64(end, endEpoch+1)
	return start, end
}
