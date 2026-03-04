package validator

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"gen/tests/ibcutil"

	textutil "github.com/LumeraProtocol/lumera/pkg/text"
	pkgversion "github.com/LumeraProtocol/lumera/pkg/version"
	"github.com/stretchr/testify/suite"
)

const (
	defaultChannelInfoPath = "/shared/status/hermes/channel_transfer.json"
	defaultLumeraBin       = "lumerad"
	defaultLumeraRPC       = "http://supernova_validator_1:26657"
	defaultLumeraChainID   = "lumera-devnet-1"
	defaultLumeraKeyName   = "hermes-relayer"
	defaultLumeraGasPrices = "0.025ulume"
	defaultLumeraDenom     = "ulume"
	defaultSimdAddrFile    = "/shared/hermes/simd-test.address"
	defaultSimdREST        = "http://hermes:1317"
	defaultValidatorsFile  = "/shared/config/validators.json"
	defaultIBCRetries      = 40
	defaultIBCRetryDelay   = 3 * time.Second
)

type lumeraValidatorSuite struct {
	suite.Suite
	channelInfoPath string
	lumeraBin       string
	lumeraRPC       string
	portID          string
	channelID       string
	lumeraChainID   string
	lumeraKeyName   string
	lumeraGasPrices string
	lumeraDenom     string
	simdRecipient   string
	simdREST        string

	info         ibcutil.ChannelInfo
	channels     []ibcutil.Channel
	channel      *ibcutil.Channel
	connections  []ibcutil.Connection
	connection   *ibcutil.Connection
	clientStatus string
	csClientID   string
	csHeight     int64
	csType       string
}

func (s *lumeraValidatorSuite) SetupSuite() {
	// Load environment-driven configuration and shared channel metadata.
	s.channelInfoPath = textutil.EnvOrDefault("CHANNEL_INFO_FILE", defaultChannelInfoPath)
	s.lumeraBin = textutil.EnvOrDefault("LUMERA_BIN", defaultLumeraBin)
	s.lumeraRPC = resolveLumeraRPC()
	s.lumeraChainID = textutil.EnvOrDefault("LUMERA_CHAIN_ID", defaultLumeraChainID)
	if val := os.Getenv("LUMERA_KEY_NAME"); val != "" {
		s.lumeraKeyName = val
	} else {
		s.lumeraKeyName = resolveLumeraKeyName()
	}
	s.lumeraGasPrices = textutil.EnvOrDefault("LUMERA_GAS_PRICES", defaultLumeraGasPrices)
	s.lumeraDenom = textutil.EnvOrDefault("LUMERA_DENOM", defaultLumeraDenom)
	s.simdREST = textutil.EnvOrDefault("SIMD_REST_ADDR", defaultSimdREST)

	info, err := ibcutil.LoadChannelInfo(s.channelInfoPath)
	s.Require().NoError(err, "load channel info")
	s.info = info
	counterpartyChain := info.CounterpartyChainID
	if counterpartyChain == "" && info.AChainID != "" && info.BChainID != "" {
		if s.lumeraChainID == info.AChainID {
			counterpartyChain = info.BChainID
		} else if s.lumeraChainID == info.BChainID {
			counterpartyChain = info.AChainID
		}
	}
	info.CounterpartyChainID = counterpartyChain
	s.T().Logf("Loaded channel info: port=%s channel=%s counterparty_chain=%s a_chain=%s b_chain=%s",
		info.PortID, info.ChannelID, info.CounterpartyChainID, info.AChainID, info.BChainID)
	s.T().Logf("Using lumera key name: %s", s.lumeraKeyName)

	// Resolve port/channel IDs from env or the generated channel info file.
	portID := textutil.EnvOrDefault("PORT_ID", "")
	if portID == "" {
		portID = info.PortID
	}
	if portID == "" {
		portID = ibcutil.DefaultPortID
	}
	s.portID = portID

	s.channelID = textutil.EnvOrDefault("CHANNEL_ID", info.ChannelID)
	s.Require().NotEmpty(s.channelID, "channel_id missing in %s", s.channelInfoPath)

	// Default simd recipient from shared file for transfer tests.
	simdAddrFile := textutil.EnvOrDefault("SIMD_RECIPIENT_ADDR_FILE", defaultSimdAddrFile)
	addr, err := ibcutil.ReadAddress(simdAddrFile)
	s.Require().NoError(err, "read simd recipient address")
	s.simdRecipient = addr

	s.T().Logf("Testing IBC on Lumera (port=%s channel=%s rpc=%s)", s.portID, s.channelID, s.lumeraRPC)

	// Discover channel/connection/client on lumera and cache it for the suite.
	channels, err := ibcutil.QueryChannels(s.lumeraBin, s.lumeraRPC)
	s.Require().NoError(err, "query channels")
	s.channels = channels
	s.T().Logf("Discovered %d lumera channels", len(channels))
	for _, ch := range channels {
		s.T().Logf("lumera channel: port=%s channel=%s state=%s counterparty_port=%s counterparty_channel=%s conn_hops=%v",
			ch.PortID, ch.ChannelID, ch.State, ch.Counterparty.PortID, ch.Counterparty.ChannelID, ch.ConnectionHops)
	}

	channel := ibcutil.FindChannelByID(channels, s.portID, s.channelID)
	s.Require().NotNil(channel, "channel %s/%s not found", s.portID, s.channelID)
	s.channel = channel

	s.Require().NotEmpty(channel.ConnectionHops, "channel %s/%s missing connection hop", channel.PortID, channel.ChannelID)
	connectionID := channel.ConnectionHops[0]
	s.T().Logf("Channel open; connection=%s counterparty_channel=%s", connectionID, channel.Counterparty.ChannelID)

	connections, err := ibcutil.QueryConnections(s.lumeraBin, s.lumeraRPC)
	s.Require().NoError(err, "query connections")
	s.connections = connections
	s.T().Logf("Discovered %d lumera connections", len(connections))
	for _, conn := range connections {
		s.T().Logf("lumera connection: id=%s state=%s client_id=%s counterparty_client_id=%s counterparty_connection_id=%s",
			conn.ID, conn.State, conn.ClientID, conn.Counterparty.ClientID, conn.Counterparty.ConnectionID)
	}

	connection := ibcutil.FindConnectionByID(connections, connectionID)
	if connection == nil {
		connection = ibcutil.FirstOpenConnection(connections)
		s.Require().NotNil(connection, "connection %s not found and no open connections", connectionID)
		s.T().Logf("connection %s not found; using open connection %s", connectionID, connection.ID)
	}
	s.connection = connection
	s.Require().NotEmpty(connection.ClientID, "connection %s missing client_id", connection.ID)

	// Capture client status and channel client-state for dedicated assertions.
	status, err := ibcutil.QueryClientStatus(s.lumeraBin, s.lumeraRPC, connection.ClientID)
	s.Require().NoError(err, "query client status")
	s.clientStatus = status

	csClientID, csHeight, csType, err := ibcutil.QueryChannelClientState(s.lumeraBin, s.lumeraRPC, s.portID, s.channelID)
	s.Require().NoError(err, "query channel client-state")
	s.csClientID = csClientID
	s.csHeight = csHeight
	s.csType = csType
}

