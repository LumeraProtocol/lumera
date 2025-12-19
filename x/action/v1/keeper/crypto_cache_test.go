package keeper

import (
	"encoding/base64"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/codec/address"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

func TestVerifySignatureUsesCachedAppPubKeyForICA(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	signerBz := sdk.AccAddress(priv.PubKey().Address())

	addrCodec := address.NewBech32Codec("lumera")
	signer, err := addrCodec.BytesToString(signerBz)
	require.NoError(t, err)

	data := "payload"
	sig, err := priv.Sign([]byte(data))
	require.NoError(t, err)
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	// Keeper with only address codec; auth keeper not needed because we supply pubkey in cache.
	k := Keeper{addressCodec: addrCodec}

	ctx := sdk.Context{}.WithContext(context.Background())
	ctx = ctx.WithValue(creatorAccountCtxKey, &creatorAccountInfo{
		isICA:     true,
		appPubkey: priv.PubKey().Bytes(),
	})

	require.NoError(t, k.VerifySignature(ctx, data, sigB64, signer))
}

func TestVerifySignatureUsesCachedAccountPubKey(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	signerBz := sdk.AccAddress(priv.PubKey().Address())

	addrCodec := address.NewBech32Codec("lumera")
	signer, err := addrCodec.BytesToString(signerBz)
	require.NoError(t, err)

	data := "payload"
	sig, err := priv.Sign([]byte(data))
	require.NoError(t, err)
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	k := Keeper{addressCodec: addrCodec}

	baseAcc := authtypes.NewBaseAccountWithAddress(signerBz)
	require.NoError(t, baseAcc.SetPubKey(priv.PubKey()))

	ctx := sdk.Context{}.WithContext(context.Background())
	ctx = ctx.WithValue(creatorAccountCtxKey, &creatorAccountInfo{
		account: baseAcc,
	})

	require.NoError(t, k.VerifySignature(ctx, data, sigB64, signer))
}
