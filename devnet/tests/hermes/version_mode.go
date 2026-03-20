package hermes

import (
	"encoding/json"
	"os"
	"strings"

	pkgversion "github.com/LumeraProtocol/lumera/pkg/version"
	sdkcrypto "github.com/LumeraProtocol/sdk-go/pkg/crypto"
)

const (
	defaultFirstEVMVersion = "v1.12.0"
	defaultConfigPath      = "/shared/config/config.json"
)

type devnetChainConfig struct {
	Chain struct {
		Version        string `json:"version"`
		EVMFromVersion string `json:"evm_from_version"`
	} `json:"chain"`
}

func readDevnetChainConfig() devnetChainConfig {
	paths := []string{
		strings.TrimSpace(os.Getenv("LUMERA_CONFIG_JSON")),
		defaultConfigPath,
		"config/config.json",
		"../../config/config.json",
	}
	for _, p := range paths {
		if p == "" {
			continue
		}
		bz, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cfg devnetChainConfig
		if json.Unmarshal(bz, &cfg) == nil {
			return cfg
		}
	}
	return devnetChainConfig{}
}

func resolveLumeraKeyStyle() string {
	explicit := strings.ToLower(strings.TrimSpace(os.Getenv("LUMERA_KEY_STYLE")))
	if explicit == "evm" || explicit == "cosmos" {
		return explicit
	}

	cfg := readDevnetChainConfig()

	current := strings.TrimSpace(os.Getenv("LUMERA_VERSION"))
	if current == "" {
		current = strings.TrimSpace(cfg.Chain.Version)
	}

	evmFrom := strings.TrimSpace(os.Getenv("LUMERA_FIRST_EVM_VERSION"))
	if evmFrom == "" {
		evmFrom = strings.TrimSpace(cfg.Chain.EVMFromVersion)
	}
	if evmFrom == "" {
		evmFrom = defaultFirstEVMVersion
	}

	if current == "" {
		// EVM is the default for current devnet when version is not provided.
		return "evm"
	}
	if pkgversion.GTE(current, evmFrom) {
		return "evm"
	}
	return "cosmos"
}

func (s *lumeraHermesSuite) lumeraKeyType() sdkcrypto.KeyType {
	if strings.EqualFold(s.lumeraKeyStyle, "cosmos") {
		return sdkcrypto.KeyTypeCosmos
	}
	return sdkcrypto.KeyTypeEVM
}
