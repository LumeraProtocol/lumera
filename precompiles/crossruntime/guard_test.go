package crossruntime

import (
	"testing"

	"cosmossdk.io/log"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func freshCtx() sdk.Context {
	return sdk.NewContext(nil, tmproto.Header{}, false, log.NewNopLogger())
}

func TestGetCrossRuntimeDepth_ZeroByDefault(t *testing.T) {
	ctx := freshCtx()
	if d := GetCrossRuntimeDepth(ctx); d != 0 {
		t.Fatalf("expected depth 0 on fresh context, got %d", d)
	}
}

func TestWithIncrementedDepth_IncrementsOnce(t *testing.T) {
	ctx := freshCtx()
	ctx = WithIncrementedDepth(ctx)
	if d := GetCrossRuntimeDepth(ctx); d != 1 {
		t.Fatalf("expected depth 1, got %d", d)
	}
}

func TestWithIncrementedDepth_IncrementsTwice(t *testing.T) {
	ctx := freshCtx()
	ctx = WithIncrementedDepth(ctx)
	ctx = WithIncrementedDepth(ctx)
	if d := GetCrossRuntimeDepth(ctx); d != 2 {
		t.Fatalf("expected depth 2, got %d", d)
	}
}

func TestWithIncrementedDepth_DoesNotMutateParent(t *testing.T) {
	parent := freshCtx()
	_ = WithIncrementedDepth(parent)
	if d := GetCrossRuntimeDepth(parent); d != 0 {
		t.Fatalf("parent context mutated: expected depth 0, got %d", d)
	}
}

func TestCheckAndIncrementDepth_SucceedsAtZero(t *testing.T) {
	ctx := freshCtx()
	newCtx, err := CheckAndIncrementDepth(ctx)
	if err != nil {
		t.Fatalf("unexpected error at depth 0: %v", err)
	}
	if d := GetCrossRuntimeDepth(newCtx); d != 1 {
		t.Fatalf("expected depth 1 after increment, got %d", d)
	}
}

func TestCheckAndIncrementDepth_FailsAtMax(t *testing.T) {
	ctx := freshCtx()
	ctx = WithIncrementedDepth(ctx) // depth = 1 = MaxCrossRuntimeDepth

	_, err := CheckAndIncrementDepth(ctx)
	if err == nil {
		t.Fatal("expected reentrancy error at max depth, got nil")
	}
	if err != ErrReentrancyNotAllowed {
		t.Fatalf("expected ErrReentrancyNotAllowed, got %v", err)
	}
}

func TestCheckAndIncrementDepth_FailsBeyondMax(t *testing.T) {
	ctx := freshCtx()
	ctx = WithIncrementedDepth(ctx) // 1
	ctx = WithIncrementedDepth(ctx) // 2

	_, err := CheckAndIncrementDepth(ctx)
	if err != ErrReentrancyNotAllowed {
		t.Fatalf("expected ErrReentrancyNotAllowed at depth 2, got %v", err)
	}
}

func TestCheckAndIncrementDepth_DoesNotMutateOnError(t *testing.T) {
	ctx := freshCtx()
	ctx = WithIncrementedDepth(ctx)

	returned, _ := CheckAndIncrementDepth(ctx)
	// On error, returned context should still have depth 1 (the input)
	if d := GetCrossRuntimeDepth(returned); d != 1 {
		t.Fatalf("expected depth 1 on error path, got %d", d)
	}
}

func TestMaxCrossRuntimeDepth_IsOne(t *testing.T) {
	if MaxCrossRuntimeDepth != 1 {
		t.Fatalf("Phase 1 requires MaxCrossRuntimeDepth=1, got %d", MaxCrossRuntimeDepth)
	}
}
