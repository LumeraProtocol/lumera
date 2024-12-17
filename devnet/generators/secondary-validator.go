package generators

import (
	"fmt"
	confg "gen/config"
	"os"
	"strings"
)

type SecondaryScriptBuilder struct {
	config     *confg.ChainConfig
	validators []confg.Validator
	lines      []string
}

func NewSecondaryScriptBuilder(config *confg.ChainConfig, validators []confg.Validator) *SecondaryScriptBuilder {
	return &SecondaryScriptBuilder{
		config:     config,
		validators: validators,
		lines:      []string{"#!/bin/bash", "set -e\n"},
	}
}

func (sb *SecondaryScriptBuilder) addScriptParameters() {
	sb.lines = append(sb.lines, []string{
		`if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ]; then`,
		`    echo "Error: Key name, stake amount, moniker, and balance are required"`,
		`    echo "Usage: $0 <key-name> <stake-amount> <moniker> <balance>"`,
		"    exit 1",
		"fi",
		"",
		`echo "Received arguments: key-name=$1, stake-amount=$2, moniker=$3"`,
		"",
		"KEY_NAME=$1",
		"STAKE_AMOUNT=$2",
		"MONIKER=$3",
		"BALANCE=$4",
		"",
		fmt.Sprintf("mkdir -p /root/%s/config", sb.config.Paths.Directories.Daemon),
	}...)
}

func (sb *SecondaryScriptBuilder) waitForPrimaryInit() {
	sb.lines = append(sb.lines, []string{
		"# Wait for primary validator to set up accounts",
		`echo "Waiting for primary validator to set up accounts..."`,
		"while [ ! -f /shared/genesis_accounts_ready ]; do",
		"    sleep 1",
		"done",
		"",
	}...)
}

func (sb *SecondaryScriptBuilder) initAndCreateGentx() {
	sb.lines = append(sb.lines, []string{
		fmt.Sprintf(`if [[ ! -f /root/%s/config/genesis.json ]] || [[ ! -f /root/%s/config/priv_validator_key.json ]]; then`,
			sb.config.Paths.Directories.Daemon, sb.config.Paths.Directories.Daemon),
		`    echo "First time initialization for secondary validator $MONIKER..."`,
		fmt.Sprintf("    %s init $MONIKER --chain-id %s --overwrite",
			sb.config.Daemon.Binary, sb.config.Chain.ID),
		"",
		fmt.Sprintf("    cp /shared/genesis.json /root/%s/config/genesis.json", sb.config.Paths.Directories.Daemon),
		"",
		fmt.Sprintf("	%s keys add $KEY_NAME --keyring-backend %s",
			sb.config.Daemon.Binary, sb.config.Daemon.KeyringBackend),
		"",
		fmt.Sprintf(`    echo "Adding genesis account for $KEY_NAME..."`),
		fmt.Sprintf("    ADDR=$(%s keys show $KEY_NAME -a --keyring-backend %s)",
			sb.config.Daemon.Binary, sb.config.Daemon.KeyringBackend),
		fmt.Sprintf("    %s genesis add-genesis-account $ADDR $BALANCE",
			sb.config.Daemon.Binary),
		`    echo "Creating gentx for ${MONIKER}..."`,
		fmt.Sprintf("    %s genesis gentx $KEY_NAME $STAKE_AMOUNT \\", sb.config.Daemon.Binary),
		fmt.Sprintf("        --chain-id %s \\", sb.config.Chain.ID),
		fmt.Sprintf("        --keyring-backend %s", sb.config.Daemon.KeyringBackend),

		"    # Share gentx and address with primary validator",
		"    mkdir -p /shared/addresses",
		fmt.Sprintf("    echo $BALANCE > /shared/addresses/${ADDR}"),
		"    mkdir -p /shared/gentx",
		fmt.Sprintf("    cp /root/%s/config/gentx/* /shared/gentx/${MONIKER}_gentx.json", sb.config.Paths.Directories.Daemon),

		"else",
		`    echo "Secondary validator $MONIKER already initialized..."`,
		"fi",
		"",
	}...)
}

