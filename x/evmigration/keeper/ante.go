package keeper

import (
	"fmt"
	"time"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	lcfg "github.com/LumeraProtocol/lumera/config"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
	"github.com/LumeraProtocol/lumera/x/evmigration/types/sigverify"
)

// VerifyMigrationProofsForAnte gates fee-free, unsigned migration txs at the
// ante — before they are admitted to the app mempool or selected for proposals.
//
// It enforces two things:
//
//  1. The migration admission window. Migration txs carry no fee and no
//     envelope signature, so without this gate anyone could flood the mempool
//     and proposals with zero-fee migration txs at any time. Rejecting txs at
//     the ante when migration is disabled or the window has closed bounds that
//     exposure to the operator-defined migration window. (Mirrors preChecks
//     steps 1–2 in msg_server_claim_legacy.go; message execution re-checks
//     against the canonical block time, so this is a best-effort mempool
//     filter, not the authoritative gate.)
//
//  2. The same cryptographic proof checks message execution performs, so a tx
//     with invalid embedded proofs never reaches the mempool.
func (k Keeper) VerifyMigrationProofsForAnte(ctx sdk.Context, msg sdk.Msg) error {
	var kind string
	var legacyAddress string
	var newAddress string
	var legacyProof *types.MigrationProof
	var newProof *types.MigrationProof

	switch m := msg.(type) {
	case *types.MsgClaimLegacyAccount:
		kind = migrationPayloadKindClaim
		legacyAddress = m.LegacyAddress
		newAddress = m.NewAddress
		legacyProof = &m.LegacyProof
		newProof = &m.NewProof
	case *types.MsgMigrateValidator:
		kind = migrationPayloadKindValidator
		legacyAddress = m.LegacyAddress
		newAddress = m.NewAddress
		legacyProof = &m.LegacyProof
		newProof = &m.NewProof
	default:
		return fmt.Errorf("unsupported evmigration ante message type %T", msg)
	}

	legacyAddr, err := sdk.AccAddressFromBech32(legacyAddress)
	if err != nil {
		return err
	}
	newAddr, err := sdk.AccAddressFromBech32(newAddress)
	if err != nil {
		return err
	}

	params, err := k.Params.Get(ctx)
	if err != nil {
		return err
	}

	// Admission gate: keep zero-fee, zero-signature migration txs out of the
	// mempool once migration is switched off or the window has closed.
	if !params.EnableMigration {
		return types.ErrMigrationDisabled
	}
	if params.MigrationEndTime > 0 && ctx.BlockTime().After(time.Unix(params.MigrationEndTime, 0)) {
		return types.ErrMigrationWindowClosed
	}
	if err := k.verifyMigrationAdmissionState(ctx, msg, legacyAddr, newAddr); err != nil {
		return err
	}

	if err := legacyProof.ValidateParams(params.MaxMultisigSubKeys); err != nil {
		return err
	}
	if err := newProof.ValidateParams(params.MaxMultisigSubKeys); err != nil {
		return err
	}
	if err := types.ValidateProofPair(legacyProof, newProof); err != nil {
		return err
	}
	if err := VerifyMigrationProof(
		ctx.ChainID(), lcfg.EVMChainID, kind,
		legacyAddr, newAddr, legacyAddr,
		legacyProof, sigverify.SubKeyTypeCosmosSecp256k1,
	); err != nil {
		return err
	}
	return VerifyMigrationProof(
		ctx.ChainID(), lcfg.EVMChainID, kind,
		legacyAddr, newAddr, newAddr,
		newProof, sigverify.SubKeyTypeEthSecp256k1,
	)
}

func (k Keeper) verifyMigrationAdmissionState(ctx sdk.Context, msg sdk.Msg, legacyAddr, newAddr sdk.AccAddress) error {
	if legacyAddr.Equals(newAddr) {
		return types.ErrSameAddress
	}

	has, err := k.MigrationRecords.Has(ctx, legacyAddr.String())
	if err != nil {
		return err
	}
	if has {
		return types.ErrAlreadyMigrated
	}

	has, err = k.MigrationRecords.Has(ctx, newAddr.String())
	if err != nil {
		return err
	}
	if has {
		return types.ErrNewAddressWasMigrated
	}

	has, err = k.MigrationRecordByNewAddress.Has(ctx, newAddr.String())
	if err != nil {
		return err
	}
	if has {
		return types.ErrNewAddressAlreadyUsed
	}

	legacyAcc := k.accountKeeper.GetAccount(ctx, legacyAddr)
	if legacyAcc == nil {
		return types.ErrLegacyAccountNotFound
	}
	if _, ok := legacyAcc.(sdk.ModuleAccountI); ok {
		return types.ErrCannotMigrateModuleAccount
	}

	if _, ok := msg.(*types.MsgMigrateValidator); !ok {
		return nil
	}

	_, err = k.stakingKeeper.GetValidator(ctx, sdk.ValAddress(legacyAddr))
	switch {
	case err == nil:
		return nil
	case errorsmod.IsOf(err, stakingtypes.ErrNoValidatorFound):
		return types.ErrNotValidator
	default:
		return fmt.Errorf("lookup source validator: %w", err)
	}
}
