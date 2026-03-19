package app

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/x/auth/migrations/legacytx"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/stretchr/testify/require"
)

// TestRegisterLumeraLegacyAminoCodecEnablesEthSecp256k1StdSignature ensures
// SDK ante gas-size estimation paths that marshal legacy StdSignature work for
// EVM eth_secp256k1 account pubkeys.
func TestRegisterLumeraLegacyAminoCodecEnablesEthSecp256k1StdSignature(t *testing.T) {
	t.Parallel()

	oldLegacyCodec := legacy.Cdc
	t.Cleanup(func() {
		legacy.Cdc = oldLegacyCodec
	})

	ethPrivKey, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)

	// NOTE: legacytx.StdSignature is deprecated, but this is the exact type still
	// marshaled by SDK ConsumeTxSizeGasDecorator (x/auth/ante/basic.go) when
	// charging tx size gas. Keep this until the upstream ante path is migrated.
	sig := legacytx.StdSignature{ // SA1019: intentional regression guard for current SDK behavior.
		PubKey:    ethPrivKey.PubKey(),
		Signature: make([]byte, 65),
	}

	baseCodec := codec.NewLegacyAmino()
	baseCodec.RegisterInterface((*cryptotypes.PubKey)(nil), nil)
	baseCodec.RegisterInterface((*cryptotypes.PrivKey)(nil), nil)
	baseCodec.Seal()

	legacy.Cdc = baseCodec
	// we didn't register eth_secp256k1 types, so this should panic when trying to marshal the StdSignature with an eth_secp256k1 pubkey.
	require.Panics(t, func() {
		legacy.Cdc.MustMarshal(sig)
	})

	evmCodec := codec.NewLegacyAmino()
	evmCodec.RegisterInterface((*cryptotypes.PubKey)(nil), nil)
	evmCodec.RegisterInterface((*cryptotypes.PrivKey)(nil), nil)
	registerLumeraLegacyAminoCodec(evmCodec)
	evmCodec.Seal()

	require.Same(t, evmCodec, legacy.Cdc)
	legacy.Cdc = evmCodec
	// now that we've registered eth_secp256k1 types, this should no longer panic.
	require.NotPanics(t, func() {
		legacy.Cdc.MustMarshal(sig)
	})
}
