package common

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SupernodeStateEntry is one state-history entry for a supernode.
type SupernodeStateEntry struct {
	State  string `json:"state"`
	Height string `json:"height"`
	Reason string `json:"reason"`
}

// SupernodeIPEntry is one IP-history entry for a supernode.
type SupernodeIPEntry struct {
	Address string `json:"address"`
	Height  string `json:"height"`
}

// SupernodeRecord holds the supernode state returned by the CLI query.
type SupernodeRecord struct {
	ValidatorAddress string                `json:"validator_address"`
	SupernodeAccount string                `json:"supernode_account"`
	States           []SupernodeStateEntry `json:"states"`
	PrevIPAddresses  []SupernodeIPEntry    `json:"prev_ip_addresses"`
}

// supernodeGatewayStatus mirrors the supernode HTTP gateway status response.
type supernodeGatewayStatus struct {
	Network *struct {
		PeersCount int32 `json:"peers_count"`
	} `json:"network"`
}

// LatestSupernodeState returns the state string from the highest-height entry.
func LatestSupernodeState(sn *SupernodeRecord) string {
	if sn == nil {
		return ""
	}
	best, bestHeight := "", int64(-1)
	for _, s := range sn.States {
		if h := parseHeight(s.Height); h > bestHeight {
			bestHeight = h
			best = strings.TrimSpace(s.State)
		}
	}
	return best
}

// LatestSupernodeHost returns the host (port stripped) of the highest-height
// registered supernode endpoint.
func LatestSupernodeHost(sn *SupernodeRecord) string {
	if sn == nil {
		return ""
	}
	best, bestHeight := "", int64(-1)
	for _, ip := range sn.PrevIPAddresses {
		if h := parseHeight(ip.Height); h > bestHeight {
			bestHeight = h
			best = strings.TrimSpace(ip.Address)
		}
	}
	if best == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(best); err == nil && host != "" {
		return host
	}
	return best
}

func parseHeight(s string) int64 {
	h, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return -1
	}
	return h
}

// SupernodeByValoper queries a supernode record by validator operator address.
// Returns (nil, nil) when no supernode is registered.
func (c *ChainCLI) SupernodeByValoper(valoper string) (*SupernodeRecord, error) {
	out, err := c.Run("query", "supernode", "get-supernode", valoper)
	if err != nil {
		if strings.Contains(out, "not found") || strings.Contains(out, "rpc error") {
			return nil, nil
		}
		return nil, fmt.Errorf("query supernode %s: %s: %w", valoper, truncate(out, 200), err)
	}
	var resp struct {
		SuperNode SupernodeRecord `json:"supernode"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, fmt.Errorf("parse supernode %s: %w", valoper, err)
	}
	return &resp.SuperNode, nil
}

// querySupernodeGatewayStatus queries the supernode HTTP gateway status API.
func querySupernodeGatewayStatus(host string) (*supernodeGatewayStatus, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, fmt.Errorf("empty supernode host")
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s:8002/api/v1/status?include_p2p_metrics=true", host))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}
	var status supernodeGatewayStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}

// WaitForReadySupernodes polls until at least one ACTIVE supernode reports
// peers > 1 on its gateway status API, or the timeout expires. It satisfies the
// gen-activity supernodeGate interface.
func (c *ChainCLI) WaitForReadySupernodes(validators []string, timeout time.Duration) bool {
	if len(validators) == 0 {
		return false
	}
	deadline := time.Now().Add(timeout)
	lastActive, lastStatus, lastPeers := -1, -1, -1
	for {
		active, statusReported, peersReady := 0, 0, 0
		for _, valoper := range validators {
			sn, err := c.SupernodeByValoper(valoper)
			if err != nil || sn == nil || sn.SupernodeAccount == "" || LatestSupernodeState(sn) != "SUPERNODE_STATE_ACTIVE" {
				continue
			}
			active++
			status, err := querySupernodeGatewayStatus(LatestSupernodeHost(sn))
			if err != nil || status == nil {
				continue
			}
			statusReported++
			if status.Network != nil && status.Network.PeersCount > 1 {
				peersReady++
			}
		}
		if active != lastActive || statusReported != lastStatus || peersReady != lastPeers {
			log.Printf("  INFO: cascade supernode readiness: active=%d status_reported=%d peers_ready=%d total=%d",
				active, statusReported, peersReady, len(validators))
			lastActive, lastStatus, lastPeers = active, statusReported, peersReady
		}
		if peersReady > 0 {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(2 * time.Second)
	}
}
