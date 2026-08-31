package cli

import (
	"bufio"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/input"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	clienttx "github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	evmcryptotypes "github.com/cosmos/evm/crypto/ethsecp256k1"

	lcfg "github.com/LumeraProtocol/lumera/config"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

const (
	migrationProofKindClaim     = "claim"
	migrationProofKindValidator = "validator"

	flagTxTimeout    = "tx-timeout"
	defaultTxTimeout = "30s"
)

// GetTxCmd returns the custom tx commands for evmigration.
// These commands derive the destination-account proof locally, then build and
// broadcast an unsigned Cosmos tx whose authentication is fully embedded in the
// message payload.
func GetTxCmd() *cobra.Command {
	evmigrationTxCmd := &cobra.Command{
		Use:   types.ModuleName,
		Short: "EVM migration transaction commands",
		RunE:  client.ValidateCmd,
	}

	evmigrationTxCmd.AddCommand(
		cmdClaimLegacyAccount(),
		cmdMigrateValidator(),
		cmdGenerateProofPayload(),
		cmdSignProof(),
		cmdCombineProof(),
		cmdSubmitProof(),
	)

	return evmigrationTxCmd
}

func cmdClaimLegacyAccount() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claim-legacy-account <legacy-key> <new-key>",
		Short: "Migrate on-chain state from legacy to new address",
		Long: `Migrate on-chain state from a legacy (coin-type 118) address to a new (coin-type 60) address.

Both keys must be in the keyring. The CLI derives addresses, generates
both proofs, simulates gas, and broadcasts. No fee is charged.

  lumerad tx evmigration claim-legacy-account legacy-key new-key

<legacy-key> must be a secp256k1 key (coin-type 118).
<new-key> must be an eth_secp256k1 key (coin-type 60).
The keys may come from the same mnemonic or different mnemonics; no mnemonic relationship is required.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := preflightChainIDCheck(cmd); err != nil {
				return err
			}
			msg, _, err := resolveClaimMsg(cmd, args[0], args[1])
			if err != nil {
				return err
			}
			return runMigrationTx(cmd, msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	cmd.Flags().String(flagTxTimeout, defaultTxTimeout, "How long to wait for transaction inclusion; 0s returns after broadcast (e.g. 30s, 1m)")
	return cmd
}

func cmdMigrateValidator() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate-validator <legacy-validator-key> <new-validator-evm-key>",
		Short: "Migrate a validator operator from legacy to new address",
		Long: `Migrate a validator operator from a legacy (coin-type 118) address to a new (coin-type 60) address.

Both keys must be in the keyring. The CLI derives addresses, generates
both proofs, simulates gas, and broadcasts. No fee is charged.

  lumerad tx evmigration migrate-validator legacy-validator-key new-validator-evm-key

<legacy-validator-key> must be a secp256k1 key (coin-type 118).
<new-validator-evm-key> must be an eth_secp256k1 key (coin-type 60).
The keys may come from the same mnemonic or different mnemonics; no mnemonic relationship is required.

WARNING: Stop your validator node before running this command.
Restart it immediately after the transaction is confirmed.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := preflightChainIDCheck(cmd); err != nil {
				return err
			}
			msg, _, err := resolveValidatorMsg(cmd, args[0], args[1])
			if err != nil {
				return err
			}
			return runMigrationTx(cmd, msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	cmd.Flags().String(flagTxTimeout, defaultTxTimeout, "How long to wait for transaction inclusion; 0s returns after broadcast (e.g. 30s, 1m)")
	return cmd
}

// resolveClaimMsg builds a MsgClaimLegacyAccount from the two key names (positional args).
// Both LegacyProof and NewProof are fully assembled here so that runMigrationTx only
// needs to validate, simulate gas, and broadcast the pre-assembled message.
func resolveClaimMsg(cmd *cobra.Command, legacyKeyName, newKeyName string) (*types.MsgClaimLegacyAccount, string, error) {
	clientCtx, err := client.GetClientTxContext(cmd)
	if err != nil {
		return nil, "", err
	}
	newAddr, legacyAddr, pubKey, sig, err := signLegacyProofFromKeyring(clientCtx, legacyKeyName, newKeyName, migrationProofKindClaim)
	if err != nil {
		return nil, "", err
	}
	newProof, err := buildNewSingleProof(clientCtx, newKeyName, migrationProofKindClaim, legacyAddr, newAddr)
	if err != nil {
		return nil, "", err
	}
	return &types.MsgClaimLegacyAccount{
		NewAddress:    newAddr,
		LegacyAddress: legacyAddr,
		LegacyProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    pubKey,
			Signature: sig,
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
		NewProof: newProof,
	}, newKeyName, nil
}

// resolveValidatorMsg builds a MsgMigrateValidator from the two key names (positional args).
// Both LegacyProof and NewProof are fully assembled here so that runMigrationTx only
// needs to validate, simulate gas, and broadcast the pre-assembled message.
func resolveValidatorMsg(cmd *cobra.Command, legacyKeyName, newKeyName string) (*types.MsgMigrateValidator, string, error) {
	clientCtx, err := client.GetClientTxContext(cmd)
	if err != nil {
		return nil, "", err
	}
	newAddr, legacyAddr, pubKey, sig, err := signLegacyProofFromKeyring(clientCtx, legacyKeyName, newKeyName, migrationProofKindValidator)
	if err != nil {
		return nil, "", err
	}
	newProof, err := buildNewSingleProof(clientCtx, newKeyName, migrationProofKindValidator, legacyAddr, newAddr)
	if err != nil {
		return nil, "", err
	}
	return &types.MsgMigrateValidator{
		NewAddress:    newAddr,
		LegacyAddress: legacyAddr,
		LegacyProof: types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
			PubKey:    pubKey,
			Signature: sig,
			SigFormat: types.SigFormat_SIG_FORMAT_CLI,
		}}},
		NewProof: newProof,
	}, newKeyName, nil
}

