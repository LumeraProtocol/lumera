package keeper

import (
	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"fmt"
	"github.com/LumeraProtocol/lumera/x/action/types"
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

		bankKeeper      types.BankKeeper
		accountKeeper   types.AccountKeeper
		stakingKeeper   types.StakingKeeper
		supernodeKeeper types.SupernodeKeeper

		// Action handling
		actionRegistry *ActionRegistry
	}
)

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	logger log.Logger,
	authority string,

	bankKeeper types.BankKeeper,
	accountKeeper types.AccountKeeper,
	stakingKeeper types.StakingKeeper,
	supernodeKeeper types.SupernodeKeeper,
) Keeper {
	if _, err := sdk.AccAddressFromBech32(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address: %s", authority))
	}

	// Create the keeper instance
	keeper := Keeper{
		cdc:             cdc,
		storeService:    storeService,
		logger:          logger,
		authority:       authority,
		bankKeeper:      bankKeeper,
		accountKeeper:   accountKeeper,
		stakingKeeper:   stakingKeeper,
		supernodeKeeper: supernodeKeeper,
	}

	// Initialize action registry (requires keeper to be initialized first)
	keeper.actionRegistry = keeper.InitializeActionRegistry()

	return keeper
}

// GetAuthority returns the module's authority.
func (k *Keeper) GetAuthority() string {
	return k.authority
}

// Logger returns a module-specific logger.
func (k *Keeper) Logger() log.Logger {
	return k.logger.With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// GetCodec returns the codec
func (k *Keeper) GetCodec() codec.BinaryCodec {
	return k.cdc
}

func (k *Keeper) GetSupernodeKeeper() types.SupernodeKeeper {
	return k.supernodeKeeper
}

func (k *Keeper) GetBankKeeper() types.BankKeeper {
	return k.bankKeeper
}

func (k *Keeper) GetAccountKeeper() types.AccountKeeper {
	return k.accountKeeper
}

// GetActionRegistry returns the action registry
func (k *Keeper) GetActionRegistry() *ActionRegistry {
	return k.actionRegistry
}
