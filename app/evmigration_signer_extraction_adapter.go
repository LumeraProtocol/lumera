package app

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"

	lumante "github.com/LumeraProtocol/lumera/ante"
	evmigrationtypes "github.com/LumeraProtocol/lumera/x/evmigration/types"
)

// evmigrationSignerExtractionAdapter is a SignerExtractionAdapter that
// understands EVM-migration transactions.
//
// Migration messages (MsgClaimLegacyAccount, MsgMigrateValidator) are
// authenticated by the proof bytes embedded in the message — they declare
// zero envelope signers. The Cosmos SDK mempool's default
// DefaultSignerExtractionAdapter calls tx.GetSignaturesV2() and refuses any
// tx whose signature set is empty (priority_nonce.go: "tx must have at
// least one signer"). That refusal prevents valid migration txs from being
// admitted to the app-side mempool or selected for proposals, even though
// the migration ante decorators authenticate them by proof.
//
// For migration-only txs we synthesize a SignerData from the message's
// legacy_address: that string is a deterministic, on-chain canonical bytes
// representation of the source account, which is what the nonce mempool
// needs for (sender, nonce) ordering and dedupe — exactly the role normally
// served by the envelope signer. Sequence is held at 0 because migration is
// a one-shot, replay-prevented-by-keeper operation; the nonce mempool's
// dedup-by-sender path will still reject a duplicate insert in the same
// block window, which is the correct mempool semantics.
//
// All non-migration txs fall through to the supplied fallback unchanged
// (typically the SDK default adapter or, when wrapped by the EVM proposal
// handler, the EVM-aware adapter).
type evmigrationSignerExtractionAdapter struct {
	fallback sdkmempool.SignerExtractionAdapter
}

var _ sdkmempool.SignerExtractionAdapter = evmigrationSignerExtractionAdapter{}

// newEVMigrationSignerExtractionAdapter constructs an adapter that returns a
// synthetic signer for migration-only txs and delegates everything else to
// fallback.
func newEVMigrationSignerExtractionAdapter(fallback sdkmempool.SignerExtractionAdapter) evmigrationSignerExtractionAdapter {
	if fallback == nil {
		fallback = sdkmempool.NewDefaultSignerExtractionAdapter()
	}
	return evmigrationSignerExtractionAdapter{fallback: fallback}
}

// GetSigners implements sdkmempool.SignerExtractionAdapter.
func (s evmigrationSignerExtractionAdapter) GetSigners(tx sdk.Tx) ([]sdkmempool.SignerData, error) {
	if !lumante.IsEVMigrationOnlyTx(tx) {
		return s.fallback.GetSigners(tx)
	}

	msgs := tx.GetMsgs()
	if len(msgs) == 0 {
		// Defensive: IsEVMigrationOnlyTx already returns false for empty
		// msg sets, but keep the invariant local.
		return s.fallback.GetSigners(tx)
	}
	if len(msgs) != 1 {
		return nil, fmt.Errorf("evmigration tx must contain exactly one migration message for mempool signer derivation, got %d", len(msgs))
	}

	// submit-proof produces a single-message tx. Keep the mempool identity
	// equally narrow: one migration operation, one legacy_address bucket.
	legacyAddr, err := legacyAddressOfMigrationMsg(msgs[0])
	if err != nil {
		return nil, err
	}
	if legacyAddr == "" {
		return nil, fmt.Errorf("evmigration tx has empty legacy_address; cannot derive mempool signer")
	}

	acc, err := sdk.AccAddressFromBech32(legacyAddr)
	if err != nil {
		return nil, fmt.Errorf("evmigration tx legacy_address %q is not a valid bech32: %w", legacyAddr, err)
	}

	return []sdkmempool.SignerData{
		sdkmempool.NewSignerData(acc, 0),
	}, nil
}

// legacyAddressOfMigrationMsg extracts the legacy_address from a recognized
// migration message. Returns ("", nil) only for unrecognized message types,
// which IsEVMigrationOnlyTx should have already rejected upstream.
func legacyAddressOfMigrationMsg(msg sdk.Msg) (string, error) {
	switch m := msg.(type) {
	case *evmigrationtypes.MsgClaimLegacyAccount:
		return m.LegacyAddress, nil
	case *evmigrationtypes.MsgMigrateValidator:
		return m.LegacyAddress, nil
	default:
		return "", fmt.Errorf("evmigration signer adapter: unexpected message type %T in migration-only tx", msg)
	}
}