// signLegacyProofFromKeyring extracts both keys from the keyring, validates
// their types, derives bech32 addresses, builds the migration payload, signs
// SHA256(payload) with the legacy key, and returns all values needed for the message.
func signLegacyProofFromKeyring(clientCtx client.Context, legacyKeyName, newKeyName, proofKind string) (newAddr, legacyAddr string, pubKeyBytes, signature []byte, err error) {
	// Resolve new address from the new key name.
	newRecord, err := clientCtx.Keyring.Key(newKeyName)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("new key %q not found in keyring: %w", newKeyName, err)
	}
	newPubKey, err := newRecord.GetPubKey()
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("cannot get public key for %q: %w", newKeyName, err)
	}
	newAddr = sdk.AccAddress(newPubKey.Address()).String()

	// Look up the legacy key.
	legacyRecord, err := clientCtx.Keyring.Key(legacyKeyName)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("legacy key %q not found in keyring: %w", legacyKeyName, err)
	}

	legacyPubKey, err := legacyRecord.GetPubKey()
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("cannot get public key for %q: %w", legacyKeyName, err)
	}

	// Ensure it is a secp256k1 key (legacy cosmos key type).
	if _, ok := legacyPubKey.(*secp256k1.PubKey); !ok {
		return "", "", nil, nil, fmt.Errorf("key %q must be secp256k1 (legacy), got %T; use --coin-type 118 --algo secp256k1 when importing", legacyKeyName, legacyPubKey)
	}

	legacyAddr = sdk.AccAddress(legacyPubKey.Address()).String()
	pubKeyBytes = legacyPubKey.Bytes()

	if newAddr == legacyAddr {
		return "", "", nil, nil, fmt.Errorf("new key address %s and legacy key address %s are identical; they must be different keys (coin-type 60 vs 118)", newAddr, legacyAddr)
	}

	// Build and sign the payload.
	payload := fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s",
		clientCtx.ChainID, lcfg.EVMChainID, proofKind, legacyAddr, newAddr)
	hash := sha256.Sum256([]byte(payload))

	signature, _, err = clientCtx.Keyring.Sign(legacyKeyName, hash[:], signingtypes.SignMode_SIGN_MODE_UNSPECIFIED)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("failed to sign legacy proof with key %q: %w", legacyKeyName, err)
	}

	return newAddr, legacyAddr, pubKeyBytes, signature, nil
}

type migrationProofMsg interface {
	sdk.Msg
	MigrationNewAddress() string
	MigrationLegacyAddress() string
}

