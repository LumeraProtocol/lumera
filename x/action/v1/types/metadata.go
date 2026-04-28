package types

// CascadeArtifactCountsWithFallback returns the (index, symbol) artifact
// counts from a CascadeMetadata, falling back to len(RqIdsIds) when either
// field is zero. This is the single-source-of-truth helper enforcing the
// 122-F2 fallback rule across all sites that consume cascade artifact
// counts (Process, GetUpdatedMetadata, FinalizeAction → audit hook).
//
// If meta is nil, returns (0, 0).
func CascadeArtifactCountsWithFallback(meta *CascadeMetadata) (uint32, uint32) {
	if meta == nil {
		return 0, 0
	}
	idx := meta.GetIndexArtifactCount()
	sym := meta.GetSymbolArtifactCount()
	fallback := uint32(len(meta.GetRqIdsIds()))
	if idx == 0 {
		idx = fallback
	}
	if sym == 0 {
		sym = fallback
	}
	return idx, sym
}
