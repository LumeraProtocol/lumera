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
	// This module uses human-readable ASCII prefixes plus fixed-width binary window IDs.
	// - Prefixes are short and unambiguous when iterating by prefix.
	// - window_id is encoded as 8-byte big-endian so lexicographic ordering matches numeric ordering.
	// - supernode accounts are stored as their bech32 string bytes.
	//
	// Snapshotting:
	// - Each window stores a WindowSnapshot under "ws/<window_id>".
	// - The snapshot is intended to be an immutable per-window source-of-truth for:
	//   - prober -> targets mapping (assignments)
	//
	// Formats:
	// - WindowOriginHeightKey: "origin_height" -> 8 bytes (u64be(height))
	// - WindowSnapshotKey:     "ws/" + u64be(window_id)
	// - ReportKey:             "r/"  + u64be(window_id) + reporter_supernode_account
	// - ReportIndexKey:        "ri/" + reporter_supernode_account + "/" + u64be(window_id)
	// - SupernodeReportIndexKey: "sr/" + supernode_account + "/" + u64be(window_id) + "/" + reporter_supernode_account
	// - SelfReportIndexKey:      "ss/" + reporter_supernode_account + "/" + u64be(window_id)
	//
	// Examples (shown as pseudo strings; the u64be bytes will appear as non-printable in raw dumps):
	// - WindowSnapshotKey(1)          => "ws/" + u64be(1)
	// - ReportKey(1, "<reporter>")    => "r/"  + u64be(1) + "<reporter>"
	// - EvidenceKey(1, "<target>")    => "e/"  + u64be(1) + "<target>"

	windowOriginHeightKey = []byte("origin_height")

	windowSnapshotPrefix = []byte("ws/")
	reportPrefix         = []byte("r/")

	reportIndexPrefix = []byte("ri/")

	// supernodeReportIndexPrefix indexes reports that include an observation for a given supernode.
	// Format: "sr/" + supernode_account + "/" + u64be(window_id) + "/" + reporter_supernode_account
	supernodeReportIndexPrefix = []byte("sr/")

	// selfReportIndexPrefix indexes all submitted reports (for listing self reports across reporters/windows).
	// Format: "ss/" + reporter_supernode_account + "/" + u64be(window_id)
	selfReportIndexPrefix = []byte("ss/")
)

// WindowOriginHeightKey returns the store key for the module's fixed window origin height.
// The stored value is u64be(height).
func WindowOriginHeightKey() []byte {
	return windowOriginHeightKey
}

// WindowSnapshotKey returns the store key for the WindowSnapshot identified by windowID.
func WindowSnapshotKey(windowID uint64) []byte {
	key := make([]byte, 0, len(windowSnapshotPrefix)+8) // "ws/" + u64be(window_id)
	key = append(key, windowSnapshotPrefix...)          // "ws/"
	key = binary.BigEndian.AppendUint64(key, windowID)  // u64be(window_id)
	return key
}

// ReportKey returns the store key for the AuditReport identified by (windowID, reporterSupernodeAccount).
func ReportKey(windowID uint64, reporterSupernodeAccount string) []byte {
	key := make([]byte, 0, len(reportPrefix)+8+len(reporterSupernodeAccount)) // "r/" + u64be(window_id) + reporter
	key = append(key, reportPrefix...)                                        // "r/"
	key = binary.BigEndian.AppendUint64(key, windowID)                        // u64be(window_id)
	key = append(key, reporterSupernodeAccount...)                            // reporter (bech32)
	return key
}

