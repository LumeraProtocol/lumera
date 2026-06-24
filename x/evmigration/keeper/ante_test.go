package keeper_test

import (
	"strings"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/LumeraProtocol/lumera/x/evmigration/types"
)

func TestVerifyMigrationProofsForAnte(t *testing.T) {
	legacyPriv := secp256k1.GenPrivKey()
	legacyAddr := sdk.AccAddress(legacyPriv.PubKey().Address())
	newPriv, newAddr := testNewMigrationAccount(t)

	t.Run("claim valid proofs", func(t *testing.T) {
		fixture := initMsgServerFixture(t)
		fixture.accountKeeper.EXPECT().
			GetAccount(gomock.Any(), legacyAddr).
			Return(authtypes.NewBaseAccountWithAddress(legacyAddr))

		msg := newClaimMigrationMsg(t, legacyPriv, legacyAddr, newPriv, newAddr)
		require.NoError(t, fixture.keeper.VerifyMigrationProofsForAnte(fixture.ctx, msg))
	})

	t.Run("claim invalid legacy proof", func(t *testing.T) {
		fixture := initMsgServerFixture(t)
		fixture.accountKeeper.EXPECT().
			GetAccount(gomock.Any(), legacyAddr).
			Return(authtypes.NewBaseAccountWithAddress(legacyAddr))

		msg := newClaimMigrationMsg(t, legacyPriv, legacyAddr, newPriv, newAddr)
		msg.LegacyProof.GetSingle().Signature[0] ^= 0x01

		err := fixture.keeper.VerifyMigrationProofsForAnte(fixture.ctx, msg)
		require.Error(t, err)
		require.Contains(t, strings.ToLower(err.Error()), "signature")
	})

	t.Run("claim invalid new proof", func(t *testing.T) {
		fixture := initMsgServerFixture(t)
		fixture.accountKeeper.EXPECT().
			GetAccount(gomock.Any(), legacyAddr).
			Return(authtypes.NewBaseAccountWithAddress(legacyAddr))

		msg := newClaimMigrationMsg(t, legacyPriv, legacyAddr, newPriv, newAddr)
		msg.NewProof.GetSingle().Signature[0] ^= 0x01

		err := fixture.keeper.VerifyMigrationProofsForAnte(fixture.ctx, msg)
		require.Error(t, err)
		require.Contains(t, strings.ToLower(err.Error()), "signature")
	})

	t.Run("validator valid proofs", func(t *testing.T) {
		fixture := initMsgServerFixture(t)
		oldValAddr := sdk.ValAddress(legacyAddr)
		fixture.accountKeeper.EXPECT().
			GetAccount(gomock.Any(), legacyAddr).
			Return(authtypes.NewBaseAccountWithAddress(legacyAddr))
		fixture.stakingKeeper.EXPECT().
			GetValidator(gomock.Any(), oldValAddr).
			Return(stakingtypes.Validator{OperatorAddress: oldValAddr.String()}, nil)

		msg := newValidatorMigrationMsg(t, legacyPriv, legacyAddr, newPriv, newAddr)
		require.NoError(t, fixture.keeper.VerifyMigrationProofsForAnte(fixture.ctx, msg))
	})

	t.Run("unsupported message type", func(t *testing.T) {
		fixture := initMsgServerFixture(t)
		msg := banktypes.NewMsgSend(legacyAddr, newAddr, sdk.NewCoins())

		err := fixture.keeper.VerifyMigrationProofsForAnte(fixture.ctx, msg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported evmigration ante message type")
	})
}

// TestVerifyMigrationProofsForAnte_AdmissionGate pins the mempool admission
// gate: when migration is disabled or the window has closed, a proof-valid
// migration tx must be rejected at the ante — before mempool insertion — so
// zero-fee migration txs cannot flood the mempool outside the operator-defined
// window. This is one cheap defense against the zero-fee spam vector opened by
// admitting zero-signer migration txs (PR #167); per-account plausibility checks
// are pinned separately in TestVerifyMigrationProofsForAnte_CheapStateAdmission.
func TestVerifyMigrationProofsForAnte_AdmissionGate(t *testing.T) {
	legacyPriv := secp256k1.GenPrivKey()
	legacyAddr := sdk.AccAddress(legacyPriv.PubKey().Address())
	newPriv, newAddr := testNewMigrationAccount(t)

	t.Run("rejected when migration disabled", func(t *testing.T) {
		fixture := initMsgServerFixture(t)
		// EnableMigration=false; otherwise default params.
		require.NoError(t, fixture.keeper.Params.Set(fixture.ctx, types.NewParams(false, 0, 50, 2000, 20)))

		msg := newClaimMigrationMsg(t, legacyPriv, legacyAddr, newPriv, newAddr)
		err := fixture.keeper.VerifyMigrationProofsForAnte(fixture.ctx, msg)
		require.ErrorIs(t, err, types.ErrMigrationDisabled)
	})

	t.Run("rejected when window closed", func(t *testing.T) {
		fixture := initMsgServerFixture(t)
		// Window ends at unix 1000; block time is well past it.
		require.NoError(t, fixture.keeper.Params.Set(fixture.ctx, types.NewParams(true, 1000, 50, 2000, 20)))
		ctx := fixture.ctx.WithBlockTime(time.Unix(2000, 0))

		msg := newClaimMigrationMsg(t, legacyPriv, legacyAddr, newPriv, newAddr)
		err := fixture.keeper.VerifyMigrationProofsForAnte(ctx, msg)
		require.ErrorIs(t, err, types.ErrMigrationWindowClosed)
	})

	t.Run("accepted inside open window before proof checks change nothing", func(t *testing.T) {
		fixture := initMsgServerFixture(t)
		// Window ends at unix 5000; block time is before it -> gate passes,
		// valid proofs accepted.
		require.NoError(t, fixture.keeper.Params.Set(fixture.ctx, types.NewParams(true, 5000, 50, 2000, 20)))
		ctx := fixture.ctx.WithBlockTime(time.Unix(1000, 0))
		fixture.accountKeeper.EXPECT().
			GetAccount(gomock.Any(), legacyAddr).
			Return(authtypes.NewBaseAccountWithAddress(legacyAddr))

		msg := newClaimMigrationMsg(t, legacyPriv, legacyAddr, newPriv, newAddr)
		require.NoError(t, fixture.keeper.VerifyMigrationProofsForAnte(ctx, msg))
	})
}

func TestVerifyMigrationProofsForAnte_CheapStateAdmission(t *testing.T) {
	legacyPriv := secp256k1.GenPrivKey()
	legacyAddr := sdk.AccAddress(legacyPriv.PubKey().Address())
	newPriv, newAddr := testNewMigrationAccount(t)

	t.Run("rejects nonexistent legacy account before mempool admission", func(t *testing.T) {
		fixture := initMsgServerFixture(t)
		fixture.accountKeeper.EXPECT().
			GetAccount(gomock.Any(), legacyAddr).
			Return(nil)

		msg := newClaimMigrationMsg(t, legacyPriv, legacyAddr, newPriv, newAddr)
		err := fixture.keeper.VerifyMigrationProofsForAnte(fixture.ctx, msg)
		require.ErrorIs(t, err, types.ErrLegacyAccountNotFound)
	})

	t.Run("rejects already migrated legacy account", func(t *testing.T) {
		fixture := initMsgServerFixture(t)
		require.NoError(t, fixture.keeper.MigrationRecords.Set(fixture.ctx, legacyAddr.String(), types.MigrationRecord{
			LegacyAddress: legacyAddr.String(),
			NewAddress:    newAddr.String(),
		}))

		msg := newClaimMigrationMsg(t, legacyPriv, legacyAddr, newPriv, newAddr)
		err := fixture.keeper.VerifyMigrationProofsForAnte(fixture.ctx, msg)
		require.ErrorIs(t, err, types.ErrAlreadyMigrated)
	})

	t.Run("rejects reused migration destination", func(t *testing.T) {
		fixture := initMsgServerFixture(t)
		otherLegacy := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
		require.NoError(t, fixture.keeper.MigrationRecordByNewAddress.Set(fixture.ctx, newAddr.String(), otherLegacy.String()))

		msg := newClaimMigrationMsg(t, legacyPriv, legacyAddr, newPriv, newAddr)
		err := fixture.keeper.VerifyMigrationProofsForAnte(fixture.ctx, msg)
		require.ErrorIs(t, err, types.ErrNewAddressAlreadyUsed)
	})

	t.Run("rejects validator migration for non-validator source", func(t *testing.T) {
		fixture := initMsgServerFixture(t)
		oldValAddr := sdk.ValAddress(legacyAddr)
		fixture.accountKeeper.EXPECT().
			GetAccount(gomock.Any(), legacyAddr).
			Return(authtypes.NewBaseAccountWithAddress(legacyAddr))
		fixture.stakingKeeper.EXPECT().
			GetValidator(gomock.Any(), oldValAddr).
			Return(stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound)

		msg := newValidatorMigrationMsg(t, legacyPriv, legacyAddr, newPriv, newAddr)
		err := fixture.keeper.VerifyMigrationProofsForAnte(fixture.ctx, msg)
		require.ErrorIs(t, err, types.ErrNotValidator)
	})
}
