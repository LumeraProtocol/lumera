package keeper_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	keepertest "github.com/LumeraProtocol/lumera/testutil/keeper"
	"github.com/LumeraProtocol/lumera/x/action/v1/keeper"
	"github.com/cosmos/btcutil/base58"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"lukechampine.com/blake3"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/LumeraProtocol/lumera/testutil/cryptotestutils"
	"github.com/stretchr/testify/require"
)

const separatorByte = byte('.')

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

// Mocked Layout/Block types to match actual implementation
type block struct {
	BlockID           int      `json:"block_id"`
	EncoderParameters []int    `json:"encoder_parameters"`
	OriginalOffset    int64    `json:"original_offset"`
	Size              int64    `json:"size"`
	Symbols           []string `json:"symbols"`
	Hash              string   `json:"hash"`
}

type layout struct {
	Blocks []block `json:"blocks"`
}

func TestGenerateSupernodeRQIDs(t *testing.T) {
	type testCase struct {
		name      string
		layout    layout
		signature string
		rqMax     uint32
	}

	rand.Seed(time.Now().UnixNano())
	ic := uint32(rand.Intn(100000))

	tests := []testCase{
		{
			name: "basic metadata with 1 block and 2 symbols",
			layout: layout{
				Blocks: []block{
					{
						BlockID:           1,
						EncoderParameters: []int{10, 20},
						OriginalOffset:    0,
						Size:              2048,
						Symbols:           []string{"S1", "S2"},
						Hash:              "xyz123",
					},
				},
			},
			signature: "mock_supernode_signature",
			rqMax:     5,
		},
		{
			name: "empty blocks",
			layout: layout{
				Blocks: []block{},
			},
			signature: "sig",
			rqMax:     3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ids, encMetadataWithSignature, err := generateSupernodeRQIDs(tc.layout, tc.signature, ic, tc.rqMax)
			require.NoError(t, err)
			require.Len(t, ids, int(tc.rqMax))

			// Validate with Lumera logic
			err = keeper.VerifyKademliaIDs(ids, string(encMetadataWithSignature), uint64(ic), uint64(tc.rqMax))
			require.NoError(t, err, "expected RQID validation to pass")
		})
	}
}

// Supernode-style generator with BLAKE3
func generateSupernodeRQIDs(metadata layout, signature string, ic, max uint32) ([]string, []byte, error) {
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal metadata: %w", err)
	}

	b64encodedMetadata := make([]byte, base64.StdEncoding.EncodedLen(len(metadataBytes)))
	base64.StdEncoding.Encode(b64encodedMetadata, metadataBytes)
	var signedBuffer bytes.Buffer
	signedBuffer.Write(b64encodedMetadata)
	signedBuffer.WriteByte(separatorByte)
	signedBuffer.WriteString(signature)
	encMetadataWithSignature := signedBuffer.Bytes()

	var ids []string
	for i := uint32(0); i < max; i++ {
		var b bytes.Buffer
		b.Write(encMetadataWithSignature)
		b.WriteByte(separatorByte)
		b.WriteString(strconv.Itoa(int(ic + i)))

		compressed, err := keeper.ZstdCompress(b.Bytes())
		if err != nil {
			return nil, nil, fmt.Errorf("compression error: %w", err)
		}

		hash := blake3.Sum256(compressed)
		ids = append(ids, base58.Encode(hash[:]))
	}
	return ids, encMetadataWithSignature, nil
}
