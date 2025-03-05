package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/LumeraProtocol/lumera/x/lumeraid/legroast"
)

func CmdLegroastVerify() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "legroast-verify [text] [base64_public_key] [base64_signature]",
		Short: "Verify a text (plain or base64-encoded) using a LegRoast algorithm",
		Args:  cobra.ExactArgs(3), // Expect exactly three positional arguments: text, pubkey, and signature
		RunE: func(cmd *cobra.Command, args []string) error {
			// Decode the text
			var text []byte
			if isBase64Encoded(args[0]) {
				// Decode the text
				var err error
				text, err = base64.StdEncoding.DecodeString(args[0])
				if err != nil {
					return fmt.Errorf("failed to decode base64-encoded text: %w", err)
				}
			} else {
				text = []byte(args[0])
			}

			// Decode the public key
			pubkey, err := base64.StdEncoding.DecodeString(args[1])
			if err != nil {
				return fmt.Errorf("failed to decode base64-encoded public key: %w", err)
			}

			// Decode the signature
			signature, err := base64.StdEncoding.DecodeString(args[2])
			if err != nil {
				return fmt.Errorf("failed to decode base64-encoded signature: %w", err)
			}

			// Validate text to verify
			if len(text) == 0 {
				return fmt.Errorf("text cannot be empty")
			}

			// Validate public key
			if len(pubkey) == 0 {
				return fmt.Errorf("public key cannot be empty")
			}

			// Validate signature
			if len(signature) == 0 {
				return fmt.Errorf("signature cannot be empty")
			}

			// Get the LegRoast public key
			err = legroast.Verify(text, pubkey, signature)
			if err != nil {
				return fmt.Errorf("failed to verify text signature: %w", err)
			}

			outputFormat, _ := cmd.Flags().GetString(flags.FlagOutput)
			if outputFormat == flags.OutputFormatJSON {
				output, err := json.Marshal(map[string]interface{}{
					"verified": true,
				})
				if err != nil {
					return fmt.Errorf("failed to marshal output to JSON: %w", err)
				}
				cmd.Println(string(output))
			} else {
				cmd.Println("Verification successful")
			}

			return nil
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
