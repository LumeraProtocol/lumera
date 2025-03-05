package generators

import (
	"fmt"
	confg "gen/config"
	"os"

	"gopkg.in/yaml.v2"
)

type DockerComposeConfig struct {
	Services map[string]DockerComposeService `yaml:"services"`
	Networks map[string]DockerComposeNetwork `yaml:"networks"`
}

type DockerComposeService struct {
	Build         string            `yaml:"build"`
	ContainerName string            `yaml:"container_name"`
	Ports         []string          `yaml:"ports"`
	Volumes       []string          `yaml:"volumes"`
	Environment   map[string]string `yaml:"environment"`
	Command       string            `yaml:"command,omitempty"`
	DependsOn     []string          `yaml:"depends_on,omitempty"`
}

type DockerComposeNetwork struct {
	Name string `yaml:"name"`
}

const (
	DefaultP2PPort  = 26656
	DefaultRPCPort  = 26657
	DefaultRESTPort = 1317
	DefaultGRPCPort = 9090
)

func GenerateDockerCompose(config *confg.ChainConfig, validators []confg.Validator, useExistingGenesis bool) (*DockerComposeConfig, error) {
	compose := &DockerComposeConfig{
		Services: make(map[string]DockerComposeService),
		Networks: map[string]DockerComposeNetwork{
			"default": {
				Name: config.Docker.NetworkName,
			},
		},
	}

	for index, validator := range validators {
		serviceName := fmt.Sprintf("%s-%s", config.Docker.ContainerPrefix, validator.Name)
		env := map[string]string{
			"MONIKER": validator.Moniker,
		}

		// Pass useExistingGenesis to containers via ENV
		if useExistingGenesis {
			env["USE_EXISTING_GENESIS"] = "1"
		}

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
				fmt.Sprintf("/tmp/lumera-devnet/%s-data:/root/%s", validator.Name, config.Paths.Directories.Daemon),
				"/tmp/lumera-devnet/shared:/shared",
			},
			Environment: env,
		}

		if index == 0 {
			service.Command = "bash /root/scripts/primary-validator.sh"
		} else {
			service.Command = fmt.Sprintf("bash /root/scripts/secondary-validator.sh %s %s %s %s",
				validator.KeyName,
				validator.InitialDistribution.ValidatorStake,
				validator.Name,
				validator.InitialDistribution.AccountBalance)
			service.DependsOn = []string{validators[0].Name}
		}

		compose.Services[validator.Name] = service
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

func GenerateStartDockerCompose(config *confg.ChainConfig, validators []confg.Validator) (*DockerComposeConfig, error) {
	compose := &DockerComposeConfig{
		Services: make(map[string]DockerComposeService),
		Networks: map[string]DockerComposeNetwork{
			"default": {
				Name: config.Docker.NetworkName,
			},
		},
	}

	for index, validator := range validators {
		serviceName := fmt.Sprintf("%s-%s", config.Docker.ContainerPrefix, validator.Name)

		service := DockerComposeService{
			ContainerName: serviceName,
			Ports: []string{
				fmt.Sprintf("%d:%d", validator.Port, DefaultP2PPort),
				fmt.Sprintf("%d:%d", validator.RPCPort, DefaultRPCPort),
				fmt.Sprintf("%d:%d", validator.RESTPort, DefaultRESTPort),
				fmt.Sprintf("%d:%d", validator.GRPCPort, DefaultGRPCPort),
			},
			Volumes: []string{
				fmt.Sprintf("/tmp/lumera-devnet/%s-data:/root/%s", validator.Name, config.Paths.Directories.Daemon),
				"/tmp/lumera-devnet/shared:/shared",
			},
			Command: "bash /root/scripts/start.sh",
		}

		if index > 0 {
			service.DependsOn = []string{validators[0].Name}
		}

		compose.Services[validator.Name] = service
	}
	return compose, nil
}
