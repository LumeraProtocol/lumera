package ibctesting

//go:generate mockgen -copyright_file=../../testutil/mock_header.txt -destination=mocks/ibcapp_mock.go -package=ibcappmocks -source=ibcapp.go

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
)

type IBCAppInterface interface {
	OnChanOpenInit(ctx sdk.Context, order channeltypes.Order, connectionHops []string, portID, 
		channelID string, counterparty channeltypes.Counterparty, version string) (string, error)
	OnChanOpenTry(ctx sdk.Context, order channeltypes.Order, connectionHops []string, portID, 
		channelID string, counterparty channeltypes.Counterparty, counterpartyVersion string) (string, error)
	OnChanOpenAck(ctx sdk.Context, portID, channelID, counterpartyChannelID, counterpartyVersion string) error
	OnChanOpenConfirm(ctx sdk.Context, portID, channelID string) error
	OnChanCloseInit(ctx sdk.Context, portID, channelID string) error
	OnChanCloseConfirm(ctx sdk.Context, portID, channelID string) error
	OnRecvPacket(ctx sdk.Context, channelVersion string, packet channeltypes.Packet, 
		relayer sdk.AccAddress) ibcexported.Acknowledgement
	OnAcknowledgementPacket(ctx sdk.Context, channelVersion string, packet channeltypes.Packet, 
		acknowledgement []byte, relayer sdk.AccAddress) error
	OnTimeoutPacket(ctx sdk.Context, channelVersion string, packet channeltypes.Packet, relayer sdk.AccAddress) error
}
