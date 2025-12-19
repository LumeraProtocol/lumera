package wasm_test

import (
	"os"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/libs/rand"
	"github.com/cosmos/gogoproto/proto"
	icacontrollertypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/controller/types"
	hosttypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibctst "github.com/cosmos/ibc-go/v10/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/address"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	lcfg "github.com/LumeraProtocol/lumera/config"
	"github.com/LumeraProtocol/lumera/tests/ibctesting"
)

func TestICA(t *testing.T) {
	// High-level scenario:
	// - We spin up two in-process chains (controller + host) using ibc-go's testing harness.
	// - The controller registers an interchain account (ICA) on the host via ICS-27 handshake.
	// - The controller then sends an "execute tx" packet containing a normal Cosmos SDK Msg (bank send).
	// - The host executes that Msg as if it was signed by the ICA address, and we assert host state changes.
	os.Setenv("SYSTEM_TESTS", "true")

	// Set up two chains and configure the host ICA allowlist to only accept bank sends.
	coord := ibctesting.NewCoordinator(t, 2)
	hostChain := coord.GetChain(ibctesting.GetChainID(1))
	hostParams := hosttypes.NewParams(true, []string{sdk.MsgTypeURL(&banktypes.MsgSend{})})
	hostApp := hostChain.GetLumeraApp()
	hostApp.ICAHostKeeper.SetParams(hostChain.GetContext(), hostParams)

	controllerChain := coord.GetChain(ibctesting.GetChainID(2))

	// Create an IBC path and set up clients + connections between controller and host.
	path := ibctesting.NewPath(controllerChain, hostChain)
	path.SetupConnections()

	// Run the same scenario twice, once with protobuf tx encoding and once with proto3json encoding.
	specs := map[string]struct {
		icaVersion string
		encoding   string
	}{
		"proto": {
			icaVersion: "", // empty string defaults to the proto3 encoding type
			encoding:   icatypes.EncodingProtobuf,
		},
		"json": {
			icaVersion: string(icatypes.ModuleCdc.MustMarshalJSON(&icatypes.Metadata{
				Version:                icatypes.Version,
				ControllerConnectionId: path.EndpointA.ConnectionID,
				HostConnectionId:       path.EndpointB.ConnectionID,
				Encoding:               icatypes.EncodingProto3JSON, // use proto3json
				TxType:                 icatypes.TxTypeSDKMultiMsg,
			})),
			encoding: icatypes.EncodingProto3JSON,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			// Controller-side owner account: signs MsgRegisterInterchainAccount and MsgSendTx on the controller chain.
			icaControllerKey := secp256k1.GenPrivKey()
			icaControllerAddr := sdk.AccAddress(icaControllerKey.PubKey().Address().Bytes())
			controllerChain.Fund(icaControllerAddr, sdkmath.NewInt(1_000))

			// 1) Register an ICA on the host chain (this is an IBC handshake flow).
			msg := icacontrollertypes.NewMsgRegisterInterchainAccount(path.EndpointA.ConnectionID, icaControllerAddr.String(), spec.icaVersion, channeltypes.UNORDERED)
			res, err := controllerChain.SendNonDefaultSenderMsgs(icaControllerKey, msg)
			require.NoError(t, err)
			chanID, portID, version := parseIBCChannelEvents(t, res)

			// 2) Open the ICA channel on both sides with the negotiated port/version.
			path.EndpointA.ChannelID = chanID
			path.EndpointA.ChannelConfig = &ibctesting.ChannelConfig{
				PortID:  portID,
				Version: version,
				Order:   channeltypes.ORDERED,
			}
			path.EndpointB.ChannelID = ""
			path.EndpointB.ChannelConfig = &ibctesting.ChannelConfig{
				PortID:  icatypes.HostPortID,
				Version: icatypes.Version,
				Order:   channeltypes.ORDERED,
			}
			path.CreateChannels()

			// 3) Query the controller-side ICA address (on the host chain) and fund it on the host.
			contApp := controllerChain.GetLumeraApp()
			icaRsp, err := contApp.ICAControllerKeeper.InterchainAccount(controllerChain.GetContext(), &icacontrollertypes.QueryInterchainAccountRequest{
				Owner:        icaControllerAddr.String(),
				ConnectionId: path.EndpointA.ConnectionID,
			})
			require.NoError(t, err)
			icaAddr := sdk.MustAccAddressFromBech32(icaRsp.GetAddress())
			hostChain.Fund(icaAddr, sdkmath.NewInt(1_000))

			// 4) Build an "execute tx" ICA packet that contains a normal MsgSend signed by the ICA address.
			targetAddr := sdk.AccAddress(rand.Bytes(address.Len))
			sendCoin := sdk.NewCoin(lcfg.ChainDenom, sdkmath.NewInt(100))
			payloadMsg := banktypes.NewMsgSend(icaAddr, targetAddr, sdk.NewCoins(sendCoin))
			rawPayloadData, err := icatypes.SerializeCosmosTx(controllerChain.Codec, []proto.Message{payloadMsg}, spec.encoding)
			require.NoError(t, err)
			payloadPacket := icatypes.InterchainAccountPacketData{
				Type: icatypes.EXECUTE_TX,
				Data: rawPayloadData,
				Memo: "testing",
			}
			relativeTimeout := uint64(time.Minute.Nanoseconds()) // note this is in nanoseconds

			// 5) Send MsgSendTx on the controller chain and relay the packet + acknowledgement.
			msgSendTx := icacontrollertypes.NewMsgSendTx(icaControllerAddr.String(), path.EndpointA.ConnectionID, relativeTimeout, payloadPacket)
			_, err = controllerChain.SendNonDefaultSenderMsgs(icaControllerKey, msgSendTx)
			require.NoError(t, err)

			assert.Equal(t, 1, len(*controllerChain.PendingSendPackets))
			require.NoError(t, path.RelayAndAckPendingPackets())

			// 6) Assert the host chain state changed: the target address received funds.
			gotBalance := hostChain.Balance(targetAddr, lcfg.ChainDenom)
			assert.Equal(t, sendCoin.String(), gotBalance.String())
		})
	}
}

func parseIBCChannelEvents(t *testing.T, res *abci.ExecTxResult) (string, string, string) {
	t.Helper()
	chanID, err := ibctst.ParseChannelIDFromEvents(res.GetEvents())
	require.NoError(t, err)
	portID, err := ibctesting.ParsePortIDFromEvents(res.GetEvents())
	require.NoError(t, err)
	version, err := ibctesting.ParseChannelVersionFromEvents(res.GetEvents())
	require.NoError(t, err)
	return chanID, portID, version
}
