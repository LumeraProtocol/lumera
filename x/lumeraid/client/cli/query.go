package cli

import (
	"encoding/base64"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"

	"github.com/LumeraProtocol/lumera/x/lumeraid/types"
)

// isBase64Encoded checks if a string is base64-encoded.
func isBase64Encoded(s string) bool {
	// Base64 strings should have lengths that are multiples of 4
	if len(s)%4 != 0 {
		return false
	}

	// Validate using the base64 decoder
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}

// GetCustomQueryCmd returns the cli query commands for this module
// These commands do not rely on gRPC and cannot be autogenerated
func GetCustomQueryCmd() *cobra.Command {
	lumeraidQueryCmd := &cobra.Command{
		Use:   types.ModuleName,
		Short: "LumeraID query commands",
		RunE:  client.ValidateCmd,
	}

	lumeraidQueryCmd.AddCommand(
		CmdLegroastSign(),
		CmdLegroastVerify(),
	)

	return lumeraidQueryCmd
}
