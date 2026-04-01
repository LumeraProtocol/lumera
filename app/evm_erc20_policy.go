package app

import (
	"bytes"
	"strings"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"cosmossdk.io/store/prefix"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	erc20policytypes "github.com/LumeraProtocol/lumera/x/erc20policy/types"

	evmibc "github.com/cosmos/evm/ibc"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	"github.com/cosmos/ibc-go/v10/modules/core/exported"
)

// Policy mode and KV key constants are defined in erc20policytypes.
// Local aliases for conciseness within this file.
var (
	policyModeKey         = erc20policytypes.PolicyModeKey
	policyAllowPfx        = erc20policytypes.PolicyAllowPfx
	policyAllowBaseTracePfx = erc20policytypes.PolicyAllowBaseTracePfx
)

// erc20KeeperWithDenomCheck extends the upstream Erc20Keeper interface with
// IsDenomRegistered, used to skip policy checks for already-registered denoms.
// The concrete erc20keeper.Keeper satisfies this interface.
type erc20KeeperWithDenomCheck interface {
	erc20types.Erc20Keeper
	IsDenomRegistered(ctx sdk.Context, denom string) bool
}

// Compile-time check that erc20PolicyKeeperWrapper satisfies the Erc20Keeper interface.
var _ erc20types.Erc20Keeper = (*erc20PolicyKeeperWrapper)(nil)

// erc20PolicyKeeperWrapper wraps an erc20 keeper and applies a governance-controlled
// registration policy before delegating OnRecvPacket.
// Only OnRecvPacket contains policy logic; the other methods pass through.
type erc20PolicyKeeperWrapper struct {
	inner    erc20KeeperWithDenomCheck
	storeKey *storetypes.KVStoreKey
}

// newERC20PolicyKeeperWrapper creates a policy-aware keeper wrapper.
// The storeKey should be the erc20 module's KV store key (shared prefix namespace).
func newERC20PolicyKeeperWrapper(inner erc20KeeperWithDenomCheck, storeKey *storetypes.KVStoreKey) *erc20PolicyKeeperWrapper {
	return &erc20PolicyKeeperWrapper{
		inner:    inner,
		storeKey: storeKey,
	}
}

// OnRecvPacket intercepts the ERC20 auto-registration path. If the registration
// policy blocks the denom, the IBC transfer still succeeds (ack is returned as-is)
// but no ERC20 token pair is created.
func (w *erc20PolicyKeeperWrapper) OnRecvPacket(
	ctx sdk.Context,
	packet channeltypes.Packet,
	ack exported.Acknowledgement,
) exported.Acknowledgement {
	mode := w.getRegistrationMode(ctx)

	// Fast path: "all" mode delegates unconditionally (default behavior).
	if mode == erc20policytypes.PolicyModeAll {
		return w.inner.OnRecvPacket(ctx, packet, ack)
	}

	// Parse the packet to determine the received denom.
	var data transfertypes.FungibleTokenPacketData
	if err := transfertypes.ModuleCdc.UnmarshalJSON(packet.GetData(), &data); err != nil {
		// Can't parse — let upstream handle (it will also fail and return an error ack).
		return w.inner.OnRecvPacket(ctx, packet, ack)
	}

	token := transfertypes.Token{
		Denom:  transfertypes.ExtractDenomFromPath(data.Denom),
		Amount: data.Amount,
	}
	coin := evmibc.GetReceivedCoin(packet, token)

	// Non-IBC denoms always pass through (upstream handles native/factory exclusions).
	if !strings.HasPrefix(coin.Denom, "ibc/") {
		return w.inner.OnRecvPacket(ctx, packet, ack)
	}

	// Already registered → pass through (no new registration will happen).
	if w.inner.IsDenomRegistered(ctx, coin.Denom) {
		return w.inner.OnRecvPacket(ctx, packet, ack)
	}

	// Extract the base denom (e.g. "uatom") for provenance-bound matching.
	baseDenom := token.Denom.Base

	// Apply policy for unregistered IBC denoms.
	switch mode {
	case erc20policytypes.PolicyModeNone:
		// IBC transfer succeeds; ERC20 registration is skipped.
		return ack
	case erc20policytypes.PolicyModeAllowlist:
		if w.isIBCDenomAllowed(ctx, coin.Denom) {
			return w.inner.OnRecvPacket(ctx, packet, ack)
		}
		fullTrace := buildFullTrace(packet, token.Denom.Trace)
		if w.isBaseDenomTraceAllowed(ctx, baseDenom, fullTrace) {
			return w.inner.OnRecvPacket(ctx, packet, ack)
		}
		// Not in any allowlist — skip registration.
		return ack
	default:
		// Unknown mode, fall back to permissive behavior.
		return w.inner.OnRecvPacket(ctx, packet, ack)
	}
}

