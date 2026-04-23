package integration_test

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	evmcryptotypes "github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/stretchr/testify/require"

	lcfg "github.com/LumeraProtocol/lumera/config"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
	"github.com/LumeraProtocol/lumera/x/evmigration/types/sigverify"
)

// BuildMultisigLegacyAccount creates an in-memory K-of-N multisig of
// secp256k1 sub-keys and returns the multisig pubkey, sub-private-keys,
// and derived bech32 address.
func BuildMultisigLegacyAccount(t *testing.T, k, n int) (*multisig.LegacyAminoPubKey, []*secp256k1.PrivKey, sdk.AccAddress) {
	t.Helper()
	privs := make([]*secp256k1.PrivKey, n)
	pubs := make([]cryptotypes.PubKey, n)
	for i := 0; i < n; i++ {
		privs[i] = secp256k1.GenPrivKey()
		pubs[i] = privs[i].PubKey()
	}
	multiPK := multisig.NewLegacyAminoPubKey(k, pubs)
	return multiPK, privs, sdk.AccAddress(multiPK.Address())
}

// SignMultisigProof builds a MultisigProof signed by the K sub-keys at
// signerIdxs. format selects CLI (SHA256) or ADR-036 (canonical JSON) envelope.
// chainID must match the integration suite's chain_id.
func SignMultisigProof(
	t *testing.T,
	chainID string,
	kind string,
	multiPK *multisig.LegacyAminoPubKey,
	privs []*secp256k1.PrivKey,
	signerIdxs []int,
	legacyAddr, newAddr sdk.AccAddress,
	format types.SigFormat,
) *types.MigrationProof {
	t.Helper()
	payload := fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s",
		chainID, lcfg.EVMChainID, kind, legacyAddr.String(), newAddr.String())

	sort.Ints(signerIdxs)
	indices := make([]uint32, len(signerIdxs))
	sigs := make([][]byte, len(signerIdxs))
	for i, idx := range signerIdxs {
		indices[i] = uint32(idx)
		if format == types.SigFormat_SIG_FORMAT_ADR036 {
			signerAddr := sdk.AccAddress(privs[idx].PubKey().Address()).String()
			doc := []byte(fmt.Sprintf(`{"account_number":"0","chain_id":"","fee":{"amount":[],"gas":"0"},"memo":"","msgs":[{"type":"sign/MsgSignData","value":{"data":"%s","signer":"%s"}}],"sequence":"0"}`,
				base64.StdEncoding.EncodeToString([]byte(payload)), signerAddr))
			sig, err := privs[idx].Sign(doc)
			require.NoError(t, err)
			sigs[i] = sig
			continue
		}
		hash := sha256.Sum256([]byte(payload))
		sig, err := privs[idx].Sign(hash[:])
		require.NoError(t, err)
		sigs[i] = sig
	}

	subPubKeys := make([][]byte, len(multiPK.GetPubKeys()))
	for i, p := range multiPK.GetPubKeys() {
		subPubKeys[i] = p.Bytes()
	}
	return &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
		Threshold:     uint32(multiPK.Threshold),
		SubPubKeys:    subPubKeys,
		SignerIndices: indices,
		SubSignatures: sigs,
		SigFormat:     format,
	}}}
}

// BuildMultisigNewAccount creates an in-memory K-of-N multisig of
// eth_secp256k1 sub-keys and returns the multisig pubkey, sub-private-keys,
// and derived bech32 address. Mirrors BuildMultisigLegacyAccount for the
// new-side (coin-type-60) of a multisig→multisig migration.
func BuildMultisigNewAccount(t *testing.T, k, n int) (*multisig.LegacyAminoPubKey, []*evmcryptotypes.PrivKey, sdk.AccAddress) {
	t.Helper()
	privs := make([]*evmcryptotypes.PrivKey, n)
	pubs := make([]cryptotypes.PubKey, n)
	for i := 0; i < n; i++ {
		p, err := evmcryptotypes.GenerateKey()
		require.NoError(t, err)
		privs[i] = p
		pubs[i] = p.PubKey()
	}
	multiPK := multisig.NewLegacyAminoPubKey(k, pubs)
	return multiPK, privs, sdk.AccAddress(multiPK.Address())
}

