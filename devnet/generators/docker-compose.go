package generators

import (
	"fmt"
	confg "gen/config"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

const (
	defaultNetworkDriver  = "bridge"
	defaultNetworkSubnet  = "172.28.0.0/24"
	defaultNetworkPrefix  = "172.28.0."
	defaultServiceIPStart = 10
)

var semverPattern = regexp.MustCompile(`v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?`)

type DockerComposeLogging struct {
	Driver  string            `yaml:"driver"`
	Options map[string]string `yaml:"options,omitempty"`
}

type DockerComposeConfig struct {
	Services map[string]DockerComposeService `yaml:"services"`
	Networks map[string]DockerComposeNetwork `yaml:"networks"`
}

type DockerComposeService struct {
	Build         string                                 `yaml:"build"`
	Image         string                                 `yaml:"image,omitempty"`
	ContainerName string                                 `yaml:"container_name"`
	Ports         []string                               `yaml:"ports"`
	Volumes       []string                               `yaml:"volumes"`
	Environment   map[string]string                      `yaml:"environment,omitempty"`
	Command       string                                 `yaml:"command,omitempty"`
	DependsOn     []string                               `yaml:"depends_on,omitempty"`
	Networks      map[string]DockerComposeServiceNetwork `yaml:"networks,omitempty"`
	CapAdd        []string                               `yaml:"cap_add,omitempty"`
	SecurityOpt   []string                               `yaml:"security_opt,omitempty"`
	Logging       *DockerComposeLogging                  `yaml:"logging,omitempty"`
}

type DockerComposeNetwork struct {
	Name   string             `yaml:"name"`
	Driver string             `yaml:"driver,omitempty"`
	IPAM   *DockerComposeIPAM `yaml:"ipam,omitempty"`
}

type DockerComposeIPAM struct {
	Config []DockerComposeIPAMConfig `yaml:"config,omitempty"`
}

type DockerComposeIPAMConfig struct {
	Subnet string `yaml:"subnet,omitempty"`
}

type DockerComposeServiceNetwork struct {
	IPv4Address string `yaml:"ipv4_address,omitempty"`
}

func supernodeBinaryHostPath() (string, bool) {
	candidates := []string{
		filepath.Join(SubFolderBin, SupernodeBinary),
		filepath.Join("devnet", SubFolderBin, SupernodeBinary),
	}
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, true
		}
	}
	return "", false
}

func normalizeVersion(version string) string {
	out := strings.TrimSpace(version)
	if out == "" {
		return ""
	}
	match := semverPattern.FindString(out)
	if match == "" {
		return ""
	}
	if match[0] >= '0' && match[0] <= '9' {
		return "v" + match
	}
	return match
}

func detectLumeraVersion(binaryName string) string {
	binaryName = strings.TrimSpace(binaryName)
	if binaryName == "" {
		binaryName = "lumerad"
	}

	candidates := make([]string, 0, 4)
	if dir := strings.TrimSpace(os.Getenv("DEVNET_BIN_DIR")); dir != "" {
		candidates = append(candidates, filepath.Join(dir, binaryName))
	}
	candidates = append(candidates, filepath.Join(SubFolderBin, binaryName))
	if strings.ContainsRune(binaryName, os.PathSeparator) {
		candidates = append(candidates, binaryName)
	} else {
		candidates = append(candidates, binaryName)
	}

	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}

		resolved := candidate
		if strings.ContainsRune(candidate, os.PathSeparator) {
			info, err := os.Stat(candidate)
			if err != nil || info.IsDir() {
				continue
			}
		} else {
			path, err := exec.LookPath(candidate)
			if err != nil {
				continue
			}
			resolved = path
		}

		out, err := exec.Command(resolved, "version").CombinedOutput()
		if err != nil {
			continue
		}
		if version := normalizeVersion(string(out)); version != "" {
			return version
		}
	}

	return ""
}

