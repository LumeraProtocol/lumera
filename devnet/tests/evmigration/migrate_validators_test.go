package main

import (
	"testing"

	_ "github.com/LumeraProtocol/lumera/config"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func setLumeraBech32Prefixes() {
}

func mustValoperFromAcc(t *testing.T, acc string) string {
	t.Helper()

	addr, err := sdk.AccAddressFromBech32(acc)
	if err != nil {
		t.Fatalf("parse acc address %s: %v", acc, err)
	}
	return sdk.ValAddress(addr).String()
}

func TestPickValidatorCandidatesAutoDetectSkipsMigratedDestinationKey(t *testing.T) {
	setLumeraBech32Prefixes()
	*flagValidatorKeys = ""

	legacyAddr := "lumera1ld2a96xxu660tk77w787rd33rlw9gutlp7f767"
	newAddr := "lumera1nkwn2v94h7vzgqnc2pdhwel26cc3mmpnnvlafv"

	keys := []keyRecord{
		{
			Name:    "supernova_validator_1_key",
			Address: legacyAddr,
			PubKey:  `{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"legacy"}`,
		},
		{
			Name:    "evm-supernova-validator-1-key",
			Address: newAddr,
			PubKey:  `{"@type":"/ethermint.crypto.v1.ethsecp256k1.PubKey","key":"new"}`,
		},
	}

	candidates := pickValidatorCandidates([]string{
		mustValoperFromAcc(t, legacyAddr),
		mustValoperFromAcc(t, newAddr),
	}, keys)

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d: %#v", len(candidates), candidates)
	}
	if candidates[0].KeyName != "supernova_validator_1_key" {
		t.Fatalf("expected legacy validator key, got %s", candidates[0].KeyName)
	}
}

func TestPickValidatorCandidatesExplicitKeyRejectsMigratedDestinationKey(t *testing.T) {
	setLumeraBech32Prefixes()
	t.Cleanup(func() { *flagValidatorKeys = "" })

	legacyAddr := "lumera1ld2a96xxu660tk77w787rd33rlw9gutlp7f767"
	newAddr := "lumera1nkwn2v94h7vzgqnc2pdhwel26cc3mmpnnvlafv"

	keys := []keyRecord{
		{
			Name:    "supernova_validator_1_key",
			Address: legacyAddr,
			PubKey:  `{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"legacy"}`,
		},
		{
			Name:    "evm-supernova-validator-1-key",
			Address: newAddr,
			PubKey:  `{"@type":"/ethermint.crypto.v1.ethsecp256k1.PubKey","key":"new"}`,
		},
	}

	*flagValidatorKeys = "evm-supernova-validator-1-key,supernova_validator_1_key"
	candidates := pickValidatorCandidates([]string{
		mustValoperFromAcc(t, legacyAddr),
		mustValoperFromAcc(t, newAddr),
	}, keys)

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate after filtering explicit names, got %d: %#v", len(candidates), candidates)
	}
	if candidates[0].KeyName != "supernova_validator_1_key" {
		t.Fatalf("expected legacy validator key, got %s", candidates[0].KeyName)
	}
}
