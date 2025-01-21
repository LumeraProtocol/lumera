package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// ChainConfig represents the chain configuration structure
type ChainConfig struct {
	Chain struct {
		ID    string `json:"id"`
		Denom struct {
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
}

type Validator struct {
	Name                string `json:"name"`
	Moniker             string `json:"moniker"`
	KeyName             string `json:"key_name"`
	Port                int    `json:"port"`
	RPCPort             int    `json:"rpc_port"`
	RESTPort            int    `json:"rest_port"`
	GRPCPort            int    `json:"grpc_port"`
	InitialDistribution struct {
		AccountBalance string `json:"account_balance"`
		ValidatorStake string `json:"validator_stake"`
	} `json:"initial_distribution"`
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
