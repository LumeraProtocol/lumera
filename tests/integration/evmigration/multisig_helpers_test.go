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
	"github.com/stretchr/testify/require"

	lcfg "github.com/LumeraProtocol/lumera/config"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
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
) *types.LegacyProof {
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
	return &types.LegacyProof{Proof: &types.LegacyProof_Multisig{Multisig: &types.MultisigProof{
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
