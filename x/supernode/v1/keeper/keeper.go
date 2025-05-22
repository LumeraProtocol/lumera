package keeper

import (
	"fmt"

	types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type (
	Keeper struct {
		cdc          codec.BinaryCodec
		storeService store.KVStoreService
		logger       log.Logger

		// the address capable of executing a MsgUpdateParams message. Typically, this
		// should be the x/gov module account.
		authority string

		bankKeeper     types2.BankKeeper
		stakingKeeper  types2.StakingKeeper
		slashingKeeper types2.SlashingKeeper

		//auditKeeper     types.AuditKeeper // future Audit module
	}
)

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	logger log.Logger,
	authority string,

	bankKeeper types2.BankKeeper,
	stakingKeeper types2.StakingKeeper,
	slashingKeeper types2.SlashingKeeper,
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
	return k.logger.With("module", fmt.Sprintf("x/%s", types2.ModuleName))
}

// GetCodec returns the codec
func (k Keeper) GetCodec() codec.BinaryCodec {
	return k.cdc
}

func (k Keeper) GetStakingKeeper() types2.StakingKeeper {
	return k.stakingKeeper
}
