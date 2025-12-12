package system

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// reportSupernodeMetrics submits a metrics tx via CLI and returns the tx hash.
func reportSupernodeMetrics(t *testing.T, cli *LumeradCli, fromKey, valAddr, account string, metrics sntypes.SupernodeMetrics) string {

	metricsJSON, err := json.Marshal(metrics)
	require.NoError(t, err)

	out := cli.CustomCommand(
		"tx", "supernode", "report-supernode-metrics",
		"--validator-address", valAddr,
		"--metrics", string(metricsJSON),
		"--from", fromKey,
	)
	hash := gjson.Get(out, "txhash").String()
	require.NotEmpty(t, hash, "tx hash missing: %s", out)
	return hash
}

// waitForTx fetches a transaction by hash with retries and returns the raw JSON.
func waitForTx(t *testing.T, cli *LumeradCli, hash string) string {

	var last string
	for i := 0; i < 10; i++ {
		last = cli.CustomQuery("q", "tx", hash)
		if gjson.Get(last, "tx_response.code").Exists() || gjson.Get(last, "code").Exists() {
			return last
		}
		time.Sleep(300 * time.Millisecond)
	}
	require.FailNow(t, "tx not found", "hash=%s last=%s", hash, last)
	return ""
}

type txResponse struct {
	Code   uint32
	RawLog string
	TxHash string
}

// decodeTxResponse extracts minimal tx response fields from JSON output.
func decodeTxResponse(t *testing.T, txJSON string) *txResponse {
	t.Helper()
	resp := txResponse{}
	if gjson.Get(txJSON, "tx_response").Exists() {
		resp.Code = uint32(gjson.Get(txJSON, "tx_response.code").Uint())
		resp.RawLog = gjson.Get(txJSON, "tx_response.raw_log").String()
		resp.TxHash = gjson.Get(txJSON, "tx_response.txhash").String()
		return &resp
	}
	resp.Code = uint32(gjson.Get(txJSON, "code").Uint())
	resp.RawLog = gjson.Get(txJSON, "raw_log").String()
	resp.TxHash = gjson.Get(txJSON, "txhash").String()
	return &resp
}

// decodeMetricsMsg extracts the first message from a tx JSON and unmarshals it into MsgReportSupernodeMetrics.
func decodeMetricsMsg(t *testing.T, txJSON string) sntypes.MsgReportSupernodeMetrics {

	var msg sntypes.MsgReportSupernodeMetrics

	if gjson.Get(txJSON, "tx.body.messages.0.value").Exists() {
		msgB64 := gjson.Get(txJSON, "tx.body.messages.0.value").String()
		require.NotEmpty(t, msgB64, "message not found in tx: %s", txJSON)
		raw, err := base64.StdEncoding.DecodeString(msgB64)
		require.NoError(t, err)
		require.NoError(t, msg.Unmarshal(raw))
		return msg
	}

	msgRaw := gjson.Get(txJSON, "tx.body.messages.0").Raw
	require.NotEmpty(t, msgRaw, "message not found in tx: %s", txJSON)
	require.NoError(t, json.Unmarshal([]byte(msgRaw), &msg))
	return msg
}

// querySupernodeByAddress queries supernode state by account address.
func querySupernodeByAddress(t *testing.T, cli *LumeradCli, account string) *sntypes.QueryGetSuperNodeBySuperNodeAddressResponse {

	out := cli.CustomQuery("q", "supernode", "get-supernode-by-address", account)

	sn := gjson.Get(out, "supernode")
	require.True(t, sn.Exists(), "supernode not found: %s", out)

	rawStates := sn.Get("states").Array()
	states := make([]*sntypes.SuperNodeStateRecord, 0, len(rawStates))
	for _, st := range rawStates {
		stateField := st.Get("state")
		var stateVal sntypes.SuperNodeState
		switch stateField.Type {
		case gjson.String:
			if v, ok := sntypes.SuperNodeState_value[stateField.String()]; ok {
				stateVal = sntypes.SuperNodeState(v)
			} else {
				require.FailNow(t, "unknown supernode state", "state=%s out=%s", stateField.String(), out)
			}
		default:
			stateVal = sntypes.SuperNodeState(stateField.Int())
		}

		states = append(states, &sntypes.SuperNodeStateRecord{
			State:  stateVal,
			Height: st.Get("height").Int(),
		})
	}

	return &sntypes.QueryGetSuperNodeBySuperNodeAddressResponse{
		Supernode: &sntypes.SuperNode{
			ValidatorAddress: sn.Get("validator_address").String(),
			SupernodeAccount: sn.Get("supernode_account").String(),
			P2PPort:          sn.Get("p2p_port").String(),
			States:           states,
		},
	}
}

// querySupernodeParams fetches supernode params.
func querySupernodeParams(t *testing.T, cli *LumeradCli) *sntypes.QueryParamsResponse {

	out := cli.CustomQuery("q", "supernode", "params")
	params := gjson.Get(out, "params")
	require.True(t, params.Exists(), "params not found: %s", out)

	ports := make([]uint32, 0, len(params.Get("required_open_ports").Array()))
	for _, p := range params.Get("required_open_ports").Array() {
		ports = append(ports, uint32(p.Uint()))
	}

	return &sntypes.QueryParamsResponse{
		Params: sntypes.Params{
			RequiredOpenPorts: ports,
			MinCpuCores:       params.Get("min_cpu_cores").Uint(),
		},
	}
}
