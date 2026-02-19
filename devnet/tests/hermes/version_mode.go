package hermes

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"

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
	if versionGTE(current, evmFrom) {
		return "evm"
	}
	return "cosmos"
}

func (s *ibcSimdSuite) lumeraKeyType() sdkcrypto.KeyType {
	if strings.EqualFold(s.lumeraKeyStyle, "cosmos") {
		return sdkcrypto.KeyTypeCosmos
	}
	return sdkcrypto.KeyTypeEVM
}

func versionGTE(current, floor string) bool {
	cMaj, cMin, cPatch, okC := parseVersion(current)
	fMaj, fMin, fPatch, okF := parseVersion(floor)
	if !okC || !okF {
		return strings.EqualFold(strings.TrimSpace(current), strings.TrimSpace(floor))
	}
	if cMaj != fMaj {
		return cMaj > fMaj
	}
	if cMin != fMin {
		return cMin > fMin
	}
	return cPatch >= fPatch
}

func parseVersion(v string) (int, int, int, bool) {
	norm := strings.TrimSpace(v)
	norm = strings.TrimPrefix(strings.TrimPrefix(norm, "v"), "V")
	if idx := strings.Index(norm, "-"); idx >= 0 {
		norm = norm[:idx]
	}
	if idx := strings.Index(norm, "+"); idx >= 0 {
		norm = norm[:idx]
	}
	parts := strings.Split(norm, ".")
	if len(parts) < 2 {
		return 0, 0, 0, false
	}
	maj, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, false
	}
	min, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, false
	}
	patch := 0
	if len(parts) > 2 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return 0, 0, 0, false
		}
	}
	return maj, min, patch, true
}
