package cli

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/input"
	clienttx "github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	evmcryptotypes "github.com/cosmos/evm/crypto/ethsecp256k1"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

const (
	migrationProofKindClaim     = "claim"
	migrationProofKindValidator = "validator"
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
	)

	return evmigrationTxCmd
}

func cmdClaimLegacyAccount() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claim-legacy-account [new-address] [legacy-address] [legacy-pub-key] [legacy-signature]",
		Short: "Migrate on-chain state from legacy to new address",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			msg, err := buildClaimLegacyAccountMsg(args)
			if err != nil {
				return err
			}
			return runMigrationTx(cmd, msg, migrationProofKindClaim)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func cmdMigrateValidator() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate-validator [new-address] [legacy-address] [legacy-pub-key] [legacy-signature]",
		Short: "Migrate a validator operator from legacy to new address",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			msg, err := buildMigrateValidatorMsg(args)
			if err != nil {
				return err
			}
			return runMigrationTx(cmd, msg, migrationProofKindValidator)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func buildClaimLegacyAccountMsg(args []string) (*types.MsgClaimLegacyAccount, error) {
	pubKey, err := decodeCLIBase64Arg("legacy-pub-key", args[2])
	if err != nil {
		return nil, err
	}
	signature, err := decodeCLIBase64Arg("legacy-signature", args[3])
	if err != nil {
		return nil, err
	}

	msg := &types.MsgClaimLegacyAccount{
		NewAddress:      args[0],
		LegacyAddress:   args[1],
		LegacyPubKey:    pubKey,
		LegacySignature: signature,
	}
	return msg, nil
}

func buildMigrateValidatorMsg(args []string) (*types.MsgMigrateValidator, error) {
	pubKey, err := decodeCLIBase64Arg("legacy-pub-key", args[2])
	if err != nil {
		return nil, err
	}
	signature, err := decodeCLIBase64Arg("legacy-signature", args[3])
	if err != nil {
		return nil, err
	}

	msg := &types.MsgMigrateValidator{
		NewAddress:      args[0],
		LegacyAddress:   args[1],
		LegacyPubKey:    pubKey,
		LegacySignature: signature,
	}
	return msg, nil
}

type migrationProofMsg interface {
	sdk.Msg
	MigrationNewAddress() string
	MigrationLegacyAddress() string
	MigrationSetNewProof(pubKey, signature []byte)
}

func runMigrationTx(cmd *cobra.Command, msg migrationProofMsg, proofKind string) error {
	clientCtx, err := client.GetClientTxContext(cmd)
	if err != nil {
		return err
	}
	if err := ensureFromMatchesNewAddress(clientCtx, msg.MigrationNewAddress()); err != nil {
		return err
	}

	pubKey, signature, err := signNewMigrationProof(clientCtx, proofKind, msg.MigrationLegacyAddress(), msg.MigrationNewAddress())
	if err != nil {
		return err
	}
	msg.MigrationSetNewProof(pubKey, signature)
	if validateBasic, ok := msg.(sdk.HasValidateBasic); ok {
		if err := validateBasic.ValidateBasic(); err != nil {
			return err
		}
	}

	txf, err := clienttx.NewFactoryCLI(clientCtx, cmd.Flags())
	if err != nil {
		return err
	}

	// The tx itself remains unsigned. Generate-only and offline modes still
	// operate on the standard unsigned tx builder.
	if clientCtx.GenerateOnly {
		return txf.PrintUnsignedTx(clientCtx, msg)
	}

	if txf.SimulateAndExecute() || clientCtx.Simulate {
		if clientCtx.Offline {
			return errors.New("cannot estimate gas in offline mode")
		}

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

func signNewMigrationProof(clientCtx client.Context, proofKind, legacyAddress, newAddress string) ([]byte, []byte, error) {
	payload := []byte(fmt.Sprintf("lumera-evm-migration:%s:%s:%s", proofKind, legacyAddress, newAddress))

	sig, pubKey, err := clientCtx.Keyring.Sign(clientCtx.FromName, payload, signingtypes.SignMode_SIGN_MODE_LEGACY_AMINO_JSON)
	if err != nil {
		return nil, nil, err
	}

	ethPubKey, ok := pubKey.(*evmcryptotypes.PubKey)
	if !ok {
		return nil, nil, fmt.Errorf("key %q must use eth_secp256k1, got %T", clientCtx.FromName, pubKey)
	}

	return ethPubKey.Bytes(), sig, nil
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

func ensureFromMatchesNewAddress(clientCtx client.Context, newAddress string) error {
	fromAddress := clientCtx.GetFromAddress()
	if fromAddress.Empty() {
		return errors.New("missing --from address")
	}
	if fromAddress.String() != newAddress {
		return fmt.Errorf("--from address %s must match new-address %s", fromAddress.String(), newAddress)
	}
	return nil
}

func decodeCLIBase64Arg(name, value string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be base64-encoded: %w", name, err)
	}
	if len(decoded) == 0 {
		return nil, fmt.Errorf("%s must not be empty", name)
	}
	return decoded, nil
}
