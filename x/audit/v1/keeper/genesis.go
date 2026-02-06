package keeper

import (
	"context"
	"errors"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx context.Context, genState types.GenesisState) error {
	// Genesis is the initial source of truth for module params. After genesis, params can
	// only be updated via governance (MsgUpdateParams).
	if err := k.SetParams(ctx, genState.Params); err != nil {
		return err
	}

	sdkCtx, ok := ctx.(sdk.Context)
	if !ok {
		sdkCtx = sdk.UnwrapSDKContext(ctx)
	}

	params := genState.Params.WithDefaults()
	if err := params.Validate(); err != nil {
		return err
	}

	var nextEvidenceID uint64
	if genState.NextEvidenceId != 0 {
		nextEvidenceID = genState.NextEvidenceId
	}

	for _, ev := range genState.Evidence {
		if err := k.SetEvidence(sdkCtx, ev); err != nil {
			return err
		}
		k.SetEvidenceBySubjectIndex(sdkCtx, ev.SubjectAddress, ev.EvidenceId)
		if ev.ActionId != "" {
			k.SetEvidenceByActionIndex(sdkCtx, ev.ActionId, ev.EvidenceId)
		}
		if ev.EvidenceId >= nextEvidenceID {
			nextEvidenceID = ev.EvidenceId + 1
		}
	}

	if nextEvidenceID == 0 {
		nextEvidenceID = 1
	}
	k.SetNextEvidenceID(sdkCtx, nextEvidenceID)

	return nil
}

// ExportGenesis returns the module's exported genesis.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	genesis := types.DefaultGenesis()
	genesis.Params = k.GetParams(ctx)

	sdkCtx, ok := ctx.(sdk.Context)
	if !ok {
		sdkCtx = sdk.UnwrapSDKContext(ctx)
	}

	evidence, err := k.GetAllEvidence(sdkCtx)
	if err != nil {
		return nil, err
	}
	genesis.Evidence = evidence
	genesis.NextEvidenceId = k.GetNextEvidenceID(sdkCtx)

	if genesis.NextEvidenceId == 0 {
		return nil, errors.New("invalid next evidence id")
	}

	return genesis, nil
}
