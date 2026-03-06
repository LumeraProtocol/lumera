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

// makeIBCPacket builds a minimal IBC packet with the given denom (as FungibleTokenPacketData).
func makeIBCPacket(t *testing.T, denom, amount string) channeltypes.Packet {
	t.Helper()
	data := transfertypes.FungibleTokenPacketData{
		Denom:    denom,
		Amount:   amount,
		Sender:   "cosmos1sender",
		Receiver: "cosmos1receiver",
	}
	bz, err := transfertypes.ModuleCdc.MarshalJSON(&data)
	require.NoError(t, err)
	return channeltypes.Packet{
		SourcePort:         "transfer",
		SourceChannel:      "channel-0",
		DestinationPort:    "transfer",
		DestinationChannel: "channel-1",
		Data:               bz,
		Sequence:           1,
	}
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
	require.Equal(t, PolicyModeAllowlist, wrapper.getRegistrationMode(ctx))
}

// The IBC denom hash for "uatom" received on dest port/channel "transfer/channel-1"
// from source "transfer/channel-0" is: ibc/C4CFF46FD6DE35CA4CF4CE031E643C8FDC9BA4B99AE598E9B0ED98FE3A2319F9
const testIBCDenom = "ibc/C4CFF46FD6DE35CA4CF4CE031E643C8FDC9BA4B99AE598E9B0ED98FE3A2319F9"

func TestERC20Policy_AllMode_DelegatesToInner(t *testing.T) {
	ctx, wrapper, mock := makePolicyWrapper(t)
	wrapper.setRegistrationMode(ctx, PolicyModeAll)

	// "uatom" as packet denom = foreign token, will become ibc/... on our chain.
	packet := makeIBCPacket(t, "uatom", "1000")
	inputAck := channeltypes.NewResultAcknowledgement([]byte("input"))

	result := wrapper.OnRecvPacket(ctx, packet, inputAck)
	require.True(t, mock.onRecvCalled, "inner keeper should have been called in 'all' mode")
	require.Equal(t, mock.returnAck, result)
}

func TestERC20Policy_NoneMode_SkipsRegistration(t *testing.T) {
	ctx, wrapper, mock := makePolicyWrapper(t)
	wrapper.setRegistrationMode(ctx, PolicyModeNone)

	// Foreign token → ibc/ denom, not yet registered.
	packet := makeIBCPacket(t, "uatom", "1000")
	inputAck := channeltypes.NewResultAcknowledgement([]byte("input"))

	result := wrapper.OnRecvPacket(ctx, packet, inputAck)
	require.False(t, mock.onRecvCalled, "inner keeper should NOT be called in 'none' mode for unregistered IBC denom")
	require.Equal(t, inputAck, result, "should return original ack (IBC transfer succeeds, no ERC20 registration)")
}

func TestERC20Policy_NoneMode_PassesThroughNonIBC(t *testing.T) {
	ctx, wrapper, mock := makePolicyWrapper(t)
	wrapper.setRegistrationMode(ctx, PolicyModeNone)

	// "transfer/channel-0/uatom" = token returning to our chain → received as "uatom" (not ibc/).
	packet := makeIBCPacket(t, "transfer/channel-0/uatom", "1000")
	inputAck := channeltypes.NewResultAcknowledgement([]byte("input"))

	result := wrapper.OnRecvPacket(ctx, packet, inputAck)
	require.True(t, mock.onRecvCalled, "non-IBC denoms should always pass through")
	require.Equal(t, mock.returnAck, result)
}

func TestERC20Policy_NoneMode_PassesThroughAlreadyRegistered(t *testing.T) {
	ctx, wrapper, mock := makePolicyWrapper(t)
	wrapper.setRegistrationMode(ctx, PolicyModeNone)

	// Pre-register the IBC denom in the mock.
	mock.registeredDenoms[testIBCDenom] = true

	packet := makeIBCPacket(t, "uatom", "1000")
	inputAck := channeltypes.NewResultAcknowledgement([]byte("input"))

	result := wrapper.OnRecvPacket(ctx, packet, inputAck)
	require.True(t, mock.onRecvCalled, "already-registered IBC denoms should pass through even in 'none' mode")
	require.Equal(t, mock.returnAck, result)
}

