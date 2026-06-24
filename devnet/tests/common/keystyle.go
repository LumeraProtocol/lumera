package common

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// KeyStyle describes the HD-derivation/signing style for accounts on a given
// lumerad runtime: pre-EVM (Cosmos secp256k1, coin type 118) or EVM-enabled
// (eth_secp256k1, coin type 60).
type KeyStyle struct {
	Algo     string `json:"algo"`
	CoinType uint32 `json:"coin_type"`
	EVM      bool   `json:"evm"`
}

// KeyStyleLegacy is the pre-EVM Cosmos key style.
var KeyStyleLegacy = KeyStyle{Algo: "secp256k1", CoinType: 118, EVM: false}

// KeyStyleEVM is the EVM-enabled key style.
var KeyStyleEVM = KeyStyle{Algo: "eth_secp256k1", CoinType: 60, EVM: true}

// Name returns a stable short label ("legacy" or "evm") for the registry.
func (k KeyStyle) Name() string {
	if k.EVM {
		return "evm"
	}
	return "legacy"
}

var (
	semverExact    = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`)
	semverLabelled = regexp.MustCompile(`\bversion["']?\s*[:=]\s*"?v?(\d+)\.(\d+)\.(\d+)`)
	semverAny      = regexp.MustCompile(`v?(\d+)\.(\d+)\.(\d+)`)
)

// ExtractSemver parses a semantic version (vX.Y.Z) from a string, trying
// exact match, labelled "version:" lines, and fallback line scanning. It skips
// dependency lines and the Cosmos SDK version line to avoid misidentification.
func ExtractSemver(s string) (string, bool) {
	trimmed := strings.TrimSpace(s)
	if m := semverExact.FindStringSubmatch(trimmed); len(m) == 4 {
		return fmt.Sprintf("v%s.%s.%s", m[1], m[2], m[3]), true
	}
	if m := semverLabelled.FindStringSubmatch(s); len(m) == 4 {
		return fmt.Sprintf("v%s.%s.%s", m[1], m[2], m[3]), true
	}
	for line := range strings.SplitSeq(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "- ") || strings.Contains(line, "@v") {
			continue
		}
		if strings.Contains(line, "sdk_version") {
			continue
		}
		if m := semverAny.FindStringSubmatch(line); len(m) == 4 {
			return fmt.Sprintf("v%s.%s.%s", m[1], m[2], m[3]), true
		}
	}
	return "", false
}

// CompareSemver returns -1, 0, or 1 based on the ordering of two semver strings.
func CompareSemver(a, b string) (int, error) {
	parse := func(v string) ([3]int, error) {
		s, ok := ExtractSemver(v)
		if !ok {
			return [3]int{}, fmt.Errorf("invalid semver %q", v)
		}
		s = strings.TrimPrefix(s, "v")
		parts := strings.Split(s, ".")
		if len(parts) != 3 {
			return [3]int{}, fmt.Errorf("invalid semver %q", v)
		}
		var out [3]int
		for i, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil {
				return [3]int{}, fmt.Errorf("invalid semver %q: %w", v, err)
			}
			out[i] = n
		}
		return out, nil
	}
	av, err := parse(a)
	if err != nil {
		return 0, err
	}
	bv, err := parse(b)
	if err != nil {
		return 0, err
	}
	for i := range 3 {
		switch {
		case av[i] > bv[i]:
			return 1, nil
		case av[i] < bv[i]:
			return -1, nil
		}
	}
	return 0, nil
}

// KeyStyleForVersion maps a detected lumerad version to a KeyStyle, using the
// EVM cutover version as the threshold: versions >= cutover are EVM-enabled.
func KeyStyleForVersion(version, cutover string) (KeyStyle, error) {
	cmp, err := CompareSemver(version, cutover)
	if err != nil {
		return KeyStyle{}, err
	}
	if cmp >= 0 {
		return KeyStyleEVM, nil
	}
	return KeyStyleLegacy, nil
}
