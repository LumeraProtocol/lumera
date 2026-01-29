package ibctesting

import (
	"fmt"
	"context"
	"strings"
	"bytes"
	"os"
	"time"
	"compress/gzip"
	"encoding/json"

	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/libs/rand"

	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	channeltypesv2 "github.com/cosmos/ibc-go/v10/modules/core/04-channel/v2/types"
	host "github.com/cosmos/ibc-go/v10/modules/core/24-host"
	hostv2 "github.com/cosmos/ibc-go/v10/modules/core/24-host/v2"
	ibctst "github.com/cosmos/ibc-go/v10/testing"
	
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	gogoproto "github.com/cosmos/gogoproto/proto"

	lcfg "github.com/LumeraProtocol/lumera/config"
)

var wasmIdent       = []byte("\x00\x61\x73\x6D")

type PendingAckPacketV2 struct {
	channeltypesv2.Packet
	Ack []byte
}

func (chain *TestChain) CaptureIBCEvents(result *abci.ExecTxResult) {
	toSend, _ := ibctst.ParseIBCV1Packets(channeltypes.EventTypeSendPacket, result.Events)

	// IBCv1 and IBCv2 `EventTypeSendPacket` are the same
	// and ParseIBCV1Packets parses both of them as they were IBCv1
	// so we have to filter them here.
	//
	// While parsing IBC2 events in IBC1 context the only overlapping event is the
	// `AttributeKeyTimeoutTimestamp` so to determine if the wrong set of events was parsed
	// we should be able to check if any other field in the packet is not set.
	var toSendFiltered []channeltypes.Packet
	for _, packet := range toSend {
		if packet.SourcePort != "" {
			toSendFiltered = append(toSendFiltered, packet)
		}
	}

	// require.NoError(chain, err)
	if len(toSendFiltered) > 0 {
		// Keep a queue on the chain that we can relay in tests
		*chain.PendingSendPackets = append(*chain.PendingSendPackets, toSendFiltered...)
	}
}

func (chain *TestChain) CaptureIBCEventsV2(result *abci.ExecTxResult) {
	toSend, err := ibctst.ParseIBCV2Packets(channeltypesv2.EventTypeSendPacket, result.Events)
	if err != nil {
		if err.Error() == "no IBC v2 packets found in events" {
			return
		}
		require.NoError(chain, err)
	}
	if len(toSend) > 0 {
		// Keep a queue on the chain that we can relay in tests
		*chain.PendingSendPacketsV2 = append(*chain.PendingSendPacketsV2, toSend...)
	}
}

func (chain *TestChain) OverrideSendMsgs(msgs ...sdk.Msg) (*abci.ExecTxResult, error) {
	chain.SendMsgsOverride = nil
	result, err := chain.SendMsgs(msgs...)
	chain.SendMsgsOverride = chain.OverrideSendMsgs
	chain.CaptureIBCEvents(result)
	chain.CaptureIBCEventsV2(result)
	return result, err
}

func (chain *TestChain) StoreCodeFile(filename string) wasmtypes.MsgStoreCodeResponse {
	wasmCode, err := os.ReadFile(filename)
	require.NoError(chain.TB, err)
	if strings.HasSuffix(filename, "wasm") { // compress for gas limit
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err := gz.Write(wasmCode)
		require.NoError(chain.TB, err)
		err = gz.Close()
		require.NoError(chain.TB, err)
		wasmCode = buf.Bytes()
	}
	return chain.StoreCode(wasmCode)
}

func (chain *TestChain) StoreCode(byteCode []byte) wasmtypes.MsgStoreCodeResponse {
	storeMsg := &wasmtypes.MsgStoreCode{
		Sender:       chain.SenderAccount.GetAddress().String(),
		WASMByteCode: byteCode,
	}
	r, err := chain.SendMsgs(storeMsg)
	require.NoError(chain.TB, err)

	var pInstResp wasmtypes.MsgStoreCodeResponse
	chain.UnwrapExecTXResult(r, &pInstResp)

	require.NotEmpty(chain.TB, pInstResp.CodeID)
	require.NotEmpty(chain.TB, pInstResp.Checksum)
	return pInstResp
}

// UnwrapExecTXResult is a helper to unpack execution result from proto any type
func (chain *TestChain) UnwrapExecTXResult(r *abci.ExecTxResult, target gogoproto.Message) {
	var wrappedRsp sdk.TxMsgData
	require.NoError(chain.TB, chain.App.AppCodec().Unmarshal(r.Data, &wrappedRsp))

	// unmarshal protobuf response from data
	require.Len(chain.TB, wrappedRsp.MsgResponses, 1)
	require.NoError(chain.TB, gogoproto.Unmarshal(wrappedRsp.MsgResponses[0].Value, target))
}

