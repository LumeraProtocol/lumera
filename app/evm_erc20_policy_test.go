package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	erc20policytypes "github.com/LumeraProtocol/lumera/x/erc20policy/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	"github.com/cosmos/ibc-go/v10/modules/core/exported"
)

// ---------------------------------------------------------------------------
// Mock inner keeper — satisfies erc20KeeperWithDenomCheck
// ---------------------------------------------------------------------------

// mockErc20Keeper records calls and returns a configurable ack.
type mockErc20Keeper struct {
	onRecvCalled     bool
	onAckCalled      bool
	onTimeoutCalled  bool
	registeredDenoms map[string]bool
	returnAck        exported.Acknowledgement
}

var _ erc20KeeperWithDenomCheck = (*mockErc20Keeper)(nil)

func newMockErc20Keeper() *mockErc20Keeper {
	return &mockErc20Keeper{
		registeredDenoms: make(map[string]bool),
		returnAck:        channeltypes.NewResultAcknowledgement([]byte("ok")),
	}
}

func (m *mockErc20Keeper) OnRecvPacket(_ sdk.Context, _ channeltypes.Packet, _ exported.Acknowledgement) exported.Acknowledgement {
	m.onRecvCalled = true
	return m.returnAck
}

func (m *mockErc20Keeper) OnAcknowledgementPacket(_ sdk.Context, _ channeltypes.Packet, _ transfertypes.FungibleTokenPacketData, _ channeltypes.Acknowledgement) error {
	m.onAckCalled = true
	return nil
}

func (m *mockErc20Keeper) OnTimeoutPacket(_ sdk.Context, _ channeltypes.Packet, _ transfertypes.FungibleTokenPacketData) error {
	m.onTimeoutCalled = true
	return nil
}

func (m *mockErc20Keeper) Logger(_ sdk.Context) log.Logger {
	return log.NewNopLogger()
}

func (m *mockErc20Keeper) IsDenomRegistered(_ sdk.Context, denom string) bool {
	return m.registeredDenoms[denom]
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// makePolicyTestCtx creates an in-memory store and SDK context for policy tests.
func makePolicyTestCtx(t *testing.T) (sdk.Context, *storetypes.KVStoreKey) {
	t.Helper()
	storeKey := storetypes.NewKVStoreKey("erc20_test")
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	cms.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, cmtproto.Header{}, false, log.NewNopLogger())
	return ctx, storeKey
}

// testIBCPacketOpts configures a test IBC packet with full control over
// source/destination ports and channels, and the raw denom path.
type testIBCPacketOpts struct {
	denom      string // raw denom path in packet data (e.g., "uatom", "transfer/channel-5/uatom")
	amount     string
	srcPort    string
	srcChannel string
	dstPort    string
	dstChannel string
}

// makeIBCPacketWith builds a fully configurable IBC packet.
func makeIBCPacketWith(t *testing.T, opts testIBCPacketOpts) channeltypes.Packet {
	t.Helper()
	if opts.amount == "" {
		opts.amount = "1000"
	}
	if opts.srcPort == "" {
		opts.srcPort = "transfer"
	}
	if opts.srcChannel == "" {
		opts.srcChannel = "channel-0"
	}
	if opts.dstPort == "" {
		opts.dstPort = "transfer"
	}
	if opts.dstChannel == "" {
		opts.dstChannel = "channel-1"
	}

	data := transfertypes.FungibleTokenPacketData{
		Denom:    opts.denom,
		Amount:   opts.amount,
		Sender:   "cosmos1sender",
		Receiver: "cosmos1receiver",
	}
	bz, err := transfertypes.ModuleCdc.MarshalJSON(&data)
	require.NoError(t, err)
	return channeltypes.Packet{
		SourcePort:         opts.srcPort,
		SourceChannel:      opts.srcChannel,
		DestinationPort:    opts.dstPort,
		DestinationChannel: opts.dstChannel,
		Data:               bz,
		Sequence:           1,
	}
}

// makeIBCPacket builds a minimal IBC packet with default ports/channels.
// Kept for existing test compatibility.
func makeIBCPacket(t *testing.T, denom, amount string) channeltypes.Packet {
	t.Helper()
	return makeIBCPacketWith(t, testIBCPacketOpts{denom: denom, amount: amount})
}

// hop is a shorthand for creating a SourceHop.
func hop(port, channel string) *erc20policytypes.SourceHop {
	return &erc20policytypes.SourceHop{PortId: port, ChannelId: channel}
}

