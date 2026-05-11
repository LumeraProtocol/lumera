package app

import (
	"bytes"
	"context"
	"fmt"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	erc20policytypes "github.com/LumeraProtocol/lumera/x/erc20policy/types"
	host "github.com/cosmos/ibc-go/v10/modules/core/24-host"
)

// erc20PolicyMsgServer implements the erc20policy MsgServer at the app level.
// It validates governance authority and delegates policy updates to the wrapper.
type erc20PolicyMsgServer struct {
	erc20policytypes.UnimplementedMsgServer
	wrapper   *erc20PolicyKeeperWrapper
	authority []byte // governance module address bytes
}

var _ erc20policytypes.MsgServer = (*erc20PolicyMsgServer)(nil)

// SetRegistrationPolicy handles the governance message to update the ERC20
// IBC auto-registration policy.
func (s *erc20PolicyMsgServer) SetRegistrationPolicy(
	goCtx context.Context,
	msg *erc20policytypes.MsgSetRegistrationPolicy,
) (*erc20policytypes.MsgSetRegistrationPolicyResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate authority.
	if msg.Authority == "" {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "empty authority")
	}

	authorityBytes, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		return nil, errorsmod.Wrap(err, "invalid authority address")
	}

	if !bytes.Equal(s.authority, authorityBytes) {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrUnauthorized,
			"invalid authority; expected %s, got %s",
			sdk.AccAddress(s.authority).String(), msg.Authority,
		)
	}

	// Validate and apply mode change.
	if msg.Mode != "" {
		switch msg.Mode {
		case erc20policytypes.PolicyModeAll, erc20policytypes.PolicyModeAllowlist, erc20policytypes.PolicyModeNone:
			s.wrapper.setRegistrationMode(ctx, msg.Mode)
		default:
			return nil, errorsmod.Wrapf(
				sdkerrors.ErrInvalidRequest,
				"invalid mode %q; must be %q, %q, or %q",
				msg.Mode, erc20policytypes.PolicyModeAll, erc20policytypes.PolicyModeAllowlist, erc20policytypes.PolicyModeNone,
			)
		}
	}

	// Apply allowlist additions.
	for _, denom := range msg.AddDenoms {
		if err := validateIBCDenom(denom); err != nil {
			return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "invalid add_denom: %v", err)
		}
		s.wrapper.setIBCDenomAllowed(ctx, denom)
	}

	// Apply allowlist removals.
	for _, denom := range msg.RemoveDenoms {
		s.wrapper.removeIBCDenomAllowed(ctx, denom)
	}

	// Apply provenance-bound base denom trace additions.
	for _, entry := range msg.AddBaseDenomTraces {
		if err := validateBaseDenomTrace(entry); err != nil {
			return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "invalid add_base_denom_trace: %v", err)
		}
		s.wrapper.setBaseDenomTraceAllowed(ctx, entry.BaseDenom, entry.Trace)
	}

	// Apply provenance-bound base denom trace removals.
	for _, entry := range msg.RemoveBaseDenomTraces {
		s.wrapper.removeBaseDenomTraceAllowed(ctx, entry.BaseDenom, entry.Trace)
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"erc20_registration_policy_updated",
			sdk.NewAttribute("authority", msg.Authority),
			sdk.NewAttribute("mode", msg.Mode),
			sdk.NewAttribute("add_denoms_count", fmt.Sprintf("%d", len(msg.AddDenoms))),
			sdk.NewAttribute("remove_denoms_count", fmt.Sprintf("%d", len(msg.RemoveDenoms))),
			sdk.NewAttribute("add_base_denom_traces_count", fmt.Sprintf("%d", len(msg.AddBaseDenomTraces))),
			sdk.NewAttribute("remove_base_denom_traces_count", fmt.Sprintf("%d", len(msg.RemoveBaseDenomTraces))),
		),
	)

	return &erc20policytypes.MsgSetRegistrationPolicyResponse{}, nil
}

// validateIBCDenom performs basic validation on an IBC denom string.
func validateIBCDenom(denom string) error {
	if denom == "" {
		return fmt.Errorf("empty denom")
	}
	if len(denom) > 128 {
		return fmt.Errorf("denom too long: %d > 128", len(denom))
	}
	return nil
}

// validateBaseDenomTrace validates a provenance-bound base denom entry.
// Hop port and channel IDs are validated using ibc-go's canonical validators
// to ensure they cannot contain structural delimiters ('/' and '\x00') used
// in the trace-key encoding.
func validateBaseDenomTrace(entry *erc20policytypes.AllowedBaseDenomTrace) error {
	if entry.BaseDenom == "" {
		return fmt.Errorf("empty base denom")
	}
	if len(entry.BaseDenom) > 64 {
		return fmt.Errorf("base denom too long: %d > 64", len(entry.BaseDenom))
	}
	// Empty trace is valid (placeholder entry).
	for i, hop := range entry.Trace {
		if err := host.PortIdentifierValidator(hop.PortId); err != nil {
			return fmt.Errorf("hop %d: invalid port_id: %w", i, err)
		}
		if err := host.ChannelIdentifierValidator(hop.ChannelId); err != nil {
			return fmt.Errorf("hop %d: invalid channel_id: %w", i, err)
		}
	}
	return nil
}
