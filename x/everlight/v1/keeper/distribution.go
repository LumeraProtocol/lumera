package keeper

import (
	"fmt"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	lcfg "github.com/LumeraProtocol/lumera/config"
	"github.com/LumeraProtocol/lumera/x/everlight/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// countEligibleSNs returns the number of supernodes currently eligible for distribution.
func (k Keeper) countEligibleSNs(ctx sdk.Context) uint64 {
	params := k.GetParams(ctx)

	supernodes, err := k.supernodeKeeper.GetAllSuperNodes(
		ctx,
		sntypes.SuperNodeStateActive,
		sntypes.SuperNodeStateStorageFull,
	)
	if err != nil {
		return 0
	}

	var count uint64
	for _, sn := range supernodes {
		valAddr, err := sdk.ValAddressFromBech32(sn.ValidatorAddress)
		if err != nil {
			continue
		}

		metricsState, found := k.supernodeKeeper.GetMetricsState(ctx, valAddr)
		if !found {
			continue
		}

		rawBytes := float64(0)
		if metricsState.Metrics != nil {
			rawBytes = metricsState.Metrics.CascadeKademliaDbBytes
		}

		distState, exists := k.GetSNDistState(ctx, sn.ValidatorAddress)
		smoothedBytes := rawBytes
		if exists {
			cappedBytes := applyGrowthCap(rawBytes, distState.PrevRawBytes, params.UsageGrowthCapBpsPerPeriod)
			smoothedBytes = applyEMA(distState.SmoothedBytes, cappedBytes, params.MeasurementSmoothingPeriods)
		}

		if floatToUint64(smoothedBytes) >= params.MinCascadeBytesForPayment {
			count++
		}
	}

	return count
}

// snCandidate holds the intermediate distribution data for a single supernode.
type snCandidate struct {
	validatorAddr    string
	supernodeAccount string
	rawBytes         float64
	cappedBytes      float64
	smoothedBytes    float64
	rampWeight       float64
	effectiveWeight  float64
	distState        SNDistState
}

// distributePool is the core distribution logic called by EndBlocker when
// payment_period_blocks have elapsed.
func (k Keeper) distributePool(ctx sdk.Context) error {
	params := k.GetParams(ctx)
	currentHeight := ctx.BlockHeight()

	// 1. Get pool balance.
	poolBalance := k.GetPoolBalance(ctx)
	poolUlume := poolBalance.AmountOf(lcfg.ChainDenom)

	// If pool balance is zero, emit event and return (AT44).
	if poolUlume.IsZero() {
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeDistribution,
				sdk.NewAttribute(types.AttributeKeySkipReason, "pool_balance_zero"),
				sdk.NewAttribute(types.AttributeKeyPoolBalance, "0"),
			),
		)
		k.Logger().Info("everlight distribution skipped: pool balance is zero")
		k.SetLastDistributionHeight(ctx, currentHeight)
		return nil
	}

	// 2. Get all ACTIVE or STORAGE_FULL supernodes.
	supernodes, err := k.supernodeKeeper.GetAllSuperNodes(
		ctx,
		sntypes.SuperNodeStateActive,
		sntypes.SuperNodeStateStorageFull,
	)
	if err != nil {
		return fmt.Errorf("failed to get supernodes: %w", err)
	}

	// 3. Build candidates, applying anti-gaming rules.
	candidates := make([]snCandidate, 0, len(supernodes))
	for _, sn := range supernodes {
		valAddr, err := sdk.ValAddressFromBech32(sn.ValidatorAddress)
		if err != nil {
			k.Logger().Error("invalid validator address in supernode", "addr", sn.ValidatorAddress, "err", err)
			continue
		}

		// Read metrics state.
		metricsState, found := k.supernodeKeeper.GetMetricsState(ctx, valAddr)
		if !found {
			// SN has no metrics reported yet; skip.
			continue
		}

		rawBytes := float64(0)
		if metricsState.Metrics != nil {
			rawBytes = metricsState.Metrics.CascadeKademliaDbBytes
		}

		// Load existing per-SN distribution state.
		distState, exists := k.GetSNDistState(ctx, sn.ValidatorAddress)
		if !exists {
			distState = SNDistState{
				EligibilityStartHeight: currentHeight,
				PeriodsActive:          0,
				SmoothedBytes:          0,
				PrevRawBytes:           0,
			}
		}

		// Apply growth cap.
		cappedBytes := applyGrowthCap(rawBytes, distState.PrevRawBytes, params.UsageGrowthCapBpsPerPeriod)

		// Apply EMA smoothing.
		smoothedBytes := applyEMA(distState.SmoothedBytes, cappedBytes, params.MeasurementSmoothingPeriods)

		// Check minimum threshold (AT36).
		if floatToUint64(smoothedBytes) < params.MinCascadeBytesForPayment {
			// Update state but don't include in distribution.
			distState.SmoothedBytes = smoothedBytes
			distState.PrevRawBytes = rawBytes
			distState.PeriodsActive++
			k.SetSNDistState(ctx, sn.ValidatorAddress, distState)
			continue
		}

		// Compute ramp-up weight (AT37).
		rampWeight := computeRampUpWeight(distState.PeriodsActive, params.NewSnRampUpPeriods)

		// Effective weight = smoothed bytes * ramp-up weight.
		effectiveWeight := smoothedBytes * rampWeight

		candidates = append(candidates, snCandidate{
			validatorAddr:    sn.ValidatorAddress,
			supernodeAccount: sn.SupernodeAccount,
			rawBytes:         rawBytes,
			cappedBytes:      cappedBytes,
			smoothedBytes:    smoothedBytes,
			rampWeight:       rampWeight,
			effectiveWeight:  effectiveWeight,
			distState:        distState,
		})
	}

	// If no eligible SNs, emit event and return (AT45).
	if len(candidates) == 0 {
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeDistribution,
				sdk.NewAttribute(types.AttributeKeySkipReason, "no_eligible_supernodes"),
				sdk.NewAttribute(types.AttributeKeyPoolBalance, poolUlume.String()),
			),
		)
		k.Logger().Info("everlight distribution skipped: no eligible supernodes")
		k.SetLastDistributionHeight(ctx, currentHeight)
		return nil
	}

	// 4. Compute total effective weight.
	var totalWeight float64
	for _, c := range candidates {
		totalWeight += c.effectiveWeight
	}

	if totalWeight <= 0 {
		k.Logger().Info("everlight distribution skipped: total weight is zero")
		k.SetLastDistributionHeight(ctx, currentHeight)
		return nil
	}

	// 5. Distribute pool balance proportionally.
	totalDistributed := sdkmath.ZeroInt()
	payouts := make([]struct {
		addr   sdk.AccAddress
		amount sdkmath.Int
		cand   snCandidate
	}, 0, len(candidates))

	for _, c := range candidates {
		share := c.effectiveWeight / totalWeight
		// Calculate integer amount; truncate fractions (dust stays in pool).
		shareDec := sdkmath.LegacyNewDecWithPrec(int64(share*1e18), 18)
		payoutAmount := sdkmath.LegacyNewDecFromInt(poolUlume).MulTruncate(shareDec).TruncateInt()

		if payoutAmount.IsPositive() {
			recipientAddr, err := sdk.AccAddressFromBech32(c.supernodeAccount)
			if err != nil {
				k.Logger().Error("invalid supernode account address", "addr", c.supernodeAccount, "err", err)
				continue
			}
			payouts = append(payouts, struct {
				addr   sdk.AccAddress
				amount sdkmath.Int
				cand   snCandidate
			}{recipientAddr, payoutAmount, c})
			totalDistributed = totalDistributed.Add(payoutAmount)
		}
	}

	// 6. Execute payouts via bank module.
	for _, p := range payouts {
		coins := sdk.NewCoins(sdk.NewCoin(lcfg.ChainDenom, p.amount))
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, p.addr, coins); err != nil {
			return fmt.Errorf("failed to send distribution to %s: %w", p.addr, err)
		}

		// Emit per-SN distribution event.
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeDistribution,
				sdk.NewAttribute(types.AttributeKeyRecipient, p.addr.String()),
				sdk.NewAttribute(types.AttributeKeyValidator, p.cand.validatorAddr),
				sdk.NewAttribute(types.AttributeKeyAmount, p.amount.String()),
				sdk.NewAttribute(types.AttributeKeySmoothedBytes, strconv.FormatFloat(p.cand.smoothedBytes, 'f', 0, 64)),
				sdk.NewAttribute(types.AttributeKeyRawBytes, strconv.FormatFloat(p.cand.rawBytes, 'f', 0, 64)),
			),
		)
	}

	// 7. Update per-SN distribution state for all candidates (including non-paying ones
	// which were already updated above).
	for _, c := range candidates {
		newDistState := c.distState
		newDistState.SmoothedBytes = c.smoothedBytes
		newDistState.PrevRawBytes = c.rawBytes
		newDistState.PeriodsActive++
		k.SetSNDistState(ctx, c.validatorAddr, newDistState)
	}

	// 8. Update global state.
	totalPayoutCoins := sdk.NewCoins(sdk.NewCoin(lcfg.ChainDenom, totalDistributed))
	k.AddTotalDistributed(ctx, totalPayoutCoins)
	k.SetLastDistributionHeight(ctx, currentHeight)

	// 9. Emit summary event.
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeDistribution,
			sdk.NewAttribute(types.AttributeKeyEligibleCount, strconv.Itoa(len(candidates))),
			sdk.NewAttribute(types.AttributeKeyTotalPayout, totalDistributed.String()),
			sdk.NewAttribute(types.AttributeKeyPoolBalance, poolUlume.String()),
		),
	)

	k.Logger().Info("everlight distribution completed",
		"eligible_count", len(candidates),
		"total_distributed", totalDistributed.String(),
		"pool_balance_before", poolUlume.String(),
	)

	return nil
}
