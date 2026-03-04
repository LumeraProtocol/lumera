package hermes

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"gen/tests/ibcutil"

	textutil "github.com/LumeraProtocol/lumera/pkg/text"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"
)

const (
	defaultChannelInfoPath = "/shared/status/hermes/channel_transfer.json"
	defaultSimdBin         = "simd"
	defaultSimdRPC         = "http://127.0.0.1:26657"
	defaultSimdGRPCAddr    = "http://127.0.0.1:9090"
	defaultSimdChainID     = "hermes-simd-1"
	defaultSimdHome        = "/root/.simd"
	defaultSimdKeyName     = "simd-test"
	defaultSimdGasPrices   = "0.025stake"
	defaultSimdDenom       = "stake"
	defaultSimdKeyring     = "test"
	defaultSimdAddrFile    = "/shared/hermes/simd-test.address"
	defaultSimdMnemonic    = "/shared/hermes/simd-test.mnemonic"
	defaultLumeraChainID   = "lumera-devnet-1"
	defaultLumeraDenom     = "ulume"
	defaultLumeraGRPCAddr  = "http://supernova_validator_1:9090"
	defaultLumeraRPCAddr   = "http://supernova_validator_1:26657"
	defaultLumeraKeyName   = "hermes-relayer"
	defaultLumeraMnemonic  = "/shared/hermes/lumera-hermes-relayer.mnemonic"
	defaultLumeraAddrFile  = "/shared/hermes/lumera-hermes-relayer.address"
	defaultLumeraREST      = "http://supernova_validator_1:1317"
	defaultLumeraICAFund   = "1000000"
	defaultLumeraICAFeeBuf = "10000"
	actionPollRetries      = 40
	actionPollDelay        = 3 * time.Second
	simdQueryTimeout       = 20 * time.Second
	simdTxTimeout          = 2 * time.Minute
	icaTestTimeout         = 20 * time.Minute
	defaultIBCRetries      = 40
	defaultIBCRetryDelay   = 3 * time.Second
)

