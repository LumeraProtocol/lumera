package generators

import (
	"fmt"
	confg "gen/config"
	"os"
	"strings"
)

type PrimaryScriptBuilder struct {
	config     *confg.ChainConfig
	validators []confg.Validator
	lines      []string
}

func NewPrimaryScriptBuilder(config *confg.ChainConfig, validators []confg.Validator) *PrimaryScriptBuilder {
	return &PrimaryScriptBuilder{
		config:     config,
		validators: validators,
		lines:      []string{"#!/bin/bash", "set -e\n"},
	}
}

func (sb *PrimaryScriptBuilder) addInitAndDenom() {
	sb.lines = append(sb.lines, []string{
		fmt.Sprintf("mkdir -p /root/%s/config", sb.config.Paths.Directories.Daemon),
		"",
		fmt.Sprintf(`if [[ ! -f /root/%s/config/genesis.json ]] || [[ ! -f /root/%s/config/priv_validator_key.json ]]; then`,
			sb.config.Paths.Directories.Daemon, sb.config.Paths.Directories.Daemon),
		fmt.Sprintf(`    echo "First time initialization for primary validator %s..."`, sb.validators[0].Moniker),
		"",
		fmt.Sprintf("    %s init %s --chain-id %s --overwrite",
			sb.config.Daemon.Binary, sb.validators[0].Moniker, sb.config.Chain.ID),
		"",
		"    # Update bond denomination",
		fmt.Sprintf(`    cat /root/%s/config/genesis.json | jq '.app_state.staking.params.bond_denom = "%s"' > /root/%s/config/tmp_genesis.json`,
			sb.config.Paths.Directories.Daemon, sb.config.Chain.Denom.Bond, sb.config.Paths.Directories.Daemon),
		fmt.Sprintf("    mv /root/%s/config/tmp_genesis.json /root/%s/config/genesis.json",
			sb.config.Paths.Directories.Daemon, sb.config.Paths.Directories.Daemon),
		"",
		"    # Update mint denomination",
		fmt.Sprintf(`    cat /root/%s/config/genesis.json | jq '.app_state.mint.params.mint_denom = "%s"' > /root/%s/config/tmp_genesis.json`,
			sb.config.Paths.Directories.Daemon, sb.config.Chain.Denom.Mint, sb.config.Paths.Directories.Daemon),
		fmt.Sprintf("    mv /root/%s/config/tmp_genesis.json /root/%s/config/genesis.json",
			sb.config.Paths.Directories.Daemon, sb.config.Paths.Directories.Daemon),
		"",
		"    # Update crisis constant fee denomination",
		fmt.Sprintf(`    cat /root/%s/config/genesis.json | jq '.app_state.crisis.constant_fee.denom = "%s"' > /root/%s/config/tmp_genesis.json`,
			sb.config.Paths.Directories.Daemon, sb.config.Chain.Denom.Bond, sb.config.Paths.Directories.Daemon),
		fmt.Sprintf("    mv /root/%s/config/tmp_genesis.json /root/%s/config/genesis.json",
			sb.config.Paths.Directories.Daemon, sb.config.Paths.Directories.Daemon),
		"",
		"    # Update gov min deposit denomination",
		fmt.Sprintf(`    cat /root/%s/config/genesis.json | jq '.app_state.gov.params.min_deposit[0].denom = "%s"' > /root/%s/config/tmp_genesis.json`,
			sb.config.Paths.Directories.Daemon, sb.config.Chain.Denom.Bond, sb.config.Paths.Directories.Daemon),
		fmt.Sprintf("    mv /root/%s/config/tmp_genesis.json /root/%s/config/genesis.json",
			sb.config.Paths.Directories.Daemon, sb.config.Paths.Directories.Daemon),
		"",
		"    # Update gov expedited min deposit denomination",
		fmt.Sprintf(`    cat /root/%s/config/genesis.json | jq '.app_state.gov.params.expedited_min_deposit[0].denom = "%s"' > /root/%s/config/tmp_genesis.json`,
			sb.config.Paths.Directories.Daemon, sb.config.Chain.Denom.Bond, sb.config.Paths.Directories.Daemon),
		fmt.Sprintf("    mv /root/%s/config/tmp_genesis.json /root/%s/config/genesis.json",
			sb.config.Paths.Directories.Daemon, sb.config.Paths.Directories.Daemon),
	}...)
}

func (sb *PrimaryScriptBuilder) addAccountsAndGenesis() {
	sb.lines = append(sb.lines, "\n    # Create all accounts and add to genesis")
	for _, validator := range sb.validators {
		sb.lines = append(sb.lines,
			fmt.Sprintf(`    echo "Creating key for %s..."`, validator.KeyName),
			fmt.Sprintf("    %s keys add %s --keyring-backend %s",
				sb.config.Daemon.Binary, validator.KeyName, sb.config.Daemon.KeyringBackend),
			"",
			fmt.Sprintf(`    echo "Adding genesis account for %s..."`, validator.KeyName),
			fmt.Sprintf("    ADDR=$(%s keys show %s -a --keyring-backend %s)",
				sb.config.Daemon.Binary, validator.KeyName, sb.config.Daemon.KeyringBackend),
			fmt.Sprintf("    %s genesis add-genesis-account $ADDR %s",
				sb.config.Daemon.Binary, validator.InitialDistribution.AccountBalance),
			"")
	}
}

