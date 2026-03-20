package keeper

import (
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

type Keeper struct {
	storeService corestore.KVStoreService
	cdc          codec.Codec
	addressCodec address.Codec
	authority    []byte

	Schema collections.Schema
	Params collections.Item[types.Params]

	// MigrationRecords stores completed migration records keyed by legacy address.
	MigrationRecords collections.Map[string, types.MigrationRecord]

	// MigrationCounter stores the total number of completed migrations.
	MigrationCounter collections.Item[uint64]

	// ValidatorMigrationCounter stores the total number of validator migrations.
	ValidatorMigrationCounter collections.Item[uint64]

	// BlockMigrationCounter stores per-block migration count keyed by block height.
	BlockMigrationCounter collections.Map[int64, uint64]

	// External keeper dependencies for migration logic.
	accountKeeper      types.AccountKeeper
	bankKeeper         types.BankKeeper
	stakingKeeper      types.StakingKeeper
	distributionKeeper types.DistributionKeeper
	authzKeeper        types.AuthzKeeper
	feegrantKeeper     types.FeegrantKeeper
	supernodeKeeper    types.SupernodeKeeper
	actionKeeper       types.ActionKeeper
	claimKeeper        types.ClaimKeeper
}

func NewKeeper(
	storeService corestore.KVStoreService,
	cdc codec.Codec,
	addressCodec address.Codec,
	authority []byte,
	accountKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
	stakingKeeper types.StakingKeeper,
	distributionKeeper types.DistributionKeeper,
	authzKeeper types.AuthzKeeper,
	feegrantKeeper types.FeegrantKeeper,
	supernodeKeeper types.SupernodeKeeper,
	actionKeeper types.ActionKeeper,
	claimKeeper types.ClaimKeeper,
) Keeper {
	if _, err := addressCodec.BytesToString(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address %s: %s", authority, err))
	}

	sb := collections.NewSchemaBuilder(storeService)

	k := Keeper{
		storeService: storeService,
		cdc:          cdc,
		addressCodec: addressCodec,
		authority:    authority,

		Params:                    collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
		MigrationRecords:          collections.NewMap(sb, types.MigrationRecordKeyPrefix, "migration_records", collections.StringKey, codec.CollValue[types.MigrationRecord](cdc)),
		MigrationCounter:          collections.NewItem(sb, types.MigrationCounterKey, "migration_counter", collections.Uint64Value),
		ValidatorMigrationCounter: collections.NewItem(sb, types.ValidatorMigrationCounterKey, "validator_migration_counter", collections.Uint64Value),
		BlockMigrationCounter:     collections.NewMap(sb, types.BlockMigrationCounterPrefix, "block_migration_counter", collections.Int64Key, collections.Uint64Value),

		accountKeeper:      accountKeeper,
		bankKeeper:         bankKeeper,
		stakingKeeper:      stakingKeeper,
		distributionKeeper: distributionKeeper,
		authzKeeper:        authzKeeper,
		feegrantKeeper:     feegrantKeeper,
		supernodeKeeper:    supernodeKeeper,
		actionKeeper:       actionKeeper,
		claimKeeper:        claimKeeper,
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}

// GetAuthority returns the module's authority.
func (k Keeper) GetAuthority() []byte {
	return k.authority
}
