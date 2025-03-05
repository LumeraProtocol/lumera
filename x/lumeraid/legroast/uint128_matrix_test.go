package legroast_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"lukechampine.com/uint128"

	. "github.com/LumeraProtocol/lumera/x/lumeraid/legroast"
)

func TestNewUint128Matrix(t *testing.T) {
	x, y, z := uint32(3), uint32(4), uint32(5)
	matrix := NewUint128Matrix(x, y, z)

	require.NotNil(t, matrix)
	assert.Equal(t, x, matrix.X, "x dimension should be correct")
	assert.Equal(t, y, matrix.Y, "y dimension should be correct")
	assert.Equal(t, z, matrix.Z, "z dimension should be correct")
	assert.Equal(t, y*z, matrix.XSize, "XSize should be y * z")
	assert.Equal(t, int(x*y*z), len(matrix.Data), "Data slice should have correct size")
}

func TestUint128Matrix_Clear(t *testing.T) {
	matrix := NewUint128Matrix(3, 4, 5)

	// Fill the data with non-zero values
	for i := range matrix.Data {
		matrix.Data[i] = uint128.From64(uint64(i + 1))
	}

	matrix.Clear()

	// Ensure all values are reset to zero
	for _, value := range matrix.Data {
		assert.Equal(t, uint128.Zero, value, "Matrix data should be cleared to zero")
	}
}

func TestUint128Matrix_GetPlainSlice(t *testing.T) {
	x, y, z := uint32(3), uint32(4), uint32(5)
	matrix := NewUint128Matrix(x, y, z)

	// Assign values to the matrix
	for i := range matrix.Data {
		matrix.Data[i] = uint128.From64(uint64(i + 1))
	}

	// Test accessing the first plane (n = 0)
	plane := matrix.GetPlainSlice(0)
	require.Equal(t, int(y*z), len(plane), "Plane slice should have the correct size")
	assert.Equal(t, matrix.Data[:len(plane)], plane, "Plane slice should match the first plane of the matrix")

	// Test accessing another plane (n = 1)
	plane = matrix.GetPlainSlice(1)
	start := y * z
	require.Equal(t, int(y*z), len(plane), "Plane slice should have the correct size")
	assert.Equal(t, matrix.Data[start:start+uint32(len(plane))], plane, "Plane slice should match the second plane of the matrix")
}

func TestUint128Matrix_GetAsBytes(t *testing.T) {
	matrix := NewUint128Matrix(2, 3, 4)

	// Populate the matrix with specific values
	for i := range matrix.Data {
		matrix.Data[i] = uint128.From64(uint64(i + 1))
	}

	bytes := matrix.GetAsBytes()
	expectedSize := len(matrix.Data) * int(Uint128Size)
	require.Equal(t, expectedSize, len(bytes), "Byte slice should have the correct size")

	// Verify that each uint128 value is correctly represented in bytes
	for i, value := range matrix.Data {
		start := i * int(Uint128Size)
		end := start + int(Uint128Size)

		expected := make([]byte, Uint128Size)
		value.PutBytes(expected)
		assert.Equal(t, expected, bytes[start:end], "Byte representation should match")
	}
}

func TestUint128Matrix_GetPlaneBaseIndex_OutOfBounds(t *testing.T) {
	matrix := NewUint128Matrix(2, 3, 4)

	assert.PanicsWithValue(t, "openings planeBaseIndex: n out of bounds", func() {
		matrix.GetPlainSlice(3)
	}, "Accessing an out-of-bounds plane should panic")
}
