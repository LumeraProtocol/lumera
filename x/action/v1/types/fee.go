package types

// RoundBytesToKB rounds a byte count up to the nearest kilobyte using ceiling division.
// This is the canonical implementation of the rounding formula used throughout the Lumera system.
//
// Formula: ⌈bytes / 1024⌉ = (bytes + 1023) / 1024
//
// Examples:
//   - 0 bytes → 0 KB
//   - 1 byte → 1 KB
//   - 1024 bytes → 1 KB
//   - 1025 bytes → 2 KB
func RoundBytesToKB(bytes int) int {
	if bytes < 0 {
		return 0
	}
	return (bytes + 1023) / 1024
}

// RoundBytesToKB64 is the int64 variant for use with file sizes from os.FileInfo.Size().
func RoundBytesToKB64(bytes int64) int {
	if bytes < 0 {
		return 0
	}
	return int((bytes + 1023) / 1024)
}
