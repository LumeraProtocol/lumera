package keeper

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

func (k Keeper) ProcessStorageTruthHealOpsAtEpochEnd(ctx sdk.Context, epochID uint64, params types.Params) error {
	healOps, err := k.GetAllHealOps(ctx)
	if err != nil {
		return err
	}

	healOps, err = k.expireStorageTruthHealOpsAtEpochEnd(ctx, epochID, healOps)
	if err != nil {
		return err
	}
	return k.scheduleStorageTruthHealOpsAtEpochEnd(ctx, epochID, params, healOps)
}

func (k Keeper) expireStorageTruthHealOpsAtEpochEnd(ctx sdk.Context, epochID uint64, healOps []types.HealOp) ([]types.HealOp, error) {
	for i, healOp := range healOps {
		if isHealOpFinalStatus(healOp.Status) {
			continue
		}
		if healOp.DeadlineEpochId == 0 || healOp.DeadlineEpochId > epochID {
			continue
		}

		healOp.Status = types.HealOpStatus_HEAL_OP_STATUS_EXPIRED
		healOp.UpdatedHeight = uint64(ctx.BlockHeight())
		if err := k.SetHealOp(ctx, healOp); err != nil {
			return nil, err
		}
		healOps[i] = healOp

		ticketState, found := k.GetTicketDeteriorationState(ctx, healOp.TicketId)
		if found && ticketState.ActiveHealOpId == healOp.HealOpId {
			ticketState.ActiveHealOpId = 0
			if err := k.SetTicketDeteriorationState(ctx, ticketState); err != nil {
				return nil, err
			}
		}

		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeHealOpExpired,
				sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
				sdk.NewAttribute(types.AttributeKeyEpochID, strconv.FormatUint(epochID, 10)),
				sdk.NewAttribute(types.AttributeKeyHealOpID, strconv.FormatUint(healOp.HealOpId, 10)),
				sdk.NewAttribute(types.AttributeKeyTicketID, healOp.TicketId),
			),
		)
	}

	return healOps, nil
}

func (k Keeper) scheduleStorageTruthHealOpsAtEpochEnd(ctx sdk.Context, epochID uint64, params types.Params, healOps []types.HealOp) error {
	if params.StorageTruthMaxSelfHealOpsPerEpoch == 0 {
		return nil
	}

	activeAccounts, err := k.storageTruthSchedulerAccounts(ctx, epochID)
	if err != nil {
		return err
	}
	if len(activeAccounts) == 0 {
		return nil
	}

	nonFinalByID := make(map[uint64]types.HealOp, len(healOps))
	openByTicket := make(map[string]types.HealOp, len(healOps))
	for _, healOp := range healOps {
		if isHealOpFinalStatus(healOp.Status) {
			continue
		}
		nonFinalByID[healOp.HealOpId] = healOp
		openByTicket[healOp.TicketId] = healOp
	}

	ticketStates, err := k.GetAllTicketDeteriorationStates(ctx)
	if err != nil {
		return err
	}

	type candidate struct {
		ticketID string
		score    int64
	}
	candidates := make([]candidate, 0, len(ticketStates))

	for _, state := range ticketStates {
		if state.TicketId == "" {
			continue
		}
		if state.DeteriorationScore < params.StorageTruthTicketDeteriorationHealThreshold {
			continue
		}
		if state.ProbationUntilEpoch > epochID {
			continue
		}

		if state.ActiveHealOpId != 0 {
			if activeOp, found := nonFinalByID[state.ActiveHealOpId]; found {
				openByTicket[state.TicketId] = activeOp
				continue
			}
			// Clear stale pointer to a non-existing/finalized op to keep state self-consistent.
			state.ActiveHealOpId = 0
			if err := k.SetTicketDeteriorationState(ctx, state); err != nil {
				return err
			}
		}

		if _, hasOpen := openByTicket[state.TicketId]; hasOpen {
			continue
		}

		candidates = append(candidates, candidate{ticketID: state.TicketId, score: state.DeteriorationScore})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].ticketID < candidates[j].ticketID
		}
		return candidates[i].score > candidates[j].score
	})

	scheduled := uint32(0)
	for _, cand := range candidates {
		if scheduled >= params.StorageTruthMaxSelfHealOpsPerEpoch {
			break
		}

		healer, verifiers := assignStorageTruthHealParticipants(activeAccounts, cand.ticketID, epochID)
		healOpID := k.GetNextHealOpID(ctx)
		healOp := types.HealOp{
			HealOpId:                  healOpID,
			TicketId:                  cand.ticketID,
			ScheduledEpochId:          epochID,
			HealerSupernodeAccount:    healer,
			VerifierSupernodeAccounts: verifiers,
			Status:                    types.HealOpStatus_HEAL_OP_STATUS_SCHEDULED,
			CreatedHeight:             uint64(ctx.BlockHeight()),
			UpdatedHeight:             uint64(ctx.BlockHeight()),
			DeadlineEpochId:           epochID + 1,
		}

		if err := k.SetHealOp(ctx, healOp); err != nil {
			return err
		}
		k.SetNextHealOpID(ctx, healOpID+1)

		ticketState, found := k.GetTicketDeteriorationState(ctx, cand.ticketID)
		if !found {
			return fmt.Errorf("ticket deterioration state not found for ticket %q while scheduling heal op", cand.ticketID)
		}
		ticketState.ActiveHealOpId = healOpID
		if err := k.SetTicketDeteriorationState(ctx, ticketState); err != nil {
			return err
		}

		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeHealOpScheduled,
				sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
				sdk.NewAttribute(types.AttributeKeyEpochID, strconv.FormatUint(epochID, 10)),
				sdk.NewAttribute(types.AttributeKeyHealOpID, strconv.FormatUint(healOpID, 10)),
				sdk.NewAttribute(types.AttributeKeyTicketID, cand.ticketID),
				sdk.NewAttribute(types.AttributeKeyHealerSupernodeAccount, healer),
				sdk.NewAttribute(types.AttributeKeyDeadlineEpochID, strconv.FormatUint(healOp.DeadlineEpochId, 10)),
			),
		)
		scheduled++
	}

	return nil
}

