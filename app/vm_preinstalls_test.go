package app

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// TestEVMAddPreinstallsMatrix verifies AddPreinstalls creates accounts/code for
// valid entries and rejects invalid preinstall inputs.
//
// Matrix:
// - valid preinstall creates account and stores code/code-hash
// - empty code is rejected
// - preinstall address with existing account is rejected
// - same existing code hash is accepted
// - different existing code hash is rejected
func TestEVMAddPreinstallsMatrix(t *testing.T) {
	testCases := []struct {
		name            string
		preinstall      evmtypes.Preinstall
		setupExisting   bool
		setupCodeHash   string
		expectErrSubstr string
	}{
		{
			name: "valid preinstall",
			preinstall: evmtypes.Preinstall{
				Address: "0x1000000000000000000000000000000000000001",
				Code:    "0x6001600055",
			},
		},
		{
			name: "rejects preinstall without code",
			preinstall: evmtypes.Preinstall{
				Address: "0x1000000000000000000000000000000000000002",
				Code:    "0x",
			},
			expectErrSubstr: "has no code",
		},
		{
			name: "rejects preinstall with existing account",
			preinstall: evmtypes.Preinstall{
				Address: "0x1000000000000000000000000000000000000003",
				Code:    "0x6001600055",
			},
			setupExisting:   true,
			expectErrSubstr: "already has an account in account keeper",
		},
		{
			name: "allows preinstall when same code hash already exists",
			preinstall: evmtypes.Preinstall{
				Address: "0x1000000000000000000000000000000000000004",
				Code:    "0x6001600055",
			},
			setupCodeHash: "0x6001600055",
		},
		{
			name: "rejects preinstall when different code hash already exists",
			preinstall: evmtypes.Preinstall{
				Address: "0x1000000000000000000000000000000000000005",
				Code:    "0x6001600055",
			},
			setupCodeHash:   "0x6002600055",
			expectErrSubstr: "already has a code hash with a different code hash",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			app := Setup(t)
			ctx := app.BaseApp.NewContext(false)
			addr := common.HexToAddress(tc.preinstall.Address)
			accAddr := sdk.AccAddress(addr.Bytes())

			if tc.setupExisting {
				account := app.AuthKeeper.NewAccountWithAddress(ctx, accAddr)
				app.AuthKeeper.SetAccount(ctx, account)
			}
			if tc.setupCodeHash != "" {
				existingCode := common.FromHex(tc.setupCodeHash)
				existingHash := crypto.Keccak256Hash(existingCode)
				app.EVMKeeper.SetCodeHash(ctx, addr.Bytes(), existingHash.Bytes())
				app.EVMKeeper.SetCode(ctx, existingHash.Bytes(), existingCode)
			}

			err := app.EVMKeeper.AddPreinstalls(ctx, []evmtypes.Preinstall{tc.preinstall})
			if tc.expectErrSubstr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErrSubstr)
				return
			}

			require.NoError(t, err)

			account := app.AuthKeeper.GetAccount(ctx, accAddr)
			require.NotNil(t, account)

			expectedCode := common.FromHex(tc.preinstall.Code)
			expectedHash := crypto.Keccak256Hash(expectedCode)

			gotHash := app.EVMKeeper.GetCodeHash(ctx, addr)
			require.Equal(t, expectedHash, gotHash)
			require.Equal(t, expectedCode, app.EVMKeeper.GetCode(ctx, gotHash))
		})
	}
}
