package keeper

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"
)

// BUG-17: after fixing the proto.Clone panic (BUG-16), migrating a legacy account
// holding a governance deposit failed with
//
//	stale governance deposit source for proposal 2: value changed
//
// on a deposit that nothing had modified. The message came from
// verifyCollectionValue, which used proto.Equal to compare the freshly-read
// on-chain value against the plan's expectation.
//
// ROOT CAUSE (proved below): gogoproto's proto.Equal returns FALSE for two
// byte-identical gov Deposits. Deposit.Amount is []sdk.Coin whose Amount is an
// sdkmath.Int wrapping *big.Int; the same reflection machinery that outright
// panics in proto.Clone silently misreports equality here. Fail-closed, so no
// state was corrupted -- but a legitimate migration was permanently blocked and
// the error pointed at concurrent mutation that never occurred.
//
// FIX: verifyCollectionValue (and the authz/vote staleness checks) now compare
// marshalled bytes, which is the deterministic, consensus-relevant notion of
// "unchanged" and avoids reflection entirely.

// TestProtoEqualIsBrokenForGovDeposits is a CHARACTERIZATION test: it documents
// the upstream defect that motivated the fix. If gogoproto ever repairs this,
// this test fails and the byte-comparison helper can be reconsidered.
func TestProtoEqualIsBrokenForGovDeposits(t *testing.T) {
	mk := func() *govv1.Deposit {
		return &govv1.Deposit{
			ProposalId: 2,
			Depositor:  "lumera1gm0f4jhgygj9j4w685x4plfgkckevsqps08s3w",
			Amount:     sdk.NewCoins(sdk.NewCoin("ulume", sdkmath.NewInt(2000000000))),
		}
	}
	a, b := mk(), mk()

	require.False(t, proto.Equal(a, b),
		"CHARACTERIZATION: gogoproto proto.Equal is expected to WRONGLY report two "+
			"identical Coin-bearing gov Deposits as unequal. If this now passes, "+
			"upstream fixed it and protoBytesEqual may be revisited.")
}

// TestProtoBytesEqualFixesTheFalseStaleness is the real regression test: the
// replacement comparator must report identical deposits as identical, which is
// what unblocks the migration.
func TestProtoBytesEqualFixesTheFalseStaleness(t *testing.T) {
	mk := func() *govv1.Deposit {
		return &govv1.Deposit{
			ProposalId: 2,
			Depositor:  "lumera1gm0f4jhgygj9j4w685x4plfgkckevsqps08s3w",
			Amount:     sdk.NewCoins(sdk.NewCoin("ulume", sdkmath.NewInt(2000000000))),
		}
	}

	same, err := protoBytesEqual(mk(), mk())
	require.NoError(t, err)
	require.True(t, same,
		"two identical deposits must compare equal, or the staleness guard fires "+
			"on unchanged state and blocks a legitimate migration (BUG-17)")

	// The plan's real shape: expectation built via cloneGovDeposit.
	src := *mk()
	cloned := cloneGovDeposit(src)
	same, err = protoBytesEqual(&src, &cloned)
	require.NoError(t, err)
	require.True(t, same,
		"a cloneGovDeposit result must compare equal to its source")
}

// TestProtoBytesEqualDetectsRealChanges is the NEGATIVE CONTROL. A comparator
// that always returns true would also "fix" the bug while destroying the
// guard's entire purpose -- silently permitting migration over genuinely
// mutated state. Each field must be detected.
func TestProtoBytesEqualDetectsRealChanges(t *testing.T) {
	base := &govv1.Deposit{
		ProposalId: 2,
		Depositor:  "lumera1gm0f4jhgygj9j4w685x4plfgkckevsqps08s3w",
		Amount:     sdk.NewCoins(sdk.NewCoin("ulume", sdkmath.NewInt(2000000000))),
	}

	cases := map[string]*govv1.Deposit{
		"different amount": {
			ProposalId: 2, Depositor: base.Depositor,
			Amount: sdk.NewCoins(sdk.NewCoin("ulume", sdkmath.NewInt(1))),
		},
		"different depositor": {
			ProposalId: 2, Depositor: "lumera1other",
			Amount: base.Amount,
		},
		"different proposal": {
			ProposalId: 3, Depositor: base.Depositor,
			Amount: base.Amount,
		},
		"extra denom": {
			ProposalId: 2, Depositor: base.Depositor,
			Amount: sdk.NewCoins(
				sdk.NewCoin("ulume", sdkmath.NewInt(2000000000)),
				sdk.NewCoin("uatom", sdkmath.NewInt(1)),
			),
		},
		"emptied amount": {
			ProposalId: 2, Depositor: base.Depositor, Amount: nil,
		},
	}

	for name, mutated := range cases {
		t.Run(name, func(t *testing.T) {
			same, err := protoBytesEqual(base, mutated)
			require.NoError(t, err)
			require.False(t, same,
				"a genuinely changed deposit MUST be detected as changed; the guard "+
					"must still fail closed on real mutation")
		})
	}
}