func (k Keeper) storageTruthSchedulerAccounts(ctx sdk.Context, epochID uint64) ([]string, error) {
	if anchor, found := k.GetEpochAnchor(ctx, epochID); found && len(anchor.ActiveSupernodeAccounts) > 0 {
		return append([]string(nil), anchor.ActiveSupernodeAccounts...), nil
	}

	active, err := k.supernodeKeeper.GetAllSuperNodes(ctx, sntypes.SuperNodeStateActive)
	if err != nil {
		return nil, err
	}
	accounts, err := supernodeAccountsFromSet(active)
	if err != nil {
		return nil, err
	}
	sort.Strings(accounts)
	return accounts, nil
}

func assignStorageTruthHealParticipants(activeAccounts []string, ticketID string, epochID uint64) (string, []string) {
	if len(activeAccounts) == 0 {
		return "", nil
	}

	idx := deterministicStorageTruthIndex(ticketID, epochID, len(activeAccounts))
	healer := activeAccounts[idx]

	if len(activeAccounts) == 1 {
		return healer, nil
	}

	verifierCount := 2
	if verifierCount > len(activeAccounts)-1 {
		verifierCount = len(activeAccounts) - 1
	}
	verifiers := make([]string, 0, verifierCount)
	for i := 1; i <= verifierCount; i++ {
		verifiers = append(verifiers, activeAccounts[(idx+i)%len(activeAccounts)])
	}
	return healer, verifiers
}

func deterministicStorageTruthIndex(ticketID string, epochID uint64, n int) int {
	if n <= 1 {
		return 0
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(ticketID))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strconv.FormatUint(epochID, 10)))
	return int(h.Sum64() % uint64(n))
}

func isHealOpFinalStatus(status types.HealOpStatus) bool {
	switch status {
	case types.HealOpStatus_HEAL_OP_STATUS_VERIFIED,
		types.HealOpStatus_HEAL_OP_STATUS_FAILED,
		types.HealOpStatus_HEAL_OP_STATUS_EXPIRED:
		return true
	default:
		return false
	}
}
