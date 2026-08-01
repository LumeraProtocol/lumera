package keeper

import (
	"math/big"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"
)

// TestProtoCloneOnGovDepositPanics is the RED test for the defect this file's
// cloneGovDeposit helper exists to fix.
//
// DEFECT (found on a mainnet-shaped devnet, 2026-07-31):
// buildGovernancePlan called proto.Clone on a govv1.Deposit. Deposit.Amount is
// []sdk.Coin, whose Amount is an sdkmath.Int wrapping *big.Int. gogoproto's
// reflection-based table merge descends into big.Int's unexported
// `abs []big.Word` slice, finds no registered merger for big.Word, and panics:
//
//	panic: recovered: merger not found for type:big.Word
//	  gogoproto/proto.(*mergeInfo).computeMergeInfo
//	  x/gov/types/v1.(*Deposit).XXX_Merge
//	  gogoproto/proto.Clone
//	  keeper.buildGovernancePlan
//
// Operator-visible symptom: a legacy account holding an ACTIVE governance
// deposit could not migrate, failing with an opaque "merger not found" message
// that names neither governance nor the deposit. The tx aborts so no state is
// corrupted, but the account stays unmigratable while the deposit exists.
//
// This test pins the upstream behaviour so nobody "simplifies" cloneGovDeposit
// back into proto.Clone.
func TestProtoCloneOnGovDepositPanics(t *testing.T) {
	dep := govv1.Deposit{
		ProposalId: 3,
		Depositor:  "lumera1gm0f4jhgygj9j4w685x4plfgkckevsqps08s3w",
		Amount:     sdk.NewCoins(sdk.NewCoin("ulume", sdkmath.NewInt(2000000000))),
	}

	require.Panics(t, func() {
		_ = proto.Clone(&dep)
	}, "proto.Clone on a gov Deposit with non-zero Coins must still panic; "+
		"if this ever stops panicking upstream, cloneGovDeposit may be simplified")
}

// TestCloneGovDepositIsCorrectDeepCopy proves the replacement both avoids the
// panic AND produces an independent copy — a shallow copy would let a later
// mutation of the result leak back into the source deposit, silently corrupting
// the migration plan's "source" record used for rollback/verification.
func TestCloneGovDepositIsCorrectDeepCopy(t *testing.T) {
	src := govv1.Deposit{
		ProposalId: 7,
		Depositor:  "lumera1gm0f4jhgygj9j4w685x4plfgkckevsqps08s3w",
		Amount: sdk.NewCoins(
			sdk.NewCoin("ulume", sdkmath.NewInt(2000000000)),
			sdk.NewCoin("uatom", sdkmath.NewInt(42)),
		),
	}

	var out govv1.Deposit
	require.NotPanics(t, func() {
		out = cloneGovDeposit(src)
	}, "cloneGovDeposit must not panic where proto.Clone does")

	require.Equal(t, src.ProposalId, out.ProposalId)
	require.Equal(t, src.Depositor, out.Depositor)
	require.True(t, sdk.Coins(out.Amount).Equal(sdk.Coins(src.Amount)),
		"cloned coins must be value-equal")
	require.True(t, sdk.Coins(out.Amount).IsValid(),
		"cloned coins must remain a valid, sorted Coins set")

	// Independence: mutating the clone must not touch the source.
	out.Depositor = "lumera1changed"
	out.Amount[0] = sdk.NewCoin(out.Amount[0].Denom, sdkmath.NewInt(1))
	require.Equal(t, "lumera1gm0f4jhgygj9j4w685x4plfgkckevsqps08s3w", src.Depositor,
		"mutating the clone must not change the source depositor")
	require.True(t, sdk.Coins(src.Amount).Equal(sdk.NewCoins(
		sdk.NewCoin("ulume", sdkmath.NewInt(2000000000)),
		sdk.NewCoin("uatom", sdkmath.NewInt(42)),
	)), "mutating the clone must not change the source coins")

	// The backing arrays must be distinct.
	if len(src.Amount) > 0 && len(out.Amount) > 0 {
		require.NotSame(t, &src.Amount[0], &out.Amount[0],
			"clone must not share the Coins backing array with the source")
	}
}

// TestCloneGovDepositEdgeCases covers the shapes a real chain will hand us:
// an empty deposit, a nil Amount, and a very large Int (multi-word big.Int,
// which is precisely what makes the reflective merge walk big.Word at all).
func TestCloneGovDepositEdgeCases(t *testing.T) {
	t.Run("nil amount", func(t *testing.T) {
		out := cloneGovDeposit(govv1.Deposit{ProposalId: 1, Depositor: "a"})
		require.Nil(t, out.Amount)
		require.EqualValues(t, 1, out.ProposalId)
	})

	t.Run("empty amount slice", func(t *testing.T) {
		out := cloneGovDeposit(govv1.Deposit{
			ProposalId: 2, Depositor: "b", Amount: []sdk.Coin{},
		})
		require.NotNil(t, out.Amount)
		require.Len(t, out.Amount, 0)
	})

	t.Run("multi-word big.Int amount", func(t *testing.T) {
		// 2^200 needs several big.Words — the exact case that trips the
		// reflective merge path.
		huge := sdkmath.NewIntFromBigInt(new(big.Int).Lsh(big.NewInt(1), 200))
		src := govv1.Deposit{
			ProposalId: 9,
			Depositor:  "c",
			Amount:     []sdk.Coin{{Denom: "ulume", Amount: huge}},
		}
		var out govv1.Deposit
		require.NotPanics(t, func() { out = cloneGovDeposit(src) })
		require.True(t, out.Amount[0].Amount.Equal(huge),
			"a multi-word big.Int amount must survive the copy exactly")
	})
}