// buildFullTrace constructs the complete received denom trace by prepending
// the packet's destination hop to the incoming trace from the packet data.
func buildFullTrace(packet channeltypes.Packet, incomingTrace []transfertypes.Hop) []*erc20policytypes.SourceHop {
	hops := make([]*erc20policytypes.SourceHop, 0, 1+len(incomingTrace))
	hops = append(hops, &erc20policytypes.SourceHop{
		PortId:    packet.GetDestPort(),
		ChannelId: packet.GetDestChannel(),
	})
	for _, hop := range incomingTrace {
		hops = append(hops, &erc20policytypes.SourceHop{
			PortId:    hop.PortId,
			ChannelId: hop.ChannelId,
		})
	}
	return hops
}

// OnAcknowledgementPacket passes through to the inner keeper.
func (w *erc20PolicyKeeperWrapper) OnAcknowledgementPacket(
	ctx sdk.Context,
	packet channeltypes.Packet,
	data transfertypes.FungibleTokenPacketData,
	ack channeltypes.Acknowledgement,
) error {
	return w.inner.OnAcknowledgementPacket(ctx, packet, data, ack)
}

// OnTimeoutPacket passes through to the inner keeper.
func (w *erc20PolicyKeeperWrapper) OnTimeoutPacket(
	ctx sdk.Context,
	packet channeltypes.Packet,
	data transfertypes.FungibleTokenPacketData,
) error {
	return w.inner.OnTimeoutPacket(ctx, packet, data)
}

// Logger passes through to the inner keeper.
func (w *erc20PolicyKeeperWrapper) Logger(ctx sdk.Context) log.Logger {
	return w.inner.Logger(ctx)
}

// ---------------------------------------------------------------------------
// Policy KV store helpers
// ---------------------------------------------------------------------------

// getRegistrationMode returns the current policy mode from the KV store.
// Returns erc20policytypes.PolicyModeAllowlist if no mode has been set (secure default for new chains).
func (w *erc20PolicyKeeperWrapper) getRegistrationMode(ctx sdk.Context) string {
	store := ctx.KVStore(w.storeKey)
	bz := store.Get(policyModeKey)
	if bz == nil {
		return erc20policytypes.PolicyModeAllowlist
	}
	return string(bz)
}

// setRegistrationMode persists the policy mode to the KV store.
func (w *erc20PolicyKeeperWrapper) setRegistrationMode(ctx sdk.Context, mode string) {
	store := ctx.KVStore(w.storeKey)
	store.Set(policyModeKey, []byte(mode))
}

// SetERC20RegistrationMode sets the ERC20 IBC auto-registration policy mode.
// Valid values: "all", "allowlist", "none".
// Exposed for test use — production code should use governance proposals.
func (app *App) SetERC20RegistrationMode(ctx sdk.Context, mode string) {
	app.erc20PolicyWrapper.setRegistrationMode(ctx, mode)
}

// isIBCDenomAllowed checks whether the given denom is in the allowlist.
func (w *erc20PolicyKeeperWrapper) isIBCDenomAllowed(ctx sdk.Context, denom string) bool {
	store := prefix.NewStore(ctx.KVStore(w.storeKey), policyAllowPfx)
	return store.Has([]byte(denom))
}

// setIBCDenomAllowed adds a denom to the allowlist.
func (w *erc20PolicyKeeperWrapper) setIBCDenomAllowed(ctx sdk.Context, denom string) {
	store := prefix.NewStore(ctx.KVStore(w.storeKey), policyAllowPfx)
	store.Set([]byte(denom), []byte{1})
}

// removeIBCDenomAllowed removes a denom from the allowlist.
func (w *erc20PolicyKeeperWrapper) removeIBCDenomAllowed(ctx sdk.Context, denom string) {
	store := prefix.NewStore(ctx.KVStore(w.storeKey), policyAllowPfx)
	store.Delete([]byte(denom))
}

// getAllowedDenoms returns all denoms currently in the exact ibc/ allowlist.
func (w *erc20PolicyKeeperWrapper) getAllowedDenoms(ctx sdk.Context) []string {
	store := prefix.NewStore(ctx.KVStore(w.storeKey), policyAllowPfx)
	iter := store.Iterator(nil, nil)
	defer func() { _ = iter.Close() }()

	var denoms []string
	for ; iter.Valid(); iter.Next() {
		denoms = append(denoms, string(iter.Key()))
	}
	return denoms
}

// ---------------------------------------------------------------------------
// Provenance-bound base denom trace helpers
// ---------------------------------------------------------------------------

// baseDenomTraceStoreKey constructs the full KV key for a base denom + trace entry.
// Format: baseDenom + "\x00" + traceKey
func baseDenomTraceStoreKey(baseDenom string, trace []*erc20policytypes.SourceHop) []byte {
	traceKey := erc20policytypes.EncodeTraceKey(trace)
	key := make([]byte, 0, len(baseDenom)+1+len(traceKey))
	key = append(key, []byte(baseDenom)...)
	key = append(key, 0x00)
	key = append(key, traceKey...)
	return key
}

// setBaseDenomTraceAllowed adds a provenance-bound base denom entry.
func (w *erc20PolicyKeeperWrapper) setBaseDenomTraceAllowed(ctx sdk.Context, baseDenom string, trace []*erc20policytypes.SourceHop) {
	store := prefix.NewStore(ctx.KVStore(w.storeKey), policyAllowBaseTracePfx)
	store.Set(baseDenomTraceStoreKey(baseDenom, trace), []byte{1})
}

