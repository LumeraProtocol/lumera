//go:build integration
// +build integration

package mempool_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
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
	legacyPriv := secp256k1.GenPrivKey()
	addGenesisLegacyAccount(t, node, sdk.AccAddress(legacyPriv.PubKey().Address().Bytes()))
	node.StartAndWaitRPC()
	defer node.Stop()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	txBytes := validZeroSignerMigrationTxBytes(t, node.ChainID(), legacyPriv)
	res := broadcastSync(t, node, txBytes)

	require.Zero(t, res.Code, "zero-signer migration tx must pass CheckTx with app-side mempool enabled: %s", res.Log)
	require.NotContains(t, res.Log, "tx must have at least one signer")
}

func TestEVMigrationZeroSignerTxBroadcastSyncAfterLegacyMainnetConfigMigration(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-mainnet-1", 20)
	evmtest.WriteLegacyPreEVMAppToml(t, node.HomeDir(), -1)
	legacyPriv := secp256k1.GenPrivKey()
	addGenesisLegacyAccount(t, node, sdk.AccAddress(legacyPriv.PubKey().Address().Bytes()))
	node.StartAndWaitRPC()
	defer node.Stop()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	appTomlBytes, err := os.ReadFile(filepath.Join(node.HomeDir(), "config", "app.toml"))
	require.NoError(t, err)
	appToml := string(appTomlBytes)
	require.Contains(t, appToml, "max-txs = 10000")
	require.Contains(t, appToml, "[evm.mempool]")
	require.Contains(t, appToml, "global-slots = 5120")
	require.Contains(t, appToml, "global-queue = 1024")
	require.NotContains(t, appToml, "insert-queue-size")

	txBytes := validZeroSignerMigrationTxBytes(t, node.ChainID(), legacyPriv)
	res := broadcastSync(t, node, txBytes)

	require.Zero(t, res.Code, "zero-signer migration tx must pass CheckTx after legacy config migration: %s", res.Log)
	require.NotContains(t, res.Log, "tx must have at least one signer")
}

func TestEVMigrationProofValidNonexistentLegacyAccountRejectedByAnte(t *testing.T) {
	node := evmtest.NewEVMNode(t, "lumera-evmigration-no-legacy", 20)
	node.StartAndWaitRPC()
	defer node.Stop()
	node.WaitForBlockNumberAtLeast(t, 1, 20*time.Second)

	txBytes := validZeroSignerMigrationTxBytes(t, node.ChainID(), secp256k1.GenPrivKey())
	res := broadcastSync(t, node, txBytes)

	require.NotZero(t, res.Code)
	require.Contains(t, res.Log, "legacy account not found",
		"proof-valid migration txs from nonexistent legacy accounts must fail before mempool admission")
	require.NotContains(t, res.Log, "at least one signer")
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

func validZeroSignerMigrationTxBytes(t *testing.T, chainID string, legacyPriv *secp256k1.PrivKey) []byte {
	t.Helper()

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

func addGenesisLegacyAccount(t *testing.T, node *evmtest.Node, legacyAddr sdk.AccAddress) {
	t.Helper()

	encCfg := lumeraapp.MakeEncodingConfig(t)
	genesisPath := filepath.Join(node.HomeDir(), "config", "genesis.json")
	genesisBytes, err := os.ReadFile(genesisPath)
	require.NoError(t, err)

	var genesisDoc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(genesisBytes, &genesisDoc))

	var appState map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(genesisDoc["app_state"], &appState))

	authGenesis := authtypes.GetGenesisStateFromAppState(encCfg.Codec, appState)
	accounts, err := authtypes.UnpackAccounts(authGenesis.Accounts)
	require.NoError(t, err)
	accounts = append(accounts, authtypes.NewBaseAccount(legacyAddr, nil, uint64(len(accounts)), 0))
	authGenesis.Accounts, err = authtypes.PackAccounts(accounts)
	require.NoError(t, err)
	appState[authtypes.ModuleName] = encCfg.Codec.MustMarshalJSON(&authGenesis)

	bankGenesis := banktypes.GetGenesisStateFromAppState(encCfg.Codec, appState)
	coins := sdk.NewCoins(sdk.NewInt64Coin(lcfg.ChainDenom, 1_000_000))
	bankGenesis.Balances = append(bankGenesis.Balances, banktypes.Balance{
		Address: legacyAddr.String(),
		Coins:   coins,
	})
	bankGenesis.Supply = bankGenesis.Supply.Add(coins...)
	appState[banktypes.ModuleName] = encCfg.Codec.MustMarshalJSON(bankGenesis)

	genesisDoc["app_state"], err = json.Marshal(appState)
	require.NoError(t, err)

	updated, err := json.MarshalIndent(genesisDoc, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(genesisPath, updated, 0o644))
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