// TestIsNilMessageCatchesTypedNil is the regression test for the third defect in
// this guard family (BUG-19).
//
// verifyOptionalCollectionValue took `expected proto.Message` and checked
// `expected == nil` to mean "the plan saw no destination entry". But callers pass
// a CONCRETE typed pointer:
//
//	var destination *govv1.Deposit          // nil
//	verifyOptionalCollectionValue(ctx, m, key, destination)
//
// A nil *govv1.Deposit wrapped in a proto.Message interface is NOT == nil, so the
// "absent is fine" branch never executed. The guard fell through to the
// value-comparison path, the store lookup returned ErrNotFound, and migration
// failed with:
//
//	stale governance deposit destination for proposal 2:
//	  collections: not found: key '("2","lumera1k7del...")' of type ...gov.v1.Deposit
//
// It demanded the destination exist, then failed because it did not — blocking
// every legacy account whose target address had no pre-existing deposit, i.e. the
// normal case.
func TestIsNilMessageCatchesTypedNil(t *testing.T) {
	t.Run("bare nil interface", func(t *testing.T) {
		require.True(t, isNilMessage(nil))
	})

	t.Run("typed nil pointer - the actual bug", func(t *testing.T) {
		// The trap: a nil typed pointer boxed in an interface. Go's `== nil` is
		// FALSE here because the interface carries a type — which is exactly what
		// the old `expected == nil` guard tested, and why it never fired.
		//
		// nilInterface() returns through a proto.Message return type so staticcheck
		// cannot statically resolve the concrete type (SA4023) — the dynamic
		// behaviour is the whole point of the test.
		asMessage := nilDepositMessage()

		require.False(t, asMessage == nil,
			"CHARACTERIZATION: a nil *Deposit boxed in proto.Message is NOT == nil; "+
				"this is precisely why the old `expected == nil` check never fired")

		require.True(t, isNilMessage(asMessage),
			"isNilMessage must detect a typed-nil pointer, or an absent optional "+
				"destination is misread as a hard error (BUG-19)")
	})

	t.Run("non-nil message is not nil", func(t *testing.T) {
		require.False(t, isNilMessage(&govv1.Deposit{ProposalId: 1}),
			"a real message must not be treated as absent, or the guard would skip "+
				"verification entirely")
	})

	t.Run("typed nil vote pointer", func(t *testing.T) {
		require.True(t, isNilMessage(nilVoteMessage()),
			"the same trap applies to the vote destination check")
	})
}

// nilDepositMessage returns a nil *govv1.Deposit as a proto.Message — the exact
// shape callers pass for an absent optional destination.
func nilDepositMessage() proto.Message {
	var d *govv1.Deposit
	return d
}

// nilVoteMessage is the vote equivalent of nilDepositMessage.
func nilVoteMessage() proto.Message {
	var v *govv1.Vote
	return v
}

// TestProtoBytesEqualNilVsEmptyCoins pins the nil-vs-empty decision. sdk.NewCoins()
// yields an empty non-nil slice while a decoded Deposit with no coins may carry
// nil. Both marshal to the same bytes (an absent repeated field), so they compare
// equal -- which is correct: they are semantically identical, and treating them as
// different would reintroduce a false staleness positive.
func TestProtoBytesEqualNilVsEmptyCoins(t *testing.T) {
	nilCoins := &govv1.Deposit{ProposalId: 2, Depositor: "a", Amount: nil}
	emptyCoins := &govv1.Deposit{ProposalId: 2, Depositor: "a", Amount: []sdk.Coin{}}

	same, err := protoBytesEqual(nilCoins, emptyCoins)
	require.NoError(t, err)
	require.True(t, same,
		"nil and empty Coins are semantically identical and must not trip the guard")

	// cloneGovDeposit must not silently convert one into the other either.
	require.Nil(t, cloneGovDeposit(*nilCoins).Amount)
	require.NotNil(t, cloneGovDeposit(*emptyCoins).Amount)
}
