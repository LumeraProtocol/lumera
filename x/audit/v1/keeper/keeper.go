package keeper

import (
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

type Keeper struct {
	storeService corestore.KVStoreService
	cdc          codec.Codec
	addressCodec address.Codec
	// Address capable of executing a MsgUpdateParams message.
	// Typically, this should be the x/gov module account.
	authority []byte

	Schema     collections.Schema
	Params     collections.Item[types.Params]
	EvidenceID collections.Sequence
	Evidences  collections.Map[uint64, types.Evidence]
	BySubject  collections.KeySet[collections.Pair[string, uint64]]
	ByActionID collections.KeySet[collections.Pair[string, uint64]]
}

func NewKeeper(
	storeService corestore.KVStoreService,
	cdc codec.Codec,
	addressCodec address.Codec,
	authority []byte,

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

		Params:     collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
		EvidenceID: collections.NewSequence(sb, types.EvidenceIDKey, "evidence_id"),
		Evidences:  collections.NewMap(sb, types.EvidenceKey, "evidences", collections.Uint64Key, codec.CollValue[types.Evidence](cdc)),
		BySubject: collections.NewKeySet(
			sb,
			types.EvidenceBySubjectKey,
			"evidence_by_subject",
			collections.PairKeyCodec(collections.StringKey, collections.Uint64Key),
			collections.WithKeySetSecondaryIndex(),
		),
		ByActionID: collections.NewKeySet(
			sb,
			types.EvidenceByActionKey,
			"evidence_by_action",
			collections.PairKeyCodec(collections.StringKey, collections.Uint64Key),
			collections.WithKeySetSecondaryIndex(),
		),
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