func resolveLumeraChainVersion(config *confg.ChainConfig) (string, error) {
	if config == nil {
		return "", fmt.Errorf("nil chain config")
	}
	if version := normalizeVersion(config.Chain.Version); version != "" {
		return version, nil
	}
	if strings.TrimSpace(config.Chain.Version) != "" {
		return "", fmt.Errorf("invalid chain.version %q", config.Chain.Version)
	}
	if detected := detectLumeraVersion(config.Daemon.Binary); detected != "" {
		return detected, nil
	}
	binaryName := strings.TrimSpace(config.Daemon.Binary)
	if binaryName == "" {
		binaryName = "lumerad"
	}
	return "", fmt.Errorf(
		"failed to resolve Lumera version from binary %q; set chain.version in config.json or ensure DEVNET_BIN_DIR points to a working %s binary",
		binaryName,
		binaryName,
	)
}

func GenerateDockerCompose(config *confg.ChainConfig, validators []confg.Validator, useExistingGenesis bool) (*DockerComposeConfig, error) {
	compose := &DockerComposeConfig{
		Services: make(map[string]DockerComposeService),
		Networks: map[string]DockerComposeNetwork{
			"default": {
				Name:   config.Docker.NetworkName,
				Driver: defaultNetworkDriver,
				IPAM: &DockerComposeIPAM{
					Config: []DockerComposeIPAMConfig{
						{
							Subnet: defaultNetworkSubnet,
						},
					},
				},
			},
		},
	}

	_, snPresent := supernodeBinaryHostPath()

	folderMount := fmt.Sprintf("/tmp/%s", config.Chain.ID)
	validatorBaseIP := defaultServiceIPStart + 1
	chainVersion, err := resolveLumeraChainVersion(config)
	if err != nil {
		return nil, err
	}
	evmFromVersion := strings.TrimSpace(config.Chain.EVMFromVersion)
	if evmFromVersion == "" {
		evmFromVersion = confg.DefaultEVMFromVersion
	}

	for index, validator := range validators {
		serviceName := fmt.Sprintf("%s-%s", config.Docker.ContainerPrefix, validator.Name)
		env := map[string]string{
			"MONIKER":                  validator.Moniker,
			"LUMERA_VERSION":           chainVersion,
			"LUMERA_FIRST_EVM_VERSION": evmFromVersion,
		}

		// Pass useExistingGenesis to containers via ENV
		if useExistingGenesis {
			env["USE_EXISTING_GENESIS"] = "1"
		}
		env["INTEGRATION_TEST"] = "true"

		service := DockerComposeService{
			Build:         ".",
			ContainerName: serviceName,
			Ports: []string{
				fmt.Sprintf("%d:%d", validator.Port, DefaultP2PPort),
				fmt.Sprintf("%d:%d", validator.RPCPort, DefaultRPCPort),
				fmt.Sprintf("%d:%d", validator.RESTPort, DefaultRESTPort),
				fmt.Sprintf("%d:%d", validator.GRPCPort, DefaultGRPCPort),
				fmt.Sprintf("%d:%d", DefaultDebugPort+index, DefaultDebugPort),
			},
			Volumes: []string{
				fmt.Sprintf("%s/%s-data:/root/%s", folderMount, validator.Name, config.Paths.Directories.Daemon),
				fmt.Sprintf("%s/shared:/shared", folderMount),
			},
			Environment: env,
			Command:     fmt.Sprintf("bash %s/%s", FolderScripts, StartScript),
			CapAdd: []string{
				"SYS_PTRACE",
			},
			Networks: map[string]DockerComposeServiceNetwork{
				"default": {
					IPv4Address: fmt.Sprintf("%s%d", defaultNetworkPrefix, validatorBaseIP+index),
				},
			},
			SecurityOpt: []string{
				"seccomp=unconfined",
			},
			Logging: &DockerComposeLogging{
				Driver: "json-file",
				Options: map[string]string{
					"max-size": "10m",
					"max-file": "5",
				},
			},
		}

		if snPresent {
			// add supernode port mappings, if provided
			// container ports are fixed by supernode: 4444 (service), 4445 (p2p), 8002 (gateway)
			if validator.Supernode.Port > 0 {
				service.Ports = append(service.Ports, fmt.Sprintf("%d:%d", validator.Supernode.Port, DefaultSupernodePort))
			}
			if validator.Supernode.P2PPort > 0 {
				service.Ports = append(service.Ports, fmt.Sprintf("%d:%d", validator.Supernode.P2PPort, DefaultSupernodeP2PPort))
			}
			if validator.Supernode.GatewayPort > 0 {
				service.Ports = append(service.Ports, fmt.Sprintf("%d:%d", validator.Supernode.GatewayPort, DefaultSupernodeGatewayPort))
			}
		}

		// Optional JSON-RPC host bindings per validator.
		// Container ports are fixed by lumerad: 8545 (HTTP) and 8546 (WebSocket).
		if validator.JSONRPC.Port > 0 {
			service.Ports = append(service.Ports, fmt.Sprintf("%d:%d", validator.JSONRPC.Port, DefaultJSONRPCPort))
		}
		if validator.JSONRPC.WSPort > 0 {
			service.Ports = append(service.Ports, fmt.Sprintf("%d:%d", validator.JSONRPC.WSPort, DefaultJSONRPCWSPort))
		}

		if index > 0 {
			service.DependsOn = []string{validators[0].Name}
		}

		if validator.NetworkMaker.Enabled {
			nmGrpc := validator.NetworkMaker.GRPCPort
			if nmGrpc == 0 {
				nmGrpc = DefaultNetworkMakerGRPCPort
			}
			nmHTTP := validator.NetworkMaker.HTTPPort
			if nmHTTP == 0 {
				nmHTTP = DefaultNetworkMakerHTTPPort
			}
			service.Ports = append(service.Ports,
				fmt.Sprintf("%d:%d", nmGrpc, DefaultNetworkMakerGRPCPort),
				fmt.Sprintf("%d:%d", nmHTTP, DefaultNetworkMakerHTTPPort),
				fmt.Sprintf("%d:%d", DefaultNetworkMakerUIPort, DefaultNetworkMakerUIPort),
			)

			if config.NetworkMaker.GRPCPort > 0 {
				env[EnvNMAPIBase] = fmt.Sprintf("http://localhost:%d", nmHTTP)
			}
			if config.NetworkMaker.AccountBalance != "" {
				// reserve env slot for key if provided in config (optional)
			}
		}

		compose.Services[validator.Name] = service
	}

	if config.Hermes.Enabled {
		hermesService := DockerComposeService{
			Build:         "./hermes",
			ContainerName: fmt.Sprintf("%s-hermes", config.Docker.ContainerPrefix),
			Ports: []string{
				fmt.Sprintf("%d:%d", DefaultHermesSimdHostP2PPort, DefaultP2PPort),
				fmt.Sprintf("%d:%d", DefaultHermesSimdHostRPCPort, DefaultRPCPort),
				fmt.Sprintf("%d:%d", DefaultHermesSimdHostAPIPort, DefaultRESTPort),
				fmt.Sprintf("%d:%d", DefaultHermesSimdHostGRPCPort, DefaultGRPCPort),
				fmt.Sprintf("%d:%d", DefaultHermesSimdHostGRPCWebPort, DefaultGRPCWebPort),
			},
			Volumes: []string{
				fmt.Sprintf("%s/hermes-simd-data:%s", folderMount, HermesSimdHome),
				fmt.Sprintf("%s/hermes-router:%s", folderMount, HermesStateHome),
				fmt.Sprintf("%s/shared:/shared", folderMount),
			},
			Networks: map[string]DockerComposeServiceNetwork{
				"default": {
					IPv4Address: fmt.Sprintf("%s%d", defaultNetworkPrefix, defaultServiceIPStart),
				},
			},
			Environment: map[string]string{
				"HERMES_CONFIG":            "/root/.hermes/config.toml",
				"LUMERA_VERSION":           chainVersion,
				"LUMERA_FIRST_EVM_VERSION": evmFromVersion,
			},
			Logging: &DockerComposeLogging{
				Driver: "json-file",
				Options: map[string]string{
					"max-size": "10m",
					"max-file": "5",
				},
			},
		}

		if len(validators) > 0 {
			hermesService.DependsOn = []string{validators[0].Name}
		}

		compose.Services["hermes"] = hermesService
	}
	return compose, nil
}

func WriteDockerCompose(compose *DockerComposeConfig, filename string) error {
	data, err := yaml.Marshal(compose)
	if err != nil {
		return fmt.Errorf("error marshaling docker-compose: %v", err)
	}
	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("error writing docker-compose file: %v", err)
	}
	return nil
}
