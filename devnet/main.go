package main

import (
	"fmt"
	"gen/config"
	"gen/generators"
	"log"
)

func main() {
	config, validators, err := config.LoadConfigs()
	if err != nil {
		log.Fatalf("Failed to load configurations: %v", err)
	}

	compose, err := generators.GenerateDockerCompose(config, validators)
	if err != nil {
		log.Fatalf("Failed to generate docker-compose configuration: %v", err)
	}

	err = generators.WriteDockerCompose(compose, "docker-compose.yml")
	if err != nil {
		log.Fatalf("Failed to write docker-compose.yml: %v", err)
	}

	err = generators.GeneratePrimaryValidatorScript(config, validators)
	if err != nil {
		log.Fatalf("Failed to generate validator script: %v", err)
	}

	err = generators.GenerateSecondaryValidatorScript(config, validators)
	if err != nil {
		log.Fatalf("Failed to generate validator script: %v", err)
	}

	fmt.Println("Successfully generated docker-compose.yml")
}
