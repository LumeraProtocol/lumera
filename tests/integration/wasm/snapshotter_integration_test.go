package wasm_test

import (
	"os"
	"testing"
	"time"

	wasmvm "github.com/CosmWasm/wasmvm/v3"
	wasmvmtypes "github.com/CosmWasm/wasmvm/v3/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmtypes "github.com/cometbft/cometbft/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/CosmWasm/wasmd/x/wasm/keeper"
	"github.com/CosmWasm/wasmd/x/wasm/types"

	"github.com/LumeraProtocol/lumera/app"
	lcfg "github.com/LumeraProtocol/lumera/config"
)

func TestSnapshotter(t *testing.T) {
	specs := map[string]struct {
		wasmFiles []string
	}{
		"single contract": {
			wasmFiles: []string{GetTestDataFilePath("reflect_1_5.wasm")},
		},
		"multiple contract": {
			wasmFiles: []string{
				GetTestDataFilePath("reflect_1_5.wasm"),
				GetTestDataFilePath("burner.wasm"),
				GetTestDataFilePath("reflect_1_5.wasm"),
			},
		},
		"duplicate contracts": {
			wasmFiles: []string{GetTestDataFilePath("reflect_1_5.wasm"), GetTestDataFilePath("reflect_1_5.wasm")},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			// setup source app
			srcWasmApp, genesisAddr := newWasmExampleApp(t)

			// store wasm codes on chain
			ctx := srcWasmApp.NewUncachedContext(false, tmproto.Header{
				ChainID: "foo",
				Height:  srcWasmApp.LastBlockHeight() + 1,
				Time:    time.Now(),
			})
			wasmKeeper := srcWasmApp.WasmKeeper
			contractKeeper := keeper.NewDefaultPermissionKeeper(wasmKeeper)

			srcCodeIDToChecksum := make(map[uint64][]byte, len(spec.wasmFiles))
			for i, v := range spec.wasmFiles {
				wasmCode, err := os.ReadFile(v)
				require.NoError(t, err)
				codeID, checksum, err := contractKeeper.Create(ctx, genesisAddr, wasmCode, nil)
				require.NoError(t, err)
				require.Equal(t, uint64(i+1), codeID)
				srcCodeIDToChecksum[codeID] = checksum
			}
			// create snapshot
			_, err := srcWasmApp.Commit()
			require.NoError(t, err)

			snapshotHeight := uint64(srcWasmApp.LastBlockHeight())
			snapshot, err := srcWasmApp.SnapshotManager().Create(snapshotHeight)
			require.NoError(t, err)
			assert.NotNil(t, snapshot)

			originalMaxWasmSize := types.MaxWasmSize
			types.MaxWasmSize = 1
			t.Cleanup(func() {
				types.MaxWasmSize = originalMaxWasmSize
			})

			// when snapshot imported into dest app instance
			destWasmApp := app.SetupWithEmptyStore(t)
			require.NoError(t, destWasmApp.SnapshotManager().Restore(*snapshot))
			for i := uint32(0); i < snapshot.Chunks; i++ {
				chunkBz, err := srcWasmApp.SnapshotManager().LoadChunk(snapshot.Height, snapshot.Format, i)
				require.NoError(t, err)
				end, err := destWasmApp.SnapshotManager().RestoreChunk(chunkBz)
				require.NoError(t, err)
				if end {
					break
				}
			}

			// then all wasm contracts are imported
			wasmKeeper = destWasmApp.WasmKeeper
			ctx = destWasmApp.NewUncachedContext(false, tmproto.Header{
				ChainID: "foo",
				Height:  destWasmApp.LastBlockHeight() + 1,
				Time:    time.Now(),
			})

			destCodeIDToChecksum := make(map[uint64][]byte, len(spec.wasmFiles))
			wasmKeeper.IterateCodeInfos(ctx, func(id uint64, info types.CodeInfo) bool {
				bz, err := wasmKeeper.GetByteCode(ctx, id)
				require.NoError(t, err)

				hash, err := wasmvm.CreateChecksum(bz)
				require.NoError(t, err)
				destCodeIDToChecksum[id] = hash[:]
				assert.Equal(t, hash[:], wasmvmtypes.Checksum(info.CodeHash))
				return false
			})
			assert.Equal(t, srcCodeIDToChecksum, destCodeIDToChecksum)
		})
	}
}

func newWasmExampleApp(t *testing.T) (*app.App, sdk.AccAddress) {
	senderPrivKey, senderAddr := keyPubAddr()
	pubKey := senderPrivKey.PubKey()
	sdkPubKey, err := cryptocodec.FromCmtPubKeyInterface(pubKey)
	require.NoError(t, err)

	acc := authtypes.NewBaseAccount(senderAddr, sdkPubKey, 0, 0)
	amount, ok := sdkmath.NewIntFromString("10000000000000000000")
	require.True(t, ok)

	balance := banktypes.Balance{
		Address: acc.GetAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(lcfg.ChainDenom, amount)),
	}
	validator := tmtypes.NewValidator(pubKey, 1)
	valSet := tmtypes.NewValidatorSet([]*tmtypes.Validator{validator})
	wasmApp := app.SetupWithGenesisValSet(t, valSet, []authtypes.GenesisAccount{acc}, "testing", sdk.DefaultPowerReduction, []banktypes.Balance{balance})

	return wasmApp, senderAddr
}
