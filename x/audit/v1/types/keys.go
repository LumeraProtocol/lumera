package types

import "encoding/binary"

const (
	ModuleName = "audit"

	StoreKey    = ModuleName
	MemStoreKey = "mem_audit"
)

var (
	ParamsKey = []byte("p_audit")

	windowOriginHeightKey = []byte{0x01}

	windowSnapshotPrefix = []byte{0x10}
	reportPrefix         = []byte{0x11}
	evidencePrefix       = []byte{0x12}
	statusPrefix         = []byte{0x13}
)

func WindowOriginHeightKey() []byte {
	return windowOriginHeightKey
}

func WindowSnapshotKey(windowID uint64) []byte {
	key := make([]byte, 0, len(windowSnapshotPrefix)+8)
	key = append(key, windowSnapshotPrefix...)
	key = binary.BigEndian.AppendUint64(key, windowID)
	return key
}

func ReportKey(windowID uint64, reporterValidatorAddress string) []byte {
	key := make([]byte, 0, len(reportPrefix)+8+len(reporterValidatorAddress))
	key = append(key, reportPrefix...)
	key = binary.BigEndian.AppendUint64(key, windowID)
	key = append(key, reporterValidatorAddress...)
	return key
}

func EvidenceKey(windowID uint64, targetValidatorAddress string, portIndex uint32) []byte {
	key := make([]byte, 0, len(evidencePrefix)+8+len(targetValidatorAddress)+1+4)
	key = append(key, evidencePrefix...)
	key = binary.BigEndian.AppendUint64(key, windowID)
	key = append(key, targetValidatorAddress...)
	key = append(key, 0x00)
	key = binary.BigEndian.AppendUint32(key, portIndex)
	return key
}

func AuditStatusKey(validatorAddress string) []byte {
	key := make([]byte, 0, len(statusPrefix)+len(validatorAddress))
	key = append(key, statusPrefix...)
	key = append(key, validatorAddress...)
	return key
}
