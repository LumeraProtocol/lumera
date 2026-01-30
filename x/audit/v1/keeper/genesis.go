package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"
	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx context.Context, genState types.GenesisState) error {
	if err := k.Params.Set(ctx, genState.Params); err != nil {
		return err
	}

	var maxID uint64
	for _, ev := range genState.Evidence {
		if err := k.Evidences.Set(ctx, ev.EvidenceId, ev); err != nil {
			return err
		}
		if err := k.BySubject.Set(ctx, collections.Join(ev.SubjectAddress, ev.EvidenceId)); err != nil {
			return err
		}
		if ev.ActionId != "" {
			if err := k.ByActionID.Set(ctx, collections.Join(ev.ActionId, ev.EvidenceId)); err != nil {
				return err
			}
		}
		if ev.EvidenceId > maxID {
			maxID = ev.EvidenceId
		}
	}

	nextID := genState.NextEvidenceId
	if nextID == 0 && len(genState.Evidence) > 0 {
		nextID = maxID + 1
	}
	if nextID != 0 {
		if err := k.EvidenceID.Set(ctx, nextID); err != nil {
			return err
		}
	}

	return nil
}

// ExportGenesis returns the module's exported genesis.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	var err error

	genesis := types.DefaultGenesis()
	genesis.Params, err = k.Params.Get(ctx)
	if err != nil {
		if !errors.Is(err, collections.ErrNotFound) {
			return nil, err
		}
	}

	evidence := make([]types.Evidence, 0)
	if err := k.Evidences.Walk(ctx, nil, func(_ uint64, ev types.Evidence) (bool, error) {
		evidence = append(evidence, ev)
		return false, nil
	}); err != nil {
		return nil, err
	}
	genesis.Evidence = evidence

	nextID, err := k.EvidenceID.Peek(ctx)
	if err != nil {
		return nil, err
	}
	genesis.NextEvidenceId = nextID

	return genesis, nil
}
