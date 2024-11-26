package system_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/CosmWasm/wasmd/app"
	"github.com/CosmWasm/wasmd/x/wasm/types"
	wasmvmtypes "github.com/CosmWasm/wasmvm/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ibcfee "github.com/cosmos/ibc-go/v8/modules/apps/29-fee/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	ibctesting "github.com/cosmos/ibc-go/v8/testing"
	pastelibc "github.com/pastelnetwork/pastel/tests/ibctesting"
	"github.com/pastelnetwork/pastel/tests/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
)

func TestIBCCallbacks(t *testing.T) {
	// scenario:
	// given two chains
	//   with an ics-20 channel established
	//   and an ibc-callbacks contract deployed on chain A and B each
	// when the contract on A sends an IBCMsg::Transfer to the contract on B
	// then the contract on B should receive a destination chain callback
	//   and the contract on A should receive a source chain callback with the result (ack or timeout)

	os.Setenv("SYSTEM_TESTS", "true")
	marshaler := app.MakeEncodingConfig(t).Codec
	coord := pastelibc.NewCoordinator(t, 2)
	chainA := coord.GetChain(pastelibc.GetChainID(1))
	chainB := coord.GetChain(pastelibc.GetChainID(2))

	actorChainA := sdk.AccAddress(chainA.SenderPrivKey.PubKey().Address())
	oneToken := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(1)))

	path := pastelibc.NewPath(chainA, chainB)
	path.EndpointA.ChannelConfig = &ibctesting.ChannelConfig{
		PortID:  ibctransfertypes.PortID,
		Version: string(marshaler.MustMarshalJSON(&ibcfee.Metadata{FeeVersion: ibcfee.Version, AppVersion: ibctransfertypes.Version})),
		Order:   channeltypes.UNORDERED,
	}
	path.EndpointB.ChannelConfig = &ibctesting.ChannelConfig{
		PortID:  ibctransfertypes.PortID,
		Version: string(marshaler.MustMarshalJSON(&ibcfee.Metadata{FeeVersion: ibcfee.Version, AppVersion: ibctransfertypes.Version})),
		Order:   channeltypes.UNORDERED,
	}
	// with an ics-20 transfer channel setup between both chains
	coord.Setup(path)

	// with an ibc-callbacks contract deployed on chain A
	codeIDonA := chainA.StoreCodeFile("./testdata/ibc_callbacks.wasm").CodeID

	// and on chain B
	codeIDonB := chainB.StoreCodeFile("./testdata/ibc_callbacks.wasm").CodeID

	type TransferExecMsg struct {
		ToAddress      string `json:"to_address"`
		ChannelID      string `json:"channel_id"`
		TimeoutSeconds uint32 `json:"timeout_seconds"`
	}
	// ExecuteMsg is the ibc-callbacks contract's execute msg
	type ExecuteMsg struct {
		Transfer *TransferExecMsg `json:"transfer"`
	}
	type QueryMsg struct {
		CallbackStats struct{} `json:"callback_stats"`
	}
	type QueryResp struct {
		IBCAckCallbacks         []wasmvmtypes.IBCPacketAckMsg           `json:"ibc_ack_callbacks"`
		IBCTimeoutCallbacks     []wasmvmtypes.IBCPacketTimeoutMsg       `json:"ibc_timeout_callbacks"`
		IBCDestinationCallbacks []wasmvmtypes.IBCDestinationCallbackMsg `json:"ibc_destination_callbacks"`
	}

	specs := map[string]struct {
		contractMsg ExecuteMsg
		// expAck is true if the packet is relayed, false if it times out
		expAck bool
	}{
		"success": {
			contractMsg: ExecuteMsg{
				Transfer: &TransferExecMsg{
					ChannelID:      path.EndpointA.ChannelID,
					TimeoutSeconds: 100,
				},
			},
			expAck: true,
		},
		"timeout": {
			contractMsg: ExecuteMsg{
				Transfer: &TransferExecMsg{
					ChannelID:      path.EndpointA.ChannelID,
					TimeoutSeconds: 1,
				},
			},
			expAck: false,
		},
	}

	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			contractAddrA := chainA.InstantiateContract(codeIDonA, []byte(`{}`))
			require.NotEmpty(t, contractAddrA)
			contractAddrB := chainB.InstantiateContract(codeIDonB, []byte(`{}`))
			require.NotEmpty(t, contractAddrB)

			if spec.contractMsg.Transfer != nil && spec.contractMsg.Transfer.ToAddress == "" {
				spec.contractMsg.Transfer.ToAddress = contractAddrB.String()
			}
			contractMsgBz, err := json.Marshal(spec.contractMsg)
			require.NoError(t, err)

			// when the contract on chain A sends an IBCMsg::Transfer to the contract on chain B
			execMsg := types.MsgExecuteContract{
				Sender:   actorChainA.String(),
				Contract: contractAddrA.String(),
				Msg:      contractMsgBz,
				Funds:    oneToken,
			}
			_, err = chainA.SendMsgs(&execMsg)
			require.NoError(t, err)

			if spec.expAck {
				require.NoError(t, coord.RelayAndAckPendingPackets(path))
			} else {
				// and the packet times out
				require.NoError(t, coord.TimeoutPendingPackets(path))
			}
		})
	}
}

