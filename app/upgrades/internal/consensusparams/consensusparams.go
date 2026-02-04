package consensusparams

import (
	"errors"
	"fmt"

	"cosmossdk.io/collections"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"

	appParams "github.com/LumeraProtocol/lumera/app/upgrades/params"
)

func MigrateFromLegacy(ctx sdk.Context, p appParams.AppUpgradeParams, upgradeName string) error {
	if err := requireKeepers(p, upgradeName); err != nil {
		return err
	}

	legacySubspace := p.ParamsKeeper.Subspace(baseapp.Paramspace).WithKeyTable(paramstypes.ConsensusParamsKeyTable())
	legacyParams := baseapp.GetConsensusParams(ctx, legacySubspace)
	if legacyParams == nil {
		p.Logger.Info("Legacy consensus params missing; skipping migration", "upgrade", upgradeName)
		return nil
	}
	if !isConsensusParamsComplete(*legacyParams) {
		p.Logger.Info("Legacy consensus params incomplete; skipping migration", "upgrade", upgradeName)
		return nil
	}

	if err := baseapp.MigrateParams(ctx, legacySubspace, p.ConsensusParamsKeeper.ParamsStore); err != nil {
		return fmt.Errorf("failed to migrate consensus params: %w", err)
	}
	p.Logger.Info("Legacy consensus params migrated", "upgrade", upgradeName)
	return nil
}

func EnsurePresent(ctx sdk.Context, p appParams.AppUpgradeParams, upgradeName string) error {
	if err := requireKeepers(p, upgradeName); err != nil {
		return err
	}

	legacySubspace := p.ParamsKeeper.Subspace(baseapp.Paramspace).WithKeyTable(paramstypes.ConsensusParamsKeyTable())
	defaultParams := cmttypes.DefaultConsensusParams().ToProto()
	legacyParams := baseapp.GetConsensusParams(ctx, legacySubspace)

	targetParams := mergeConsensusParams(defaultParams, legacyParams)

	storeParams, err := p.ConsensusParamsKeeper.ParamsStore.Get(ctx)
	switch {
	case err == nil:
		if !isConsensusParamsComplete(storeParams) {
			fixed := mergeConsensusParams(targetParams, &storeParams)
			if !isConsensusParamsComplete(fixed) {
				return fmt.Errorf("consensus params remain incomplete after merge")
			}
			if err := p.ConsensusParamsKeeper.ParamsStore.Set(ctx, fixed); err != nil {
				return fmt.Errorf("failed to repair consensus params: %w", err)
			}
			p.Logger.Info("Consensus params were incomplete; repaired using legacy/defaults")
		} else {
			p.Logger.Info("Consensus params already set; skipping repair")
		}
	case errors.Is(err, collections.ErrNotFound):
		if err := p.ConsensusParamsKeeper.ParamsStore.Set(ctx, targetParams); err != nil {
			return fmt.Errorf("failed to seed consensus params: %w", err)
		}
		logSource := "defaults"
		if legacyParams != nil && isConsensusParamsComplete(*legacyParams) {
			logSource = "legacy"
		}
		p.Logger.Info("Consensus params missing; seeded", "source", logSource)
	default:
		return fmt.Errorf("failed to read consensus params: %w", err)
	}

	return nil
}

func requireKeepers(p appParams.AppUpgradeParams, upgradeName string) error {
	if p.ParamsKeeper == nil || p.ConsensusParamsKeeper == nil {
		return fmt.Errorf("%s upgrade requires ParamsKeeper and ConsensusParamsKeeper", upgradeName)
	}
	return nil
}

func isConsensusParamsComplete(p cmtproto.ConsensusParams) bool {
	if p.Block == nil || p.Evidence == nil || p.Validator == nil {
		return false
	}
	if p.Block.MaxBytes == 0 {
		return false
	}
	if p.Evidence.MaxAgeNumBlocks <= 0 || p.Evidence.MaxAgeDuration <= 0 {
		return false
	}
	if len(p.Validator.PubKeyTypes) == 0 {
		return false
	}
	return true
}

func mergeConsensusParams(base cmtproto.ConsensusParams, override *cmtproto.ConsensusParams) cmtproto.ConsensusParams {
	if override == nil {
		return base
	}

	if override.Block != nil && override.Block.MaxBytes != 0 {
		base.Block = override.Block
	}
	if override.Evidence != nil && override.Evidence.MaxAgeNumBlocks > 0 && override.Evidence.MaxAgeDuration > 0 {
		base.Evidence = override.Evidence
	}
	if override.Validator != nil && len(override.Validator.PubKeyTypes) > 0 {
		base.Validator = override.Validator
	}
	if override.Version != nil {
		base.Version = override.Version
	}
	if override.Abci != nil {
		base.Abci = override.Abci
	}
	return base
}
