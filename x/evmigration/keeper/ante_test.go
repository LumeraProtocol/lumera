package keeper_test

import (
	"strings"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/require"
)

func TestVerifyMigrationProofsForAnte(t *testing.T) {
	fixture := initMsgServerFixture(t)

	legacyPriv := secp256k1.GenPrivKey()
	legacyAddr := sdk.AccAddress(legacyPriv.PubKey().Address())
	newPriv, newAddr := testNewMigrationAccount(t)

	t.Run("claim valid proofs", func(t *testing.T) {
		msg := newClaimMigrationMsg(t, legacyPriv, legacyAddr, newPriv, newAddr)
		require.NoError(t, fixture.keeper.VerifyMigrationProofsForAnte(fixture.ctx, msg))
	})

	t.Run("claim invalid legacy proof", func(t *testing.T) {
		msg := newClaimMigrationMsg(t, legacyPriv, legacyAddr, newPriv, newAddr)
		msg.LegacyProof.GetSingle().Signature[0] ^= 0x01

		err := fixture.keeper.VerifyMigrationProofsForAnte(fixture.ctx, msg)
		require.Error(t, err)
		require.Contains(t, strings.ToLower(err.Error()), "signature")
	})

	t.Run("claim invalid new proof", func(t *testing.T) {
		msg := newClaimMigrationMsg(t, legacyPriv, legacyAddr, newPriv, newAddr)
		msg.NewProof.GetSingle().Signature[0] ^= 0x01

		err := fixture.keeper.VerifyMigrationProofsForAnte(fixture.ctx, msg)
		require.Error(t, err)
		require.Contains(t, strings.ToLower(err.Error()), "signature")
	})

	t.Run("validator valid proofs", func(t *testing.T) {
		msg := newValidatorMigrationMsg(t, legacyPriv, legacyAddr, newPriv, newAddr)
		require.NoError(t, fixture.keeper.VerifyMigrationProofsForAnte(fixture.ctx, msg))
	})

	t.Run("unsupported message type", func(t *testing.T) {
		msg := banktypes.NewMsgSend(legacyAddr, newAddr, sdk.NewCoins())

		err := fixture.keeper.VerifyMigrationProofsForAnte(fixture.ctx, msg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported evmigration ante message type")
	})
}