func (chain *TestChain) InstantiateContract(codeID uint64, initMsg []byte) sdk.AccAddress {
	instantiateMsg := &wasmtypes.MsgInstantiateContract{
		Sender: chain.SenderAccount.GetAddress().String(),
		Admin:  chain.SenderAccount.GetAddress().String(),
		CodeID: codeID,
		Label:  "ibc-test",
		Msg:    initMsg,
		Funds:  sdk.Coins{TestCoin},
	}

	r, err := chain.SendMsgs(instantiateMsg)
	require.NoError(chain.TB, err)

	var pExecResp wasmtypes.MsgInstantiateContractResponse
	chain.UnwrapExecTXResult(r, &pExecResp)

	a, err := sdk.AccAddressFromBech32(pExecResp.Address)
	require.NoError(chain.TB, err)
	return a
}

// SeedNewContractInstance stores some wasm code and instantiates a new contract on this chain.
// This method can be called to prepare the store with some valid CodeInfo and ContractInfo. The returned
// Address is the contract address for this instance. Test should make use of this data and/or use NewIBCContractMockWasmEngine
// for using a contract mock in Go.
func (chain *TestChain) SeedNewContractInstance() sdk.AccAddress {
	pInstResp := chain.StoreCode(append(wasmIdent, rand.Bytes(10)...))
	codeID := pInstResp.CodeID

	anyAddressStr := chain.SenderAccount.GetAddress().String()
	initMsg := []byte(fmt.Sprintf(`{"verifier": %q, "beneficiary": %q}`, anyAddressStr, anyAddressStr))
	return chain.InstantiateContract(codeID, initMsg)
}

func (chain *TestChain) ContractInfo(contractAddr sdk.AccAddress) *wasmtypes.ContractInfo {
	return chain.App.GetWasmKeeper().GetContractInfo(chain.GetContext(), contractAddr)
}

// Fund an address with the given amount in default denom
func (chain *TestChain) Fund(addr sdk.AccAddress, amount sdkmath.Int) {
	_, err := chain.SendMsgs(&banktypes.MsgSend{
		FromAddress: chain.SenderAccount.GetAddress().String(),
		ToAddress:   addr.String(),
		Amount:      sdk.NewCoins(sdk.NewCoin(lcfg.ChainDenom, amount)),
	})
	require.NoError(chain.TB, err)
}

func (chain *TestChain) Balance(acc sdk.AccAddress, denom string) sdk.Coin {
	return chain.GetLumeraApp().GetBankKeeper().GetBalance(chain.GetContext(), acc, denom)
}

func (chain *TestChain) AllBalances(acc sdk.AccAddress) sdk.Coins {
	return chain.GetLumeraApp().GetBankKeeper().GetAllBalances(chain.GetContext(), acc)
}

// SendNonDefaultSenderMsgs is the same as SendMsgs but with a custom signer/account
func (chain *TestChain) SendNonDefaultSenderMsgs(senderPrivKey cryptotypes.PrivKey, msgs ...sdk.Msg) (*abci.ExecTxResult, error) {
	require.NotEqual(chain.TB, chain.SenderPrivKey, senderPrivKey, "use SendMsgs method")

	addr := sdk.AccAddress(senderPrivKey.PubKey().Address().Bytes())
	account := chain.GetLumeraApp().GetAuthKeeper().GetAccount(chain.GetContext(), addr)
	prevAccount := chain.SenderAccount
	prevSenderPrivKey := chain.SenderPrivKey
	chain.SenderAccount = account
	chain.SenderPrivKey = senderPrivKey

	require.NotNil(chain.TB, account)
	result, err := chain.SendMsgs(msgs...)

	chain.SenderAccount = prevAccount
	chain.SenderPrivKey = prevSenderPrivKey

	return result, err
}

// SmartQuery This will serialize the query message and submit it to the contract.
// The response is parsed into the provided interface.
// Usage: SmartQuery(addr, QueryMsg{Foo: 1}, &response)
func (chain *TestChain) SmartQuery(contractAddr string, queryMsg, response interface{}) error {
	msg, err := json.Marshal(queryMsg)
	if err != nil {
		return err
	}

	req := wasmtypes.QuerySmartContractStateRequest{
		Address:   contractAddr,
		QueryData: msg,
	}
	reqBin, err := gogoproto.Marshal(&req)
	if err != nil {
		return err
	}

	res, err := chain.App.Query(context.TODO(), &abci.RequestQuery{
		Path: "/cosmwasm.wasm.v1.Query/SmartContractState",
		Data: reqBin,
	})
	require.NoError(chain.TB, err)

	if res.Code != 0 {
		return fmt.Errorf("smart query failed: (%d) %s", res.Code, res.Log)
	}

	// unpack protobuf
	var resp wasmtypes.QuerySmartContractStateResponse
	err = gogoproto.Unmarshal(res.Value, &resp)
	if err != nil {
		return err
	}
	// unpack json content
	return json.Unmarshal(resp.Data, response)
}

