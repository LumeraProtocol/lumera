package keeper

import (
	"cosmossdk.io/core/store"
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/pastelnetwork/pastel/x/supernode/types"
)

type (
	Keeper struct {
		cdc          codec.BinaryCodec
		storeService store.KVStoreService
		logger       log.Logger

		// the address capable of executing a MsgUpdateParams message. Typically, this
		// should be the x/gov module account.
		authority string

		bankKeeper     types.BankKeeper
		stakingKeeper  types.StakingKeeper
		slashingKeeper types.SlashingKeeper
		hooks          types.StakingHooksWrapper

		//auditKeeper     types.AuditKeeper // future Audit module
	}
)

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	logger log.Logger,
	authority string,

	bankKeeper types.BankKeeper,
	stakingKeeper types.StakingKeeper,
	slashingKeeper types.SlashingKeeper,
) Keeper {
	if _, err := sdk.AccAddressFromBech32(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address: %s", authority))
	}

	return Keeper{
		cdc:          cdc,
		storeService: storeService,
		authority:    authority,
		logger:       logger,

		bankKeeper:     bankKeeper,
		stakingKeeper:  stakingKeeper,
		slashingKeeper: slashingKeeper,
	}
}

// GetAuthority returns the module's authority.
func (k Keeper) GetAuthority() string {
	return k.authority
}

// Logger returns a module-specific logger.
func (k Keeper) Logger() log.Logger {
	return k.logger.With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// GetCodec returns the codec
func (k Keeper) GetCodec() codec.BinaryCodec {
	return k.cdc
}
func (k *Keeper) AddStakingHooks(hooks types.StakingHooksWrapper) {
	if !k.hooks.IsNil() {
		panic("cannot set staking hooks twice")
	}

	k.hooks = hooks
}

// GetHooks returns the staking hooks
func (k Keeper) GetHooks() types.StakingHooksWrapper {
	return k.hooks
}

// EnableSuperNode enables a validator's SuperNode status
func (k Keeper) EnableSuperNode(ctx sdk.Context, valAddr sdk.ValAddress) error {
	valOperAddr, err := sdk.ValAddressFromBech32(valAddr.String())
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator address: %s", err)
	}

	supernode, found := k.QuerySuperNode(ctx, valOperAddr)
	if !found {
		return errorsmod.Wrapf(sdkerrors.ErrNotFound, "no supernode found for validator")
	}

	if len(supernode.States) == 0 {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "supernode is in an invalid state")
	}

	if supernode.States[len(supernode.States)-1].State != types.SuperNodeStateActive {
		supernode.States = append(supernode.States, &types.SuperNodeStateRecord{
			State:  types.SuperNodeStateActive,
			Height: ctx.BlockHeight(),
		})
	}
	if err := k.SetSuperNode(ctx, supernode); err != nil {
		k.logger.With("module", fmt.Sprintf("error updating supernode state: %s", valAddr)).Error(fmt.Sprintf("x/%s", types.ModuleName))
		return errorsmod.Wrapf(sdkerrors.ErrNotFound, "eror updating supernode state")
	}

	return nil
}

// DisableSuperNode disables a validator's SuperNode status
func (k Keeper) DisableSuperNode(ctx sdk.Context, valAddr sdk.ValAddress) error {
	valOperAddr, err := sdk.ValAddressFromBech32(valAddr.String())
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator address: %s", err)
	}

	supernode, found := k.QuerySuperNode(ctx, valOperAddr)
	if !found {
		return errorsmod.Wrapf(sdkerrors.ErrNotFound, "no supernode found for validator")
	}

	if len(supernode.States) == 0 {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "supernode is in an invalid state")
	}

	if supernode.States[len(supernode.States)-1].State != types.SuperNodeStateDisabled {
		supernode.States = append(supernode.States, &types.SuperNodeStateRecord{
			State:  types.SuperNodeStateDisabled,
			Height: ctx.BlockHeight(),
		})
	}

	if err := k.SetSuperNode(ctx, supernode); err != nil {
		k.logger.With("module", fmt.Sprintf("error updating supernode state: %s", valAddr)).Error(fmt.Sprintf("x/%s", types.ModuleName))
		return errorsmod.Wrapf(sdkerrors.ErrNotFound, "eror updating supernode state")
	}

	return nil
}

func (k Keeper) IsSuperNodeActive(ctx sdk.Context, valAddr sdk.ValAddress) bool {
	valOperAddr, err := sdk.ValAddressFromBech32(valAddr.String())
	if err != nil {
		return false
	}

	supernode, found := k.QuerySuperNode(ctx, valOperAddr)
	if !found {
		return false
	}

	if len(supernode.States) == 0 {
		return false
	}

	return supernode.States[len(supernode.States)-1].State == types.SuperNodeStateActive
}

func (k Keeper) MeetsSuperNodeRequirements(ctx sdk.Context, valAddr sdk.ValAddress) bool {
	validator, err := k.stakingKeeper.Validator(ctx, valAddr)
	if err != nil || validator == nil {
		return false
	}

	err = k.CheckValidatorSupernodeEligibility(ctx, validator, valAddr.String())
	return err == nil
}

func (k Keeper) GetMinStake(ctx sdk.Context) sdkmath.Int {
	minStake := k.GetParams(ctx).MinimumStakeForSn
	minStakeInt := sdkmath.NewIntFromUint64(minStake)

	return minStakeInt
}

func (k Keeper) GetStakingKeeper() types.StakingKeeper {
	return k.stakingKeeper
}
