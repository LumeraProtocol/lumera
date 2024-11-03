package cli

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/input"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/pastelnetwork/pasteld/x/pastelid/types"
)

func CmdCreatePastelID() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-pastel-id [--new-passphrase]",
		Short: "Create a new PastelID",
		Args:  cobra.NoArgs, // No positional args needed
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			// Get new-passphrase flag (this is only used locally)
			newPassphrase, err := cmd.Flags().GetBool("new-passphrase")
			if err != nil {
				return err
			}

			// Get the from address (funding address)
			fromAddr := clientCtx.GetFromAddress()

			// Get passphrase for secure container (either from keyring or new)
			passphrase := getPassphrase(newPassphrase, clientCtx) // Modified function

			// Generate Ed448 and LegRoast key pairs securely
			ed448Pub, ed448Priv, err := generateEd448KeyPair() // Implement this function
			if err != nil {
				return err
			}
			pqPub, pqPriv, err := generateLegRoastKeyPair() // Implement this function
			if err != nil {
				return err
			}

			// Store private keys in secure container locally
			err = storeKeysInSecureContainer(ed448Priv, pqPriv, passphrase) // Implement this function
			if err != nil {
				return err
			}

			timestamp := time.Now().UTC().String()

			// Create signature
			signature, err := createSignature(ed448Priv, fromAddr.String(), timestamp)
			if err != nil {
				return err
			}

			msg := types.NewMsgCreatePastelId(
				fromAddr.String(),             // address
				"personal",                    // idType
				hex.EncodeToString(ed448Pub),  // pastelID
				hex.EncodeToString(pqPub),     // pqKey
				hex.EncodeToString(signature), // signature
				timestamp,                     // timeStamp
				1,                             // version
			)

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().Bool("new-passphrase", false, "Use a new passphrase for the secure container")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// Helper functions (to be implemented)

func generateEd448KeyPair() ([]byte, []byte, error) {
	// ... (Implementation for generating Ed448 key pair) ...
	return nil, nil, nil
}

func generateLegRoastKeyPair() ([]byte, []byte, error) {
	// ... (Implementation for generating LegRoast key pair) ...
	return nil, nil, nil
}

func getPassphrase(newPassphrase bool, clientCtx client.Context) string {
	if newPassphrase {
		// Prompt user for new passphrase (twice for confirmation)
		reader := bufio.NewReader(os.Stdin) // Create a bufio.Reader

		passphrase, err := input.GetPassword("Enter new passphrase: ", reader)
		if err != nil {
			panic(err) // Handle error appropriately
		}
		confirmPassphrase, err := input.GetPassword("Confirm new passphrase: ", reader)
		if err != nil {
			panic(err) // Handle error appropriately
		}
		if passphrase != confirmPassphrase {
			panic(fmt.Errorf("passphrases do not match")) // Handle error appropriately
		}
		return passphrase
	} else {
		// Get passphrase from keyring
		//keyringEntry, err := clientCtx.Keyring.Key( .Get(clientCtx.From)
		//if err != nil {
		//	panic(err) // Handle error appropriately
		//}
		//passphrase, err := keyringEntry.GetPassphrase() // Use GetPassphrase method
		//if err != nil {
		//	panic(err) // Handle error appropriately
		//}
		//return passphrase
		return "" // Placeholder
	}
}

func storeKeysInSecureContainer(ed448Priv, pqPriv []byte, passphrase string) error {
	// ... (Implementation for storing private keys securely) ...
	return nil
}

func createSignature(ed448Priv []byte, address, timestamp string) ([]byte, error) {
	// ... (Implementation for creating signature using Ed448) ...
	return nil, nil
}