// ---------------------------------------------------------------------------
// Policy wrapper tests
// ---------------------------------------------------------------------------

func makePolicyWrapper(t *testing.T) (sdk.Context, *erc20PolicyKeeperWrapper, *mockErc20Keeper) {
	t.Helper()
	ctx, storeKey := makePolicyTestCtx(t)
	mock := newMockErc20Keeper()
	wrapper := newERC20PolicyKeeperWrapper(mock, storeKey)
	return ctx, wrapper, mock
}

func TestERC20Policy_DefaultModeIsAllowlist(t *testing.T) {
	ctx, wrapper, _ := makePolicyWrapper(t)
	require.Equal(t, erc20policytypes.PolicyModeAllowlist, wrapper.getRegistrationMode(ctx))
}

// The IBC denom hash for "uatom" received on dest port/channel "transfer/channel-1"
// from source "transfer/channel-0" is: ibc/C4CFF46FD6DE35CA4CF4CE031E643C8FDC9BA4B99AE598E9B0ED98FE3A2319F9
const testIBCDenom = "ibc/C4CFF46FD6DE35CA4CF4CE031E643C8FDC9BA4B99AE598E9B0ED98FE3A2319F9"

func TestERC20Policy_AllMode_DelegatesToInner(t *testing.T) {
	ctx, wrapper, mock := makePolicyWrapper(t)
	wrapper.setRegistrationMode(ctx, erc20policytypes.PolicyModeAll)

	packet := makeIBCPacket(t, "uatom", "1000")
	inputAck := channeltypes.NewResultAcknowledgement([]byte("input"))

	result := wrapper.OnRecvPacket(ctx, packet, inputAck)
	require.True(t, mock.onRecvCalled, "inner keeper should have been called in 'all' mode")
	require.Equal(t, mock.returnAck, result)
}

func TestERC20Policy_NoneMode_SkipsRegistration(t *testing.T) {
	ctx, wrapper, mock := makePolicyWrapper(t)
	wrapper.setRegistrationMode(ctx, erc20policytypes.PolicyModeNone)

	packet := makeIBCPacket(t, "uatom", "1000")
	inputAck := channeltypes.NewResultAcknowledgement([]byte("input"))

	result := wrapper.OnRecvPacket(ctx, packet, inputAck)
	require.False(t, mock.onRecvCalled, "inner keeper should NOT be called in 'none' mode for unregistered IBC denom")
	require.Equal(t, inputAck, result, "should return original ack (IBC transfer succeeds, no ERC20 registration)")
}

func TestERC20Policy_NoneMode_PassesThroughNonIBC(t *testing.T) {
	ctx, wrapper, mock := makePolicyWrapper(t)
	wrapper.setRegistrationMode(ctx, erc20policytypes.PolicyModeNone)

	// "transfer/channel-0/uatom" = token returning to our chain → received as "uatom" (not ibc/).
	packet := makeIBCPacket(t, "transfer/channel-0/uatom", "1000")
	inputAck := channeltypes.NewResultAcknowledgement([]byte("input"))

	result := wrapper.OnRecvPacket(ctx, packet, inputAck)
	require.True(t, mock.onRecvCalled, "non-IBC denoms should always pass through")
	require.Equal(t, mock.returnAck, result)
}

func TestERC20Policy_NoneMode_PassesThroughAlreadyRegistered(t *testing.T) {
	ctx, wrapper, mock := makePolicyWrapper(t)
	wrapper.setRegistrationMode(ctx, erc20policytypes.PolicyModeNone)

	mock.registeredDenoms[testIBCDenom] = true

	packet := makeIBCPacket(t, "uatom", "1000")
	inputAck := channeltypes.NewResultAcknowledgement([]byte("input"))

	result := wrapper.OnRecvPacket(ctx, packet, inputAck)
	require.True(t, mock.onRecvCalled, "already-registered IBC denoms should pass through even in 'none' mode")
	require.Equal(t, mock.returnAck, result)
}

func TestERC20Policy_AllowlistMode_BlocksUnlisted(t *testing.T) {
	ctx, wrapper, mock := makePolicyWrapper(t)
	wrapper.setRegistrationMode(ctx, erc20policytypes.PolicyModeAllowlist)

	packet := makeIBCPacket(t, "uatom", "1000")
	inputAck := channeltypes.NewResultAcknowledgement([]byte("input"))

	result := wrapper.OnRecvPacket(ctx, packet, inputAck)
	require.False(t, mock.onRecvCalled, "unlisted IBC denom should not pass through in 'allowlist' mode")
	require.Equal(t, inputAck, result)
}

