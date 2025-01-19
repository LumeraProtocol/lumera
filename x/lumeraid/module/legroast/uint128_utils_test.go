package legroast

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"lukechampine.com/uint128"
)

func TestSafeAddUint128(t *testing.T) {
	tests := []struct {
		u, v, expected uint128.Uint128
		expectedCarry  uint64
	}{
		{uint128.From64(1), uint128.From64(2), uint128.From64(3), 0},
		{uint128.Max, uint128.From64(1), uint128.Zero, 1}, // Overflow case
		{uint128.New(1, 1), uint128.New(1, 1), uint128.New(2, 2), 0},
	}

	for _, tt := range tests {
		result, carry := SafeAddUint128(&tt.u, &tt.v)
		assert.Equal(t, tt.expected, result, "Result mismatch")
		assert.Equal(t, tt.expectedCarry, carry, "Carry mismatch")
	}
}

func TestReduceModP(t *testing.T) {
	tests := []struct {
		input, expected uint128.Uint128
	}{
		{uint128.From64(1), uint128.From64(1)},                                   // Input < m127
		{uint128.New(0xFFFFFFFFFFFFFFFF, 0), uint128.New(0xFFFFFFFFFFFFFFFF, 0)}, // Input < m127
		{uint128.New(1, 1), uint128.New(1, 1).Mod(m127)},                         // Input > m127
		{m127.Add(uint128.From64(1)), uint128.From64(1)},                         // Exact overflow by 1
	}

	for _, tt := range tests {
		input := tt.input
		reduceModP(&input)
		assert.Equal(t, tt.expected, input, "Result mismatch for input %v", tt.input)
	}
}

func TestAddModP(t *testing.T) {
	tests := []struct {
		a, b, expected uint128.Uint128
	}{
		{uint128.From64(1), uint128.From64(2), uint128.From64(3)}, // Simple addition
		{m127, uint128.From64(1), uint128.New(0, 0x8000000000000000)},
		{m127.Sub(uint128.From64(1)), uint128.From64(2), uint128.New(0, 0x8000000000000000)}, // Adding 2 to m127 - 1
		{uint128.From64(0), m127, m127},                                                      // Adding m127 to 0
		{uint128.From64(0), m127.Add(uint128.From64(1)), uint128.New(0, 0x8000000000000000)}, // Adding m127 + 1 to 0
	}

	for _, tt := range tests {
		a := tt.a
		addModP(&a, tt.b)
		assert.Equal(t, tt.expected, a, "Result mismatch for a=%v, b=%v", tt.a, tt.b)
	}
}

func TestSquareModP(t *testing.T) {
	tests := []struct {
		input, expected uint128.Uint128
	}{
		{uint128.From64(2), uint128.From64(4)},                            // 2^2 = 4
		{m127.Sub(uint128.From64(1)), uint128.New(0, 0x8000000000000000)}, // (m127 - 1)^2 mod m127 = 1
		{uint128.New(1, 1), uint128.New(3, 2)},                            // (2^64 + 1)^2 = 2^128 + 2^65 + 1
		{m127, uint128.From64(0)},                                         // m127^2 mod m127 = 0
	}

	for _, tt := range tests {
		output := uint128.Uint128{}
		squareModP(&output, &tt.input)
		assert.Equal(t, tt.expected, output, "Result mismatch for input %v", tt.input)
	}
}

func TestMulAddModP(t *testing.T) {
	tests := []struct {
		a, b, expected uint128.Uint128
	}{
		{uint128.From64(2), uint128.From64(3), uint128.From64(6)}, // 2 * 3 = 6
		// Multiplication at the boundary of m127
		{m127, uint128.From64(1), uint128.From64(0)},
		{uint128.New(1, 1), uint128.From64(2), uint128.New(2, 2)},
	}

	for _, tt := range tests {
		output := uint128.Zero
		mulAddModP(&output, &tt.a, &tt.b)
		assert.Equal(t, tt.expected, output, "Result mismatch for a=%s, b=%s", tt.a.String(), tt.b.String())
	}
}

func TestLegendreSymbolCT(t *testing.T) {
	tests := []struct {
		input    uint128.Uint128
		expected byte
	}{
		{uint128.From64(0), 0},             
		{uint128.From64(2), 0},             
		{uint128.From64(3), 1},             
		{m127.Sub(uint128.From64(1)), 1},   
		{uint128.From64(16), 0},            
		{uint128.From64(17), 0},          
	}

	for _, tt := range tests {
		result := legendreSymbolCT(&tt.input)
		assert.Equal(t, tt.expected, result, "Legendre symbol mismatch for input %v", tt.input)
	}
}