func TestERC20Policy_AllowlistMode_BlocksUnlisted(t *testing.T) {
	ctx, wrapper, mock := makePolicyWrapper(t)
	wrapper.setRegistrationMode(ctx, PolicyModeAllowlist)

	packet := makeIBCPacket(t, "uatom", "1000")
	inputAck := channeltypes.NewResultAcknowledgement([]byte("input"))

	result := wrapper.OnRecvPacket(ctx, packet, inputAck)
	require.False(t, mock.onRecvCalled, "unlisted IBC denom should not pass through in 'allowlist' mode")
	require.Equal(t, inputAck, result)
}

func TestERC20Policy_AllowlistMode_AllowsListed(t *testing.T) {
	ctx, wrapper, mock := makePolicyWrapper(t)
	wrapper.setRegistrationMode(ctx, PolicyModeAllowlist)

	// Add the IBC denom to the allowlist.
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

	// Initially empty.
	require.False(t, wrapper.isIBCDenomAllowed(ctx, denom1))
	require.Empty(t, wrapper.getAllowedDenoms(ctx))

	// Add denom1.
	wrapper.setIBCDenomAllowed(ctx, denom1)
	require.True(t, wrapper.isIBCDenomAllowed(ctx, denom1))
	require.False(t, wrapper.isIBCDenomAllowed(ctx, denom2))
	require.Equal(t, []string{denom1}, wrapper.getAllowedDenoms(ctx))

	// Add denom2.
	wrapper.setIBCDenomAllowed(ctx, denom2)
	require.True(t, wrapper.isIBCDenomAllowed(ctx, denom2))
	denoms := wrapper.getAllowedDenoms(ctx)
	require.Len(t, denoms, 2)

	// Remove denom1.
	wrapper.removeIBCDenomAllowed(ctx, denom1)
	require.False(t, wrapper.isIBCDenomAllowed(ctx, denom1))
	require.True(t, wrapper.isIBCDenomAllowed(ctx, denom2))
	require.Equal(t, []string{denom2}, wrapper.getAllowedDenoms(ctx))

	// Remove denom2.
	wrapper.removeIBCDenomAllowed(ctx, denom2)
	require.Empty(t, wrapper.getAllowedDenoms(ctx))
}

func TestERC20Policy_AllowlistMode_AllowsBaseDenom(t *testing.T) {
	ctx, wrapper, mock := makePolicyWrapper(t)
	wrapper.setRegistrationMode(ctx, PolicyModeAllowlist)

	// Add "uatom" as an allowed base denom (channel-independent).
	wrapper.setBaseDenomAllowed(ctx, "uatom")

	// "uatom" arriving from any channel should now be allowed.
	packet := makeIBCPacket(t, "uatom", "1000")
	inputAck := channeltypes.NewResultAcknowledgement([]byte("input"))

	result := wrapper.OnRecvPacket(ctx, packet, inputAck)
	require.True(t, mock.onRecvCalled, "base-denom-allowlisted token should pass through")
	require.Equal(t, mock.returnAck, result)
}

func TestERC20Policy_AllowlistMode_BlocksUnlistedBaseDenom(t *testing.T) {
	ctx, wrapper, mock := makePolicyWrapper(t)
	wrapper.setRegistrationMode(ctx, PolicyModeAllowlist)

	// Only allow "uosmo", not "uatom".
	wrapper.setBaseDenomAllowed(ctx, "uosmo")

	packet := makeIBCPacket(t, "uatom", "1000")
	inputAck := channeltypes.NewResultAcknowledgement([]byte("input"))

	result := wrapper.OnRecvPacket(ctx, packet, inputAck)
	require.False(t, mock.onRecvCalled, "token with unlisted base denom should be blocked")
	require.Equal(t, inputAck, result)
}