// runMigrationTx validates, simulates gas, and broadcasts the pre-assembled migration
// message. Both proofs must already be populated by the caller (resolveClaimMsg /
// resolveValidatorMsg build the full MigrationProof{Single} up-front for both halves).
func runMigrationTx(cmd *cobra.Command, msg migrationProofMsg) error {
	clientCtx, err := client.GetClientTxContext(cmd)
	if err != nil {
		return err
	}

	if validateBasic, ok := msg.(sdk.HasValidateBasic); ok {
		if err := validateBasic.ValidateBasic(); err != nil {
			return err
		}
	}

	txf, err := clienttx.NewFactoryCLI(clientCtx, cmd.Flags())
	if err != nil {
		return err
	}

	// Migration transactions are fee-free (EVMigrationFeeDecorator zeroes
	// min-gas-prices), but still require a gas limit. Default to auto-simulation
	// with 1.5x adjustment so the user doesn't need --gas or --gas-prices flags.
	if txf.GasAdjustment() <= 1.0 {
		txf = txf.WithGasAdjustment(1.5)
	}

	// The tx itself remains unsigned. Generate-only and offline modes still
	// operate on the standard unsigned tx builder.
	if clientCtx.GenerateOnly {
		return txf.PrintUnsignedTx(clientCtx, msg)
	}

	if !clientCtx.Offline {
		// Migration txs are intentionally unsigned at the Cosmos tx layer, so the
		// SDK's generic gas estimator cannot be used here: it injects a simulated
		// signer based on --from, which makes the tx invalid ("expected 0, got 1").
		_, adjustedGas, err := simulateMigrationGas(clientCtx, txf, msg)
		if err != nil {
			return err
		}
		txf = txf.WithGas(adjustedGas)
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", clienttx.GasEstimateResponse{GasEstimate: txf.Gas()})
	}

	if clientCtx.Simulate {
		return nil
	}

	txBuilder, err := txf.BuildUnsignedTx(msg)
	if err != nil {
		return err
	}

	if !clientCtx.SkipConfirm {
		ok, err := confirmMigrationTx(clientCtx, txBuilder)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}

	txBytes, err := clientCtx.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return err
	}

	res, err := clientCtx.BroadcastTx(txBytes)
	if err != nil {
		return err
	}

	// BroadcastTx in sync mode only confirms mempool acceptance (code 0).
	// Use the SDK's wait-tx command to get the actual execution result so we
	// surface on-chain errors (e.g. out-of-gas) instead of silently reporting success.
	if res.Code == 0 && res.Height == 0 && !clientCtx.Offline {
		txTimeout, _ := cmd.Flags().GetString(flagTxTimeout)
		if txTimeout == "" {
			txTimeout = defaultTxTimeout
		}
		waitDuration, err := time.ParseDuration(txTimeout)
		if err != nil {
			return fmt.Errorf("invalid --%s value %q: %w", flagTxTimeout, txTimeout, err)
		}
		if waitDuration < 0 {
			return fmt.Errorf("--%s must not be negative", flagTxTimeout)
		}
		// Wrappers that support HTTP-only RPC endpoints confirm inclusion by
		// polling `query tx`, because those endpoints reject WebSocket upgrades.
		// A zero timeout makes this command return the accepted broadcast result
		// immediately so the wrapper can perform that confirmation without a
		// misleading WebSocket handshake error.
		if waitDuration == 0 {
			return clientCtx.PrintProto(res)
		}
		waitCmd := rpc.WaitTxCmd()
		waitCmd.SetArgs([]string{res.TxHash, "--timeout", txTimeout})
		waitCmd.SetContext(cmd.Context())
		// WaitTxCmd already registers query flags internally, so we only
		// inject the client context (which carries the node URL) — do NOT
		// call AddQueryFlagsToCmd again or --node will panic on redefinition.
		if err := client.SetCmdClientContextHandler(clientCtx, waitCmd); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "transaction broadcast succeeded (hash: %s) but could not confirm execution: %v\n", res.TxHash, err)
			return clientCtx.PrintProto(res)
		}
		if err := waitCmd.Execute(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "transaction broadcast succeeded (hash: %s) but could not confirm execution: %v\n", res.TxHash, err)
			return clientCtx.PrintProto(res)
		}
		return nil
	}

	return clientCtx.PrintProto(res)
}

