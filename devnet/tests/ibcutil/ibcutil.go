package ibcutil

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
)

const (
	DefaultPortID  = "transfer"
	commandTimeout = 20 * time.Second
	longTimeout    = 120 * time.Second
)

type ChannelInfo struct {
	ChannelID            string `json:"channel_id"`
	PortID               string `json:"port_id"`
	CounterpartyChainID  string `json:"counterparty_chain_id"`
	CounterpartyChannel  string `json:"counterparty_channel_id"`
	AChainID             string `json:"a_chain_id"`
	BChainID             string `json:"b_chain_id"`
	CounterpartyClientID string `json:"counterparty_client_id"`
}

type Channel struct {
	State          string   `json:"state"`
	PortID         string   `json:"port_id"`
	ChannelID      string   `json:"channel_id"`
	ConnectionHops []string `json:"connection_hops"`
	Counterparty   struct {
		PortID    string `json:"port_id"`
		ChannelID string `json:"channel_id"`
	} `json:"counterparty"`
}

type ChannelsResponse struct {
	Channels []Channel `json:"channels"`
}

type Connection struct {
	ID           string `json:"id"`
	ClientID     string `json:"client_id"`
	State        string `json:"state"`
	Counterparty struct {
		ClientID     string `json:"client_id"`
		ConnectionID string `json:"connection_id"`
	} `json:"counterparty"`
}

type ConnectionsResponse struct {
	Connections []Connection `json:"connections"`
}

func LoadChannelInfo(path string) (ChannelInfo, error) {
	var info ChannelInfo
	data, err := os.ReadFile(path)
	if err != nil {
		return info, fmt.Errorf("read channel info: %w", err)
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return info, fmt.Errorf("parse channel info: %w", err)
	}
	return info, nil
}

func RunJSON(bin string, args ...string) ([]byte, error) {
	out, err := runWithTimeout(commandTimeout, bin, args...)
	return out, err
}

func RunWithOutput(bin string, args ...string) (string, error) {
	out, err := runWithTimeout(commandTimeout, bin, args...)
	return string(out), err
}

func QueryChannels(bin, rpc string) ([]Channel, error) {
	args := []string{"q", "ibc", "channel", "channels", "--output", "json"}
	args = append(args, nodeArgs(rpc)...)
	out, err := RunJSON(bin, args...)
	if err != nil {
		return nil, fmt.Errorf("query channels: %w", err)
	}
	var resp ChannelsResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("parse channels: %w", err)
	}
	return resp.Channels, nil
}

func QueryConnections(bin, rpc string) ([]Connection, error) {
	args := []string{"q", "ibc", "connection", "connections", "--output", "json"}
	args = append(args, nodeArgs(rpc)...)
	out, err := RunJSON(bin, args...)
	if err != nil {
		return nil, fmt.Errorf("query connections: %w", err)
	}
	var resp ConnectionsResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("parse connections: %w", err)
	}
	return resp.Connections, nil
}

