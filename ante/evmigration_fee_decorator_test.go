package ante

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	evmigrationtypes "github.com/LumeraProtocol/lumera/x/evmigration/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

type testMsgsTx struct {
	msgs []sdk.Msg
}

func (m testMsgsTx) GetMsgs() []sdk.Msg { return m.msgs }

func (m testMsgsTx) GetMsgsV2() ([]proto.Message, error) { return nil, nil }

func (m testMsgsTx) ValidateBasic() error { return nil }

// TestEVMigrationFeeDecorator_AllMigrationMessages verifies that when all tx
// messages are migration messages, min-gas-prices are cleared for downstream
// decorators.
func TestEVMigrationFeeDecorator_AllMigrationMessages(t *testing.T) {
	dec := EVMigrationFeeDecorator{}
	ctx := sdk.Context{}.WithMinGasPrices(sdk.DecCoins{sdk.NewDecCoinFromDec("ulume", sdkmath.LegacyNewDec(1))})
	tx := testMsgsTx{
		msgs: []sdk.Msg{
			&evmigrationtypes.MsgClaimLegacyAccount{},
			&evmigrationtypes.MsgMigrateValidator{},
		},
	}

	nextCalled := false
	_, err := dec.AnteHandle(ctx, tx, false, func(nextCtx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
		nextCalled = true
		require.Empty(t, nextCtx.MinGasPrices(), "migration tx should clear min gas prices")
		return nextCtx, nil
	})

	require.NoError(t, err)
	require.True(t, nextCalled)
}

// TestEVMigrationFeeDecorator_MixedMessages verifies that fee waiving does not
// apply when at least one non-migration message is present.
func TestEVMigrationFeeDecorator_MixedMessages(t *testing.T) {
	dec := EVMigrationFeeDecorator{}
	originalMinGas := sdk.DecCoins{sdk.NewDecCoinFromDec("ulume", sdkmath.LegacyNewDec(1))}
	ctx := sdk.Context{}.WithMinGasPrices(originalMinGas)
	tx := testMsgsTx{
		msgs: []sdk.Msg{
			&evmigrationtypes.MsgClaimLegacyAccount{},
			&banktypes.MsgSend{},
		},
	}

	nextCalled := false
	_, err := dec.AnteHandle(ctx, tx, false, func(nextCtx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
		nextCalled = true
		require.Equal(t, originalMinGas, nextCtx.MinGasPrices(), "mixed tx must keep normal min gas prices")
		return nextCtx, nil
	})

	require.NoError(t, err)
	require.True(t, nextCalled)
}