// SignNewMultisigProof builds a MultisigProof signed by the K eth_secp256k1
// sub-keys at signerIdxs. format selects the per-sub-sig envelope:
//   - SIG_FORMAT_CLI    : sign raw payload. The eth keyring applies Keccak256
//     internally; this matches VerifyEthSecp256k1 which also Keccak256's.
//   - SIG_FORMAT_EIP191 : sign EIP191PersonalSignPayload(payload). The keyring
//     still Keccak256's, which matches the verifier.
//   - SIG_FORMAT_ADR036 : sign ADR036SignDoc(signerAddr, payload) where
//     signerAddr is the eth sub-key's own bech32 lumera address.
//
// chainID must match the integration suite's chain_id.
func SignNewMultisigProof(
	t *testing.T,
	chainID string,
	kind string,
	multiPK *multisig.LegacyAminoPubKey,
	privs []*evmcryptotypes.PrivKey,
	signerIdxs []int,
	legacyAddr, newAddr sdk.AccAddress,
	format types.SigFormat,
) *types.MigrationProof {
	t.Helper()
	payload := fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s",
		chainID, lcfg.EVMChainID, kind, legacyAddr.String(), newAddr.String())

	sort.Ints(signerIdxs)
	indices := make([]uint32, len(signerIdxs))
	sigs := make([][]byte, len(signerIdxs))
	for i, idx := range signerIdxs {
		indices[i] = uint32(idx)
		var toSign []byte
		switch format {
		case types.SigFormat_SIG_FORMAT_EIP191:
			toSign = sigverify.EIP191PersonalSignPayload([]byte(payload))
		case types.SigFormat_SIG_FORMAT_ADR036:
			signerAddr := sdk.AccAddress(privs[idx].PubKey().Address()).String()
			toSign = sigverify.ADR036SignDoc(signerAddr, []byte(payload))
		default:
			// SIG_FORMAT_CLI: sign the raw payload; eth keyring Keccak256's internally.
			toSign = []byte(payload)
		}
		sig, err := privs[idx].Sign(toSign)
		require.NoError(t, err)
		sigs[i] = sig
	}

	subPubKeys := make([][]byte, len(multiPK.GetPubKeys()))
	for i, p := range multiPK.GetPubKeys() {
		subPubKeys[i] = p.Bytes()
	}
	return &types.MigrationProof{Proof: &types.MigrationProof_Multisig{Multisig: &types.MultisigProof{
		Threshold:     uint32(multiPK.Threshold),
		SubPubKeys:    subPubKeys,
		SignerIndices: indices,
		SubSignatures: sigs,
		SigFormat:     format,
	}}}
}

// createFundedMultisigAccount creates a K-of-N secp256k1 multisig account,
// registers it in auth with its pubkey, and funds it via the bank module.
func (s *MigrationIntegrationSuite) createFundedMultisigAccount(k, n int, coins sdk.Coins) (*multisig.LegacyAminoPubKey, []*secp256k1.PrivKey, sdk.AccAddress) {
	multiPK, privs, addr := BuildMultisigLegacyAccount(s.T(), k, n)

	acc := s.app.AuthKeeper.NewAccountWithAddress(s.ctx, addr)
	baseAcc, ok := acc.(*authtypes.BaseAccount)
	s.Require().True(ok)
	s.Require().NoError(baseAcc.SetPubKey(multiPK))
	s.app.AuthKeeper.SetAccount(s.ctx, baseAcc)

	if !coins.IsZero() {
		s.Require().NoError(s.app.BankKeeper.MintCoins(s.ctx, "mint", coins))
		s.Require().NoError(s.app.BankKeeper.SendCoinsFromModuleToAccount(s.ctx, "mint", addr, coins))
	}

	return multiPK, privs, addr
}
