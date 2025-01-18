package cli_test

import (
	"testing"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	

	"github.com/pastelnetwork/pastel/x/pastelid/client/cli"
	"github.com/pastelnetwork/pastel/x/pastelid/module/legroast"
)

func TestCmdLegroastSign(t *testing.T) {
	textToSign := "text_to_sign"
	mockedAddress := "mocked_address"
	mockedPublicKey := "mocked_public_key"
	mockedSignature := "mocked_signature"

	b64TextToSign := base64.StdEncoding.EncodeToString([]byte(textToSign))
	b64MockedPublicKey := base64.StdEncoding.EncodeToString([]byte(mockedPublicKey))
	b64MockedSignature := base64.StdEncoding.EncodeToString([]byte(mockedSignature))

	// Mock the legroast.Sign function
	originalSign := legroast.Sign
	legroast.Sign = func(address string, kr keyring.Keyring, text []byte, algo legroast.LegRoastAlgorithm) ([]byte, []byte, error) {
		return []byte(mockedSignature), []byte(mockedPublicKey), nil
	}
	defer func() { legroast.Sign = originalSign }()

	// Create the command
	cmd := cli.CmdLegroastSign()

	// Define test cases
    testCases := []struct {
        name       string
        args       []string
        expectErr  bool
		expectJson bool
		expected   map[string]string
    }{
        {
            name:      "successful signing plain text with default algo (json output)",
            args:      []string{mockedAddress, textToSign, "--output", "json"},
            expectErr: false,
			expectJson: true,
			expected: map[string]string{
				"address": mockedAddress,
				"algorithm": legroast.DefaultAlgorithm.String(),
				"public_key": b64MockedPublicKey,
				"signature": b64MockedSignature,
			},
        },
        {
            name:      "successful signing plain text with PowerFast algo (json output)",
            args:      []string{mockedAddress, textToSign, "--algo", "PowerFast"},
            expectErr: false,
			expectJson: true,
			expected: map[string]string{
				"address": mockedAddress,
				"algorithm": "PowerFast",
				"public_key": b64MockedPublicKey,
				"signature": b64MockedSignature,
			},
        },
        {
            name:      "successful signing base64-encoded text with LegendreFast algo (json output)",
            args:      []string{mockedAddress, b64TextToSign, "--algo", "LegendreFast"},
            expectErr: false,
			expectJson: true,
			expected: map[string]string{
				"address": mockedAddress,
				"algorithm": "LegendreFast",
				"public_key": b64MockedPublicKey,
				"signature": b64MockedSignature,
			},
        },
        {
            name:      "successful signing plain text with PowerCompact algo (text output)",
			args:      []string{mockedAddress, textToSign, "--algo", "PowerCompact", "--output", "text"},
			expectErr: false,
			expectJson: false,
			expected: map[string]string{
				"signature": b64MockedSignature,
			},
		},
        {
            name:      "missing address",
            args:      []string{"", textToSign},
            expectErr: true,
        },
		{
			name:      "missing text to sign",
			args:      []string{mockedAddress},
			expectErr: true,
		},
        {
            name:      "invalid algo option",
			args:      []string{mockedAddress, textToSign, "--algo", "invalid"},
			expectErr: true,
        },
		{
			name:      "invalid base654-encoded text to sign",
			args:      []string{mockedAddress, b64TextToSign + "="},
			expectErr: true,
		},
    }

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set args for the command
			cmd.SetArgs(tc.args)

			// capture output 
			out := new(bytes.Buffer)
			cmd.SetOut(out)
			cmd.SetErr(io.Discard)

			err := cmd.Execute()
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tc.expectJson {
					var actual map[string]string
					err := json.Unmarshal(out.Bytes(), &actual)
					require.NoError(t, err, "failed to unmarshal json output")

					// Compare actual with expected
					require.Equal(t, tc.expected, actual, "unexpected json output")
				} else {
					// Compare actual with expected
					require.Equal(t, tc.expected["signature"], strings.TrimSuffix(out.String(), "\n"), "unexpected text output")
				}
			}
		})
	}
}
