package types_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

func TestParamsValidateCanaryLegacyAddresses(t *testing.T) {
	addresses := make([]string, types.MaxCanaryLegacyAddresses+1)
	for i := range addresses {
		addresses[i] = sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	}
	sort.Strings(addresses)

	tests := []struct {
		name      string
		addresses []string
		wantErr   string
	}{
		{name: "open empty"},
		{name: "valid sorted unique", addresses: addresses[:2]},
		{name: "duplicate", addresses: []string{addresses[0], addresses[0]}, wantErr: "duplicates"},
		{name: "unsorted", addresses: []string{addresses[1], addresses[0]}, wantErr: "sorted lexicographically"},
		{name: "empty entry", addresses: []string{""}, wantErr: "must not be empty"},
		{name: "noncanonical alternate encoding", addresses: []string{strings.ToUpper(addresses[0])}, wantErr: "canonical account encoding"},
		{name: "at cap", addresses: addresses[:types.MaxCanaryLegacyAddresses]},
		{name: "cap plus one", addresses: addresses, wantErr: "at most 64"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params := types.DefaultParams()
			params.CanaryLegacyAddresses = tc.addresses
			err := params.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

func TestNewParamsDefaultsToOpenCanaryList(t *testing.T) {
	params := types.NewParams(true, 0, 50, 2500, 20)
	require.Empty(t, params.CanaryLegacyAddresses)
	require.NoError(t, params.Validate())
}
