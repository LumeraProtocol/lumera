package legroast

import (
	"lukechampine.com/uint128"
)

type Uint128Matrix struct {
	X     uint32 // x-dimension
	Y     uint32 // y-dimension
	Z     uint32 // z-dimension
	XSize uint32 // number of uint128s in one x-dimension

	Data []uint128.Uint128 // Flattened data [x][y][z]
}

func NewUint128Matrix(x, y, z uint32) *Uint128Matrix {
	xSize := y * z

	// Allocate sufficient space for the matrix of uint128s
	size := x * xSize
	return &Uint128Matrix{
		X:     x,
		Y:     y,
		Z:     z,
		XSize: xSize,

		Data: make([]uint128.Uint128, size),
	}
}

// Clear resets all data in the matrix to zero.
func (o *Uint128Matrix) Clear() {
	for i := range o.Data {
		o.Data[i] = uint128.Zero
	}
}

// planeBaseIndex calculates the starting index in the flattened Data slice for a specific x plane.
func (o *Uint128Matrix) planeBaseIndex(n uint32) uint32 {
	if n >= o.X {
		panic("openings planeBaseIndex: n out of bounds")
	}
	return n * o.XSize
}

// GetPlainSlice returns the slice of openings for a specific round.
func (o *Uint128Matrix) GetPlainSlice(n uint32) []uint128.Uint128 {
	base := o.planeBaseIndex(n)
	return o.Data[base : base+o.XSize]
}

// GetAsBytes returns the openings as a byte slice.
func (o *Uint128Matrix) GetAsBytes() []byte {
	// Allocate a byte slice large enough to hold all the uint128 values
	result := make([]byte, uint32(len(o.Data))*Uint128Size)

	index := uint32(0)
	// Iterate through the data and copy each uint128 value as bytes
	for _, value := range o.Data {
		value.PutBytes(result[index : index+Uint128Size])
		index += Uint128Size
	}
	return result
}
