package ibctesting

import (
	"fmt"
	"context"
	"strings"
	"bytes"
	"os"
	"compress/gzip"
	"encoding/json"

	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/libs/rand"

	ibcchanneltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibchost "github.com/cosmos/ibc-go/v10/modules/core/24-host"
	
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	gogoproto "github.com/cosmos/gogoproto/proto"
)

var wasmIdent       = []byte("\x00\x61\x73\x6D")

func (chain *TestChain) CaptureIBCEvents(result *abci.ExecTxResult) {
	toSend, _ := ParsePacketsFromEvents(ibcchanneltypes.EventTypeSendPacket, result.Events)
	// require.NoError(chain, err)
	if len(toSend) > 0 {
		// Keep a queue on the chain that we can relay in tests
		*chain.PendingSendPackets = append(*chain.PendingSendPackets, toSend...)
	}
}

func (chain *TestChain) OverrideSendMsgs(msgs ...sdk.Msg) (*abci.ExecTxResult, error) {
	chain.SendMsgsOverride = nil
	result, err := chain.SendMsgs(msgs...)
	chain.SendMsgsOverride = chain.OverrideSendMsgs
	chain.CaptureIBCEvents(result)
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
		Amount:      sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, amount)),
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

// RelayPacketWithoutAck attempts to relay the packet first on EndpointA and then on EndpointB
// if EndpointA does not contain a packet commitment for that packet. An error is returned
// if a relay step fails or the packet commitment does not exist on either endpoint.
// In contrast to RelayPacket, this function does not acknowledge the packet and expects it to have no acknowledgement yet.
// It is useful for testing async acknowledgement.
func RelayPacketWithoutAck(path *Path, packet ibcchanneltypes.Packet) error {
	pc := path.EndpointA.Chain.App.GetIBCKeeper().ChannelKeeper.GetPacketCommitment(path.EndpointA.Chain.GetContext(), packet.GetSourcePort(), packet.GetSourceChannel(), packet.GetSequence())
	if bytes.Equal(pc, ibcchanneltypes.CommitPacket(packet)) {

		// packet found, relay from A to B
		if err := path.EndpointB.UpdateClient(); err != nil {
			return err
		}

		res, err := path.EndpointB.RecvPacketWithResult(packet)
		if err != nil {
			return err
		}

		_, err = ParseAckFromEvents(res.GetEvents())
		if err == nil {
			return fmt.Errorf("tried to relay packet without ack but got ack")
		}

		return nil
	}

	pc = path.EndpointB.Chain.App.GetIBCKeeper().ChannelKeeper.GetPacketCommitment(path.EndpointB.Chain.GetContext(), packet.GetSourcePort(), packet.GetSourceChannel(), packet.GetSequence())
	if bytes.Equal(pc, ibcchanneltypes.CommitPacket(packet)) {

		// packet found, relay B to A
		if err := path.EndpointA.UpdateClient(); err != nil {
			return err
		}

		res, err := path.EndpointA.RecvPacketWithResult(packet)
		if err != nil {
			return err
		}

		_, err = ParseAckFromEvents(res.GetEvents())
		if err == nil {
			return fmt.Errorf("tried to relay packet without ack but got ack")
		}

		return nil
	}

	return fmt.Errorf("packet commitment does not exist on either endpoint for provided packet")
}

type WasmPath struct {
	Path

	chainA *TestChain
	chainB *TestChain
}

func NewWasmPath(chainA, chainB *TestChain) *WasmPath {
	return &WasmPath{
		Path:   *NewPath(chainA, chainB),
		chainA: chainA,
		chainB: chainB,
	}
}

// RelayAndAckPendingPackets sends pending packages from path.EndpointA to the counterparty chain and acks
func RelayAndAckPendingPackets(path *WasmPath) error {
	// get all the packet to relay src->dest
	src := path.EndpointA
	require.NoError(path.chainA, src.UpdateClient())
	path.chainA.Logf("Relay: %d Packets A->B, %d Packets B->A\n", len(*path.chainA.PendingSendPackets), len(*path.chainB.PendingSendPackets))
	for _, v := range *path.chainA.PendingSendPackets {
		_, _, err := path.RelayPacketWithResults(v)
		if err != nil {
			return err
		}
		*path.chainA.PendingSendPackets = (*path.chainA.PendingSendPackets)[1:]
	}

	src = path.EndpointB
	require.NoError(path.chainB, src.UpdateClient())
	for _, v := range *path.chainB.PendingSendPackets {
		_, _, err := path.RelayPacketWithResults(v)
		if err != nil {
			return err
		}
		*path.chainB.PendingSendPackets = (*path.chainB.PendingSendPackets)[1:]
	}
	return nil
}

// TimeoutPendingPackets returns the package to source chain to let the IBC app revert any operation.
// from A to B
func TimeoutPendingPackets(coord *Coordinator, path *WasmPath) error {
	src := path.EndpointA
	dest := path.EndpointB

	toSend := path.chainA.PendingSendPackets
	coord.Logf("Timeout %d Packets A->B\n", len(*toSend))
	require.NoError(coord, src.UpdateClient())

	// Increment time and commit block so that 5 second delay period passes between send and receive
	coord.IncrementTime()
	coord.CommitBlock(src.Chain, dest.Chain)
	for _, packet := range *toSend {
		// get proof of packet unreceived on dest
		packetKey := ibchost.PacketReceiptKey(packet.GetDestPort(), packet.GetDestChannel(), packet.GetSequence())
		proofUnreceived, proofHeight := dest.QueryProof(packetKey)
		timeoutMsg := ibcchanneltypes.NewMsgTimeout(packet, packet.Sequence, proofUnreceived, proofHeight, src.Chain.SenderAccount.GetAddress().String())
		_, err := path.chainA.SendMsgs(timeoutMsg)
		if err != nil {
			return err
		}
	}
	*path.chainA.PendingSendPackets = []ibcchanneltypes.Packet{}
	return nil
}

// CloseChannel close channel on both sides
func CloseChannel(coord *Coordinator, path *Path) {
	err := path.EndpointA.ChanCloseInit()
	require.NoError(coord, err)
	coord.IncrementTime()
	err = path.EndpointB.UpdateClient()
	require.NoError(coord, err)
	channelKey := ibchost.ChannelKey(path.EndpointB.Counterparty.ChannelConfig.PortID, path.EndpointB.Counterparty.ChannelID)
	proof, proofHeight := path.EndpointB.Counterparty.QueryProof(channelKey)
	msg := ibcchanneltypes.NewMsgChannelCloseConfirm(
		path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID,
		proof, proofHeight,
		path.EndpointB.Chain.SenderAccount.GetAddress().String(),
	)
	_, err = path.EndpointB.Chain.SendMsgs(msg)
	require.NoError(coord, err)
}

