package keeper

import (
	"encoding/binary"
	"encoding/json"
	"math"

	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/everlight/v1/types"
)

// SNDistState holds per-supernode distribution tracking state.
// Stored as JSON under SNDistStatePrefix + validatorAddress.
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

// GetLastDistributionHeight returns the block height of the last distribution.
func (k Keeper) GetLastDistributionHeight(ctx sdk.Context) int64 {
	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	bz := store.Get(types.LastDistributionHeightKey)
	if bz == nil {
		return 0
	}
	return int64(binary.BigEndian.Uint64(bz))
}

// SetLastDistributionHeight stores the block height of the last distribution.
func (k Keeper) SetLastDistributionHeight(ctx sdk.Context, height int64) {
	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, uint64(height))
	store.Set(types.LastDistributionHeightKey, bz)
}

// GetPoolBalance returns the current balance of the everlight module account.
func (k Keeper) GetPoolBalance(ctx sdk.Context) sdk.Coins {
	moduleAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	return k.bankKeeper.GetAllBalances(ctx, moduleAddr)
}

// GetTotalDistributed returns the cumulative amount distributed as sdk.Coins.
func (k Keeper) GetTotalDistributed(ctx sdk.Context) sdk.Coins {
	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	bz := store.Get(types.TotalDistributedKey)
	if bz == nil {
		return sdk.Coins{}
	}
	var coins sdk.Coins
	if err := json.Unmarshal(bz, &coins); err != nil {
		return sdk.Coins{}
	}
	return coins
}

// SetTotalDistributed stores the cumulative amount distributed.
func (k Keeper) SetTotalDistributed(ctx sdk.Context, coins sdk.Coins) {
	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	bz, err := json.Marshal(coins)
	if err != nil {
		panic("failed to marshal total distributed coins: " + err.Error())
	}
	store.Set(types.TotalDistributedKey, bz)
}

// AddTotalDistributed adds the given amount to the cumulative total distributed.
func (k Keeper) AddTotalDistributed(ctx sdk.Context, amt sdk.Coins) {
	current := k.GetTotalDistributed(ctx)
	k.SetTotalDistributed(ctx, current.Add(amt...))
}

// GetSNDistState returns the per-SN distribution state for a validator.
func (k Keeper) GetSNDistState(ctx sdk.Context, valAddr string) (SNDistState, bool) {
	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	bz := store.Get(types.SNDistStateKey(valAddr))
	if bz == nil {
		return SNDistState{}, false
	}
	var state SNDistState
	if err := json.Unmarshal(bz, &state); err != nil {
		return SNDistState{}, false
	}
	return state, true
}

// SetSNDistState stores the per-SN distribution state for a validator.
func (k Keeper) SetSNDistState(ctx sdk.Context, valAddr string, state SNDistState) {
	store := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	bz, err := json.Marshal(state)
	if err != nil {
		panic("failed to marshal SN dist state: " + err.Error())
	}
	store.Set(types.SNDistStateKey(valAddr), bz)
}

// applyGrowthCap limits the reported bytes growth to the configured cap per period.
// Returns the capped raw bytes value.
func applyGrowthCap(rawBytes, prevRawBytes float64, growthCapBps uint64) float64 {
	if prevRawBytes <= 0 {
		// First observation or previous was zero: no cap to apply.
		return rawBytes
	}
	maxGrowthFraction := float64(growthCapBps) / 10000.0
	maxAllowed := prevRawBytes * (1.0 + maxGrowthFraction)
	if rawBytes > maxAllowed {
		return maxAllowed
	}
	return rawBytes
}

// applyEMA computes an exponential moving average for the smoothed bytes.
// alpha = 2 / (periods + 1), which is the standard EMA formula.
func applyEMA(prevSmoothed, newValue float64, smoothingPeriods uint64) float64 {
	if smoothingPeriods == 0 {
		return newValue
	}
	alpha := 2.0 / (float64(smoothingPeriods) + 1.0)
	if prevSmoothed <= 0 {
		// First observation: use the new value directly.
		return newValue
	}
	return alpha*newValue + (1.0-alpha)*prevSmoothed
}

// computeRampUpWeight returns a fractional weight [0.0, 1.0] for new supernodes
// during their ramp-up period.
func computeRampUpWeight(periodsActive, rampUpPeriods uint64) float64 {
	if rampUpPeriods == 0 {
		return 1.0
	}
	if periodsActive >= rampUpPeriods {
		return 1.0
	}
	// Linear ramp: fraction of completed periods.
	return float64(periodsActive+1) / float64(rampUpPeriods)
}

// floatToUint64 safely converts a float64 to uint64, clamping negative values to 0.
func floatToUint64(f float64) uint64 {
	if f <= 0 || math.IsNaN(f) || math.IsInf(f, -1) {
		return 0
	}
	if math.IsInf(f, 1) || f > float64(math.MaxUint64) {
		return math.MaxUint64
	}
	return uint64(f)
}