func (sb *SecondaryScriptBuilder) waitForFinalGenesis() {
	sb.lines = append(sb.lines, []string{
		"# Wait for final genesis",
		`echo "Waiting for final genesis..."`,
		"while [ ! -f /shared/final_genesis.json ]; do",
		"    sleep 1",
		"done",
		"",
		"# Copy final genesis",
		fmt.Sprintf("cp /shared/final_genesis.json /root/%s/config/genesis.json", sb.config.Paths.Directories.Daemon),
		"",
	}...)
}

func (sb *SecondaryScriptBuilder) setupPeers() {
	sb.lines = append(sb.lines, []string{
		"# Save own node ID",
		fmt.Sprintf("nodeid=$(%s tendermint show-node-id)", sb.config.Daemon.Binary),
		"echo $nodeid > /shared/${MONIKER}_nodeid",
		"ip=$(hostname -i)",
		"echo $ip > /shared/${MONIKER}_ip",
		"",
		"# Initialize empty PEERS string",
		`PEERS=""`,
		"",
		"# Wait for and collect all other validators' node IDs and IPs",
	}...)

	var waitConditions []string
	for _, validator := range sb.validators {
		waitConditions = append(waitConditions,
			fmt.Sprintf("/shared/%s_nodeid", validator.Name),
			fmt.Sprintf("/shared/%s_ip", validator.Name))
	}
	sb.lines = append(sb.lines, fmt.Sprintf(
		"while [[ ! -f %s ]]; do",
		strings.Join(waitConditions, " || ! -f "),
	))
	sb.lines = append(sb.lines,
		`    echo "Waiting for other node IDs and IPs..."`,
		"    sleep 1",
		"done",
		"",
		"# Build peers string dynamically",
	)

	sb.lines = append(sb.lines, []string{
		"# Build peers string excluding self",
		"for v in " + strings.Join(getValidatorNames(sb.validators), " ") + "; do",
		`    if [ "$v" != "${MONIKER}" ]; then`,
		"        NODE_ID=$(cat /shared/${v}_nodeid)",
		"        NODE_IP=$(cat /shared/${v}_ip)",
		`        if [ ! -z "$PEERS" ]; then`,
		`            PEERS="$PEERS,"`,
		"        fi",
		`        PEERS="${PEERS}${NODE_ID}@${NODE_IP}:$(cat /shared/${v}_port)"`,
		"    fi",
		"done",
		"",
		"# Update peer configuration",
		fmt.Sprintf(`sed -i "s/^persistent_peers *=.*/persistent_peers = \"$PEERS\"/" /root/%s/config/config.toml`,
			sb.config.Paths.Directories.Daemon),
	}...)
}

func getValidatorNames(validators []confg.Validator) []string {
	names := make([]string, len(validators))
	for i, v := range validators {
		names[i] = v.Name
	}
	return names
}

func (sb *SecondaryScriptBuilder) addStartCommand() {
	sb.lines = append(sb.lines, []string{
		"",
		"# Wait for primary setup to complete",
		`echo "Waiting for chain setup to complete..."`,
		"while [ ! -f /shared/setup_complete ]; do",
		"    sleep 1",
		"done",
		"",
		fmt.Sprintf(`echo "Starting secondary validator %s..."`, sb.config.Daemon.Binary),
		fmt.Sprintf("%s start --minimum-gas-prices %s",
			sb.config.Daemon.Binary,
			sb.config.Chain.Denom.MinimumGasPrice),
	}...)
}

func GenerateSecondaryValidatorScript(config *confg.ChainConfig, validators []confg.Validator) error {
	sb := NewSecondaryScriptBuilder(config, validators)

	sb.addScriptParameters()
	sb.waitForPrimaryInit()
	sb.initAndCreateGentx()
	sb.waitForFinalGenesis()
	sb.setupPeers()
	sb.addStartCommand()

	script := strings.Join(sb.lines, "\n")
	return os.WriteFile("secondary-validator.sh", []byte(script), 0755)
}
