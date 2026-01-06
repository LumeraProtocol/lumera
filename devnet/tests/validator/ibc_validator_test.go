package validator

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"gen/tests/ibcutil"
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
)

type ibcLumeraSuite struct {
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

	info        ibcutil.ChannelInfo
	channels    []ibcutil.Channel
	channel     *ibcutil.Channel
	connections []ibcutil.Connection
	connection  *ibcutil.Connection
	clientStatus string
	csClientID   string
	csHeight     int64
	csType       string
}

func (s *ibcLumeraSuite) SetupSuite() {
	// Load environment-driven configuration and shared channel metadata.
	s.channelInfoPath = getenv("CHANNEL_INFO_FILE", defaultChannelInfoPath)
	s.lumeraBin = getenv("LUMERA_BIN", defaultLumeraBin)
	s.lumeraRPC = getenv("LUMERA_RPC_ADDR", defaultLumeraRPC)
	s.lumeraChainID = getenv("LUMERA_CHAIN_ID", defaultLumeraChainID)
	if val := os.Getenv("LUMERA_KEY_NAME"); val != "" {
		s.lumeraKeyName = val
	} else {
		s.lumeraKeyName = defaultLumeraKeyName
		if key := loadPrimaryValidatorKey(getenv("LUMERA_VALIDATORS_FILE", defaultValidatorsFile)); key != "" {
			s.lumeraKeyName = key
		}
	}
	s.lumeraGasPrices = getenv("LUMERA_GAS_PRICES", defaultLumeraGasPrices)
	s.lumeraDenom = getenv("LUMERA_DENOM", defaultLumeraDenom)
	s.simdREST = getenv("SIMD_REST_ADDR", defaultSimdREST)

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
	portID := getenv("PORT_ID", "")
	if portID == "" {
		portID = info.PortID
	}
	if portID == "" {
		portID = ibcutil.DefaultPortID
	}
	s.portID = portID

	s.channelID = getenv("CHANNEL_ID", info.ChannelID)
	s.Require().NotEmpty(s.channelID, "channel_id missing in %s", s.channelInfoPath)

	// Default simd recipient from shared file for transfer tests.
	simdAddrFile := getenv("SIMD_RECIPIENT_ADDR_FILE", defaultSimdAddrFile)
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

func (s *ibcLumeraSuite) TestChannelOpen() {
	s.Require().NotNil(s.channel, "channel is nil")
	s.True(ibcutil.IsOpenState(s.channel.State), "channel %s/%s not open: %s", s.channel.PortID, s.channel.ChannelID, s.channel.State)
}

func (s *ibcLumeraSuite) TestConnectionOpen() {
	s.Require().NotNil(s.connection, "connection is nil")
	s.True(ibcutil.IsOpenState(s.connection.State), "connection %s not open: %s", s.connection.ID, s.connection.State)
}

func (s *ibcLumeraSuite) TestClientActive() {
	s.True(ibcutil.IsActiveStatus(s.clientStatus), "client %s not active: %s", s.connection.ClientID, s.clientStatus)
}

func (s *ibcLumeraSuite) TestChannelClientState() {
	if s.csClientID != "" {
		s.Equal(s.connection.ClientID, s.csClientID, "client-state mismatch")
	}
	s.Greater(s.csHeight, int64(0), "client-state latest_height not positive")
	s.T().Logf("Client status active; client-state height=%d type=%s", s.csHeight, s.csType)
}

func (s *ibcLumeraSuite) TestTransferToSimd() {
	// Exercise a real packet flow from lumera -> simd and confirm balance change.
	amount := getenv("LUMERA_IBC_AMOUNT", "100"+s.lumeraDenom)
	ibcDenom := ibcutil.IBCDenom(s.portID, s.channelID, s.lumeraDenom)

	before, err := ibcutil.QueryBalanceREST(s.simdREST, s.simdRecipient, ibcDenom)
	s.Require().NoError(err, "query simd recipient balance before")

	err = ibcutil.SendIBCTransfer(
		s.lumeraBin, s.lumeraRPC, "",
		s.lumeraKeyName, s.portID, s.channelID, s.simdRecipient, amount,
		s.lumeraChainID, "test", s.lumeraGasPrices,
	)
	s.Require().NoError(err, "send ibc transfer to simd")

	after, err := ibcutil.WaitForBalanceIncreaseREST(s.simdREST, s.simdRecipient, ibcDenom, before, 20, 3*time.Second)
	s.Require().NoError(err, "wait for simd recipient balance increase")
	s.T().Logf("simd recipient balance increased: %d -> %d", before, after)
}

func TestIBCLumeraSideSuite(t *testing.T) {
	suite.Run(t, new(ibcLumeraSuite))
}

func getenv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
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