type lumeraHermesSuite struct {
	suite.Suite
	channelInfoPath     string
	simdBin             string
	portID              string
	counterpartyChannel string
	simdKeyring         string
	simdHome            string
	simdGasPrices       string
	simdAddrFile        string
	lumeraICAFund       string
	lumeraICAFeeBuffer  string
	lumeraRecipient     string
	lumeraKeyStyle      string

	simd   ChainInfo
	lumera ChainInfo

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

type ChainInfo struct {
	ChainID      string
	RPC          string
	GRPC         string
	REST         string
	Denom        string
	KeyName      string
	MnemonicFile string
}

func (s *lumeraHermesSuite) logInfo(msg string) {
	s.T().Log(formatTestLog("INFO", msg))
}

func (s *lumeraHermesSuite) logInfof(format string, args ...any) {
	s.T().Log(formatTestLog("INFO", fmt.Sprintf(format, args...)))
}

func formatTestLog(level, msg string) string {
	ts := time.Now().Format("01/02/2006 15:04:05.000")
	return fmt.Sprintf("%s %s %s", level, ts, msg)
}

func (s *lumeraHermesSuite) SetupSuite() {
	// Load environment-driven configuration and shared chain metadata.
	s.channelInfoPath = textutil.EnvOrDefault("CHANNEL_INFO_FILE", defaultChannelInfoPath)
	s.simdBin = textutil.EnvOrDefault("SIMD_BIN", defaultSimdBin)
	s.simd = ChainInfo{
		ChainID:      textutil.EnvOrDefault("SIMD_CHAIN_ID", defaultSimdChainID),
		RPC:          textutil.EnvOrDefault("SIMD_RPC_ADDR", defaultSimdRPC),
		GRPC:         normalizeGRPCAddr(textutil.EnvOrDefault("SIMD_GRPC_ADDR", defaultSimdGRPCAddr)),
		Denom:        textutil.EnvOrDefault("SIMD_DENOM", defaultSimdDenom),
		KeyName:      textutil.EnvOrDefault("SIMD_KEY_NAME", defaultSimdKeyName),
		MnemonicFile: textutil.EnvOrDefault("SIMD_KEY_MNEMONIC_FILE", defaultSimdMnemonic),
	}
	s.simdKeyring = textutil.EnvOrDefault("SIMD_KEYRING", defaultSimdKeyring)
	s.simdHome = textutil.EnvOrDefault("SIMD_HOME", defaultSimdHome)
	s.simdGasPrices = textutil.EnvOrDefault("SIMD_GAS_PRICES", defaultSimdGasPrices)
	s.simdAddrFile = textutil.EnvOrDefault("SIMD_OWNER_ADDR_FILE", defaultSimdAddrFile)
	s.lumera = ChainInfo{
		ChainID:      textutil.EnvOrDefault("LUMERA_CHAIN_ID", defaultLumeraChainID),
		GRPC:         normalizeGRPCAddr(textutil.EnvOrDefault("LUMERA_GRPC_ADDR", defaultLumeraGRPCAddr)),
		RPC:          textutil.EnvOrDefault("LUMERA_RPC_ADDR", defaultLumeraRPCAddr),
		REST:         textutil.EnvOrDefault("LUMERA_REST_ADDR", defaultLumeraREST),
		Denom:        textutil.EnvOrDefault("LUMERA_DENOM", defaultLumeraDenom),
		KeyName:      textutil.EnvOrDefault("LUMERA_KEY_NAME", defaultLumeraKeyName),
		MnemonicFile: textutil.EnvOrDefault("LUMERA_KEY_MNEMONIC_FILE", defaultLumeraMnemonic),
	}
	s.lumeraICAFund = textutil.EnvOrDefault("LUMERA_ICA_FUND_AMOUNT", defaultLumeraICAFund)
	s.lumeraICAFeeBuffer = textutil.EnvOrDefault("LUMERA_ICA_FUND_FEE_BUFFER", defaultLumeraICAFeeBuf)
	s.lumeraKeyStyle = resolveLumeraKeyStyle()
	s.T().Logf("Lumera key style for Hermes tests: %s", s.lumeraKeyStyle)

	ensureLumeraBech32Prefixes()

	info, err := ibcutil.LoadChannelInfo(s.channelInfoPath)
	s.Require().NoError(err, "load channel info")
	s.info = info
	counterpartyChain := info.CounterpartyChainID
	if counterpartyChain == "" && info.AChainID != "" && info.BChainID != "" {
		switch s.simd.ChainID {
		case info.AChainID:
			counterpartyChain = info.BChainID
		case info.BChainID:
			counterpartyChain = info.AChainID
		}
	}
	info.CounterpartyChainID = counterpartyChain
	s.T().Logf("Loaded channel info: port=%s channel=%s counterparty_chain=%s a_chain=%s b_chain=%s",
		info.PortID, info.ChannelID, info.CounterpartyChainID, info.AChainID, info.BChainID)

	// Resolve port/channel IDs from env or the generated channel info file.
	portID := textutil.EnvOrDefault("PORT_ID", "")
	if portID == "" {
		portID = info.PortID
	}
	if portID == "" {
		portID = ibcutil.DefaultPortID
	}
	s.portID = portID

	s.counterpartyChannel = textutil.EnvOrDefault("LUMERA_CHANNEL_ID", info.ChannelID)
	s.Require().NotEmpty(s.counterpartyChannel, "channel_id missing in %s", s.channelInfoPath)

	// Load the lumera recipient for transfer tests.
	lumeraAddrFile := textutil.EnvOrDefault("LUMERA_RECIPIENT_ADDR_FILE", defaultLumeraAddrFile)
	addr, err := ibcutil.ReadAddress(lumeraAddrFile)
	s.Require().NoError(err, "read lumera recipient address")
	s.lumeraRecipient = addr

	s.T().Logf("Testing IBC on simd (port=%s counterparty_channel=%s rpc=%s)", s.portID, s.counterpartyChannel, s.simd.RPC)

	// Discover channel/connection/client on simd and cache it for the suite.
	channels, err := ibcutil.QueryChannels(s.simdBin, s.simd.RPC)
	s.Require().NoError(err, "query channels")
	s.channels = channels
	s.T().Logf("Discovered %d simd channels", len(channels))
	for _, ch := range channels {
		s.T().Logf("simd channel: port=%s channel=%s state=%s counterparty_port=%s counterparty_channel=%s conn_hops=%v",
			ch.PortID, ch.ChannelID, ch.State, ch.Counterparty.PortID, ch.Counterparty.ChannelID, ch.ConnectionHops)
	}

	channel := ibcutil.FindChannelByCounterparty(channels, s.portID, s.counterpartyChannel)
	if channel == nil {
		channel = ibcutil.FirstChannelByPort(channels, s.portID)
		s.Require().NotNil(channel, "no channel found for port %s on simd", s.portID)
		s.T().Logf("channel with counterparty %s not found; using channel %s", s.counterpartyChannel, channel.ChannelID)
	}
	s.channel = channel

	s.Require().NotEmpty(channel.ConnectionHops, "channel %s/%s missing connection hop", channel.PortID, channel.ChannelID)
	connectionID := channel.ConnectionHops[0]
	s.T().Logf("Channel open; connection=%s counterparty_channel=%s", connectionID, channel.Counterparty.ChannelID)

	connections, err := ibcutil.QueryConnections(s.simdBin, s.simd.RPC)
	s.Require().NoError(err, "query connections")
	s.connections = connections
	s.T().Logf("Discovered %d simd connections", len(connections))
	for _, conn := range connections {
		s.T().Logf("simd connection: id=%s state=%s client_id=%s counterparty_client_id=%s counterparty_connection_id=%s",
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
	status, err := ibcutil.QueryClientStatus(s.simdBin, s.simd.RPC, connection.ClientID)
	s.Require().NoError(err, "query client status")
	s.clientStatus = status

	csClientID, csHeight, csType, err := ibcutil.QueryChannelClientState(s.simdBin, s.simd.RPC, channel.PortID, channel.ChannelID)
	s.Require().NoError(err, "query channel client-state")
	s.csClientID = csClientID
	s.csHeight = csHeight
	s.csType = csType
}

func (s *lumeraHermesSuite) TestChannelOpen() {
	s.Require().NotNil(s.channel, "channel is nil")
	s.True(ibcutil.IsOpenState(s.channel.State), "channel %s/%s not open: %s", s.channel.PortID, s.channel.ChannelID, s.channel.State)
	if s.channel.Counterparty.ChannelID != "" {
		s.Equal(s.counterpartyChannel, s.channel.Counterparty.ChannelID, "counterparty channel mismatch")
	}
}

func (s *lumeraHermesSuite) TestConnectionOpen() {
	s.Require().NotNil(s.connection, "connection is nil")
	s.True(ibcutil.IsOpenState(s.connection.State), "connection %s not open: %s", s.connection.ID, s.connection.State)
}

func (s *lumeraHermesSuite) TestClientActive() {
	s.True(ibcutil.IsActiveStatus(s.clientStatus), "client %s not active: %s", s.connection.ClientID, s.clientStatus)
}

func (s *lumeraHermesSuite) TestChannelClientState() {
	if s.csClientID != "" {
		s.Equal(s.connection.ClientID, s.csClientID, "client-state mismatch")
	}
	s.Greater(s.csHeight, int64(0), "client-state latest_height not positive")
	s.T().Logf("Client status active; client-state height=%d type=%s", s.csHeight, s.csType)
}

func (s *lumeraHermesSuite) TestTransferToLumera() {
	// Exercise a real packet flow from simd -> lumera and confirm balance change.
	amount := "100" + s.simd.Denom
	s.transferFromSimdToLumeraAndAssert(amount)
}

func (s *lumeraHermesSuite) TestIBCTransferWithEVMModeStillRelays() {
	s.requireLumeraEVMModeOrSkip()
	amount := "77" + s.simd.Denom
	s.transferFromSimdToLumeraAndAssert(amount)
}

func TestIBCSimdSideSuite(t *testing.T) {
	suite.Run(t, new(lumeraHermesSuite))
}

func normalizeGRPCAddr(addr string) string {
	out := strings.TrimSpace(addr)
	out = strings.TrimPrefix(out, "http://")
	out = strings.TrimPrefix(out, "https://")
	return out
}

func (s *lumeraHermesSuite) transferFromSimdToLumeraAndAssert(amount string) {
	ibcDenom := ibcutil.IBCDenom(s.portID, s.channel.ChannelID, s.simd.Denom)

	before, err := ibcutil.QueryBalanceREST(s.lumera.REST, s.lumeraRecipient, ibcDenom)
	s.Require().NoError(err, "query lumera recipient balance before")

	err = ibcutil.SendIBCTransfer(
		s.simdBin, s.simd.RPC, s.simdHome,
		s.simd.KeyName, s.portID, s.channel.ChannelID, s.lumeraRecipient, amount,
		s.simd.ChainID, "test", s.simdGasPrices,
	)
	s.Require().NoError(err, "send ibc transfer to lumera")

	after, err := ibcutil.WaitForBalanceIncreaseREST(s.lumera.REST, s.lumeraRecipient, ibcDenom, before, defaultIBCRetries, defaultIBCRetryDelay)
	s.Require().NoError(err, "wait for lumera recipient balance increase")
	s.T().Logf("lumera recipient balance increased: %d -> %d", before, after)
}

func (s *lumeraHermesSuite) requireLumeraEVMModeOrSkip() {
	if strings.EqualFold(strings.TrimSpace(s.lumeraKeyStyle), "evm") {
		return
	}
	s.T().Skipf("skip EVM-mode transfer assertion: lumera key style is %q", s.lumeraKeyStyle)
}

func ensureLumeraBech32Prefixes() {
	cfg := sdk.GetConfig()
	if cfg.GetBech32AccountAddrPrefix() == "lumera" {
		return
	}
	cfg.SetBech32PrefixForAccount("lumera", "lumerapub")
	cfg.SetBech32PrefixForValidator("lumeravaloper", "lumeravaloperpub")
	cfg.SetBech32PrefixForConsensusNode("lumeravalcons", "lumeravalconspub")
	cfg.Seal()
}