func simulateMigrationGas(clientCtx client.Context, txf clienttx.Factory, msg migrationProofMsg) (*txtypes.SimulateResponse, uint64, error) {
	txBuilder, err := txf.BuildUnsignedTx(msg)
	if err != nil {
		return nil, 0, err
	}

	txBytes, err := clientCtx.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, 0, err
	}

	txSvcClient := txtypes.NewServiceClient(clientCtx)
	simRes, err := txSvcClient.Simulate(context.Background(), &txtypes.SimulateRequest{
		TxBytes: txBytes,
	})
	if err != nil {
		return nil, 0, err
	}

	adjustedGas := uint64(txf.GasAdjustment() * float64(simRes.GasInfo.GasUsed))
	if adjustedGas < simRes.GasInfo.GasUsed {
		adjustedGas = simRes.GasInfo.GasUsed
	}

	return simRes, adjustedGas, nil
}

// buildNewSingleProof produces a fully-assembled MigrationProof{Single} for the
// new (eth) side of a migration: looks up newKeyName in the keyring, asserts
// it's an eth_secp256k1 key, signs the canonical migration payload, and wraps
// the resulting pubkey+signature into a SingleKeyProof with SIG_FORMAT_CLI.
//
// Replaces the Task 4 adapter MigrationSetNewProof + the deleted helper
// signNewMigrationProof. The full SingleKeyProof lets the side-aware
// verifier (VerifyMigrationProof with SubKeyTypeEthSecp256k1) accept the message.
func buildNewSingleProof(clientCtx client.Context, newKeyName, proofKind, legacyAddress, newAddress string) (types.MigrationProof, error) {
	payload := []byte(fmt.Sprintf("lumera-evm-migration:%s:%d:%s:%s:%s",
		clientCtx.ChainID, lcfg.EVMChainID, proofKind, legacyAddress, newAddress))

	sig, pubKey, err := clientCtx.Keyring.Sign(newKeyName, payload, signingtypes.SignMode_SIGN_MODE_LEGACY_AMINO_JSON)
	if err != nil {
		return types.MigrationProof{}, err
	}

	ethPK, ok := pubKey.(*evmcryptotypes.PubKey)
	if !ok {
		return types.MigrationProof{}, fmt.Errorf("key %q must use eth_secp256k1, got %T", newKeyName, pubKey)
	}

	return types.MigrationProof{Proof: &types.MigrationProof_Single{Single: &types.SingleKeyProof{
		PubKey:    ethPK.Key,
		Signature: sig,
		SigFormat: types.SigFormat_SIG_FORMAT_CLI,
	}}}, nil
}

func confirmMigrationTx(clientCtx client.Context, txBuilder client.TxBuilder) (bool, error) {
	encoder := clientCtx.TxConfig.TxJSONEncoder()
	if encoder == nil {
		return false, errors.New("failed to encode transaction: tx json encoder is nil")
	}

	txBytes, err := encoder(txBuilder.GetTx())
	if err != nil {
		return false, fmt.Errorf("failed to encode transaction: %w", err)
	}

	if err := clientCtx.PrintRaw(txBytes); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n%s\n", err, txBytes)
	}

	buf := bufio.NewReader(os.Stdin)
	return input.GetConfirmation("confirm transaction before broadcasting", buf, os.Stderr)
}

// preflightChainIDCheck queries the node's status and checks that --chain-id
// matches the node's chain ID. Migration proofs embed the chain ID in the
// signed payload, so a mismatch causes cryptic signature verification failures.
// Must be called before proof signing.
func preflightChainIDCheck(cmd *cobra.Command) error {
	clientCtx, err := client.GetClientTxContext(cmd)
	if err != nil || clientCtx.Offline {
		return nil // offline mode — skip check
	}
	node, err := clientCtx.GetNode()
	if err != nil {
		return nil // no node configured — skip check
	}
	status, err := node.Status(context.Background())
	if err != nil {
		return nil // node unreachable — let later RPC calls surface the error
	}
	nodeChainID := status.NodeInfo.Network
	if clientCtx.ChainID != "" && clientCtx.ChainID != nodeChainID {
		return fmt.Errorf("chain-id mismatch: --chain-id is %q but the node reports %q; "+
			"migration proofs are bound to the chain ID and will fail verification on-chain",
			clientCtx.ChainID, nodeChainID)
	}
	return nil
}
