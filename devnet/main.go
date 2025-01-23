package main

import (
	"encoding/json"
	"fmt"
	"gen/config"
	"gen/generators"
	"log"
	"os"
)

func main() {
	useExistingGenesis := os.Getenv("EXTERNAL_GENESIS_FILE") == "1"
	fmt.Printf("Use existing genesis: %v\n", useExistingGenesis)

	// Get config paths from environment variables
	configPath := os.Getenv("CONFIG_JSON")
	validatorsPath := os.Getenv("VALIDATORS_JSON")

	cfg, validators, err := config.LoadConfigs(configPath, validatorsPath)
	if err != nil {
		log.Fatalf("Failed to load configurations: %v", err)
	}

	if useExistingGenesis {
		data, err := os.ReadFile("/tmp/lumera-devnet/shared/external_genesis.json")
		if err != nil {
			log.Fatalf("Failed to read existing genesis file: %v", err)
		}
		var genesis map[string]interface{}
		if err := json.Unmarshal(data, &genesis); err != nil {
			log.Fatalf("Failed to parse existing genesis file: %v", err)
		}

		genesisChainID, ok := genesis["chain_id"].(string)
		if !ok {
			log.Fatalf("chain_id not found or not a string in existing genesis")
		}

		if genesisChainID != cfg.Chain.ID {
			log.Fatalf("Existing genesis chain_id (%s) does not match config chain_id (%s)",
				genesisChainID, cfg.Chain.ID)
		}
	}

	// Generate init docker-compose
	compose, err := generators.GenerateDockerCompose(cfg, validators, useExistingGenesis)
	if err != nil {
		log.Fatalf("Failed to generate docker-compose configuration: %v", err)
	}

	err = generators.WriteDockerCompose(compose, "docker-compose.yml")
	if err != nil {
		log.Fatalf("Failed to write docker-compose.yml: %v", err)
	}

	// Generate start docker-compose
	startCompose, err := generators.GenerateStartDockerCompose(cfg, validators)
	if err != nil {
		log.Fatalf("Failed to generate start docker-compose configuration: %v", err)
	}

	err = generators.WriteDockerCompose(startCompose, "docker-compose.start.yml")
	if err != nil {
		log.Fatalf("Failed to write docker-compose.start.yml: %v", err)
	}

	// Generate validator scripts
	err = generators.GeneratePrimaryValidatorScript(cfg, validators, useExistingGenesis)
	if err != nil {
		log.Fatalf("Failed to generate primary validator script: %v", err)
	}

	err = generators.GenerateSecondaryValidatorScript(cfg, validators)
	if err != nil {
		log.Fatalf("Failed to generate secondary validator script: %v", err)
	}

	// Generate start script
	err = generators.GenerateStartScript(cfg)
	if err != nil {
		log.Fatalf("Failed to generate start script: %v", err)
	}

	fmt.Println("Successfully generated all configuration files:")
	fmt.Println("- docker-compose.yml (for initialization)")
	fmt.Println("- docker-compose.start.yml (for starting nodes)")
	fmt.Println("- primary-validator.sh")
	fmt.Println("- secondary-validator.sh")
	fmt.Println("- start.sh")
}
