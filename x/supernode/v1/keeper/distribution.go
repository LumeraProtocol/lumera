package keeper

import (
	"fmt"
	"math"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	lcfg "github.com/LumeraProtocol/lumera/config"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// countEligibleSNs returns the number of supernodes currently eligible for distribution.
func (k Keeper) CountEligibleSNs(ctx sdk.Context) uint64 {
	params := k.GetParams(ctx)
	dist := params.RewardDistribution

	supernodes, err := k.GetAllSuperNodes(
		ctx,
		sntypes.SuperNodeStateActive,
		sntypes.SuperNodeStateStorageFull,
	)
	if err != nil {
		return 0
	}

	var count uint64
	for _, sn := range supernodes {
		rawBytes, reportHeight, found := k.getLatestCascadeBytesFromAudit(ctx, sn.SupernodeAccount)
		if !found {
			continue
		}
		if !isFreshByBlockHeight(ctx.BlockHeight(), reportHeight, params.MetricsFreshnessMaxBlocks) {
			continue
		}

		distState, exists := k.GetSNDistState(ctx, sn.ValidatorAddress)
		smoothedBytes := rawBytes
		if exists {
			cappedBytes := applyGrowthCap(rawBytes, distState.PrevRawBytes, dist.UsageGrowthCapBpsPerPeriod)
			smoothedBytes = applyEMA(distState.SmoothedBytes, cappedBytes, dist.MeasurementSmoothingPeriods)
		}

		if floatToUint64(smoothedBytes) >= dist.MinCascadeBytesForPayment {
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
	dist := params.RewardDistribution
	currentHeight := ctx.BlockHeight()

	// 1. Get pool balance.
	poolBalance := k.GetPoolBalance(ctx)
	poolUlume := poolBalance.AmountOf(lcfg.ChainDenom)

	// If pool balance is zero, emit event and return (AT44).
	if poolUlume.IsZero() {
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				sntypes.EventTypeDistribution,
				sdk.NewAttribute(sntypes.AttributeKeyRewardSkipReason, "pool_balance_zero"),
				sdk.NewAttribute(sntypes.AttributeKeyRewardPoolBalance, "0"),
			),
		)
		k.Logger().Info("everlight distribution skipped: pool balance is zero")
		k.SetLastDistributionHeight(ctx, currentHeight)
		return nil
	}

	// 2. Get all ACTIVE or STORAGE_FULL supernodes.
	supernodes, err := k.GetAllSuperNodes(
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
		rawBytes, reportHeight, found := k.getLatestCascadeBytesFromAudit(ctx, sn.SupernodeAccount)
		if !found {
			// SN has no usable audit report yet; skip.
			continue
		}
		if !isFreshByBlockHeight(currentHeight, reportHeight, params.MetricsFreshnessMaxBlocks) {
			// Report is stale by block-height freshness rule; skip.
			continue
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
		cappedBytes := applyGrowthCap(rawBytes, distState.PrevRawBytes, dist.UsageGrowthCapBpsPerPeriod)

		// Apply EMA smoothing.
		smoothedBytes := applyEMA(distState.SmoothedBytes, cappedBytes, dist.MeasurementSmoothingPeriods)

		// Check minimum threshold (AT36).
		if floatToUint64(smoothedBytes) < dist.MinCascadeBytesForPayment {
			// Update state but don't include in distribution.
			distState.SmoothedBytes = smoothedBytes
			distState.PrevRawBytes = rawBytes
			distState.PeriodsActive++
			k.SetSNDistState(ctx, sn.ValidatorAddress, distState)
			continue
		}

		// Compute ramp-up weight (AT37).
		rampWeight := computeRampUpWeight(distState.PeriodsActive, dist.NewSnRampUpPeriods)

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
				sntypes.EventTypeDistribution,
				sdk.NewAttribute(sntypes.AttributeKeyRewardSkipReason, "no_eligible_supernodes"),
				sdk.NewAttribute(sntypes.AttributeKeyRewardPoolBalance, poolUlume.String()),
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
	poolBalanceDec := sdkmath.LegacyNewDecFromInt(poolUlume)
	totalWeightDec, err := legacyDecFromFloat64(totalWeight)
	if err != nil {
		return fmt.Errorf("invalid total distribution weight: %w", err)
	}
	totalDistributed := sdkmath.ZeroInt()
	payouts := make([]struct {
		addr   sdk.AccAddress
		amount sdkmath.Int
		cand   snCandidate
	}, 0, len(candidates))

	for _, c := range candidates {
		weightDec, err := legacyDecFromFloat64(c.effectiveWeight)
		if err != nil {
			k.Logger().Error("invalid candidate distribution weight", "validator", c.validatorAddr, "err", err)
			continue
		}
		shareDec := weightDec.Quo(totalWeightDec)
		// Calculate integer amount; truncate fractions (dust stays in pool).
		payoutAmount := poolBalanceDec.MulTruncate(shareDec).TruncateInt()

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
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, sntypes.ModuleName, p.addr, coins); err != nil {
			return fmt.Errorf("failed to send distribution to %s: %w", p.addr, err)
		}

		k.AppendPayoutHistoryEntry(ctx, &sntypes.PayoutHistoryEntry{
			Height:           currentHeight,
			ValidatorAddress: p.cand.validatorAddr,
			SupernodeAccount: p.cand.supernodeAccount,
			Amount:           coins,
			RawBytes:         p.cand.rawBytes,
			SmoothedBytes:    p.cand.smoothedBytes,
			EffectiveWeight:  p.cand.effectiveWeight,
			RampWeight:       p.cand.rampWeight,
		})

		// Emit per-SN distribution event.
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				sntypes.EventTypeDistribution,
				sdk.NewAttribute(sntypes.AttributeKeyRewardRecipient, p.addr.String()),
				sdk.NewAttribute(sntypes.AttributeKeyRewardValidator, p.cand.validatorAddr),
				sdk.NewAttribute(sntypes.AttributeKeyRewardAmount, p.amount.String()),
				sdk.NewAttribute(sntypes.AttributeKeyRewardSmoothedBytes, strconv.FormatFloat(p.cand.smoothedBytes, 'f', 0, 64)),
				sdk.NewAttribute(sntypes.AttributeKeyRewardRawBytes, strconv.FormatFloat(p.cand.rawBytes, 'f', 0, 64)),
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
			sntypes.EventTypeDistribution,
			sdk.NewAttribute(sntypes.AttributeKeyRewardEligibleCount, strconv.Itoa(len(candidates))),
			sdk.NewAttribute(sntypes.AttributeKeyRewardTotalPayout, totalDistributed.String()),
			sdk.NewAttribute(sntypes.AttributeKeyRewardPoolBalance, poolUlume.String()),
		),
	)

	k.Logger().Info("everlight distribution completed",
		"eligible_count", len(candidates),
		"total_distributed", totalDistributed.String(),
		"pool_balance_before", poolUlume.String(),
	)

	return nil
}

func legacyDecFromFloat64(value float64) (sdkmath.LegacyDec, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return sdkmath.LegacyZeroDec(), fmt.Errorf("invalid float value %v", value)
	}
	return sdkmath.LegacyNewDecFromStr(strconv.FormatFloat(value, 'f', -1, 64))
}
