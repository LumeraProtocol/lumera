package validator

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultDaemonHome   = "/root/.lumera"
	defaultP2PPort      = 26656
	defaultRPCPort      = 26657
	defaultRESTPort     = 1317
	defaultGRPCPort     = 9090
	defaultJSONRPCPort  = 8545
	defaultJSONWSPort   = 8546
	defaultConfigToml   = "config.toml"
	defaultAppToml      = "app.toml"
	defaultConfigSubdir = "config"
)

type localLumeradPorts struct {
	P2P            int
	RPC            int
	REST           int
	GRPC           int
	JSONRPC        int
	JSONWS         int
	JSONRPCEnabled bool
}

func defaultLocalLumeradPorts() localLumeradPorts {
	return localLumeradPorts{
		P2P:            defaultP2PPort,
		RPC:            defaultRPCPort,
		REST:           defaultRESTPort,
		GRPC:           defaultGRPCPort,
		JSONRPC:        defaultJSONRPCPort,
		JSONWS:         defaultJSONWSPort,
		JSONRPCEnabled: true,
	}
}

func loadLocalLumeradPorts() (localLumeradPorts, error) {
	ports := defaultLocalLumeradPorts()
	daemonHome := strings.TrimSpace(os.Getenv("DAEMON_HOME"))
	if daemonHome == "" {
		daemonHome = defaultDaemonHome
	}

	configTomlPath := filepath.Join(daemonHome, defaultConfigSubdir, defaultConfigToml)
	appTomlPath := filepath.Join(daemonHome, defaultConfigSubdir, defaultAppToml)

	var errs []string
	if err := applyConfigTomlPorts(configTomlPath, &ports); err != nil {
		errs = append(errs, err.Error())
	}
	if err := applyAppTomlPorts(appTomlPath, &ports); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return ports, errors.New(strings.Join(errs, "; "))
	}
	return ports, nil
}

func applyConfigTomlPorts(path string, ports *localLumeradPorts) error {
	values, err := parseSimpleToml(path)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	if value := values["p2p"]["laddr"]; value != "" {
		if port, err := parsePortFromAddress(value); err == nil {
			ports.P2P = port
		}
	}
	if value := values["rpc"]["laddr"]; value != "" {
		if port, err := parsePortFromAddress(value); err == nil {
			ports.RPC = port
		}
	}
	return nil
}

func applyAppTomlPorts(path string, ports *localLumeradPorts) error {
	values, err := parseSimpleToml(path)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	if value := values["api"]["address"]; value != "" {
		if port, err := parsePortFromAddress(value); err == nil {
			ports.REST = port
		}
	}
	if value := values["grpc"]["address"]; value != "" {
		if port, err := parsePortFromAddress(value); err == nil {
			ports.GRPC = port
		}
	}
	if value := values["json-rpc"]["enable"]; value != "" {
		ports.JSONRPCEnabled = parseBool(value, ports.JSONRPCEnabled)
	}
	if value := values["json-rpc"]["address"]; value != "" {
		if port, err := parsePortFromAddress(value); err == nil {
			ports.JSONRPC = port
		}
	}
	if value := values["json-rpc"]["ws-address"]; value != "" {
		if port, err := parsePortFromAddress(value); err == nil {
			ports.JSONWS = port
		}
	}
	return nil
}

func parseSimpleToml(path string) (map[string]map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	out := make(map[string]map[string]string)
	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			if _, ok := out[section]; !ok {
				out[section] = make(map[string]string)
			}
			continue
		}

		eq := strings.Index(line, "=")
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		raw := strings.TrimSpace(line[eq+1:])
		value := parseTomlScalar(raw)
		if _, ok := out[section]; !ok {
			out[section] = make(map[string]string)
		}
		out[section][key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func parseTomlScalar(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "\"") {
		// common case in app/config TOML: key = "value"
		for i := 1; i < len(raw); i++ {
			if raw[i] == '"' && raw[i-1] != '\\' {
				return raw[1:i]
			}
		}
		return strings.Trim(raw, "\"")
	}
	if idx := strings.Index(raw, "#"); idx >= 0 {
		raw = raw[:idx]
	}
	return strings.TrimSpace(raw)
}

func parsePortFromAddress(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("empty address")
	}
	if idx := strings.Index(value, "://"); idx >= 0 {
		value = value[idx+3:]
	}
	colon := strings.LastIndex(value, ":")
	if colon < 0 || colon+1 >= len(value) {
		if port, err := strconv.Atoi(value); err == nil {
			return port, nil
		}
		return 0, fmt.Errorf("address %q missing port", value)
	}
	portStr := strings.TrimSpace(value[colon+1:])
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("parse port %q: %w", portStr, err)
	}
	return port, nil
}

func parseBool(value string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true":
		return true
	case "false":
		return false
	default:
		return fallback
	}
}
