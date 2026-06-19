//go:build integration
// +build integration

package mempool_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
	"time"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	evmcryptotypes "github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/stretchr/testify/require"

	lumeraapp "github.com/LumeraProtocol/lumera/app"
	lcfg "github.com/LumeraProtocol/lumera/config"
	evmtest "github.com/LumeraProtocol/lumera/tests/integration/evmtest"
	evmigrationtypes "github.com/LumeraProtocol/lumera/x/evmigration/types"
)

func TestEVMigrationZeroSignerTxBroadcastSyncWithMempoolEnabled(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-evmigration-mempool", 20)
	node.StartAndWaitRPC()
	defer node.Stop()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	txBytes := validZeroSignerMigrationTxBytes(t, node.ChainID())
	res := broadcastSync(t, node, txBytes)

	require.Zero(t, res.Code, "zero-signer migration tx must pass CheckTx with app-side mempool enabled: %s", res.Log)
	require.NotContains(t, res.Log, "tx must have at least one signer")
}

// TestEVMigrationMalformedLegacyAddressRejectedByValidateBasic confirms that a
// migration tx carrying a non-bech32 legacy_address is rejected end-to-end on a
// real node.
//
// NOTE ON LAYERING: this rejection comes from MsgClaimLegacyAccount.ValidateBasic
// ("invalid legacy_address", x/evmigration/types/types.go), which runs in the
// ante chain *before* mempool admission. The malformed address therefore never
// reaches the signer-extraction adapter's own bech32 guard — ValidateBasic
// shadows it. The adapter's "not a valid bech32" branch is exercised directly,
// without the ante in front of it, by the in-process test
// TestEVMMempool_InsertRejectsMalformedMigrationLegacyAddress in
// app/evm_mempool_evmigration_test.go. This test is the complementary
// end-to-end check that a malformed migration tx is rejected on the live path.
func TestEVMigrationMalformedLegacyAddressRejectedByValidateBasic(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-evmigration-bad-legacy", 20)
	node.StartAndWaitRPC()
	defer node.Stop()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	msg := &evmigrationtypes.MsgClaimLegacyAccount{
		NewAddress:    "lumera1ttwdmmlqf8xu5mkufrh5zcck8v8yn42a5m0xpg",
		LegacyAddress: "not-a-bech32",
		LegacyProof: evmigrationtypes.MigrationProof{Proof: &evmigrationtypes.MigrationProof_Single{Single: &evmigrationtypes.SingleKeyProof{
			PubKey:    secp256k1.GenPrivKey().PubKey().Bytes(),
			Signature: []byte("bad"),
			SigFormat: evmigrationtypes.SigFormat_SIG_FORMAT_CLI,
		}}},
		NewProof: evmigrationtypes.MigrationProof{Proof: &evmigrationtypes.MigrationProof_Single{Single: &evmigrationtypes.SingleKeyProof{
			PubKey:    make([]byte, 33),
			Signature: []byte("bad"),
			SigFormat: evmigrationtypes.SigFormat_SIG_FORMAT_CLI,
		}}},
	}
	txBytes := unsignedTxBytes(t, msg)
	res := broadcastSync(t, node, txBytes)

	require.NotZero(t, res.Code)
	require.Contains(t, res.Log, "invalid legacy_address",
		"malformed legacy_address must be rejected by ValidateBasic in the ante chain, before mempool admission")
	// And it must NOT be the mempool's zero-signer rejection: ValidateBasic
	// fires first, so the signer-extraction layer is never reached here.
	require.NotContains(t, res.Log, "at least one signer")
}

func TestZeroSignerNonMigrationBroadcastSyncStillRejected(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-evmigration-nonmigration", 20)
	node.StartAndWaitRPC()
	defer node.Stop()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	from := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address().Bytes())
	to := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address().Bytes())
	msg := banktypes.NewMsgSend(from, to, sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, 1)))
	txBytes := unsignedTxBytes(t, msg)
	res := broadcastSync(t, node, txBytes)

	require.NotZero(t, res.Code)
	require.True(t,
		strings.Contains(res.Log, "no signatures") || strings.Contains(res.Log, "at least one signer"),
		"zero-signer non-migration tx must be rejected for missing signer data, got log: %s", res.Log,
	)
}

func validZeroSignerMigrationTxBytes(t *testing.T, chainID string) []byte {
	t.Helper()

	legacyPriv := secp256k1.GenPrivKey()
	newPriv, err := evmcryptotypes.GenerateKey()
	require.NoError(t, err)

	legacy := sdk.AccAddress(legacyPriv.PubKey().Address().Bytes())
	newAddr := sdk.AccAddress(newPriv.PubKey().Address().Bytes())
	require.False(t, legacy.Equals(newAddr))

	payload := []byte(fmt.Sprintf(
		"lumera-evm-migration:%s:%d:claim:%s:%s",
		chainID,
		lcfg.EVMChainID,
		legacy.String(),
		newAddr.String(),
	))
	legacyHash := sha256.Sum256(payload)
	legacySig, err := legacyPriv.Sign(legacyHash[:])
	require.NoError(t, err)

	newSig, err := newPriv.Sign(payload)
	require.NoError(t, err)

	msg := &evmigrationtypes.MsgClaimLegacyAccount{
		LegacyAddress: legacy.String(),
		NewAddress:    newAddr.String(),
		LegacyProof: evmigrationtypes.MigrationProof{Proof: &evmigrationtypes.MigrationProof_Single{Single: &evmigrationtypes.SingleKeyProof{
			PubKey:    legacyPriv.PubKey().Bytes(),
			Signature: legacySig,
			SigFormat: evmigrationtypes.SigFormat_SIG_FORMAT_CLI,
		}}},
		NewProof: evmigrationtypes.MigrationProof{Proof: &evmigrationtypes.MigrationProof_Single{Single: &evmigrationtypes.SingleKeyProof{
			PubKey:    newPriv.PubKey().Bytes(),
			Signature: newSig,
			SigFormat: evmigrationtypes.SigFormat_SIG_FORMAT_CLI,
		}}},
	}
	return unsignedTxBytes(t, msg)
}

func unsignedTxBytes(t *testing.T, msgs ...sdk.Msg) []byte {
	t.Helper()

	encCfg := lumeraapp.MakeEncodingConfig(t)
	txBuilder := encCfg.TxConfig.NewTxBuilder()
	require.NoError(t, txBuilder.SetMsgs(msgs...))
	txBuilder.SetGasLimit(200_000)

	txBytes, err := encCfg.TxConfig.TxEncoder()(txBuilder.GetTx())
	require.NoError(t, err)
	return txBytes
}

func broadcastSync(t *testing.T, node *evmtest.Node, txBytes []byte) *coretypes.ResultBroadcastTx {
	t.Helper()

	client, err := rpchttp.New(node.CometRPCURL(), "/websocket")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	res, err := client.BroadcastTxSync(ctx, cmttypes.Tx(txBytes))
	require.NoError(t, err)
	require.NotNil(t, res)
	return res
}
