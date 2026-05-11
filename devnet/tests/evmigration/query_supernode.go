// query_supernode.go provides query helpers for the supernode module: fetching
// supernode records, metrics state, and waiting for cascade-eligible supernodes.
package main

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

// ---------------------------------------------------------------------------
// Supernode queries
// ---------------------------------------------------------------------------

// SuperNodeRecord holds the supernode state returned by the CLI query.
type SuperNodeRecord struct {
	ValidatorAddress string `json:"validator_address"`
	SupernodeAccount string `json:"supernode_account"`
	P2PPort          string `json:"p2p_port"`
	Note             string `json:"note"`

	States []struct {
		State  string `json:"state"`
		Height string `json:"height"`
		Reason string `json:"reason"`
	} `json:"states"`

	Evidence []SuperNodeEvidence `json:"evidence"`

	PrevIPAddresses []struct {
		Address string `json:"address"`
		Height  string `json:"height"`
	} `json:"prev_ip_addresses"`

	PrevSupernodeAccounts []SuperNodeAccountHistory `json:"prev_supernode_accounts"`
}

// SuperNodeEvidence mirrors the Evidence proto.
type SuperNodeEvidence struct {
	ReporterAddress  string `json:"reporter_address"`
	ValidatorAddress string `json:"validator_address"`
	ActionID         string `json:"action_id"`
	EvidenceType     string `json:"evidence_type"`
	Description      string `json:"description"`
	Severity         int    `json:"severity"`
	Height           string `json:"height"`
}

// SuperNodeAccountHistory mirrors SupernodeAccountHistory proto.
type SuperNodeAccountHistory struct {
	Account string `json:"account"`
	Height  string `json:"height"`
}

// SuperNodeMetricsState mirrors SupernodeMetricsState proto.
type SuperNodeMetricsState struct {
	ValidatorAddress string `json:"validator_address"`
	Metrics          *struct {
		PeersCount uint32 `json:"peers_count"`
	} `json:"metrics"`
	ReportCount string `json:"report_count"`
	Height      string `json:"height"`
}

// SupernodeGatewayStatus mirrors the supernode HTTP gateway status response.
type SupernodeGatewayStatus struct {
	Version   string `json:"version"`
	IPAddress string `json:"ip_address"`
	Network   *struct {
		PeersCount int32 `json:"peers_count"`
	} `json:"network"`
}

// querySupernodeByValoper queries the supernode record by its validator operator address.
// Returns nil, nil when no supernode is registered.
func querySupernodeByValoper(valoper string) (*SuperNodeRecord, error) {
	out, err := run("query", "supernode", "get-supernode", valoper)
	if err != nil {
		if strings.Contains(out, "not found") || strings.Contains(out, "rpc error") {
			return nil, nil
		}
		return nil, fmt.Errorf("query supernode %s: %s\n%w", valoper, truncate(out, 300), err)
	}
	var resp struct {
		SuperNode SuperNodeRecord `json:"supernode"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, fmt.Errorf("parse supernode %s: %s\n%w", valoper, truncate(out, 300), err)
	}
	return &resp.SuperNode, nil
}

// querySupernodeMetricsByValoper queries the metrics state for a validator.
// Returns nil, nil when no metrics exist.
func querySupernodeMetricsByValoper(valoper string) (*SuperNodeMetricsState, error) {
	out, err := run(querySupernodeMetricsArgs(valoper)...)
	if err != nil {
		if strings.Contains(out, "not found") || strings.Contains(out, "rpc error") {
			return nil, nil
		}
		return nil, fmt.Errorf("query supernode metrics %s: %s\n%w", valoper, truncate(out, 300), err)
	}
	var resp struct {
		MetricsState SuperNodeMetricsState `json:"metrics_state"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, fmt.Errorf("parse supernode metrics %s: %s\n%w", valoper, truncate(out, 300), err)
	}
	return &resp.MetricsState, nil
}

// querySupernodeMetricsArgs returns the CLI args for querying supernode metrics.
func querySupernodeMetricsArgs(valoper string) []string {
	return []string{"query", "supernode", "get-metrics", valoper}
}

// latestSupernodeState returns the state string from the highest block height entry.
func latestSupernodeState(sn *SuperNodeRecord) string {
	if sn == nil || len(sn.States) == 0 {
		return ""
	}

	bestState := ""
	var bestHeight int64 = -1
	for _, state := range sn.States {
		height, err := strconv.ParseInt(strings.TrimSpace(state.Height), 10, 64)
		if err != nil {
			height = -1
		}
		if height > bestHeight {
			bestHeight = height
			bestState = strings.TrimSpace(state.State)
		}
	}
	return bestState
}

// latestSupernodeHost returns the host portion of the latest registered supernode endpoint.
func latestSupernodeHost(sn *SuperNodeRecord) string {
	if sn == nil || len(sn.PrevIPAddresses) == 0 {
		return ""
	}

	bestAddress := ""
	var bestHeight int64 = -1
	for _, ip := range sn.PrevIPAddresses {
		height, err := strconv.ParseInt(strings.TrimSpace(ip.Height), 10, 64)
		if err != nil {
			height = -1
		}
		if height > bestHeight {
			bestHeight = height
			bestAddress = strings.TrimSpace(ip.Address)
		}
	}
	if bestAddress == "" {
		return ""
	}

	host, _, err := net.SplitHostPort(bestAddress)
	if err == nil && host != "" {
		return host
	}
	return bestAddress
}

// querySupernodeGatewayStatus queries the supernode HTTP gateway status API.
func querySupernodeGatewayStatus(host string) (*SupernodeGatewayStatus, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, fmt.Errorf("empty supernode host")
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s:8002/api/v1/status?include_p2p_metrics=true", host))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var status SupernodeGatewayStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}

// waitForEligibleCascadeSupernodes polls until at least one ACTIVE supernode
// reports peers > 1 on the local status API, or the timeout expires.
func waitForEligibleCascadeSupernodes(validators []string, timeout time.Duration) bool {
	if len(validators) == 0 {
		return false
	}

	deadline := time.Now().Add(timeout)
	lastEligible := -1
	lastStatusReported := -1
	lastPeersReady := -1

	for {
		active := 0
		statusReported := 0
		peersReady := 0

		for _, valoper := range validators {
			sn, err := querySupernodeByValoper(valoper)
			if err != nil || sn == nil || sn.SupernodeAccount == "" || latestSupernodeState(sn) != "SUPERNODE_STATE_ACTIVE" {
				continue
			}
			active++

			status, err := querySupernodeGatewayStatus(latestSupernodeHost(sn))
			if err != nil || status == nil {
				continue
			}
			statusReported++
			if status.Network != nil && status.Network.PeersCount > 1 {
				peersReady++
			}
		}

		if active != lastEligible || statusReported != lastStatusReported || peersReady != lastPeersReady {
			log.Printf("  INFO: cascade supernode readiness: active=%d status_reported=%d peers_ready=%d total=%d", active, statusReported, peersReady, len(validators))
			lastEligible = active
			lastStatusReported = statusReported
			lastPeersReady = peersReady
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