func TestERC20Policy_AllowlistMode_AllowsListed(t *testing.T) {
	ctx, wrapper, mock := makePolicyWrapper(t)
	wrapper.setRegistrationMode(ctx, erc20policytypes.PolicyModeAllowlist)

	wrapper.setIBCDenomAllowed(ctx, testIBCDenom)

	packet := makeIBCPacket(t, "uatom", "1000")
	inputAck := channeltypes.NewResultAcknowledgement([]byte("input"))

	result := wrapper.OnRecvPacket(ctx, packet, inputAck)
	require.True(t, mock.onRecvCalled, "allowlisted IBC denom should pass through")
	require.Equal(t, mock.returnAck, result)
}

func TestERC20Policy_PassthroughMethods(t *testing.T) {
	ctx, wrapper, mock := makePolicyWrapper(t)

	require.NoError(t, wrapper.OnAcknowledgementPacket(ctx, channeltypes.Packet{}, transfertypes.FungibleTokenPacketData{}, channeltypes.Acknowledgement{}))
	require.True(t, mock.onAckCalled)

	require.NoError(t, wrapper.OnTimeoutPacket(ctx, channeltypes.Packet{}, transfertypes.FungibleTokenPacketData{}))
	require.True(t, mock.onTimeoutCalled)

	logger := wrapper.Logger(ctx)
	require.NotNil(t, logger)
}

func TestERC20Policy_AllowlistCRUD(t *testing.T) {
	ctx, storeKey := makePolicyTestCtx(t)
	wrapper := &erc20PolicyKeeperWrapper{storeKey: storeKey}

	denom1 := "ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2"
	denom2 := "ibc/0000000000000000000000000000000000000000000000000000000000000001"

	require.False(t, wrapper.isIBCDenomAllowed(ctx, denom1))
	require.Empty(t, wrapper.getAllowedDenoms(ctx))

	wrapper.setIBCDenomAllowed(ctx, denom1)
	require.True(t, wrapper.isIBCDenomAllowed(ctx, denom1))
	require.False(t, wrapper.isIBCDenomAllowed(ctx, denom2))
	require.Equal(t, []string{denom1}, wrapper.getAllowedDenoms(ctx))

	wrapper.setIBCDenomAllowed(ctx, denom2)
	require.True(t, wrapper.isIBCDenomAllowed(ctx, denom2))
	denoms := wrapper.getAllowedDenoms(ctx)
	require.Len(t, denoms, 2)

	wrapper.removeIBCDenomAllowed(ctx, denom1)
	require.False(t, wrapper.isIBCDenomAllowed(ctx, denom1))
	require.True(t, wrapper.isIBCDenomAllowed(ctx, denom2))
	require.Equal(t, []string{denom2}, wrapper.getAllowedDenoms(ctx))

	wrapper.removeIBCDenomAllowed(ctx, denom2)
	require.Empty(t, wrapper.getAllowedDenoms(ctx))
}

// ---------------------------------------------------------------------------
// Provenance-bound base denom trace tests
// ---------------------------------------------------------------------------

func TestERC20Policy_AllowlistMode_DirectTransferAllowed(t *testing.T) {
	ctx, wrapper, mock := makePolicyWrapper(t)
	wrapper.setRegistrationMode(ctx, erc20policytypes.PolicyModeAllowlist)

	// Allow "uatom" only via direct transfer on destination channel-1.
	wrapper.setBaseDenomTraceAllowed(ctx, "uatom", []*erc20policytypes.SourceHop{
		hop("transfer", "channel-1"),
	})

	// Direct transfer: denom "uatom" (no trace), arriving on channel-1.
	packet := makeIBCPacketWith(t, testIBCPacketOpts{
		denom:      "uatom",
		dstPort:    "transfer",
		dstChannel: "channel-1",
	})
	inputAck := channeltypes.NewResultAcknowledgement([]byte("input"))

	result := wrapper.OnRecvPacket(ctx, packet, inputAck)
	require.True(t, mock.onRecvCalled, "direct transfer matching allowed trace should pass through")
	require.Equal(t, mock.returnAck, result)
}

