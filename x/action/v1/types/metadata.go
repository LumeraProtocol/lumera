package types

import "fmt"

// CascadeArtifactCountsWithFallback returns the (index, symbol) artifact
// counts from a CascadeMetadata, falling back to len(RqIdsIds) when either
// field is zero. This is the single-source-of-truth helper enforcing the
// 122-F2 fallback rule across all sites that consume cascade artifact
// counts (Process, GetUpdatedMetadata, FinalizeAction → audit hook).
//
// If meta is nil, returns (0, 0). Callers that make consensus/state decisions
// must use CascadeArtifactCountsWithFallbackStrict so malformed metadata cannot
// silently resolve to a zero-count artifact universe.
func CascadeArtifactCountsWithFallback(meta *CascadeMetadata) (uint32, uint32) {
	idx, sym, _ := CascadeArtifactCountsWithFallbackStrict(meta)
	return idx, sym
}

// CascadeArtifactCountsWithFallbackStrict is the consensus-safe variant of
// CascadeArtifactCountsWithFallback. It rejects metadata where explicit counts
// are missing and RqIdsIds cannot provide the backward-compatible fallback.
func CascadeArtifactCountsWithFallbackStrict(meta *CascadeMetadata) (uint32, uint32, error) {
	if meta == nil {
		return 0, 0, fmt.Errorf("cascade metadata is required")
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
	if idx == 0 || sym == 0 {
		return 0, 0, fmt.Errorf("cascade artifact counts unavailable: explicit index/symbol counts missing and rq_ids_ids empty")
	}
	return idx, sym, nil
}
