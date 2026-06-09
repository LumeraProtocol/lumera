package generators

import (
	"testing"

	confg "gen/config"
)

func TestGenerateDockerComposeDoesNotDisableValidatorOneHostReporterByDefault(t *testing.T) {
	cfg := &confg.ChainConfig{}
	cfg.Chain.ID = "lumera-devnet-1"
	cfg.Chain.Version = "v1.12.0"
	cfg.Docker.NetworkName = "lumera-devnet"
	cfg.Docker.ContainerPrefix = "lumera"
	cfg.Paths.Directories.Daemon = ".lumera"

	validators := []confg.Validator{
		{
			Name:     "supernova_validator_1",
			Moniker:  "validator-1",
			Port:     26666,
			RPCPort:  26667,
			RESTPort: 1327,
			GRPCPort: 9091,
		},
	}

	compose, err := GenerateDockerCompose(cfg, validators, false)
	if err != nil {
		t.Fatalf("GenerateDockerCompose() error = %v", err)
	}

	env := compose.Services["supernova_validator_1"].Environment
	if got := env["EVERLIGHT_TEST_TARGET"]; got != "0" {
		t.Fatalf("EVERLIGHT_TEST_TARGET = %q, want %q", got, "0")
	}
}
