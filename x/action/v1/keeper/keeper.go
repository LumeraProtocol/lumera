package keeper

import (
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	snkeeper "github.com/LumeraProtocol/lumera/x/supernode/v1/keeper"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"github.com/cosmos/cosmos-sdk/codec"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"
)

type (
	Keeper struct {
		cdc          codec.BinaryCodec
		addressCodec address.Codec
		storeService corestore.KVStoreService
		logger       log.Logger

		// the address capable of executing a MsgUpdateParams message. Typically, this
		// should be the x/gov module account.
		authority []byte

		Schema collections.Schema
		Port   collections.Item[string]

		bankKeeper           actiontypes.BankKeeper
		authKeeper           actiontypes.AuthKeeper
		stakingKeeper        actiontypes.StakingKeeper
		distributionKeeper   actiontypes.DistributionKeeper
		supernodeKeeper      actiontypes.SupernodeKeeper
		supernodeQueryServer actiontypes.SupernodeQueryServer
		ibcKeeperFn          func() *ibckeeper.Keeper

		// Action handling
		actionRegistry *ActionRegistry
	}
)

func NewKeeper(
	cdc codec.BinaryCodec,
	addressCodec address.Codec,
	storeService corestore.KVStoreService,
	logger log.Logger,
	authority []byte,

	bankKeeper actiontypes.BankKeeper,
	accountKeeper actiontypes.AuthKeeper,
	stakingKeeper actiontypes.StakingKeeper,
	distributionKeeper actiontypes.DistributionKeeper,
	supernodeKeeper sntypes.SupernodeKeeper,
	supernodeQueryServer func() sntypes.QueryServer,
	ibcKeeperFn func() *ibckeeper.Keeper,
) Keeper {
	if _, err := addressCodec.BytesToString(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address: %s", authority))
	}

	sb := collections.NewSchemaBuilder(storeService)
	var snQueryServer sntypes.QueryServer
	if supernodeQueryServer == nil {
		snQueryServer = snkeeper.NewQueryServerImpl(supernodeKeeper)
	} else {
		snQueryServer = supernodeQueryServer()
	}

	// Create the k instance
	k := Keeper{
		cdc:                  cdc,
		addressCodec:         addressCodec,
		storeService:         storeService,
		logger:               logger,
		authority:            authority,
		bankKeeper:           bankKeeper,
		authKeeper:           accountKeeper,
		stakingKeeper:        stakingKeeper,
		distributionKeeper:   distributionKeeper,
		supernodeKeeper:      supernodeKeeper,
		supernodeQueryServer: snQueryServer,
		ibcKeeperFn:          ibcKeeperFn,

		Port: collections.NewItem(sb, actiontypes.PortKey, "port", collections.StringValue),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to build schema: %s", err))
	}
	k.Schema = schema

	// Initialize action registry (requires keeper to be initialized first)
	k.actionRegistry = k.InitializeActionRegistry()

	return k
}

// GetAuthority returns the module's authority.
func (k *Keeper) GetAuthority() []byte {
	return k.authority
}

// Logger returns a module-specific logger.
func (k *Keeper) Logger() log.Logger {
	return k.logger.With("module", fmt.Sprintf("x/%s", actiontypes.ModuleName))
}

// GetCodec returns the codec
func (k *Keeper) GetCodec() codec.BinaryCodec {
	return k.cdc
}

// GetAddressCodec returns the address codec
func (k *Keeper) GetAddressCodec() address.Codec {
	return k.addressCodec
}

func (k *Keeper) GetSupernodeKeeper() actiontypes.SupernodeKeeper {
	return k.supernodeKeeper
}

func (k *Keeper) GetSupernodeQueryServer() actiontypes.SupernodeQueryServer {
	return k.supernodeQueryServer
}

func (k *Keeper) GetBankKeeper() actiontypes.BankKeeper {
	return k.bankKeeper
}

func (k *Keeper) GetAuthKeeper() actiontypes.AuthKeeper {
	return k.authKeeper
}

// GetActionRegistry returns the action registry
func (k *Keeper) GetActionRegistry() *ActionRegistry {
	return k.actionRegistry
}

func (k *Keeper) GetStakingKeeper() actiontypes.StakingKeeper {
	return k.stakingKeeper
}