func TestERC20Policy_AllowlistMode_BlocksWrongChannel(t *testing.T) {
	ctx, wrapper, mock := makePolicyWrapper(t)
	wrapper.setRegistrationMode(ctx, erc20policytypes.PolicyModeAllowlist)

	// Allow "uatom" only via channel-1.
	wrapper.setBaseDenomTraceAllowed(ctx, "uatom", []*erc20policytypes.SourceHop{
		hop("transfer", "channel-1"),
	})

	// Token arrives on channel-2 instead.
	packet := makeIBCPacketWith(t, testIBCPacketOpts{
		denom:      "uatom",
		dstPort:    "transfer",
		dstChannel: "channel-2",
	})
	inputAck := channeltypes.NewResultAcknowledgement([]byte("input"))

	result := wrapper.OnRecvPacket(ctx, packet, inputAck)
	require.False(t, mock.onRecvCalled, "wrong destination channel should be blocked")
	require.Equal(t, inputAck, result)
}

func TestERC20Policy_AllowlistMode_BlocksMultiHopOnSameChannel(t *testing.T) {
	ctx, wrapper, mock := makePolicyWrapper(t)
	wrapper.setRegistrationMode(ctx, erc20policytypes.PolicyModeAllowlist)

	// Allow "uatom" only via direct single-hop on channel-1.
	wrapper.setBaseDenomTraceAllowed(ctx, "uatom", []*erc20policytypes.SourceHop{
		hop("transfer", "channel-1"),
	})

	// Multi-hop uatom arriving on channel-1: the packet denom has an extra trace hop
	// from an intermediate chain (e.g., "transfer/channel-5/uatom").
	// Full received trace becomes [{transfer, channel-1}, {transfer, channel-5}] — 2 hops.
	packet := makeIBCPacketWith(t, testIBCPacketOpts{
		denom:      "transfer/channel-5/uatom",
		dstPort:    "transfer",
		dstChannel: "channel-1",
	})
	inputAck := channeltypes.NewResultAcknowledgement([]byte("input"))

	result := wrapper.OnRecvPacket(ctx, packet, inputAck)
	require.False(t, mock.onRecvCalled, "multi-hop uatom on same channel should be blocked by single-hop trace restriction")
	require.Equal(t, inputAck, result)
}

func TestERC20Policy_AllowlistMode_MultiHopTraceAllowed(t *testing.T) {
	ctx, wrapper, mock := makePolicyWrapper(t)
	wrapper.setRegistrationMode(ctx, erc20policytypes.PolicyModeAllowlist)

	// Allow "uatom" via a specific 2-hop path: Lumera channel-1 → intermediate channel-5.
	wrapper.setBaseDenomTraceAllowed(ctx, "uatom", []*erc20policytypes.SourceHop{
		hop("transfer", "channel-1"),
		hop("transfer", "channel-5"),
	})

	// Matching multi-hop packet.
	packet := makeIBCPacketWith(t, testIBCPacketOpts{
		denom:      "transfer/channel-5/uatom",
		dstPort:    "transfer",
		dstChannel: "channel-1",
	})
	inputAck := channeltypes.NewResultAcknowledgement([]byte("input"))

	result := wrapper.OnRecvPacket(ctx, packet, inputAck)
	require.True(t, mock.onRecvCalled, "multi-hop uatom matching 2-hop trace should pass through")
	require.Equal(t, mock.returnAck, result)
}

func TestERC20Policy_AllowlistMode_EmptyTracePlaceholder(t *testing.T) {
	ctx, wrapper, mock := makePolicyWrapper(t)
	wrapper.setRegistrationMode(ctx, erc20policytypes.PolicyModeAllowlist)

	// Add "uatom" with empty trace (placeholder — should never match).
	wrapper.setBaseDenomTraceAllowed(ctx, "uatom", nil)

	packet := makeIBCPacketWith(t, testIBCPacketOpts{
		denom:      "uatom",
		dstPort:    "transfer",
		dstChannel: "channel-1",
	})
	inputAck := channeltypes.NewResultAcknowledgement([]byte("input"))

	result := wrapper.OnRecvPacket(ctx, packet, inputAck)
	require.False(t, mock.onRecvCalled, "empty-trace placeholder should never match a real packet")
	require.Equal(t, inputAck, result)
}

