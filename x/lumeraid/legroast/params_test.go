package legroast_test

import (
	"testing"

	. "github.com/LumeraProtocol/lumera/x/lumeraid/legroast"

	"github.com/stretchr/testify/require"
)

func TestLegRoastAlgorithmString(t *testing.T) {
	tests := []struct {
		name     string
		alg      LegRoastAlgorithm
		expected string
	}{
		{name: "LegendreFast", alg: LegendreFast, expected: "LegendreFast"},
		{name: "LegendreMiddle", alg: LegendreMiddle, expected: "LegendreMiddle"},
		{name: "LegendreCompact", alg: LegendreCompact, expected: "LegendreCompact"},
		{name: "PowerFast", alg: PowerFast, expected: "PowerFast"},
		{name: "PowerMiddle", alg: PowerMiddle, expected: "PowerMiddle"},
		{name: "PowerCompact", alg: PowerCompact, expected: "PowerCompact"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.alg.String())
		})
	}
}

func TestLegRoastAlgorithmString_Unknown(t *testing.T) {
	alg := LegRoastAlgorithm(100)
	require.Equal(t, "Unknown", alg.String())
}

// GetAlgorithmBySigSize
func TestGetAlgorithmBySigSize(t *testing.T) {
	tests := []struct {
		name     string
		sigSize  int
		expected LegRoastAlgorithm
	}{
		{name: "LegendreFast", sigSize: 16480, expected: LegendreFast},
		{name: "LegendreMiddle", sigSize: 14272, expected: LegendreMiddle},
		{name: "LegendreCompact", sigSize: 12544, expected: LegendreCompact},
		{name: "PowerFast", sigSize: 8800, expected: PowerFast},
		{name: "PowerMiddle", sigSize: 7408, expected: PowerMiddle},
		{name: "PowerCompact", sigSize: 6448, expected: PowerCompact},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alg, err := GetAlgorithmBySigSize(tt.sigSize)
			require.NoError(t, err)
			require.Equal(t, tt.expected, alg)
		})
	}
}

// test GetAlgorithmBySigSize with invalid sigSize
func TestGetAlgorithmBySigSize_InvalidSigSize(t *testing.T) {
	_, err := GetAlgorithmBySigSize(100)
	require.Error(t, err)
}

// GetLegRoastAlgorithm
func TestGetLegRoastAlgorithm(t *testing.T) {
	tests := []struct {
		name     string
		alg      string
		expected LegRoastAlgorithm
	}{
		{name: "LegendreFast", alg: "LegendreFast", expected: LegendreFast},
		{name: "LegendreMiddle", alg: "LegendreMiddle", expected: LegendreMiddle},
		{name: "LegendreCompact", alg: "LegendreCompact", expected: LegendreCompact},
		{name: "PowerFast", alg: "PowerFast", expected: PowerFast},
		{name: "PowerMiddle", alg: "PowerMiddle", expected: PowerMiddle},
		{name: "PowerCompact", alg: "PowerCompact", expected: PowerCompact},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alg, err := GetLegRoastAlgorithm(tt.alg)
			require.NoError(t, err)
			require.Equal(t, tt.expected, alg)
		})
	}
}

func TestGetLegRoastAlgorithmEmpty(t *testing.T) {
	alg, err := GetLegRoastAlgorithm("")
	require.Equal(t, DefaultAlgorithm, alg)
	require.NoError(t, err)
}

func TestGetLegRoastAlgorithmUnknown(t *testing.T) {
	_, err := GetLegRoastAlgorithm("Unknown")
	require.Error(t, err)
}