func (s *lumeraValidatorSuite) TestChannelOpen() {
	s.Require().NotNil(s.channel, "channel is nil")
	s.True(ibcutil.IsOpenState(s.channel.State), "channel %s/%s not open: %s", s.channel.PortID, s.channel.ChannelID, s.channel.State)
}

func (s *lumeraValidatorSuite) TestConnectionOpen() {
	s.Require().NotNil(s.connection, "connection is nil")
	s.True(ibcutil.IsOpenState(s.connection.State), "connection %s not open: %s", s.connection.ID, s.connection.State)
}

func (s *lumeraValidatorSuite) TestClientActive() {
	s.True(ibcutil.IsActiveStatus(s.clientStatus), "client %s not active: %s", s.connection.ClientID, s.clientStatus)
}

func (s *lumeraValidatorSuite) TestChannelClientState() {
	if s.csClientID != "" {
		s.Equal(s.connection.ClientID, s.csClientID, "client-state mismatch")
	}
	s.Greater(s.csHeight, int64(0), "client-state latest_height not positive")
	s.T().Logf("Client status active; client-state height=%d type=%s", s.csHeight, s.csType)
}

func (s *lumeraValidatorSuite) TestTransferToSimd() {
	// Exercise a real packet flow from lumera -> simd and confirm balance change.
	amount := textutil.EnvOrDefault("LUMERA_IBC_AMOUNT", "100"+s.lumeraDenom)
	s.transferFromLumeraToSimdAndAssert(amount)
}

func (s *lumeraValidatorSuite) TestIBCTransferWithEVMModeStillRelays() {
	s.requireLumeraEVMModeOrSkip()
	amount := textutil.EnvOrDefault("LUMERA_IBC_EVM_MODE_AMOUNT", "77"+s.lumeraDenom)
	s.transferFromLumeraToSimdAndAssert(amount)
}

func TestIBCLumeraSideSuite(t *testing.T) {
	suite.Run(t, new(lumeraValidatorSuite))
}