func TestERC20Policy_BaseDenomTraceCRUD(t *testing.T) {
	ctx, storeKey := makePolicyTestCtx(t)
	wrapper := &erc20PolicyKeeperWrapper{storeKey: storeKey}

	// Initially empty.
	require.Empty(t, wrapper.getAllowedBaseDenomTraces(ctx))

	// Add uatom with single-hop trace.
	trace1 := []*erc20policytypes.SourceHop{hop("transfer", "channel-0")}
	wrapper.setBaseDenomTraceAllowed(ctx, "uatom", trace1)
	require.True(t, wrapper.isBaseDenomTraceAllowed(ctx, "uatom", trace1))

	// Add uatom with a different trace (multi-hop).
	trace2 := []*erc20policytypes.SourceHop{hop("transfer", "channel-1"), hop("transfer", "channel-5")}
	wrapper.setBaseDenomTraceAllowed(ctx, "uatom", trace2)
	require.True(t, wrapper.isBaseDenomTraceAllowed(ctx, "uatom", trace2))

	// Add different denom.
	trace3 := []*erc20policytypes.SourceHop{hop("transfer", "channel-2")}
	wrapper.setBaseDenomTraceAllowed(ctx, "uosmo", trace3)

	entries := wrapper.getAllowedBaseDenomTraces(ctx)
	require.Len(t, entries, 3)

	// Remove one uatom trace.
	wrapper.removeBaseDenomTraceAllowed(ctx, "uatom", trace1)
	require.False(t, wrapper.isBaseDenomTraceAllowed(ctx, "uatom", trace1))
	require.True(t, wrapper.isBaseDenomTraceAllowed(ctx, "uatom", trace2))

	// Remove all uatom traces.
	wrapper.removeAllBaseDenomTraces(ctx, "uatom")
	require.False(t, wrapper.isBaseDenomTraceAllowed(ctx, "uatom", trace2))
	require.True(t, wrapper.isBaseDenomTraceAllowed(ctx, "uosmo", trace3), "uosmo should be unaffected")

	entries = wrapper.getAllowedBaseDenomTraces(ctx)
	require.Len(t, entries, 1)
	require.Equal(t, "uosmo", entries[0].BaseDenom)
}

func TestERC20Policy_InitDefaults(t *testing.T) {
	ctx, storeKey := makePolicyTestCtx(t)
	mock := newMockErc20Keeper()
	wrapper := newERC20PolicyKeeperWrapper(mock, storeKey)

	store := ctx.KVStore(storeKey)
	require.False(t, store.Has(policyModeKey), "mode should not be set before init")

	wrapper.setRegistrationMode(ctx, erc20policytypes.PolicyModeAllowlist)
	for _, entry := range erc20policytypes.DefaultAllowedBaseDenomTraces {
		wrapper.setBaseDenomTraceAllowed(ctx, entry.BaseDenom, entry.Trace)
	}

	require.Equal(t, erc20policytypes.PolicyModeAllowlist, wrapper.getRegistrationMode(ctx))

	entries := wrapper.getAllowedBaseDenomTraces(ctx)
	require.Len(t, entries, len(erc20policytypes.DefaultAllowedBaseDenomTraces))
	for _, entry := range entries {
		require.Empty(t, entry.Trace, "default entries should have empty traces (placeholders)")
	}

	require.True(t, store.Has(policyModeKey))
}

// ---------------------------------------------------------------------------
// Governance message handler tests
// ---------------------------------------------------------------------------