// removeBaseDenomTraceAllowed removes a specific provenance-bound base denom entry.
func (w *erc20PolicyKeeperWrapper) removeBaseDenomTraceAllowed(ctx sdk.Context, baseDenom string, trace []*erc20policytypes.SourceHop) {
	store := prefix.NewStore(ctx.KVStore(w.storeKey), policyAllowBaseTracePfx)
	store.Delete(baseDenomTraceStoreKey(baseDenom, trace))
}

// removeAllBaseDenomTraces deletes all trace entries for a given base denom.
func (w *erc20PolicyKeeperWrapper) removeAllBaseDenomTraces(ctx sdk.Context, baseDenom string) {
	store := prefix.NewStore(ctx.KVStore(w.storeKey), policyAllowBaseTracePfx)
	pfx := append([]byte(baseDenom), 0x00)
	iter := store.Iterator(pfx, storetypes.PrefixEndBytes(pfx))
	defer func() { _ = iter.Close() }()

	var keys [][]byte
	for ; iter.Valid(); iter.Next() {
		keys = append(keys, append([]byte(nil), iter.Key()...))
	}
	for _, k := range keys {
		store.Delete(k)
	}
}

// isBaseDenomTraceAllowed checks whether the given base denom + full trace
// exactly matches an allowed entry. Empty-trace entries (placeholders) never
// match because fullTrace always has at least one hop for a real IBC packet.
func (w *erc20PolicyKeeperWrapper) isBaseDenomTraceAllowed(ctx sdk.Context, baseDenom string, fullTrace []*erc20policytypes.SourceHop) bool {
	if len(fullTrace) == 0 {
		return false // real IBC packets always have at least one hop
	}
	store := prefix.NewStore(ctx.KVStore(w.storeKey), policyAllowBaseTracePfx)
	return store.Has(baseDenomTraceStoreKey(baseDenom, fullTrace))
}

// getAllowedBaseDenomTraces returns all provenance-bound base denom entries.
func (w *erc20PolicyKeeperWrapper) getAllowedBaseDenomTraces(ctx sdk.Context) []erc20policytypes.AllowedBaseDenomTrace {
	store := prefix.NewStore(ctx.KVStore(w.storeKey), policyAllowBaseTracePfx)
	iter := store.Iterator(nil, nil)
	defer func() { _ = iter.Close() }()

	var entries []erc20policytypes.AllowedBaseDenomTrace
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		// Key format: baseDenom + "\x00" + traceKey
		idx := bytes.IndexByte(key, 0x00)
		if idx < 0 {
			continue
		}
		baseDenom := string(key[:idx])
		traceKey := key[idx+1:]
		hops := erc20policytypes.DecodeTraceKey(traceKey)
		entries = append(entries, erc20policytypes.AllowedBaseDenomTrace{
			BaseDenom: baseDenom,
			Trace:     hops,
		})
	}
	return entries
}

// ---------------------------------------------------------------------------
// App-level registration
// ---------------------------------------------------------------------------

// registerERC20Policy creates the ERC20 registration policy wrapper and
// registers its governance message handler and codec interfaces.
// Must be called after registerEVMModules (Erc20Keeper must exist) and before
// registerIBCModules (which wires the wrapper into the IBC transfer stacks).
func (app *App) registerERC20Policy() {
	storeKey := app.GetKey(erc20types.StoreKey)
	app.erc20PolicyWrapper = newERC20PolicyKeeperWrapper(app.Erc20Keeper, storeKey)

	// Register the proto message interfaces so governance proposals can include
	// MsgSetRegistrationPolicy as an Any-encoded message.
	erc20policytypes.RegisterInterfaces(app.interfaceRegistry)

	// Register the governance message server on the app's MsgServiceRouter.
	govAuthority := authtypes.NewModuleAddress(govtypes.ModuleName)
	erc20policytypes.RegisterMsgServer(
		app.MsgServiceRouter(),
		&erc20PolicyMsgServer{
			wrapper:   app.erc20PolicyWrapper,
			authority: govAuthority,
		},
	)
}

// initERC20PolicyDefaults writes the default provenance-bound base denom entries
// into the KV store on first genesis. Entries have empty traces (inert
// placeholders — governance must bind real channels before they match). It is a
// no-op if the mode key already exists (i.e. the chain has already been
// initialized or upgraded).
func (app *App) initERC20PolicyDefaults(ctx sdk.Context) {
	store := ctx.KVStore(app.GetKey(erc20types.StoreKey))
	if store.Has(policyModeKey) {
		return // already initialized
	}
	app.erc20PolicyWrapper.setRegistrationMode(ctx, erc20policytypes.PolicyModeAllowlist)
	for _, entry := range erc20policytypes.DefaultAllowedBaseDenomTraces {
		app.erc20PolicyWrapper.setBaseDenomTraceAllowed(ctx, entry.BaseDenom, entry.Trace)
	}
}
