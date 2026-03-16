package cli

import (
	"encoding/base64"
	"sync"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

var bech32ConfigOnce sync.Once

func ensureLumeraBech32Config() {
	bech32ConfigOnce.Do(func() {
		cfg := sdk.GetConfig()
		cfg.SetBech32PrefixForAccount("lumera", "lumerapub")
		cfg.SetBech32PrefixForValidator("lumeravaloper", "lumeravaloperpub")
		cfg.SetBech32PrefixForConsensusNode("lumeravalcons", "lumeravalconspub")
	})
}

// TestBuildClaimLegacyAccountMsg verifies the CLI decoder fills the legacy
// proof fields and leaves the destination proof to be derived locally.
func TestBuildClaimLegacyAccountMsg(t *testing.T) {
	ensureLumeraBech32Config()
	pubKey := base64.StdEncoding.EncodeToString(make([]byte, 33))
	signature := base64.StdEncoding.EncodeToString([]byte("sig"))

	msg, err := buildClaimLegacyAccountMsg([]string{
		"lumera137g46lvwtztvw8anuyl2ljpjvw7dhx5d9xqdqv",
		"lumera1aus3zltxzra56ax4ccsuqgh3r9au38ms3t8e6x",
		pubKey,
		signature,
	})
	require.NoError(t, err)
	require.Len(t, msg.LegacyPubKey, 33)
	require.Equal(t, []byte("sig"), msg.LegacySignature)
	require.Nil(t, msg.NewPubKey)
	require.Nil(t, msg.NewSignature)
}

// TestBuildMigrateValidatorMsg_InvalidBase64 verifies invalid base64 input is
// rejected before any proof derivation or tx construction runs.
func TestBuildMigrateValidatorMsg_InvalidBase64(t *testing.T) {
	ensureLumeraBech32Config()
	_, err := buildMigrateValidatorMsg([]string{
		"lumera137g46lvwtztvw8anuyl2ljpjvw7dhx5d9xqdqv",
		"lumera1aus3zltxzra56ax4ccsuqgh3r9au38ms3t8e6x",
		"not-base64",
		"also-not-base64",
	})
	require.ErrorContains(t, err, "legacy-pub-key")
}

// TestDecodeCLIBase64Arg_Empty verifies empty decoded values are rejected.
func TestDecodeCLIBase64Arg_Empty(t *testing.T) {
	_, err := decodeCLIBase64Arg("legacy-signature", base64.StdEncoding.EncodeToString(nil))
	require.ErrorContains(t, err, "must not be empty")
}
