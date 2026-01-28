package keeper

import (
	"fmt"

	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

type Keeper struct {
	cdc          codec.BinaryCodec
	addressCodec address.Codec
	storeService corestore.KVStoreService
	logger       log.Logger

	// the address capable of executing a MsgUpdateParams message. Typically, this
	// should be the x/gov module account.
	authority []byte

	supernodeKeeper sntypes.SupernodeKeeper
}

func NewKeeper(
	cdc codec.BinaryCodec,
	addressCodec address.Codec,
	storeService corestore.KVStoreService,
	logger log.Logger,
	authority []byte,
	supernodeKeeper sntypes.SupernodeKeeper,
) Keeper {
	if _, err := addressCodec.BytesToString(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address: %s", err))
	}

	return Keeper{
		cdc:             cdc,
		addressCodec:    addressCodec,
		storeService:    storeService,
		logger:          logger,
		authority:       authority,
		supernodeKeeper: supernodeKeeper,
	}
}

func (k Keeper) GetAuthority() []byte {
	return k.authority
}

func (k Keeper) Logger() log.Logger {
	return k.logger.With("module", "x/audit")
}
