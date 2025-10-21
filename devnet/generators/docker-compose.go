package generators

import (
	"fmt"
	confg "gen/config"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type DockerComposeLogging struct {
	Driver  string            `yaml:"driver"`
	Options map[string]string `yaml:"options,omitempty"`
}

type DockerComposeConfig struct {
	Services map[string]DockerComposeService `yaml:"services"`
	Networks map[string]DockerComposeNetwork `yaml:"networks"`
}

type DockerComposeService struct {
	Build         string                `yaml:"build"`
	Image         string                `yaml:"image,omitempty"`
	ContainerName string                `yaml:"container_name"`
	Ports         []string              `yaml:"ports"`
	Volumes       []string              `yaml:"volumes"`
	Environment   map[string]string     `yaml:"environment,omitempty"`
	Command       string                `yaml:"command,omitempty"`
	DependsOn     []string              `yaml:"depends_on,omitempty"`
	Logging       *DockerComposeLogging `yaml:"logging,omitempty"`
}

type DockerComposeNetwork struct {
	Name string `yaml:"name"`
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

func GenerateDockerCompose(config *confg.ChainConfig, validators []confg.Validator, useExistingGenesis bool) (*DockerComposeConfig, error) {
	compose := &DockerComposeConfig{
		Services: make(map[string]DockerComposeService),
		Networks: map[string]DockerComposeNetwork{
			"default": {
				Name: config.Docker.NetworkName,
			},
		},
	}

	_, snPresent := supernodeBinaryHostPath()

	folderMount := fmt.Sprintf("/tmp/%s", config.Chain.ID)

	for index, validator := range validators {
		serviceName := fmt.Sprintf("%s-%s", config.Docker.ContainerPrefix, validator.Name)
		env := map[string]string{
			"MONIKER": validator.Moniker,
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
			},
			Volumes: []string{
				fmt.Sprintf("%s/%s-data:/root/%s", folderMount, validator.Name, config.Paths.Directories.Daemon),
				fmt.Sprintf("%s/shared:/shared", folderMount),
			},
			Environment: env,
			Command:     fmt.Sprintf("bash %s/%s", FolderScripts, StartScript),
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
			if validator.SupernodePort > 0 {
				service.Ports = append(service.Ports, fmt.Sprintf("%d:%d", validator.SupernodePort, DefaultSupernodePort))
			}
			if validator.SupernodeP2PPort > 0 {
				service.Ports = append(service.Ports, fmt.Sprintf("%d:%d", validator.SupernodeP2PPort, DefaultSupernodeP2PPort))
			}
			if validator.SupernodeGatewayPort > 0 {
				service.Ports = append(service.Ports, fmt.Sprintf("%d:%d", validator.SupernodeGatewayPort, DefaultSupernodeGatewayPort))
			}
		}

		if index > 0 {
			service.DependsOn = []string{validators[0].Name}
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
			Environment: map[string]string{
				"HERMES_CONFIG": "/root/.hermes/config.toml",
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
