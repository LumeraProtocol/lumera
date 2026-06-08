package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	lcfg "github.com/LumeraProtocol/lumera/config"
	"github.com/LumeraProtocol/lumera/x/evmigration/types"
	"github.com/LumeraProtocol/lumera/x/evmigration/types/sigverify"
)

// VerifyMigrationProofsForAnte performs the same proof checks as msg execution
// before fee-free, unsigned migration txs are admitted to the mempool.
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
