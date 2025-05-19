package ibctesting

import (

	sdk "github.com/cosmos/cosmos-sdk/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
)

// MockIBCModule adapts a gomock.MockIBCAppInterface to an IBCModule.
type MockIBCModule struct {
	App IBCAppInterface
	PortID string
}

var _ porttypes.IBCModule = &MockIBCModule{}

func (m *MockIBCModule) OnChanOpenInit(ctx sdk.Context, order channeltypes.Order, connectionHops []string,
	portID, channelID string, counterparty channeltypes.Counterparty, version string) (string, error) {
	return m.App.OnChanOpenInit(ctx, order, connectionHops, portID, channelID, counterparty, version)
}

func (m *MockIBCModule) OnChanOpenTry(ctx sdk.Context, order channeltypes.Order, connectionHops []string,
	portID, channelID string,
	counterparty channeltypes.Counterparty, counterpartyVersion string) (string, error) {
	return m.App.OnChanOpenTry(ctx, order, connectionHops, portID, channelID, counterparty, counterpartyVersion)
}

func (m *MockIBCModule) OnChanOpenAck(ctx sdk.Context, portID, channelID, counterpartyChannelID, counterpartyVersion string) error {
	return m.App.OnChanOpenAck(ctx, portID, channelID, counterpartyChannelID, counterpartyVersion)
}

func (m *MockIBCModule) OnChanOpenConfirm(ctx sdk.Context, portID, channelID string) error {
	return m.App.OnChanOpenConfirm(ctx, portID, channelID)
}

func (m *MockIBCModule) OnChanCloseInit(ctx sdk.Context, portID, channelID string) error {
	return m.App.OnChanCloseInit(ctx, portID, channelID)
}

func (m *MockIBCModule) OnChanCloseConfirm(ctx sdk.Context, portID, channelID string) error {
	return m.App.OnChanCloseConfirm(ctx, portID, channelID)
}

func (m *MockIBCModule) OnRecvPacket(ctx sdk.Context, channelVersion string, packet channeltypes.Packet, relayer sdk.AccAddress) ibcexported.Acknowledgement {
	return m.App.OnRecvPacket(ctx, channelVersion, packet, relayer)
}

func (m *MockIBCModule) OnAcknowledgementPacket(ctx sdk.Context, channelVersion string, packet channeltypes.Packet, acknowledgement []byte, relayer sdk.AccAddress) error {
	return m.App.OnAcknowledgementPacket(ctx, channelVersion, packet, acknowledgement, relayer)
}

func (m *MockIBCModule) OnTimeoutPacket(ctx sdk.Context, channelVersion string, packet channeltypes.Packet, relayer sdk.AccAddress) error {
	return m.App.OnTimeoutPacket(ctx, channelVersion, packet, relayer)
}