func QueryClientStatus(bin, rpc, clientID string) (string, error) {
	args := []string{"q", "ibc", "client", "status", clientID, "--output", "json"}
	args = append(args, nodeArgs(rpc)...)
	out, err := RunJSON(bin, args...)
	if err != nil {
		return "", fmt.Errorf("query client status: %w", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", fmt.Errorf("parse client status: %w", err)
	}
	status := getStringFromAny(resp["status"])
	if status == "" {
		return "", fmt.Errorf("client status not found in response")
	}
	return status, nil
}

func QueryChannelClientState(bin, rpc, portID, channelID string) (string, int64, string, error) {
	args := []string{"q", "ibc", "channel", "client-state", portID, channelID, "--output", "json"}
	args = append(args, nodeArgs(rpc)...)
	out, err := RunJSON(bin, args...)
	if err != nil {
		return "", 0, "", fmt.Errorf("query channel client-state: %w", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", 0, "", fmt.Errorf("parse client-state: %w", err)
	}

	clientID := getStringPath(resp, "identified_client_state", "client_id")
	if clientID == "" {
		clientID = getStringPath(resp, "client_id")
	}

	clientState := getMapPath(resp, "identified_client_state", "client_state")
	if len(clientState) == 0 {
		clientState = getMapPath(resp, "client_state")
	}

	clientType := getStringPath(clientState, "@type")
	heightStr := getStringPath(clientState, "latest_height", "revision_height")
	height, _ := strconv.ParseInt(heightStr, 10, 64)

	return clientID, height, clientType, nil
}

func QueryBalance(bin, rpc, address, denom string) (int64, error) {
	args := []string{"q", "bank", "balances", address, "--output", "json"}
	args = append(args, nodeArgs(rpc)...)
	out, err := RunJSON(bin, args...)
	if err != nil {
		return 0, fmt.Errorf("query balance: %w", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(out, &resp); err != nil {
		return 0, fmt.Errorf("parse balance: %w", err)
	}
	balances, ok := resp["balances"].([]any)
	if !ok {
		return 0, nil
	}
	for _, b := range balances {
		m, ok := b.(map[string]any)
		if !ok {
			continue
		}
		if getStringFromAny(m["denom"]) == denom {
			amtStr := getStringFromAny(m["amount"])
			amt, _ := strconv.ParseInt(amtStr, 10, 64)
			return amt, nil
		}
	}
	return 0, nil
}

func SendIBCTransfer(bin, rpc, home, fromKey, portID, channelID, recipient, amount, chainID, keyring, gasPrices string) error {
	args := []string{}
	if home != "" {
		args = append(args, "--home", home)
	}
	args = append(args,
		"tx", "ibc-transfer", "transfer",
		portID, channelID, recipient, amount,
		"--from", fromKey,
		"--chain-id", chainID,
		"--keyring-backend", keyring,
		"--gas", "auto",
		"--gas-adjustment", "1.3",
		"--broadcast-mode", "sync",
		"--yes",
		"--packet-timeout-height", "0-0",
		"--packet-timeout-timestamp", "600000000000", // 10 minutes
	)
	if gasPrices != "" {
		args = append(args, "--gas-prices", gasPrices)
	}
	args = append(args, nodeArgs(rpc)...)
	_, err := runWithTimeout(longTimeout, bin, args...)
	if err != nil {
		return fmt.Errorf("send ibc transfer: %w", err)
	}
	return nil
}

func WaitForBalanceIncrease(bin, rpc, address, denom string, baseline int64, retries int, delay time.Duration) (int64, error) {
	for i := 0; i < retries; i++ {
		current, err := QueryBalance(bin, rpc, address, denom)
		if err != nil {
			return 0, err
		}
		if current > baseline {
			return current, nil
		}
		time.Sleep(delay)
	}
	return 0, fmt.Errorf("balance for %s did not increase after %d retries", address, retries)
}

func IBCDenom(portID, channelID, denom string) string {
	trace := fmt.Sprintf("%s/%s/%s", portID, channelID, denom)
	return transfertypes.ParseDenomTrace(trace).IBCDenom()
}

func QueryBalanceREST(restAddr, address, denom string) (int64, error) {
	if restAddr == "" {
		return 0, fmt.Errorf("rest address is required")
	}
	url := strings.TrimSuffix(restAddr, "/") + "/cosmos/bank/v1beta1/balances/" + address
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("query balance (rest): %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read balance response: %w", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, fmt.Errorf("parse balance response: %w", err)
	}
	if code, ok := payload["code"]; ok && getStringFromAny(code) != "" {
		return 0, nil
	}

	balances, ok := payload["balances"].([]any)
	if !ok {
		return 0, nil
	}
	for _, b := range balances {
		m, ok := b.(map[string]any)
		if !ok {
			continue
		}
		if getStringFromAny(m["denom"]) == denom {
			amtStr := getStringFromAny(m["amount"])
			amt, _ := strconv.ParseInt(amtStr, 10, 64)
			return amt, nil
		}
	}
	return 0, nil
}

func WaitForBalanceIncreaseREST(restAddr, address, denom string, baseline int64, retries int, delay time.Duration) (int64, error) {
	for i := 0; i < retries; i++ {
		current, err := QueryBalanceREST(restAddr, address, denom)
		if err != nil {
			return 0, err
		}
		if current > baseline {
			return current, nil
		}
		time.Sleep(delay)
	}
	return 0, fmt.Errorf("balance for %s did not increase after %d retries", address, retries)
}

func ReadAddress(path string) (string, error) {
	bz, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	addr := strings.TrimSpace(string(bz))
	if addr == "" {
		return "", fmt.Errorf("address file %s is empty", path)
	}
	return addr, nil
}

func FindChannelByID(channels []Channel, portID, channelID string) *Channel {
	for i := range channels {
		if channels[i].PortID == portID && channels[i].ChannelID == channelID {
			return &channels[i]
		}
	}
	return nil
}

func FindChannelByCounterparty(channels []Channel, portID, counterpartyChannelID string) *Channel {
	for i := range channels {
		if channels[i].PortID == portID &&
			channels[i].Counterparty.ChannelID == counterpartyChannelID {
			return &channels[i]
		}
	}
	return nil
}

func FirstChannelByPort(channels []Channel, portID string) *Channel {
	for i := range channels {
		if channels[i].PortID == portID {
			return &channels[i]
		}
	}
	return nil
}

func FindConnectionByID(conns []Connection, id string) *Connection {
	for i := range conns {
		if conns[i].ID == id {
			return &conns[i]
		}
	}
	return nil
}

func FirstOpenConnection(conns []Connection) *Connection {
	for i := range conns {
		if IsOpenState(conns[i].State) {
			return &conns[i]
		}
	}
	return nil
}

func IsOpenState(state string) bool {
	s := strings.ToUpper(strings.TrimSpace(state))
	return s == "STATE_OPEN" || s == "OPEN"
}

func IsActiveStatus(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "active")
}

func nodeArgs(rpc string) []string {
	if rpc == "" {
		return nil
	}
	return []string{"--node", rpc}
}

func getMapPath(m map[string]any, path ...string) map[string]any {
	var cur any = m
	for _, p := range path {
		next, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = next[p]
	}
	if out, ok := cur.(map[string]any); ok {
		return out
	}
	return nil
}

func getStringPath(m map[string]any, path ...string) string {
	var cur any = m
	for _, p := range path {
		next, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = next[p]
	}
	return getStringFromAny(cur)
}

func getStringFromAny(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatInt(int64(val), 10)
	case json.Number:
		return val.String()
	default:
		return ""
	}
}

func runWithTimeout(timeout time.Duration, bin string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("command timed out after %s: %s", timeout, strings.TrimSpace(string(out)))
	}
	if err != nil {
		return out, fmt.Errorf("command failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}
