package keeper_test

import (
	"fmt"
	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"testing"

	"github.com/LumeraProtocol/lumera/testutil/cryptotestutils"
	"github.com/LumeraProtocol/lumera/x/action/keeper"
	"github.com/stretchr/testify/require"
)

func TestVerifySignature(t *testing.T) {
	key, address := cryptotestutils.KeyAndAddress()
	pubKey := key.PubKey()
	pairs := []keepertest.AccountPair{{Address: address, PubKey: pubKey}}
	k, ctx := keepertest.ActionKeeperWithAddress(t, pairs)

	data := "test_data"
	validSignature, err := cryptotestutils.SignString(key, data)
	require.NoError(t, err)

	validAddress := address.String()

	invalidSignature := "invalid"
	invalidAddress := "invalid"

	// Test cases
	testCases := []struct {
		name      string
		data      string
		signature string
		address   string
		expectErr bool
	}{
		{
			name:      "valid signature verification",
			signature: validSignature,
			data:      data,
			address:   validAddress,
			expectErr: false,
		},
		{
			name:      "invalid signature format",
			signature: invalidSignature,
			address:   validAddress,
			expectErr: true,
		},
		{
			name:      "invalid address",
			signature: validSignature,
			address:   invalidAddress,
			expectErr: true,
		},
	}

	// Run tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := k.VerifySignature(ctx, tc.data, tc.signature, tc.address)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestVerifyKademliaID(t *testing.T) {
	key, _ := cryptotestutils.KeyAndAddress()

	var err error
	validSignature, err := cryptotestutils.CreateSignatureString([]secp256k1.PrivKey{key}, 50)
	require.NoError(t, err)

	validIC := uint64(111)
	validMax := uint64(20)

	var validIDs []string
	for i := validIC; i < validIC+validMax; i++ {
		id, err := keeper.CreateKademliaID(validSignature, i)
		require.NoError(t, err)
		validIDs = append(validIDs, id)
	}

	var invalidIDs []string
	for i := validIC; i < validIC+validMax; i++ {
		invalidId := fmt.Sprintf("bad_id%d", i)
		invalidIDs = append(invalidIDs, invalidId)
	}

	shortIDs := []string{"bad_id1", "bad_id2", "bad_id3"}

	var invalidIDsOneEmpty []string
	for i := validIC; i < validIC+validMax; i++ {
		id, err := keeper.CreateKademliaID(validSignature, 0)
		require.NoError(t, err)
		invalidIDsOneEmpty = append(invalidIDsOneEmpty, id)
	}
	invalidIDsOneEmpty[5] = ""

	// Test cases
	testCases := []struct {
		name      string
		ids       []string
		signature string
		ic        uint64
		max       uint64
		expectErr bool
	}{
		{
			name:      "valid IDs",
			ids:       validIDs,
			signature: validSignature,
			ic:        validIC,
			max:       validMax,
			expectErr: false,
		},
		{
			name:      "invalid IDs",
			ids:       invalidIDs,
			signature: validSignature,
			ic:        validIC,
			max:       validMax,
			expectErr: true,
		},
		{
			name:      "invalid IC",
			ids:       validIDs,
			signature: validSignature,
			ic:        0,
			max:       validMax,
			expectErr: true,
		},
		{
			name:      "invalid size of IDs",
			ids:       shortIDs,
			signature: validSignature,
			ic:        validIC,
			max:       validMax,
			expectErr: true,
		},
		{
			name:      "invalid Max",
			ids:       validIDs,
			signature: validSignature,
			ic:        validIC,
			max:       validMax - 1,
			expectErr: true,
		},
		{
			name:      "invalid max - zero",
			ids:       validIDs,
			signature: validSignature,
			ic:        validIC,
			max:       0,
			expectErr: true,
		},
		{
			name:      "in valid IDs - one empty",
			ids:       invalidIDsOneEmpty,
			signature: validSignature,
			ic:        validIC,
			max:       validMax,
			expectErr: true,
		},
	}

	// Run tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := keeper.VerifyKademliaIDs(tc.ids, tc.signature, tc.ic, tc.max)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