func MsgRecvPacketWithResultV2(endpoint *Endpoint, packet channeltypesv2.Packet) (*abci.ExecTxResult, error) {
	// get proof of packet commitment from chainA
	packetKey := hostv2.PacketCommitmentKey(packet.SourceClient, packet.Sequence)
	proof, proofHeight := endpoint.Counterparty.QueryProof(packetKey)

	msg := channeltypesv2.NewMsgRecvPacket(packet, proof, proofHeight, endpoint.Chain.SenderAccount.GetAddress().String())

	res, err := endpoint.Chain.SendMsgs(msg)
	if err != nil {
		return nil, err
	}

	return res, endpoint.Counterparty.UpdateClient()
}

// TimeoutPendingPackets returns the package to source chain to let the IBC app revert any operation.
// from A to B
func TimeoutPendingPackets(coord *Coordinator, path *Path) error {
	src, dst := path.EndpointA, path.EndpointB
	srcChain, dstChain := src.Chain, dst.Chain

	toSend := srcChain.PendingSendPackets
	coord.Logf("Timeout %d Packets A->B\n", len(*toSend))
	require.NoError(coord, src.UpdateClient())

	// Increment time and commit block so that 5 second delay period passes between send and receive
	coord.IncrementTime()
	coord.CommitBlock(src.Chain, dstChain)
	for _, packet := range *toSend {
		// get proof of packet unreceived on dest
		packetKey := host.PacketReceiptKey(packet.GetDestPort(), packet.GetDestChannel(), packet.GetSequence())
		proofUnreceived, proofHeight := dst.QueryProof(packetKey)
		timeoutMsg := channeltypes.NewMsgTimeout(packet, packet.Sequence, proofUnreceived, proofHeight, srcChain.SenderAccount.GetAddress().String())
		_, err := srcChain.SendMsgs(timeoutMsg)
		if err != nil {
			return err
		}
	}
	*srcChain.PendingSendPackets = []channeltypes.Packet{}
	return nil
}

// TimeoutPendingPacketsV2 returns the package to source chain to let the IBCv2 app revert any operation.
// from A to B
func TimeoutPendingPacketsV2(coord *Coordinator, path *Path) error {
	src, dst := path.EndpointA, path.EndpointB
	srcChain := src.Chain

	toSend := srcChain.PendingSendPacketsV2
	coord.Logf("Timeout %d Packets A->B\n", len(*toSend))
	require.NoError(coord, src.UpdateClient())

	// Increment time and commit block so that 1 minute delay period passes between send and receive
	coord.IncrementTimeBy(time.Minute)
	err := src.UpdateClient()
	require.NoError(coord, err)
	for _, packet := range *toSend {
		// get proof of packet unreceived on dest
		packetKey := hostv2.PacketReceiptKey(packet.GetDestinationClient(), packet.GetSequence())
		proofUnreceived, proofHeight := dst.QueryProof(packetKey)
		timeoutMsg := channeltypesv2.NewMsgTimeout(packet, proofUnreceived, proofHeight, src.Chain.SenderAccount.GetAddress().String())
		_, err := srcChain.SendMsgs(timeoutMsg)
		if err != nil {
			return err
		}
	}
	*srcChain.PendingSendPackets = []channeltypes.Packet{}
	return nil
}

// CloseChannel close channel on both sides
func CloseChannel(coord *Coordinator, path *Path) {
	src, dst := path.EndpointA, path.EndpointB
	err := src.ChanCloseInit()
	require.NoError(coord, err)
	coord.IncrementTime()
	err = dst.UpdateClient()
	require.NoError(coord, err)
	channelKey := host.ChannelKey(dst.Counterparty.ChannelConfig.PortID, dst.Counterparty.ChannelID)
	proof, proofHeight := dst.Counterparty.QueryProof(channelKey)
	msg := channeltypes.NewMsgChannelCloseConfirm(
		dst.ChannelConfig.PortID, dst.ChannelID,
		proof, proofHeight,
		dst.Chain.SenderAccount.GetAddress().String(),
	)
	_, err = dst.Chain.SendMsgs(msg)
	require.NoError(coord, err)
}
