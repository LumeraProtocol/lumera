package app

import (
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

// Registration policy mode constants.
const (
	// PolicyModeAll allows all IBC denoms to auto-register as ERC20 (default, backwards-compatible).
	PolicyModeAll = "all"
	// PolicyModeAllowlist only allows governance-approved IBC denoms to auto-register.
	PolicyModeAllowlist = "allowlist"
	// PolicyModeNone disables all IBC denom auto-registration.
	PolicyModeNone = "none"
)

// KV store prefixes under the erc20 store key for policy state.
var (
	policyModeKey      = []byte("lumera/erc20policy/mode")
	policyAllowPfx     = []byte("lumera/erc20policy/allow/")
	policyAllowBasePfx = []byte("lumera/erc20policy/allowbase/")
)

// DefaultAllowedBaseDenoms are well-known token base denominations that are
// pre-populated in the allowlist on genesis. Governance can add or remove
// entries at any time. Base denom matching is channel-independent: approving
// "uatom" allows ATOM arriving via any IBC channel/path.
var DefaultAllowedBaseDenoms = []string{
	"uatom",  // Cosmos Hub ATOM
	"uosmo",  // Osmosis OSMO
	"uusdc",  // Noble USDC (Circle)
}

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
	if mode == PolicyModeAll {
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

	// Extract the base denom (e.g. "uatom") for base-denom allowlist matching.
	baseDenom := token.Denom.Base

	// Apply policy for unregistered IBC denoms.
	switch mode {
	case PolicyModeNone:
		// IBC transfer succeeds; ERC20 registration is skipped.
		return ack
	case PolicyModeAllowlist:
		if w.isIBCDenomAllowed(ctx, coin.Denom) || w.isBaseDenomAllowed(ctx, baseDenom) {
			return w.inner.OnRecvPacket(ctx, packet, ack)
		}
		// Not in any allowlist — skip registration.
		return ack
	default:
		// Unknown mode, fall back to permissive behavior.
		return w.inner.OnRecvPacket(ctx, packet, ack)
	}
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
// Returns PolicyModeAllowlist if no mode has been set (secure default for new chains).
func (w *erc20PolicyKeeperWrapper) getRegistrationMode(ctx sdk.Context) string {
	store := ctx.KVStore(w.storeKey)
	bz := store.Get(policyModeKey)
	if bz == nil {
		return PolicyModeAllowlist
	}
	return string(bz)
}

// setRegistrationMode persists the policy mode to the KV store.
func (w *erc20PolicyKeeperWrapper) setRegistrationMode(ctx sdk.Context, mode string) {
	store := ctx.KVStore(w.storeKey)
	store.Set(policyModeKey, []byte(mode))
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
	defer iter.Close()

	var denoms []string
	for ; iter.Valid(); iter.Next() {
		denoms = append(denoms, string(iter.Key()))
	}
	return denoms
}

// ---------------------------------------------------------------------------
// Base denom allowlist helpers (channel-independent matching)
// ---------------------------------------------------------------------------

// isBaseDenomAllowed checks whether the given base denom (e.g. "uatom") is allowed.
func (w *erc20PolicyKeeperWrapper) isBaseDenomAllowed(ctx sdk.Context, baseDenom string) bool {
	store := prefix.NewStore(ctx.KVStore(w.storeKey), policyAllowBasePfx)
	return store.Has([]byte(baseDenom))
}

// setBaseDenomAllowed adds a base denom to the allowlist.
func (w *erc20PolicyKeeperWrapper) setBaseDenomAllowed(ctx sdk.Context, baseDenom string) {
	store := prefix.NewStore(ctx.KVStore(w.storeKey), policyAllowBasePfx)
	store.Set([]byte(baseDenom), []byte{1})
}

// removeBaseDenomAllowed removes a base denom from the allowlist.
func (w *erc20PolicyKeeperWrapper) removeBaseDenomAllowed(ctx sdk.Context, baseDenom string) {
	store := prefix.NewStore(ctx.KVStore(w.storeKey), policyAllowBasePfx)
	store.Delete([]byte(baseDenom))
}

// getAllowedBaseDenoms returns all base denoms currently in the allowlist.
func (w *erc20PolicyKeeperWrapper) getAllowedBaseDenoms(ctx sdk.Context) []string {
	store := prefix.NewStore(ctx.KVStore(w.storeKey), policyAllowBasePfx)
	iter := store.Iterator(nil, nil)
	defer iter.Close()

	var denoms []string
	for ; iter.Valid(); iter.Next() {
		denoms = append(denoms, string(iter.Key()))
	}
	return denoms
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

// initERC20PolicyDefaults writes the default allowlist base denoms into the KV
// store on first genesis. It is a no-op if the mode key already exists (i.e.
// the chain has already been initialized or upgraded).
func (app *App) initERC20PolicyDefaults(ctx sdk.Context) {
	store := ctx.KVStore(app.GetKey(erc20types.StoreKey))
	if store.Has(policyModeKey) {
		return // already initialized
	}
	app.erc20PolicyWrapper.setRegistrationMode(ctx, PolicyModeAllowlist)
	for _, base := range DefaultAllowedBaseDenoms {
		app.erc20PolicyWrapper.setBaseDenomAllowed(ctx, base)
	}
}
