package keeper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/core/address"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	icatypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/types"

	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
)

type authKeeperStub struct {
	addrCodec address.Codec
	accounts  map[string]sdk.AccountI
}

func newAuthKeeperStub(ac address.Codec) *authKeeperStub {
	return &authKeeperStub{
		addrCodec: ac,
		accounts:  make(map[string]sdk.AccountI),
	}
}

func (k *authKeeperStub) AddressCodec() address.Codec { return k.addrCodec }

func (k *authKeeperStub) GetAccount(_ context.Context, addr sdk.AccAddress) sdk.AccountI {
	return k.accounts[addr.String()]
}

func (k *authKeeperStub) SetAccount(_ context.Context, acc sdk.AccountI) {
	k.accounts[acc.GetAddress().String()] = acc
}

func (k *authKeeperStub) GetModuleAccount(_ context.Context, _ string) sdk.ModuleAccountI { return nil }
func (k *authKeeperStub) SetModuleAccount(_ context.Context, _ sdk.ModuleAccountI)        {}

func TestValidateRequestActionAppPubKey_NonICARejectsNonEmpty(t *testing.T) {
	ac := addresscodec.NewBech32Codec("lumera")
	ak := newAuthKeeperStub(ac)

	creatorAddr := sdk.AccAddress([]byte("creator_address_12345"))
	creator, err := ac.BytesToString(creatorAddr)
	require.NoError(t, err)
	ak.SetAccount(context.Background(), authtypes.NewBaseAccountWithAddress(creatorAddr))

	k := Keeper{addressCodec: ac, authKeeper: ak}

	info, err := k.getCreatorAccountInfo(context.Background(), &actiontypes.MsgRequestAction{
		Creator:   creator,
		AppPubkey: []byte{1},
	})
	require.NoError(t, err)

	err = info.validateAppPubKey()
	require.ErrorIs(t, err, actiontypes.ErrInvalidAppPubKey)
}

func TestValidateRequestActionAppPubKey_ICARequiresNonEmpty(t *testing.T) {
	ac := addresscodec.NewBech32Codec("lumera")
	ak := newAuthKeeperStub(ac)

	creatorAddr := sdk.AccAddress([]byte("creator_address_12345"))
	creator, err := ac.BytesToString(creatorAddr)
	require.NoError(t, err)
	base := authtypes.NewBaseAccountWithAddress(creatorAddr)
	ak.SetAccount(context.Background(), icatypes.NewInterchainAccount(base, "owner"))

	k := Keeper{addressCodec: ac, authKeeper: ak}

	info, err := k.getCreatorAccountInfo(context.Background(), &actiontypes.MsgRequestAction{Creator: creator})
	require.NoError(t, err)

	err = info.validateAppPubKey()
	require.ErrorIs(t, err, actiontypes.ErrInvalidAppPubKey)
}

func TestValidateRequestActionAppPubKey_ICAWithPubKeyAllowed(t *testing.T) {
	ac := addresscodec.NewBech32Codec("lumera")
	ak := newAuthKeeperStub(ac)

	creatorAddr := sdk.AccAddress([]byte("creator_address_12345"))
	creator, err := ac.BytesToString(creatorAddr)
	require.NoError(t, err)
	base := authtypes.NewBaseAccountWithAddress(creatorAddr)
	ak.SetAccount(context.Background(), icatypes.NewInterchainAccount(base, "owner"))

	k := Keeper{addressCodec: ac, authKeeper: ak}

	info, err := k.getCreatorAccountInfo(context.Background(), &actiontypes.MsgRequestAction{
		Creator:   creator,
		AppPubkey: []byte{1, 2, 3},
	})
	require.NoError(t, err)

	err = info.validateAppPubKey()
	require.NoError(t, err)
}
