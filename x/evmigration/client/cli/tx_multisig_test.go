package cli_test

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/evmigration/client/cli"
)

func TestComputePayload_StableFormat(t *testing.T) {
	got := "lumera-evm-migration:lumera-test-1:76857769:claim:lumera1abc:lumera1xyz"
	require.Equal(t, got, cli.ComputePayload("lumera-test-1", 76857769, "claim", "lumera1abc", "lumera1xyz"))
}

func TestPartialProof_RoundTrip(t *testing.T) {
	pp := &cli.PartialProof{
		Version:       2,
		Kind:          "claim",
		LegacyAddress: "lumera1abc",
		NewAddress:    "lumera1xyz",
		ChainID:       "lumera-test-1",
		EVMChainID:    76857769,
		PayloadHex:    hex.EncodeToString([]byte("p")),
		Legacy: &cli.SideSpec{
			PubKey:    "AAAA",
			SigFormat: "SIG_FORMAT_CLI",
		},
		New: &cli.SideSpec{
			PubKey:    "CCCC",
			SigFormat: "SIG_FORMAT_CLI",
		},
		PartialLegacySignatures: []cli.PartialSignature{{Index: 0, Signature: "BBBB"}},
		PartialNewSignatures:    []cli.PartialSignature{},
	}
	b, err := pp.MarshalIndent()
	require.NoError(t, err)
	require.Contains(t, string(b), "SIG_FORMAT_CLI")
}

func TestAssertPartialProofsConsistent_Matching(t *testing.T) {
	a := &cli.PartialProof{
		Version: 2, Kind: "claim", LegacyAddress: "A", NewAddress: "B", ChainID: "c", EVMChainID: 1,
		Legacy: &cli.SideSpec{Threshold: 2, SubPubKeys: []string{"x", "y", "z"}, SigFormat: "SIG_FORMAT_CLI"},
		New:    &cli.SideSpec{Threshold: 2, SubPubKeys: []string{"p", "q", "r"}, SigFormat: "SIG_FORMAT_CLI"},
	}
	b := *a
	require.NoError(t, cli.AssertPartialProofsConsistent(a, &b))
}

func TestAssertPartialProofsConsistent_ChainIDMismatch(t *testing.T) {
	a := &cli.PartialProof{
		ChainID: "c1",
		Legacy:  &cli.SideSpec{},
		New:     &cli.SideSpec{},
	}
	b := &cli.PartialProof{
		ChainID: "c2",
		Legacy:  &cli.SideSpec{},
		New:     &cli.SideSpec{},
	}
	err := cli.AssertPartialProofsConsistent(a, b)
	require.ErrorContains(t, err, "chain_id differs")
}

func TestAssertPartialProofsConsistent_SideSpecPresenceMismatch(t *testing.T) {
	a := &cli.PartialProof{Legacy: &cli.SideSpec{PubKey: "AA", SigFormat: "SIG_FORMAT_CLI"}, New: nil}
	b := &cli.PartialProof{Legacy: &cli.SideSpec{PubKey: "AA", SigFormat: "SIG_FORMAT_CLI"}, New: &cli.SideSpec{}}
	err := cli.AssertPartialProofsConsistent(a, b)
	require.ErrorContains(t, err, "new side spec presence differs")
}

func TestAssertPartialProofsConsistent_PayloadHexMismatch(t *testing.T) {
	a := &cli.PartialProof{PayloadHex: "aa", Legacy: &cli.SideSpec{}, New: &cli.SideSpec{}}
	b := &cli.PartialProof{PayloadHex: "bb", Legacy: &cli.SideSpec{}, New: &cli.SideSpec{}}
	err := cli.AssertPartialProofsConsistent(a, b)
	require.ErrorContains(t, err, "payload_hex differs")
}

// TestLoadPartialProof_V2FileWithFutureField_UnknownFieldError verifies that a v2
// file with an unrecognized field gets an "unknown field" error, not a version error.
func TestLoadPartialProof_V2FileWithFutureField_UnknownFieldError(t *testing.T) {
	raw := []byte(`{"version": 2, "future_field": "something", "kind": "claim"}`)
	tmp := filepath.Join(t.TempDir(), "v2-future.json")
	require.NoError(t, os.WriteFile(tmp, raw, 0o600))
	_, err := cli.LoadPartialProof(tmp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown field")
}
