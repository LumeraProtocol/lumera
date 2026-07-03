package keeper

import (
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// stakingStoreHandle holds a mutable reference to staking's KVStoreService.
// Populated post-depinject via SetStakingStoreService. The pointer is shared
// across all Keeper copies (value-type Keeper returned by NewKeeper gets
// cloned to app.EvmigrationKeeper and AppModule.keeper; both copies see the
// same underlying cell). Only used by DeleteValidatorRecordNoHooks.
type stakingStoreHandle struct {
	svc corestore.KVStoreService
}

// distributionStoreHandle holds a mutable reference to distribution's
// KVStoreService. It is wired post-depinject via SetDistributionStoreService and
// used only by migration code that must range over validator-scoped prefixes.
type distributionStoreHandle struct {
	svc corestore.KVStoreService
}

type Keeper struct {
	storeService corestore.KVStoreService
	cdc          codec.Codec
	addressCodec address.Codec
	authority    []byte

	Schema collections.Schema
	Params collections.Item[types.Params]

	// stakingStoreHandle grants this keeper migration-only raw write access to
	// x/staking's KV namespace. Wired post-build in app.go via
	// SetStakingStoreService; used exclusively by DeleteValidatorRecordNoHooks.
	stakingStoreHandle *stakingStoreHandle
	// distributionStoreHandle grants this keeper migration-only raw read access
	// to x/distribution's KV namespace for validator-scoped iteration.
	distributionStoreHandle *distributionStoreHandle

	// MigrationRecords stores completed migration records keyed by legacy address.
	MigrationRecords collections.Map[string, types.MigrationRecord]
	// MigrationRecordByNewAddress stores the legacy address for a completed migration keyed by new address.
	MigrationRecordByNewAddress collections.Map[string, string]

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

		Params:                      collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
		MigrationRecords:            collections.NewMap(sb, types.MigrationRecordKeyPrefix, "migration_records", collections.StringKey, codec.CollValue[types.MigrationRecord](cdc)),
		MigrationRecordByNewAddress: collections.NewMap(sb, types.MigrationRecordByNewAddressKeyPrefix, "migration_record_by_new_address", collections.StringKey, collections.StringValue),
		MigrationCounter:            collections.NewItem(sb, types.MigrationCounterKey, "migration_counter", collections.Uint64Value),
		ValidatorMigrationCounter:   collections.NewItem(sb, types.ValidatorMigrationCounterKey, "validator_migration_counter", collections.Uint64Value),
		BlockMigrationCounter:       collections.NewMap(sb, types.BlockMigrationCounterPrefix, "block_migration_counter", collections.Int64Key, collections.Uint64Value),

		accountKeeper:      accountKeeper,
		bankKeeper:         bankKeeper,
		stakingKeeper:      stakingKeeper,
		distributionKeeper: distributionKeeper,
		authzKeeper:        authzKeeper,
		feegrantKeeper:     feegrantKeeper,
		supernodeKeeper:    supernodeKeeper,
		actionKeeper:       actionKeeper,
		claimKeeper:        claimKeeper,

		// Allocate once so value-copies of Keeper (e.g. app.EvmigrationKeeper
		// and AppModule.keeper) share the same mutable handle. app.go writes
		// the staking store service into it post-build.
		stakingStoreHandle:      &stakingStoreHandle{},
		distributionStoreHandle: &distributionStoreHandle{},
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

// SetStakingStoreService wires the staking module's KVStoreService into this
// keeper. Required before DeleteValidatorRecordNoHooks is callable. Called
// from app.go after app.AppBuilder.Build completes.
//
// UNSAFE / MIGRATION-ONLY: this grants this keeper raw write access to
// staking's KV namespace, bypassing all staking keeper invariants and hooks.
// It exists exclusively to finalize validator operator migration. Do NOT use
// for any other purpose.
func (k *Keeper) SetStakingStoreService(svc corestore.KVStoreService) {
	k.stakingStoreHandle.svc = svc
}

// SetDistributionStoreService wires the distribution module's KVStoreService
// into this keeper. Required for production validator migration to iterate only
// validator-scoped distribution prefixes instead of chain-wide stores.
//
// UNSAFE / MIGRATION-ONLY: this grants raw read access to x/distribution's KV
// namespace. Do NOT use for unrelated keeper logic.
func (k *Keeper) SetDistributionStoreService(svc corestore.KVStoreService) {
	k.distributionStoreHandle.svc = svc
}
