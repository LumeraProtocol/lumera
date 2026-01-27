package keeper

import (
	"encoding/binary"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

type windowState struct {
	WindowID     uint64
	StartHeight  int64
	EndHeight    int64
	WindowBlocks uint64
}

func (ws windowState) validate() error {
	if ws.WindowBlocks == 0 {
		return fmt.Errorf("window_blocks must be > 0")
	}
	if ws.StartHeight < 0 || ws.EndHeight < 0 {
		return fmt.Errorf("window heights must be >= 0")
	}
	if ws.EndHeight < ws.StartHeight {
		return fmt.Errorf("window_end_height must be >= window_start_height")
	}
	if ws.EndHeight-ws.StartHeight+1 != int64(ws.WindowBlocks) {
		return fmt.Errorf("window length mismatch: blocks=%d start=%d end=%d", ws.WindowBlocks, ws.StartHeight, ws.EndHeight)
	}
	return nil
}

func (k Keeper) getWindowState(ctx sdk.Context) (windowState, bool, error) {
	store := k.kvStore(ctx)
	bz := store.Get(types.CurrentWindowStateKey())
	if bz == nil {
		return windowState{}, false, nil
	}
	if len(bz) != 32 {
		return windowState{}, false, fmt.Errorf("invalid current window state length: %d", len(bz))
	}

	ws := windowState{
		WindowID:     binary.BigEndian.Uint64(bz[0:8]),
		StartHeight:  int64(binary.BigEndian.Uint64(bz[8:16])),
		EndHeight:    int64(binary.BigEndian.Uint64(bz[16:24])),
		WindowBlocks: binary.BigEndian.Uint64(bz[24:32]),
	}
	if err := ws.validate(); err != nil {
		return windowState{}, false, err
	}
	return ws, true, nil
}

func (k Keeper) setWindowState(ctx sdk.Context, ws windowState) error {
	if err := ws.validate(); err != nil {
		return err
	}

	bz := make([]byte, 32)
	binary.BigEndian.PutUint64(bz[0:8], ws.WindowID)
	binary.BigEndian.PutUint64(bz[8:16], uint64(ws.StartHeight))
	binary.BigEndian.PutUint64(bz[16:24], uint64(ws.EndHeight))
	binary.BigEndian.PutUint64(bz[24:32], ws.WindowBlocks)

	store := k.kvStore(ctx)
	store.Set(types.CurrentWindowStateKey(), bz)
	return nil
}

func (k Keeper) getNextWindowBlocks(ctx sdk.Context) (uint64, bool, error) {
	store := k.kvStore(ctx)
	bz := store.Get(types.NextWindowBlocksKey())
	if bz == nil {
		return 0, false, nil
	}
	if len(bz) != 8 {
		return 0, false, fmt.Errorf("invalid next window blocks length: %d", len(bz))
	}
	v := binary.BigEndian.Uint64(bz)
	if v == 0 {
		return 0, false, fmt.Errorf("invalid next window blocks: 0")
	}
	return v, true, nil
}

func (k Keeper) setNextWindowBlocks(ctx sdk.Context, windowBlocks uint64) error {
	if windowBlocks == 0 {
		return fmt.Errorf("window_blocks must be > 0")
	}
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, windowBlocks)
	store := k.kvStore(ctx)
	store.Set(types.NextWindowBlocksKey(), bz)
	return nil
}

func (k Keeper) clearNextWindowBlocks(ctx sdk.Context) {
	store := k.kvStore(ctx)
	store.Delete(types.NextWindowBlocksKey())
}

// initWindowStateIfNeeded writes initial window state once. It prefers initializing from an existing
// origin_height (for compatibility), otherwise it uses the current block height as the first window start.
func (k Keeper) initWindowStateIfNeeded(ctx sdk.Context, params types.Params) (windowState, error) {
	if ws, found, err := k.getWindowState(ctx); err != nil {
		return windowState{}, err
	} else if found {
		return ws, nil
	}

	windowBlocks := params.ReportingWindowBlocks
	if windowBlocks == 0 {
		return windowState{}, fmt.Errorf("reporting_window_blocks must be > 0")
	}

	// Compatibility: if origin_height exists, derive the current window by the legacy math once.
	if origin, found := k.getWindowOriginHeight(ctx); found {
		if ctx.BlockHeight() < origin {
			ws := windowState{
				WindowID:     0,
				StartHeight:  origin,
				EndHeight:    origin + int64(windowBlocks) - 1,
				WindowBlocks: windowBlocks,
			}
			return ws, k.setWindowState(ctx, ws)
		}
		windowID := uint64((ctx.BlockHeight() - origin) / int64(windowBlocks))
		start := origin + int64(windowID)*int64(windowBlocks)
		ws := windowState{
			WindowID:     windowID,
			StartHeight:  start,
			EndHeight:    start + int64(windowBlocks) - 1,
			WindowBlocks: windowBlocks,
		}
		return ws, k.setWindowState(ctx, ws)
	}

	start := ctx.BlockHeight()
	ws := windowState{
		WindowID:     0,
		StartHeight:  start,
		EndHeight:    start + int64(windowBlocks) - 1,
		WindowBlocks: windowBlocks,
	}
	return ws, k.setWindowState(ctx, ws)
}

func (k Keeper) getCurrentWindowState(ctx sdk.Context, params types.Params) (windowState, error) {
	ws, err := k.initWindowStateIfNeeded(ctx, params)
	if err != nil {
		return windowState{}, err
	}
	return k.advanceWindowIfNeeded(ctx, params, ws)
}

func (k Keeper) advanceWindowIfNeeded(ctx sdk.Context, params types.Params, ws windowState) (windowState, error) {
	for ctx.BlockHeight() > ws.EndHeight {
		nextBlocks, hasNext, err := k.getNextWindowBlocks(ctx)
		if err != nil {
			return windowState{}, err
		}
		if hasNext {
			k.clearNextWindowBlocks(ctx)
			ws.WindowBlocks = nextBlocks
		}
		if ws.WindowBlocks == 0 {
			ws.WindowBlocks = params.ReportingWindowBlocks
		}
		if ws.WindowBlocks == 0 {
			return windowState{}, fmt.Errorf("reporting_window_blocks must be > 0")
		}

		nextStart := ws.EndHeight + 1
		ws.WindowID++
		ws.StartHeight = nextStart
		ws.EndHeight = nextStart + int64(ws.WindowBlocks) - 1

		if err := k.setWindowState(ctx, ws); err != nil {
			return windowState{}, err
		}
	}
	return ws, nil
}

// scheduleReportingWindowBlocksChangeAtNextBoundary stores a pending window size that will take effect
// at the next window boundary (end(current)+1). Multiple updates before the boundary overwrite the pending value.
func (k Keeper) scheduleReportingWindowBlocksChangeAtNextBoundary(ctx sdk.Context, params types.Params, newWindowBlocks uint64) error {
	if newWindowBlocks == 0 {
		return fmt.Errorf("reporting_window_blocks must be > 0")
	}

	ws, err := k.getCurrentWindowState(ctx, params)
	if err != nil {
		return err
	}
	if ws.WindowBlocks == newWindowBlocks {
		// No change needed.
		k.clearNextWindowBlocks(ctx)
		return nil
	}

	return k.setNextWindowBlocks(ctx, newWindowBlocks)
}