func (sb *PrimaryScriptBuilder) shareAndCreateGentx() {
	sb.lines = append(sb.lines, []string{
		"    # Share keyring and genesis",
		`    echo "Primary validator sharing keyring and genesis..."`,
		"    mkdir -p /shared",
		fmt.Sprintf("    cp -r /root/%s/keyring-test /shared/keyring-test", sb.config.Paths.Directories.Daemon),
		fmt.Sprintf("    cp /root/%s/config/genesis.json /shared/genesis.json", sb.config.Paths.Directories.Daemon),
		"",
		"    mkdir -p /shared/gentx",
		`    echo "true" > /shared/genesis_accounts_ready`,
		"",
		"    # Create and submit primary gentx",
		`    echo "Creating primary validator gentx..."`,
		fmt.Sprintf("    %s genesis gentx %s %s \\",
			sb.config.Daemon.Binary,
			sb.validators[0].KeyName,
			sb.validators[0].InitialDistribution.ValidatorStake),
		fmt.Sprintf("        --chain-id %s \\", sb.config.Chain.ID),
		fmt.Sprintf("        --keyring-backend %s \\", sb.config.Daemon.KeyringBackend),
	}...)
}

func (sb *PrimaryScriptBuilder) waitAndCollectGentx() {
	sb.lines = append(sb.lines, []string{
		"",
		`    echo "Primary validator waiting for other validators' gentx files..."`,
		fmt.Sprintf("    while [[ $(ls /shared/gentx/* 2>/dev/null | wc -l) -lt %d ]]; do", len(sb.validators)-1),
		fmt.Sprintf(`        echo "Found $(ls /shared/gentx/* 2>/dev/null | wc -l) of %d required gentx files..."`, len(sb.validators)-1),
		"        sleep 2",
		"    done",
		"",
		`    echo "Primary validator collecting gentxs..."`,
		fmt.Sprintf("    mkdir -p /root/%s/config/gentx", sb.config.Paths.Directories.Daemon),
		fmt.Sprintf("    cp /shared/gentx/*.json /root/%s/config/gentx/", sb.config.Paths.Directories.Daemon),
		fmt.Sprintf("    %s genesis collect-gentxs", sb.config.Daemon.Binary),
		fmt.Sprintf("    cp /root/%s/config/genesis.json /shared/final_genesis.json", sb.config.Paths.Directories.Daemon),
		`    echo "true" > /shared/setup_complete`,
		"else",
		fmt.Sprintf(`    echo "Primary validator %s already initialized, starting chain..."`, sb.validators[0].Moniker),
		"fi\n",
	}...)
}

func (sb *PrimaryScriptBuilder) setupPeers() {
	sb.lines = append(sb.lines, []string{
		"# Setup peer connections",
		fmt.Sprintf("nodeid=$(%s tendermint show-node-id)", sb.config.Daemon.Binary),
		"echo $nodeid > /shared/validator1_nodeid",
		"ip=$(hostname -i)",
		"echo $ip > /shared/validator1_ip",
		"",
		"# Wait for other validators' node IDs and IPs",
	}...)

	var waitConditions []string
	var nodeVars []string
	var peerParts []string

	for _, validator := range sb.validators[1:] {
		waitConditions = append(waitConditions,
			fmt.Sprintf("/shared/%s_nodeid", validator.Name),
			fmt.Sprintf("/shared/%s_ip", validator.Name))
		nodeVar := fmt.Sprintf("NODE_%s_ID", strings.ToUpper(validator.Name))
		nodeVars = append(nodeVars, nodeVar)
	}

	sb.lines = append(sb.lines, fmt.Sprintf(
		"while [[ ! -f %s ]]; do",
		strings.Join(waitConditions, " || ! -f "),
	))
	sb.lines = append(sb.lines,
		`    echo "Waiting for other node IDs and IPs..."`,
		"    sleep 1",
		"done",
		"")

	for i, validator := range sb.validators[1:] {
		sb.lines = append(sb.lines,
			fmt.Sprintf(`%s=$(cat /shared/%s_nodeid)`, nodeVars[i], validator.Name),
			fmt.Sprintf(`%s_IP=$(cat /shared/%s_ip)`, validator.Name, validator.Name),
			fmt.Sprintf(`peerPart%d="${%s}@${%s_IP}:26656"`, i, nodeVars[i], validator.Name))
		peerParts = append(peerParts, fmt.Sprintf("$peerPart%d", i))
	}

	sb.lines = append(sb.lines,
		fmt.Sprintf(`PEERS="%s"`, strings.Join(peerParts, ",")),
		"",
		"# Update peer configuration",
		fmt.Sprintf(`sed -i "s/^persistent_peers *=.*/persistent_peers = \"$PEERS\"/" /root/%s/config/config.toml`,
			sb.config.Paths.Directories.Daemon),
	)
}

func (sb *PrimaryScriptBuilder) addStartCommand() {
	sb.lines = append(sb.lines, []string{
		"",
		fmt.Sprintf(`echo "Starting primary validator %s..."`, sb.config.Daemon.Binary),
		fmt.Sprintf("%s start --minimum-gas-prices %s",
			sb.config.Daemon.Binary,
			sb.config.Chain.Denom.MinimumGasPrice),
	}...)
}

func GeneratePrimaryValidatorScript(config *confg.ChainConfig, validators []confg.Validator) error {
	sb := NewPrimaryScriptBuilder(config, validators)

	sb.addInitAndDenom()
	sb.addAccountsAndGenesis()
	sb.shareAndCreateGentx()
	sb.waitAndCollectGentx()
	sb.setupPeers()
	sb.addStartCommand()

	script := strings.Join(sb.lines, "\n")
	return os.WriteFile("primary-validator.sh", []byte(script), 0755)
}