func TestERC20PolicyMsg_SetRegistrationPolicy(t *testing.T) {
	ctx, storeKey := makePolicyTestCtx(t)
	wrapper := &erc20PolicyKeeperWrapper{storeKey: storeKey}
	govAddr := authtypes.NewModuleAddress(govtypes.ModuleName)

	server := &erc20PolicyMsgServer{
		wrapper:   wrapper,
		authority: govAddr,
	}

	sdkCtx := ctx.WithContext(context.Background())

	t.Run("valid mode change to none", func(t *testing.T) {
		resp, err := server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority: govAddr.String(),
			Mode:      erc20policytypes.PolicyModeNone,
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, erc20policytypes.PolicyModeNone, wrapper.getRegistrationMode(ctx))
	})

	t.Run("valid mode change to allowlist with denoms", func(t *testing.T) {
		denom := "ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2"
		resp, err := server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority: govAddr.String(),
			Mode:      erc20policytypes.PolicyModeAllowlist,
			AddDenoms: []string{denom},
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, erc20policytypes.PolicyModeAllowlist, wrapper.getRegistrationMode(ctx))
		require.True(t, wrapper.isIBCDenomAllowed(ctx, denom))
	})

	t.Run("remove denoms", func(t *testing.T) {
		denom := "ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2"
		resp, err := server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority:    govAddr.String(),
			RemoveDenoms: []string{denom},
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.False(t, wrapper.isIBCDenomAllowed(ctx, denom))
	})

	t.Run("invalid authority", func(t *testing.T) {
		_, err := server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority: "lumera1wrongauthority00000000000000000000",
			Mode:      erc20policytypes.PolicyModeAll,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid authority")
	})

	t.Run("invalid mode", func(t *testing.T) {
		_, err := server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority: govAddr.String(),
			Mode:      "invalid",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid mode")
	})

	t.Run("empty mode does not change existing mode", func(t *testing.T) {
		wrapper.setRegistrationMode(ctx, erc20policytypes.PolicyModeAllowlist)
		resp, err := server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority: govAddr.String(),
			Mode:      "",
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, erc20policytypes.PolicyModeAllowlist, wrapper.getRegistrationMode(ctx))
	})

	t.Run("empty authority", func(t *testing.T) {
		_, err := server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority: "",
			Mode:      erc20policytypes.PolicyModeAll,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty authority")
	})

	t.Run("add and remove base denom traces", func(t *testing.T) {
		resp, err := server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority: govAddr.String(),
			AddBaseDenomTraces: []*erc20policytypes.AllowedBaseDenomTrace{
				{BaseDenom: "uatom", Trace: []*erc20policytypes.SourceHop{hop("transfer", "channel-0")}},
				{BaseDenom: "uosmo", Trace: []*erc20policytypes.SourceHop{hop("transfer", "channel-1")}},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, resp)

		traceAtom := []*erc20policytypes.SourceHop{hop("transfer", "channel-0")}
		traceOsmo := []*erc20policytypes.SourceHop{hop("transfer", "channel-1")}
		require.True(t, wrapper.isBaseDenomTraceAllowed(ctx, "uatom", traceAtom))
		require.True(t, wrapper.isBaseDenomTraceAllowed(ctx, "uosmo", traceOsmo))

		// Remove one.
		resp, err = server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority: govAddr.String(),
			RemoveBaseDenomTraces: []*erc20policytypes.AllowedBaseDenomTrace{
				{BaseDenom: "uosmo", Trace: []*erc20policytypes.SourceHop{hop("transfer", "channel-1")}},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.True(t, wrapper.isBaseDenomTraceAllowed(ctx, "uatom", traceAtom))
		require.False(t, wrapper.isBaseDenomTraceAllowed(ctx, "uosmo", traceOsmo))
	})

	t.Run("invalid base denom trace - empty denom", func(t *testing.T) {
		_, err := server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority: govAddr.String(),
			AddBaseDenomTraces: []*erc20policytypes.AllowedBaseDenomTrace{
				{BaseDenom: "", Trace: []*erc20policytypes.SourceHop{hop("transfer", "channel-0")}},
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid add_base_denom_trace")
	})

	t.Run("invalid base denom trace - empty port", func(t *testing.T) {
		_, err := server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority: govAddr.String(),
			AddBaseDenomTraces: []*erc20policytypes.AllowedBaseDenomTrace{
				{BaseDenom: "uatom", Trace: []*erc20policytypes.SourceHop{hop("", "channel-0")}},
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid port_id")
	})

	t.Run("invalid base denom trace - slash in port_id", func(t *testing.T) {
		_, err := server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority: govAddr.String(),
			AddBaseDenomTraces: []*erc20policytypes.AllowedBaseDenomTrace{
				{BaseDenom: "uatom", Trace: []*erc20policytypes.SourceHop{hop("trans/fer", "channel-0")}},
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid port_id")
	})

	t.Run("invalid base denom trace - short channel_id", func(t *testing.T) {
		_, err := server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority: govAddr.String(),
			AddBaseDenomTraces: []*erc20policytypes.AllowedBaseDenomTrace{
				{BaseDenom: "uatom", Trace: []*erc20policytypes.SourceHop{hop("transfer", "ch-0")}},
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid channel_id")
	})

	t.Run("invalid base denom trace - null byte in channel_id", func(t *testing.T) {
		_, err := server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority: govAddr.String(),
			AddBaseDenomTraces: []*erc20policytypes.AllowedBaseDenomTrace{
				{BaseDenom: "uatom", Trace: []*erc20policytypes.SourceHop{hop("transfer", "channel\x00-0")}},
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid channel_id")
	})

	t.Run("valid placeholder trace (empty hops)", func(t *testing.T) {
		resp, err := server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority: govAddr.String(),
			AddBaseDenomTraces: []*erc20policytypes.AllowedBaseDenomTrace{
				{BaseDenom: "uusdc"},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
	})
}
