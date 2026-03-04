package validator

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	pkgversion "github.com/LumeraProtocol/lumera/pkg/version"
)

const (
	defaultLocalHost       = "127.0.0.1"
	metaMaskExtensionOrigin = "chrome-extension://nkbihfbeogaeaoehlefnkodbefgpgknn"
)

// TestLocalLumeradRequiredPortsAccessible verifies the local validator exposes
// the expected CometBFT/Cosmos endpoints, and JSON-RPC endpoints in EVM mode.
func (s *lumeraValidatorSuite) TestLocalLumeradRequiredPortsAccessible() {
	host := defaultLocalHost
	ports, err := loadLocalLumeradPorts()
	if err != nil {
		s.T().Logf("load local lumerad ports: %v (using defaults for missing values)", err)
	}

	s.requireTCPPortOpen(host, ports.P2P, "cometbft p2p")
	s.requireTCPPortOpen(host, ports.RPC, "cometbft rpc")
	s.requireHTTPOK(fmt.Sprintf("http://%s:%d/status", host, ports.RPC), "cometbft status")

	s.requireTCPPortOpen(host, ports.REST, "cosmos rest")
	s.requireHTTPOK(fmt.Sprintf("http://%s:%d/cosmos/base/tendermint/v1beta1/node_info", host, ports.REST), "rest node_info")

	s.requireTCPPortOpen(host, ports.GRPC, "grpc")

	// JSON-RPC endpoints are expected from the first EVM-enabled Lumera version onward.
	ver, err := resolveLumeraBinaryVersion(s.lumeraBin)
	if err != nil {
		s.T().Skipf("skip json-rpc port checks: failed to resolve %s version: %v", s.lumeraBin, err)
		return
	}
	if !pkgversion.GTE(ver, firstEVMVersion) {
		s.T().Logf("skip json-rpc port checks: %s version %s < %s", s.lumeraBin, ver, firstEVMVersion)
		return
	}
	if !ports.JSONRPCEnabled {
		s.T().Skip("skip json-rpc port checks: json-rpc is disabled in app.toml")
		return
	}

	s.requireTCPPortOpen(host, ports.JSONRPC, "json-rpc")
	rpcAddr := fmt.Sprintf("http://%s:%d", host, ports.JSONRPC)
	var netVersion string
	err = callJSONRPC(rpcAddr, "net_version", []any{}, &netVersion)
	s.Require().NoError(err, "json-rpc net_version")
	s.Require().NotEmpty(netVersion, "json-rpc net_version should not be empty")

	s.requireTCPPortOpen(host, ports.JSONWS, "json-rpc websocket")
}

// TestLocalLumeradJSONRPCCORSAllowsMetaMaskHeaders verifies JSON-RPC preflight
// accepts MetaMask's custom request headers (for example x-metamask-clientid).
func (s *lumeraValidatorSuite) TestLocalLumeradJSONRPCCORSAllowsMetaMaskHeaders() {
	host := defaultLocalHost
	ports, err := loadLocalLumeradPorts()
	if err != nil {
		s.T().Logf("load local lumerad ports: %v (using defaults for missing values)", err)
	}

	ver, err := resolveLumeraBinaryVersion(s.lumeraBin)
	if err != nil {
		s.T().Skipf("skip json-rpc CORS checks: failed to resolve %s version: %v", s.lumeraBin, err)
		return
	}
	if !pkgversion.GTE(ver, firstEVMVersion) {
		s.T().Skipf("skip json-rpc CORS checks: %s version %s < %s", s.lumeraBin, ver, firstEVMVersion)
		return
	}
	if !ports.JSONRPCEnabled {
		s.T().Skip("skip json-rpc CORS checks: json-rpc is disabled in app.toml")
		return
	}

	url := fmt.Sprintf("http://%s:%d", host, ports.JSONRPC)
	req, err := http.NewRequest(http.MethodOptions, url, nil)
	s.Require().NoError(err, "build options preflight request")
	req.Header.Set("Origin", metaMaskExtensionOrigin)
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type,x-metamask-clientid")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	s.Require().NoError(err, "send json-rpc preflight to %s", url)
	defer resp.Body.Close()
	s.Require().Less(resp.StatusCode, http.StatusBadRequest, "json-rpc preflight should not fail: %s", resp.Status)

	allowOrigin := strings.TrimSpace(resp.Header.Get("Access-Control-Allow-Origin"))
	s.Require().NotEmpty(allowOrigin, "preflight response should include Access-Control-Allow-Origin")
	s.Require().True(
		allowOrigin == "*" || strings.EqualFold(allowOrigin, metaMaskExtensionOrigin),
		"unexpected Access-Control-Allow-Origin value: %q", allowOrigin,
	)

	allowHeaders := strings.ToLower(resp.Header.Get("Access-Control-Allow-Headers"))
	s.Require().NotEmpty(allowHeaders, "preflight response should include Access-Control-Allow-Headers")
	s.Require().Contains(allowHeaders, "x-metamask-clientid", "preflight should allow x-metamask-clientid")
}

func (s *lumeraValidatorSuite) requireTCPPortOpen(host string, port int, name string) {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	s.Require().NoError(err, "%s port should be reachable at %s", name, addr)
	if conn != nil {
		_ = conn.Close()
	}
}

func (s *lumeraValidatorSuite) requireHTTPOK(url, name string) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	s.Require().NoError(err, "%s endpoint should be reachable at %s", name, url)
	defer resp.Body.Close()
	s.Require().Less(resp.StatusCode, http.StatusBadRequest, "%s endpoint returned non-success status: %s", name, resp.Status)
}
