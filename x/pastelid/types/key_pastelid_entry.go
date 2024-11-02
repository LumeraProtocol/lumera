package types

import "encoding/binary"

var _ binary.ByteOrder

const (
	// PastelidEntryKeyPrefix is the prefix to retrieve all PastelidEntry
	PastelidEntryKeyPrefix = "PastelidEntry/value/"
)

// PastelidEntryKey returns the store key to retrieve a PastelidEntry from the index fields
func PastelidEntryKey(
	address string,
) []byte {
	var key []byte

	addressBytes := []byte(address)
	key = append(key, addressBytes...)
	key = append(key, []byte("/")...)

	return key
}