func TestERC20Policy_BaseDenomCRUD(t *testing.T) {
	ctx, storeKey := makePolicyTestCtx(t)
	wrapper := &erc20PolicyKeeperWrapper{storeKey: storeKey}

	require.False(t, wrapper.isBaseDenomAllowed(ctx, "uatom"))
	require.Empty(t, wrapper.getAllowedBaseDenoms(ctx))

	wrapper.setBaseDenomAllowed(ctx, "uatom")
	wrapper.setBaseDenomAllowed(ctx, "uosmo")
	require.True(t, wrapper.isBaseDenomAllowed(ctx, "uatom"))
	require.True(t, wrapper.isBaseDenomAllowed(ctx, "uosmo"))
	require.Len(t, wrapper.getAllowedBaseDenoms(ctx), 2)

	wrapper.removeBaseDenomAllowed(ctx, "uatom")
	require.False(t, wrapper.isBaseDenomAllowed(ctx, "uatom"))
	require.True(t, wrapper.isBaseDenomAllowed(ctx, "uosmo"))
	require.Equal(t, []string{"uosmo"}, wrapper.getAllowedBaseDenoms(ctx))
}

func TestERC20Policy_InitDefaults(t *testing.T) {
	ctx, storeKey := makePolicyTestCtx(t)
	mock := newMockErc20Keeper()
	wrapper := newERC20PolicyKeeperWrapper(mock, storeKey)

	// Simulate initERC20PolicyDefaults by checking mode key isn't set then writing.
	store := ctx.KVStore(storeKey)
	require.False(t, store.Has(policyModeKey), "mode should not be set before init")

	wrapper.setRegistrationMode(ctx, PolicyModeAllowlist)
	for _, base := range DefaultAllowedBaseDenoms {
		wrapper.setBaseDenomAllowed(ctx, base)
	}

	require.Equal(t, PolicyModeAllowlist, wrapper.getRegistrationMode(ctx))
	for _, base := range DefaultAllowedBaseDenoms {
		require.True(t, wrapper.isBaseDenomAllowed(ctx, base), "default base denom %q should be allowed", base)
	}

	// Second call is no-op (mode key already set).
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
			Mode:      PolicyModeNone,
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, PolicyModeNone, wrapper.getRegistrationMode(ctx))
	})

	t.Run("valid mode change to allowlist with denoms", func(t *testing.T) {
		denom := "ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2"
		resp, err := server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority: govAddr.String(),
			Mode:      PolicyModeAllowlist,
			AddDenoms: []string{denom},
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, PolicyModeAllowlist, wrapper.getRegistrationMode(ctx))
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
			Mode:      PolicyModeAll,
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
		wrapper.setRegistrationMode(ctx, PolicyModeAllowlist)
		resp, err := server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority: govAddr.String(),
			Mode:      "", // empty = no change
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, PolicyModeAllowlist, wrapper.getRegistrationMode(ctx))
	})

	t.Run("empty authority", func(t *testing.T) {
		_, err := server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority: "",
			Mode:      PolicyModeAll,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty authority")
	})

	t.Run("add and remove base denoms", func(t *testing.T) {
		resp, err := server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority:    govAddr.String(),
			AddBaseDenoms: []string{"uatom", "uosmo"},
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.True(t, wrapper.isBaseDenomAllowed(ctx, "uatom"))
		require.True(t, wrapper.isBaseDenomAllowed(ctx, "uosmo"))

		// Remove one.
		resp, err = server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority:       govAddr.String(),
			RemoveBaseDenoms: []string{"uosmo"},
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.True(t, wrapper.isBaseDenomAllowed(ctx, "uatom"))
		require.False(t, wrapper.isBaseDenomAllowed(ctx, "uosmo"))
	})

	t.Run("invalid base denom", func(t *testing.T) {
		_, err := server.SetRegistrationPolicy(sdkCtx, &erc20policytypes.MsgSetRegistrationPolicy{
			Authority:    govAddr.String(),
			AddBaseDenoms: []string{""},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid add_base_denom")
	})
}
