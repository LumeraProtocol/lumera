package cli_test

import (
	"encoding/hex"
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
		Version:       1,
		Kind:          "claim",
		LegacyAddress: "lumera1abc",
		NewAddress:    "lumera1xyz",
		ChainID:       "lumera-test-1",
		EVMChainID:    76857769,
		PayloadHex:    hex.EncodeToString([]byte("p")),
		Single: &cli.PartialSingle{
			PubKeyB64: "AAAA",
			SigFormat: "SIG_FORMAT_CLI",
		},
		PartialSigs: []cli.PartialSubSignature{{Index: 0, SignatureB64: "BBBB"}},
	}
	b, err := pp.MarshalIndent()
	require.NoError(t, err)
	require.Contains(t, string(b), "SIG_FORMAT_CLI")
}

func TestAssertPartialProofsConsistent_Matching(t *testing.T) {
	a := &cli.PartialProof{
		Version: 1, Kind: "claim", LegacyAddress: "A", NewAddress: "B", ChainID: "c", EVMChainID: 1,
		Multisig: &cli.PartialMultisig{Threshold: 2, SubPubKeysB64: []string{"x", "y", "z"}, SigFormat: "SIG_FORMAT_CLI"},
	}
	b := *a
	require.NoError(t, cli.AssertPartialProofsConsistent(a, &b))
}

func TestAssertPartialProofsConsistent_ChainIDMismatch(t *testing.T) {
	a := &cli.PartialProof{ChainID: "c1", Multisig: &cli.PartialMultisig{}}
	b := &cli.PartialProof{ChainID: "c2", Multisig: &cli.PartialMultisig{}}
	err := cli.AssertPartialProofsConsistent(a, b)
	require.ErrorContains(t, err, "chain_id mismatch")
}

func TestAssertPartialProofsConsistent_ProofKindMismatch(t *testing.T) {
	a := &cli.PartialProof{Single: &cli.PartialSingle{}}
	b := &cli.PartialProof{Multisig: &cli.PartialMultisig{}}
	err := cli.AssertPartialProofsConsistent(a, b)
	require.ErrorContains(t, err, "proof-kind mismatch")
}
