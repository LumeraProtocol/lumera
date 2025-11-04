package generators

const (
	DefaultP2PPort                   = 26656
	DefaultRPCPort                   = 26657
	DefaultRESTPort                  = 1317
	DefaultGRPCPort                  = 9090
	DefaultDebugPort                 = 40000
	DefaultSupernodePort             = 4444
	DefaultSupernodeP2PPort          = 4445
	DefaultSupernodeGatewayPort      = 8002
	DefaultGRPCWebPort               = 9091
	DefaultHermesSimdHostP2PPort     = 36656
	DefaultHermesSimdHostRPCPort     = 36657
	DefaultHermesSimdHostAPIPort     = 31317
	DefaultHermesSimdHostGRPCPort    = 39090
	DefaultHermesSimdHostGRPCWebPort = 39091

	FolderScripts   = "/root/scripts"
	SubFolderShared = "shared"
	SubFolderConfig = "config"
	SubFolderBin    = "bin"

	SupernodeBinary      = "supernode-linux-amd64"
	SetupValidatorScript = "validator-setup.sh"
	SetupSupernodeScript = "supernode-setup.sh"
	StartScript          = "start.sh"

	HermesSimdHome  = "/root/.simd"
	HermesStateHome = "/root/.hermes"
)
