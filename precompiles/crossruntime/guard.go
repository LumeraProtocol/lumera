package crossruntime

import sdk "github.com/cosmos/cosmos-sdk/types"

// crossRuntimeDepthKeyType is an unexported typed key to prevent accidental
// context-key collisions with other packages.
type crossRuntimeDepthKeyType struct{}

var crossRuntimeDepthKey = crossRuntimeDepthKeyType{}

// MaxCrossRuntimeDepth is the maximum allowed nesting depth for cross-runtime
// calls. Phase 1 disallows any re-entry (depth > 1 requires stateDB threading).
const MaxCrossRuntimeDepth = 1

// GetCrossRuntimeDepth returns the current cross-runtime call depth from the
// SDK context. Returns 0 if no cross-runtime call is in progress.
func GetCrossRuntimeDepth(ctx sdk.Context) int {
	v := ctx.Value(crossRuntimeDepthKey)
	if v == nil {
		return 0
	}
	return v.(int)
}

// WithIncrementedDepth returns a new context with the cross-runtime call depth
// incremented by one.
func WithIncrementedDepth(ctx sdk.Context) sdk.Context {
	return ctx.WithValue(crossRuntimeDepthKey, GetCrossRuntimeDepth(ctx)+1)
}

// CheckAndIncrementDepth checks if a cross-runtime call is allowed at the
// current depth and returns the updated context. Returns an error if the
// maximum depth would be exceeded.
func CheckAndIncrementDepth(ctx sdk.Context) (sdk.Context, error) {
	if GetCrossRuntimeDepth(ctx) >= MaxCrossRuntimeDepth {
		return ctx, ErrReentrancyNotAllowed
	}
	return WithIncrementedDepth(ctx), nil
}
