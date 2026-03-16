package config

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	// DefaultEVMFromVersion is the first Lumera version where EVM key style is enabled.
	DefaultEVMFromVersion = "v1.12.0"
)

// ChainConfig represents the chain configuration structure
type ChainConfig struct {
	Chain struct {
		ID             string `json:"id"`
		Version        string `json:"version"`
		EVMFromVersion string `json:"evm_from_version"`
		Denom          struct {
			Bond            string `json:"bond"`
			Mint            string `json:"mint"`
			MinimumGasPrice string `json:"minimum_gas_price"`
		} `json:"denom"`
	} `json:"chain"`
	Docker struct {
		NetworkName     string `json:"network_name"`
		ContainerPrefix string `json:"container_prefix"`
		VolumePrefix    string `json:"volume_prefix"`
	} `json:"docker"`
	Paths struct {
		Base struct {
			Host      string `json:"host"`
			Container string `json:"container"`
		} `json:"base"`
		Directories struct {
			Daemon string `json:"daemon"`
		} `json:"directories"`
	} `json:"paths"`
	Daemon struct {
		Binary         string `json:"binary"`
		KeyringBackend string `json:"keyring_backend"`
	} `json:"daemon"`
	GenesisAccountMnemonics []string `json:"genesis-account-mnemonics"`
	SNAccountMnemonics      []string `json:"sn-account-mnemonics"`
	API struct {
		EnableUnsafeCORS bool `json:"enable_unsafe_cors"`
	} `json:"api"`
	JSONRPC struct {
		Enable        bool   `json:"enable"`
		Address       string `json:"address"`
		WSAddress     string `json:"ws_address"`
		API           string `json:"api"`
		EnableIndexer bool   `json:"enable_indexer"`
	} `json:"json-rpc"`
	NetworkMaker struct {
		MaxAccounts    int    `json:"max_accounts"`
		AccountBalance string `json:"account_balance"`
		Enabled        bool   `json:"enabled"`
		GRPCPort       int    `json:"grpc_port"`
		HTTPPort       int    `json:"http_port"`
	} `json:"network-maker"`
	Hermes struct {
		Enabled bool `json:"enabled"`
	} `json:"hermes"`
}

type Validator struct {
	Name                 string `json:"name"`
	Moniker              string `json:"moniker"`
	KeyName              string `json:"key_name"`
	Port                 int    `json:"port"`
	RPCPort              int    `json:"rpc_port"`
	RESTPort             int    `json:"rest_port"`
	GRPCPort             int    `json:"grpc_port"`
	Supernode            struct {
		Port        int `json:"port,omitempty"`
		P2PPort     int `json:"p2p_port,omitempty"`
		GatewayPort int `json:"gateway_port,omitempty"`
	} `json:"supernode,omitempty"`
	JSONRPC struct {
		Port   int `json:"port,omitempty"`
		WSPort int `json:"ws_port,omitempty"`
	} `json:"json-rpc,omitempty"`

	InitialDistribution struct {
		AccountBalance string `json:"account_balance"`
		ValidatorStake string `json:"validator_stake"`
	} `json:"initial_distribution"`

	NetworkMaker struct {
		Enabled  bool `json:"enabled,omitempty"`
		GRPCPort int  `json:"grpc_port,omitempty"`
		HTTPPort int  `json:"http_port,omitempty"`
	} `json:"network-maker,omitempty"`
}

func LoadConfigs(configPath, validatorsPath string) (*ChainConfig, []Validator, error) {
	if configPath == "" {
		configPath = "config/config.json"
	}
	if validatorsPath == "" {
		validatorsPath = "config/validators.json"
	}

	configFile, err := os.ReadFile(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading config.json from %s: %v", configPath, err)
	}

	var config ChainConfig
	if err := json.Unmarshal(configFile, &config); err != nil {
		return nil, nil, fmt.Errorf("error parsing config.json: %v", err)
	}
	if config.Chain.EVMFromVersion == "" {
		config.Chain.EVMFromVersion = DefaultEVMFromVersion
	}

	validatorsFile, err := os.ReadFile(validatorsPath)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading validators.json from %s: %v", validatorsPath, err)
	}

	var validators []Validator
	if err := json.Unmarshal(validatorsFile, &validators); err != nil {
		return nil, nil, fmt.Errorf("error parsing validators.json: %v", err)
	}

	return &config, validators, nil
}
