package evm_test

import (
	"context"
	"math"
	"testing"
	"time"

	evmantedecorators "github.com/cosmos/evm/ante/evm"
	evmmempool "github.com/cosmos/evm/mempool"
	"github.com/stretchr/testify/require"

	addresscodec "cosmossdk.io/core/address"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// TestIncrementNonceMatrix validates nonce progression checks for the EVM ante
// path.
//
// Matrix:
// - Matching nonce increments account sequence and persists account.
// - Higher tx nonce (gap) is rejected with ErrNonceGap.
// - Lower tx nonce is rejected with ErrNonceLow.
// - Max uint64 nonce is rejected (EIP-2681 overflow guard).
func TestIncrementNonceMatrix(t *testing.T) {
	testCases := []struct {
		name         string
		accountNonce uint64
		txNonce      uint64
		expectErrIs  error
		expectSubstr string
		expectSeq    uint64
		expectSet    bool
	}{
		{
			name:         "matching nonce increments sequence",
			accountNonce: 7,
			txNonce:      7,
			expectSeq:    8,
			expectSet:    true,
		},
		{
			name:         "rejects nonce gap",
			accountNonce: 7,
			txNonce:      8,
			expectErrIs:  evmmempool.ErrNonceGap,
			expectSubstr: "tx nonce",
			expectSeq:    7,
		},
		{
			name:         "rejects low nonce",
			accountNonce: 7,
			txNonce:      6,
			expectErrIs:  evmmempool.ErrNonceLow,
			expectSubstr: "invalid nonce",
			expectSeq:    7,
		},
		{
			name:         "rejects overflow at max uint64",
			accountNonce: math.MaxUint64,
			txNonce:      math.MaxUint64,
			expectErrIs:  sdkerrors.ErrInvalidSequence,
			expectSubstr: "nonce overflow",
			expectSeq:    math.MaxUint64,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var ctx sdk.Context
			ak := &nonceMockAccountKeeper{}
			acc := &authtypes.BaseAccount{Sequence: tc.accountNonce}

			err := evmantedecorators.IncrementNonce(ctx, ak, acc, tc.txNonce)
			if tc.expectErrIs != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.expectErrIs)
				require.Contains(t, err.Error(), tc.expectSubstr)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.expectSeq, acc.GetSequence())
			require.Equal(t, tc.expectSet, ak.setCalled)
		})
	}
}

type nonceDummyCodec struct{}

func (nonceDummyCodec) StringToBytes(string) ([]byte, error) { return nil, nil }
func (nonceDummyCodec) BytesToString([]byte) (string, error) { return "", nil }

type nonceMockAccountKeeper struct {
	setCalled bool
}

func (m *nonceMockAccountKeeper) NewAccountWithAddress(context.Context, sdk.AccAddress) sdk.AccountI {
	return nil
}
func (m *nonceMockAccountKeeper) GetModuleAddress(string) sdk.AccAddress { return nil }
func (m *nonceMockAccountKeeper) GetAccount(context.Context, sdk.AccAddress) sdk.AccountI {
	return nil
}
func (m *nonceMockAccountKeeper) SetAccount(context.Context, sdk.AccountI)    { m.setCalled = true }
func (m *nonceMockAccountKeeper) RemoveAccount(context.Context, sdk.AccountI) {}
func (m *nonceMockAccountKeeper) GetParams(context.Context) (params authtypes.Params) {
	return
}
func (m *nonceMockAccountKeeper) GetSequence(context.Context, sdk.AccAddress) (uint64, error) {
	return 0, nil
}
func (m *nonceMockAccountKeeper) AddressCodec() addresscodec.Codec { return nonceDummyCodec{} }
func (m *nonceMockAccountKeeper) UnorderedTransactionsEnabled() bool {
	return false
}
func (m *nonceMockAccountKeeper) RemoveExpiredUnorderedNonces(sdk.Context) error { return nil }
func (m *nonceMockAccountKeeper) TryAddUnorderedNonce(sdk.Context, []byte, time.Time) error {
	return nil
}
