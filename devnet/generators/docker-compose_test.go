package generators

import (
	"slices"
	"testing"

	confg "gen/config"
)

// evmValidator returns a validator carrying the contiguous-block EVM host-port
// scheme for index i (0-based): http=8645+i*100, ws=8646+i*100,
// metrics=8647+i*100, geth=8648+i*100.
func evmValidator(name string, i int) confg.Validator {
	v := confg.Validator{
		Name:     name,
		Moniker:  name,
		Port:     26666 + i*10,
		RPCPort:  26667 + i*10,
		RESTPort: 1327 + i*10,
		GRPCPort: 9091 + i,
	}
	v.JSONRPC.Port = 8645 + i*100
	v.JSONRPC.WSPort = 8646 + i*100
	v.JSONRPC.MetricsPort = 8647 + i*100
	v.JSONRPC.GethMetricsPort = 8648 + i*100
	return v
}

func hasPort(ports []string, mapping string) bool {
	return slices.Contains(ports, mapping)
}

// containsContainerPort reports whether any host:container mapping targets the
// given container port (the part after the colon).
func containsContainerPort(ports []string, container string) bool {
	for _, p := range ports {
		if len(p) > len(container) && p[len(p)-len(container)-1:] == ":"+container {
			return true
		}
	}
	return false
}

func newEVMChainConfig(version string) *confg.ChainConfig {
	cfg := &confg.ChainConfig{}
	cfg.Chain.ID = "lumera-devnet-1"
	cfg.Chain.Version = version
	cfg.Docker.NetworkName = "lumera-devnet"
	cfg.Docker.ContainerPrefix = "lumera"
	cfg.Paths.Directories.Daemon = ".lumera"
	return cfg
}

func TestGenerateDockerComposeOmitsEVMPortsForPreEVMVersion(t *testing.T) {
	cfg := newEVMChainConfig("v1.12.0") // pre-EVM (< default v1.20.0)
	validators := []confg.Validator{evmValidator("supernova_validator_1", 0)}

	compose, err := GenerateDockerCompose(cfg, validators, false)
	if err != nil {
		t.Fatalf("GenerateDockerCompose() error = %v", err)
	}

	ports := compose.Services["supernova_validator_1"].Ports
	for _, container := range []string{"8545", "8546", "6065", "8100"} {
		if containsContainerPort(ports, container) {
			t.Fatalf("pre-EVM compose should not publish EVM container port %s; got ports %v", container, ports)
		}
	}
}

func TestGenerateDockerComposeEmitsEVMPortsForEVMVersion(t *testing.T) {
	cfg := newEVMChainConfig("v1.20.0") // >= default evm_from_version
	validators := []confg.Validator{
		evmValidator("supernova_validator_1", 0),
		evmValidator("supernova_validator_2", 1),
	}

	compose, err := GenerateDockerCompose(cfg, validators, false)
	if err != nil {
		t.Fatalf("GenerateDockerCompose() error = %v", err)
	}

	v1 := compose.Services["supernova_validator_1"].Ports
	for _, want := range []string{"8645:8545", "8646:8546", "8647:6065", "8648:8100"} {
		if !hasPort(v1, want) {
			t.Fatalf("validator_1 missing EVM port mapping %s; got %v", want, v1)
		}
	}

	v2 := compose.Services["supernova_validator_2"].Ports
	for _, want := range []string{"8745:8545", "8746:8546", "8747:6065", "8748:8100"} {
		if !hasPort(v2, want) {
			t.Fatalf("validator_2 missing EVM port mapping %s; got %v", want, v2)
		}
	}
}

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
