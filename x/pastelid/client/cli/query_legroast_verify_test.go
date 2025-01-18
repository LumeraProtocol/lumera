package cli_test

import (
	"testing"
	"bytes"
	"encoding/base64"
	"io"
	"fmt"
	"strings"

	"github.com/stretchr/testify/require"

	"github.com/pastelnetwork/pastel/x/pastelid/client/cli"
	"github.com/pastelnetwork/pastel/x/pastelid/module/legroast"
)

func TestCmdLegroastVerify(t *testing.T) {
	textToVerify := "text_to_verify"
	mockedPublicKey := "mocked_public_key"
	mockedSignature := "mocked_signature"

	b64TextToVerify := base64.StdEncoding.EncodeToString([]byte(textToVerify))
	b64MockedPublicKey := base64.StdEncoding.EncodeToString([]byte(mockedPublicKey))
	b64MockedSignature := base64.StdEncoding.EncodeToString([]byte(mockedSignature))

	// Mock the legroast.Verify function
	originalVerify := legroast.Verify
	legroast.Verify = func(text, publicKey, signature []byte) error {
		if string(text) != textToVerify || string(publicKey) != mockedPublicKey || string(signature) != mockedSignature {
			return fmt.Errorf("verification failed")
		}
		return nil
	}
	defer func() { legroast.Verify = originalVerify }() // Restore the original function after the test

	// Create the command
	cmd := cli.CmdLegroastVerify()

	// Define test cases
	testCases := []struct {
		name       string
		args       []string
		expectErr  bool
		expected   string
		}{
			{
				name:      "successful verification with plain text",
				args:      []string{textToVerify, b64MockedPublicKey, b64MockedSignature},
				expectErr: false,
				expected:  "Verification successful",
			},
			{
				name:      "successful verification with base64-encoded text",
				args:      []string{b64TextToVerify, b64MockedPublicKey, b64MockedSignature},
				expectErr: false,
				expected:  "Verification successful",
			},
			{
				name:      "invalid public key",
				args:      []string{textToVerify, b64MockedPublicKey + "=", b64MockedSignature},
				expectErr: true,
			},
			{
				name:      "invalid signature",
				args:      []string{textToVerify, b64MockedPublicKey, b64MockedSignature + "="},
				expectErr: true,
			},
			{
				name:      "invalid base64 text",
				args:      []string{b64TextToVerify + "=", b64MockedPublicKey, b64MockedSignature},
				expectErr: true,
			},
			{
				name:      "missing public key",
				args:      []string{textToVerify, "", b64MockedSignature},
				expectErr: true,
			},
			{
				name:      "missing signature",
				args:      []string{textToVerify, b64MockedPublicKey, ""},
				expectErr: true,
			},
			{
				name:      "missing text to verify",
				args:      []string{"", b64MockedPublicKey, b64MockedSignature},
				expectErr: true,
			},
		}

		// Run the test cases
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Set args for the command
				cmd.SetArgs(tc.args)
	
				// Capture output
				out := new(bytes.Buffer)
				cmd.SetOut(out)
				cmd.SetErr(io.Discard)
	
				// Execute the command
				err := cmd.Execute()
				if tc.expectErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
	
					// Compare output
					require.Equal(t, tc.expected, strings.TrimSuffix(out.String(), "\n"), "unexpected output")
				}
			})
		}
}		