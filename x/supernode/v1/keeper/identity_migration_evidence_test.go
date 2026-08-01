package keeper

import (
	"bytes"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// G2 de-vacuuming.
//
// The devnet fixture reports evidence=0 and has_metrics=false on every
// supernode, so a devnet-level "evidence preserved" assertion passes only
// because there is nothing to preserve. The existing keeper fixture
// (rawTestSuperNode) also carries NO evidence, so that gap exists at unit level
// too: nothing anywhere proved evidence survives an identity migration.
//
// Evidence is an EMBEDDED field on the SuperNode record
// (`repeated Evidence evidence = 3` in super_node.proto), NOT a separate store
// key. Critically, ApplyIdentityMigrationPlan treats the primary record as
// VALIDATION-ONLY and deliberately does not move it - the existing test states
// this explicitly:
//
//	// Primary/account/history are owned by PR196 and are validation-only here.
//	require.Equal(t, sourcePrimaryRaw, store.Get(types.GetSupernodeKey(source)))
//	require.Nil(t, store.Get(types.GetSupernodeKey(destination)))
//
// So the correct invariant for THIS layer is not "evidence moves" but
// "evidence is left byte-identical at the source, untouched": the plan must
// never silently mutate or drop embedded evidence while relocating the
// continuity state it does own (metrics, distribution, payout history).
//
// That is the property this test pins, with the non-empty evidence + metrics
// fixture the devnet and rawTestSuperNode both lack.
func TestIdentityMigrationPreservesEvidenceAndMetrics(t *testing.T) {
	k, ctx := setupKeeperForInternalTest(t)
	source, destination, account := migrationValidators()

	// Real evidence history: multiple entries, distinct reporters/types/heights,
	// so both ordering and content are observable.
	evidence := []*types.Evidence{
		{ReporterAddress: sdk.AccAddress(bytes.Repeat([]byte{0x51}, 20)).String(), EvidenceType: "storage_challenge_fail", Height: 11},
		{ReporterAddress: sdk.AccAddress(bytes.Repeat([]byte{0x52}, 20)).String(), EvidenceType: "unavailable", Height: 22},
		{ReporterAddress: sdk.AccAddress(bytes.Repeat([]byte{0x53}, 20)).String(), EvidenceType: "bad_proof", Height: 33},
	}

	sn := rawTestSuperNode(source, account)
	sn.Evidence = evidence
	store := migrationRawStore(k, ctx)
	store.Set(types.GetSupernodeKey(source), marshalRawSuperNode(t, k, sn))
	store.Set(append(bytes.Clone(types.SuperNodeByAccountKey), []byte(account)...), source)

	metrics := types.SupernodeMetricsState{
		ValidatorAddress: source.String(),
		Metrics:          &types.SupernodeMetrics{CascadeKademliaDbBytes: 424242.5, PeersCount: 9},
		ReportCount:      31,
		Height:           789,
	}
	require.NoError(t, k.SetMetricsState(ctx, metrics))

	// Sanity: the fixture is genuinely non-empty. Without this guard a passing
	// assertion below would prove nothing - the exact devnet vacuity problem.
	require.Len(t, sn.Evidence, 3, "fixture must carry real evidence")
	preMetrics, found := k.GetMetricsState(ctx, source)
	require.True(t, found)
	require.NotNil(t, preMetrics.Metrics)

	plan, err := k.BuildIdentityMigrationPlan(ctx, source, destination)
	require.NoError(t, err)
	require.NoError(t, k.ApplyIdentityMigrationPlan(ctx, plan))

	// EVIDENCE: the primary record is validation-only at this layer, so evidence
	// must remain byte-identical AT THE SOURCE - never mutated, reordered or
	// dropped as a side effect of relocating the continuity state.
	srcRaw := store.Get(types.GetSupernodeKey(source))
	require.NotNil(t, srcRaw, "source supernode record must still exist")
	var kept types.SuperNode
	require.NoError(t, k.cdc.Unmarshal(srcRaw, &kept))

	require.Len(t, kept.Evidence, len(evidence),
		"evidence count must not change when continuity state is migrated")
	for i, want := range evidence {
		require.Equal(t, want.ReporterAddress, kept.Evidence[i].ReporterAddress,
			"evidence[%d] reporter must be unchanged", i)
		require.Equal(t, want.EvidenceType, kept.Evidence[i].EvidenceType,
			"evidence[%d] type must be unchanged", i)
		require.Equal(t, want.Height, kept.Evidence[i].Height,
			"evidence[%d] height must not be rewritten", i)
	}

	// No supernode record may be conjured at the destination by this layer.
	require.Nil(t, store.Get(types.GetSupernodeKey(destination)),
		"primary record is validation-only here and must not be written")

	// METRICS must follow the identity, exactly.
	movedMetrics, found := k.GetMetricsState(ctx, destination)
	require.True(t, found, "metrics must follow the migrated identity")
	require.NotNil(t, movedMetrics.Metrics)
	require.EqualValues(t, 424242.5, movedMetrics.Metrics.CascadeKademliaDbBytes)
	require.EqualValues(t, 9, movedMetrics.Metrics.PeersCount)
	require.EqualValues(t, 31, movedMetrics.ReportCount,
		"report count must be preserved - it feeds audit/payout accounting")

	// And must NOT remain at the old identity: duplicated metrics would
	// double-count in any downstream aggregation.
	_, stillAtSource := k.GetMetricsState(ctx, source)
	require.False(t, stillAtSource,
		"metrics must not remain under the source identity")
}