// ReportIndexKey returns the store key for the report index entry identified by (reporterSupernodeAccount, windowID).
// The value is empty; the key exists to allow querying all reports for a reporter without scanning all windows.
func ReportIndexKey(reporterSupernodeAccount string, windowID uint64) []byte {
	key := make([]byte, 0, len(reportIndexPrefix)+len(reporterSupernodeAccount)+1+8) // "ri/" + reporter + "/" + u64be(window_id)
	key = append(key, reportIndexPrefix...)                                          // "ri/"
	key = append(key, reporterSupernodeAccount...)                                   // reporter (bech32)
	key = append(key, '/')                                                           // separator
	key = binary.BigEndian.AppendUint64(key, windowID)                               // u64be(window_id)
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

// SupernodeReportIndexKey returns the store key for an index entry identified by (supernodeAccount, windowID, reporterSupernodeAccount).
// The value is empty; the key exists to allow querying reports about a given supernode without scanning all reports.
func SupernodeReportIndexKey(supernodeAccount string, windowID uint64, reporterSupernodeAccount string) []byte {
	key := make([]byte, 0, len(supernodeReportIndexPrefix)+len(supernodeAccount)+1+8+1+len(reporterSupernodeAccount)) // "sr/" + supernode + "/" + u64be(window_id) + "/" + reporter
	key = append(key, supernodeReportIndexPrefix...)                                                                  // "sr/"
	key = append(key, supernodeAccount...)                                                                            // supernode (bech32)
	key = append(key, '/')                                                                                            // separator
	key = binary.BigEndian.AppendUint64(key, windowID)                                                                // u64be(window_id)
	key = append(key, '/')                                                                                            // separator
	key = append(key, reporterSupernodeAccount...)                                                                    // reporter (bech32)
	return key
}

// SupernodeReportIndexPrefix returns the prefix under which index keys are stored for a given supernode.
func SupernodeReportIndexPrefix(supernodeAccount string) []byte {
	key := make([]byte, 0, len(supernodeReportIndexPrefix)+len(supernodeAccount)+1) // "sr/" + supernode + "/"
	key = append(key, supernodeReportIndexPrefix...)                                // "sr/"
	key = append(key, supernodeAccount...)                                          // supernode (bech32)
	key = append(key, '/')                                                          // separator
	return key
}

// SupernodeReportIndexWindowPrefix returns the prefix under which index keys are stored for a given (supernodeAccount, windowID).
func SupernodeReportIndexWindowPrefix(supernodeAccount string, windowID uint64) []byte {
	key := make([]byte, 0, len(supernodeReportIndexPrefix)+len(supernodeAccount)+1+8+1) // "sr/" + supernode + "/" + u64be(window_id) + "/"
	key = append(key, supernodeReportIndexPrefix...)                                    // "sr/"
	key = append(key, supernodeAccount...)                                              // supernode (bech32)
	key = append(key, '/')                                                              // separator
	key = binary.BigEndian.AppendUint64(key, windowID)                                  // u64be(window_id)
	key = append(key, '/')                                                              // separator
	return key
}

// SelfReportIndexKey returns the store key for an index entry identified by (reporterSupernodeAccount, windowID).
// The value is empty; the key exists to allow listing a supernode's self reports across windows without scanning all report keys.
func SelfReportIndexKey(reporterSupernodeAccount string, windowID uint64) []byte {
	key := make([]byte, 0, len(selfReportIndexPrefix)+len(reporterSupernodeAccount)+1+8) // "ss/" + reporter + "/" + u64be(window_id)
	key = append(key, selfReportIndexPrefix...)                                          // "ss/"
	key = append(key, reporterSupernodeAccount...)                                       // reporter (bech32)
	key = append(key, '/')                                                               // separator
	key = binary.BigEndian.AppendUint64(key, windowID)                                   // u64be(window_id)
	return key
}

// SelfReportIndexPrefix returns the prefix under which self report index keys are stored for a given reporter.
func SelfReportIndexPrefix(reporterSupernodeAccount string) []byte {
	key := make([]byte, 0, len(selfReportIndexPrefix)+len(reporterSupernodeAccount)+1) // "ss/" + reporter + "/"
	key = append(key, selfReportIndexPrefix...)                                        // "ss/"
	key = append(key, reporterSupernodeAccount...)                                     // reporter (bech32)
	key = append(key, '/')                                                             // separator
	return key
}
