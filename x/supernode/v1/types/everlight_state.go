package types

// SNDistState holds per-supernode distribution tracking state.
type SNDistState struct {
	// SmoothedBytes is the EMA-smoothed cascade bytes value used for weight calculation.
	SmoothedBytes float64 `json:"smoothed_bytes"`
	// PrevRawBytes is the raw cascade bytes from the previous period (for growth cap).
	PrevRawBytes float64 `json:"prev_raw_bytes"`
	// EligibilityStartHeight is the block height when this SN first became eligible.
	EligibilityStartHeight int64 `json:"eligibility_start_height"`
	// PeriodsActive is the number of distribution periods this SN has been active.
	PeriodsActive uint64 `json:"periods_active"`
}
