package cli

import (
	"fmt"
	"encoding/base64"
	"encoding/json"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	errorsmod "cosmossdk.io/errors"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/pastelnetwork/pastel/x/pastelid/module/legroast"
)

func CmdLegroastSign() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "legroast-sign [address] [text] --algo [algorithm]",
		Short: "Sign a text (plain or base64-encoded) using a LegRoast algorithm",
		Args:  cobra.ExactArgs(2), // Expect exactly two positional arguments: address and text
		RunE: func(cmd *cobra.Command, args []string) error {
			address := args[0]

			var text []byte
			if isBase64Encoded(args[1]) {
				// Decode the text
				var err error
				text, err = base64.StdEncoding.DecodeString(args[1])
				if err != nil {
					return fmt.Errorf("failed to decode text: %w", err)
				}
			} else {
				text = []byte(args[1])
			}

			if len(address) == 0 {
				return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "address cannot be empty")
			}
			
			// Validate text to sign
			if len(text) == 0 {
				return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "text cannot be empty")
			}

			// Get the optional algorithm flag
			algoFlag, err := cmd.Flags().GetString("algo")
			if err != nil {
				algoFlag = ""
			}

			alg, err := legroast.GetLegRoastAlgorithm(algoFlag)
			if err != nil {
				return status.Error(codes.InvalidArgument, err.Error())
			}
		
			// Get the client context
			clientCtx := client.GetClientContextFromCmd(cmd)
			signature, pubKey, err := legroast.Sign(address, clientCtx.Keyring, text, alg)
			if err != nil {
				return fmt.Errorf("failed to sign text: %w", err)
			}
		
			outputFormat, _ := cmd.Flags().GetString(flags.FlagOutput)
			signatureBase64 := base64.StdEncoding.EncodeToString(signature)

			if outputFormat == flags.OutputFormatJSON {
				response := map[string]string{
					"address":   address,
					"public_key":  base64.StdEncoding.EncodeToString(pubKey),
					"algorithm": alg.String(),
					"signature": signatureBase64,
				}
				jsonOutput, err := json.Marshal(response)
				if err != nil {
					return fmt.Errorf("failed to marshal response: %w", err)
				}
				cmd.Println(string(jsonOutput))
			} else {
				cmd.Println(signatureBase64)
			}

			return nil
		},
	}

	// Add the optional --algo flag
	cmd.Flags().String("algo", "", "The LegRoast algorithm to use (default: LegendreMiddle)")

	flags.AddQueryFlagsToCmd(cmd)
	flags.AddKeyringFlags(cmd.Flags())

	return cmd
}