package mock

import (
	"strings"
	"errors"
	"bytes"
	"reflect"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
)

const (
	ModuleName = "mock"
	PortID = ModuleName
	Version = "mock-version"
)

var (
	MockAcknowledgement     = channeltypes.NewResultAcknowledgement([]byte("mock acknowledgement"))
	MockFailAcknowledgement = channeltypes.NewErrorAcknowledgement(errors.New("mock failed acknowledgement"))
	MockPacketData          = []byte("mock packet data")
	MockFailPacketData      = []byte("mock failed packet data")
	MockAsyncPacketData     = []byte("mock async packet data")
	UpgradeVersion          = fmt.Sprintf("%s-v2", Version)
	// MockApplicationCallbackError should be returned when an application callback should fail. It is possible to
	// test that this error was returned using ErrorIs.
	MockApplicationCallbackError error = &applicationCallbackError{}

	_ porttypes.IBCModule = &MockIBCModule{}
	_ porttypes.PacketDataUnmarshaler = &MockIBCModule{}
	_ ibcexported.Path = KeyPath{}
	_ ibcexported.Height = Height{}
)

// MockIBCModule adapts a gomock.MockIBCAppInterface to an IBCModule.
type MockIBCModule struct {
	IBCApp porttypes.IBCModule

	PortID string // PortID is the port ID of the mock IBC module.
}

// applicationCallbackError is a custom error type that will be unique for testing purposes.
type applicationCallbackError struct{}

func (applicationCallbackError) Error() string {
	return "mock application callback failed"
}

// NewMockIBCModule creates a new MockIBCModule with the given IBCAppInterface.
func NewMockIBCModule(ibcApp porttypes.IBCModule, portID string) *MockIBCModule {
	return &MockIBCModule{
		IBCApp: ibcApp,
		PortID: portID,
	}
}

func (m *MockIBCModule) OnChanOpenInit(ctx sdk.Context, order channeltypes.Order, connectionHops []string,
	portID, channelID string, counterparty channeltypes.Counterparty, version string) (string, error) {
	if strings.TrimSpace(version) == "" {
		version = Version
	}

	if m.IBCApp != nil {
		return m.IBCApp.OnChanOpenInit(ctx, order, connectionHops, portID, channelID, counterparty, version)
	}
	return version, nil
}

func (m *MockIBCModule) OnChanOpenTry(ctx sdk.Context, order channeltypes.Order, connectionHops []string,
	portID, channelID string,
	counterparty channeltypes.Counterparty, counterpartyVersion string) (string, error) {
	if m.IBCApp != nil {
		return m.IBCApp.OnChanOpenTry(ctx, order, connectionHops, portID, channelID, counterparty, counterpartyVersion)
	}

	return Version, nil
}

func (m *MockIBCModule) OnChanOpenAck(ctx sdk.Context, portID, channelID, counterpartyChannelID, counterpartyVersion string) error {
	if m.IBCApp != nil {
		return m.IBCApp.OnChanOpenAck(ctx, portID, channelID, counterpartyChannelID, counterpartyVersion)
	}

	return nil
}

func (m *MockIBCModule) OnChanOpenConfirm(ctx sdk.Context, portID, channelID string) error {
	if m.IBCApp != nil {
		return m.IBCApp.OnChanOpenConfirm(ctx, portID, channelID)
	}

	return nil
}

func (m *MockIBCModule) OnChanCloseInit(ctx sdk.Context, portID, channelID string) error {
	if m.IBCApp != nil {
		return m.IBCApp.OnChanCloseInit(ctx, portID, channelID)
	}

	return nil
}

func (m *MockIBCModule) OnChanCloseConfirm(ctx sdk.Context, portID, channelID string) error {
	if m.IBCApp != nil {
		return m.IBCApp.OnChanCloseConfirm(ctx, portID, channelID)
	}

	return nil
}

func (m *MockIBCModule) OnRecvPacket(ctx sdk.Context, channelVersion string, packet channeltypes.Packet, relayer sdk.AccAddress) ibcexported.Acknowledgement {
	if m.IBCApp != nil {
		return m.IBCApp.OnRecvPacket(ctx, channelVersion, packet, relayer)
	}

	ctx.EventManager().EmitEvent(NewMockRecvPacketEvent())

	if bytes.Equal(MockPacketData, packet.GetData()) {
		return MockAcknowledgement
	} else if bytes.Equal(MockAsyncPacketData, packet.GetData()) {
		return nil
	}

	return MockFailAcknowledgement
}

func (m *MockIBCModule) OnAcknowledgementPacket(ctx sdk.Context, channelVersion string, packet channeltypes.Packet, acknowledgement []byte, relayer sdk.AccAddress) error {
	if m.IBCApp != nil {
		return m.IBCApp.OnAcknowledgementPacket(ctx, channelVersion, packet, acknowledgement, relayer)
	}

	ctx.EventManager().EmitEvent(NewMockAckPacketEvent())
	return nil
}

func (m *MockIBCModule) OnTimeoutPacket(ctx sdk.Context, channelVersion string, packet channeltypes.Packet, relayer sdk.AccAddress) error {
	if m.IBCApp != nil {
		return m.IBCApp.OnTimeoutPacket(ctx, channelVersion, packet, relayer)
	}

	ctx.EventManager().EmitEvent(NewMockTimeoutPacketEvent())
	return nil
}

// UnmarshalPacketData returns the MockPacketData. This function implements the optional
// PacketDataUnmarshaler interface required for ADR 008 support.
func (MockIBCModule) UnmarshalPacketData(ctx sdk.Context, portID string, channelID string, bz []byte) (interface{}, string, error) {
	if reflect.DeepEqual(bz, MockPacketData) {
		return MockPacketData, Version, nil
	}
	return nil, "", MockApplicationCallbackError
}

// KeyPath defines a placeholder struct which implements the exported.Path interface
type KeyPath struct{}

// String implements the exported.Path interface
func (KeyPath) String() string {
	return ""
}

// Empty implements the exported.Path interface
func (KeyPath) Empty() bool {
	return false
}

// Height defines a placeholder struct which implements the exported.Height interface
type Height struct {
	ibcexported.Height
}