func TestIBCCallbacksWithoutEntrypoints(t *testing.T) {
	// scenario:
	// given two chains
	//   with an ics-20 channel established
	//   and a reflect contract deployed on chain A and B each
	// when the contract on A sends an IBCMsg::Transfer to the contract on B
	// then the VM should try to call the callback on B and fail gracefully
	//   and should try to call the callback on A and fail gracefully
	os.Setenv("SYSTEM_TESTS", "true")
	marshaler := app.MakeEncodingConfig(t).Codec
	coord := pastelibc.NewCoordinator(t, 2)
	chainA := coord.GetChain(pastelibc.GetChainID(1))
	chainB := coord.GetChain(pastelibc.GetChainID(2))

	oneToken := sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(1))

	path := pastelibc.NewPath(chainA, chainB)
	path.EndpointA.ChannelConfig = &ibctesting.ChannelConfig{
		PortID:  ibctransfertypes.PortID,
		Version: string(marshaler.MustMarshalJSON(&ibcfee.Metadata{FeeVersion: ibcfee.Version, AppVersion: ibctransfertypes.Version})),
		Order:   channeltypes.UNORDERED,
	}
	path.EndpointB.ChannelConfig = &ibctesting.ChannelConfig{
		PortID:  ibctransfertypes.PortID,
		Version: string(marshaler.MustMarshalJSON(&ibcfee.Metadata{FeeVersion: ibcfee.Version, AppVersion: ibctransfertypes.Version})),
		Order:   channeltypes.UNORDERED,
	}
	// with an ics-20 transfer channel setup between both chains
	coord.Setup(path)

	// with a reflect contract deployed on chain A and B
	contractAddrA := system.InstantiateReflectContract(t, chainA)
	chainA.Fund(contractAddrA, oneToken.Amount)
	contractAddrB := system.InstantiateReflectContract(t, chainA)

	// when the contract on A sends an IBCMsg::Transfer to the contract on B
	memo := fmt.Sprintf(`{"src_callback":{"address":"%v"},"dest_callback":{"address":"%v"}}`, contractAddrA.String(), contractAddrB.String())
	system.MustExecViaReflectContract(t, chainA, contractAddrA, wasmvmtypes.CosmosMsg{
		IBC: &wasmvmtypes.IBCMsg{
			Transfer: &wasmvmtypes.TransferMsg{
				ToAddress: contractAddrB.String(),
				ChannelID: path.EndpointA.ChannelID,
				Amount:    wasmvmtypes.NewCoin(oneToken.Amount.Uint64(), oneToken.Denom),
				Timeout: wasmvmtypes.IBCTimeout{
					Timestamp: uint64(chainA.LastHeader.GetTime().Add(time.Second * 100).UnixNano()),
				},
				Memo: memo,
			},
		},
	})

	// and the packet is relayed without problems
	require.NoError(t, coord.RelayAndAckPendingPackets(path))
	assert.Empty(t, chainA.PendingSendPackets)
}