func (s *lumeraValidatorSuite) transferFromLumeraToSimdAndAssert(amount string) {
	// On the destination chain (simd), the voucher denom trace uses the
	// destination-side channel ID (counterparty from lumera's perspective).
	dstChannelID := s.info.CounterpartyChannel
	if dstChannelID == "" && s.channel != nil {
		dstChannelID = s.channel.Counterparty.ChannelID
	}
	s.Require().NotEmpty(dstChannelID, "destination channel id is empty")
	ibcDenom := ibcutil.IBCDenom(s.portID, dstChannelID, s.lumeraDenom)

	before, err := ibcutil.QueryBalanceREST(s.simdREST, s.simdRecipient, ibcDenom)
	s.Require().NoError(err, "query simd recipient balance before")

	err = ibcutil.SendIBCTransfer(
		s.lumeraBin, s.lumeraRPC, "",
		s.lumeraKeyName, s.portID, s.channelID, s.simdRecipient, amount,
		s.lumeraChainID, "test", s.lumeraGasPrices,
	)
	s.Require().NoError(err, "send ibc transfer to simd")

	after, err := ibcutil.WaitForBalanceIncreaseREST(s.simdREST, s.simdRecipient, ibcDenom, before, defaultIBCRetries, defaultIBCRetryDelay)
	s.Require().NoError(err, "wait for simd recipient balance increase")
	s.T().Logf("simd recipient balance increased: %d -> %d", before, after)
}

func (s *lumeraValidatorSuite) requireLumeraEVMModeOrSkip() {
	explicit := strings.ToLower(strings.TrimSpace(os.Getenv("LUMERA_KEY_STYLE")))
	switch explicit {
	case "evm":
		return
	case "cosmos":
		s.T().Skip("skip EVM-mode transfer assertion: LUMERA_KEY_STYLE=cosmos")
		return
	}

	ver, err := resolveLumeraBinaryVersion(s.lumeraBin)
	if err != nil {
		s.T().Skipf("skip EVM-mode transfer assertion: failed to resolve %s version: %v", s.lumeraBin, err)
		return
	}
	if !pkgversion.GTE(ver, firstEVMVersion) {
		s.T().Skipf("skip EVM-mode transfer assertion: %s version %s < %s", s.lumeraBin, ver, firstEVMVersion)
	}
}

func loadPrimaryValidatorKey(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var vals []struct {
		KeyName string `json:"key_name"`
		Primary bool   `json:"primary"`
	}
	if err := json.Unmarshal(data, &vals); err != nil {
		return ""
	}
	for _, v := range vals {
		if v.Primary && v.KeyName != "" {
			return v.KeyName
		}
	}
	if len(vals) > 0 {
		return vals[0].KeyName
	}
	return ""
}

func resolveLumeraRPC() string {
	if val := os.Getenv("LUMERA_RPC_ADDR"); val != "" {
		return val
	}

	if moniker := detectValidatorMoniker(); moniker != "" {
		return fmt.Sprintf("http://%s:26657", moniker)
	}

	return defaultLumeraRPC
}

func resolveLumeraKeyName() string {
	validatorsPath := textutil.EnvOrDefault("LUMERA_VALIDATORS_FILE", defaultValidatorsFile)
	if moniker := detectValidatorMoniker(); moniker != "" {
		if key := loadValidatorKeyByMoniker(validatorsPath, moniker); key != "" {
			return key
		}
	}

	if key := loadPrimaryValidatorKey(validatorsPath); key != "" {
		return key
	}

	return defaultLumeraKeyName
}

func detectValidatorMoniker() string {
	if val := strings.TrimSpace(os.Getenv("MONIKER")); val != "" {
		return val
	}

	if val := strings.TrimSpace(os.Getenv("HOSTNAME")); val != "" {
		if moniker := normalizeMoniker(val); moniker != "" {
			return moniker
		}
	}

	host, err := os.Hostname()
	if err != nil {
		return ""
	}
	return normalizeMoniker(host)
}

func normalizeMoniker(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	host = strings.TrimPrefix(host, "lumera-")
	if strings.HasPrefix(host, "supernova_validator_") {
		return host
	}
	return ""
}

func loadValidatorKeyByMoniker(path, moniker string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var vals []struct {
		Name    string `json:"name"`
		Moniker string `json:"moniker"`
		KeyName string `json:"key_name"`
	}
	if err := json.Unmarshal(data, &vals); err != nil {
		return ""
	}
	for _, v := range vals {
		if v.Moniker == moniker || v.Name == moniker {
			return v.KeyName
		}
	}
	return ""
}
