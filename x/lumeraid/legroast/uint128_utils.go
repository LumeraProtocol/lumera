package legroast

import (
	"math/bits"

	"lukechampine.com/uint128"
)

var (
	m1   = uint128.From64(1)
	m127 = m1.Lsh(127).Sub(uint128.From64(1))

	Uint128Size uint32 = 16
)

// SafeAddUint128 safely adds two uint128 values without panicking on overflow.
// It returns the result and the carry flag, allowing for handling overflow externally.
func SafeAddUint128(u, v *uint128.Uint128) (uint128.Uint128, uint64) {
	lo, carryLo := bits.Add64(u.Lo, v.Lo, 0)
	hi, carryHi := bits.Add64(u.Hi, v.Hi, carryLo)
	return uint128.Uint128{Lo: lo, Hi: hi}, carryHi
}

// reduceModP reduces the value modulo m127.
func reduceModP(pa *uint128.Uint128) {
	*pa = pa.Mod(m127)
}

// addModP adds two values modulo m127.
func addModP(pa *uint128.Uint128, b uint128.Uint128) {
	sum, carry := SafeAddUint128(pa, &b)
	if carry > 0 {
		reduceModP(&sum)
		sum = sum.Add64(2)
	}
	*pa = sum
}

// squareModP computes the square of a value modulo m127.
func squareModP(out, a *uint128.Uint128) {
	reduceModP(a) // Ensure the input is reduced mod m127

	// Split the 128-bit number into high and low 64-bit components
	lowa := a.Lo
	higha := a.Hi

	// Compute the components of the square
	outLow := uint128.From64(lowa).Mul64(lowa)                      // lowa * lowa
	out64 := uint128.From64(lowa).Mul64(higha).Lsh(1)               // 2 * lowa * higha
	out127 := uint128.From64(higha).Mul64(higha).Add(out64.Rsh(64)) // higha * higha + carry
	out127 = out127.Lsh(1)                                          // Shift left

	out64 = out64.Lsh(64) // Shift left to align out64 with its position in 128 bits

	// Combine results with modular addition
	*out = outLow
	addModP(out, out127)
	addModP(out, out64)
}

// mulAddModP multiplies two values and adds to the output modulo m127.
func mulAddModP(out, a, b *uint128.Uint128) {
	reduceModP(a)
	reduceModP(b)

	out0 := uint128.From64(a.Lo).Mul64(b.Lo)
	out64 := uint128.From64(a.Lo).Mul64(b.Hi).Add(uint128.From64(b.Lo).Mul64(a.Hi))
	out127 := uint128.From64(a.Hi).Mul64(b.Hi).Add(out64.Rsh(64)).Lsh(1)

	out64 = out64.Lsh(64)

	addModP(out, out0)
	addModP(out, out127)
	addModP(out, out64)
}

// legendreSymbolCT calculates the Legendre symbol in constant time.
func legendreSymbolCT(a *uint128.Uint128) byte {
	out := *a
	temp := uint128.Uint128{}
	temp2 := uint128.Uint128{}

	// Initial sequence of squarings and multiplications
	squareModP(&temp, &out)
	out = uint128.Uint128{}
	mulAddModP(&out, &temp, a)
	squareModP(&temp, &out)
	out = uint128.Uint128{}
	mulAddModP(&out, &temp, a)
	squareModP(&temp, &out)
	out = uint128.Uint128{}
	mulAddModP(&out, &temp, a)
	squareModP(&temp, &out)
	out = uint128.Uint128{}
	mulAddModP(&out, &temp, a)
	squareModP(&temp, &out)
	out = uint128.Uint128{}
	mulAddModP(&out, &temp, a)
	temp2 = out

	// Loop for further squaring and multiplication
	for i := 0; i < 20; i++ {
		squareModP(&temp, &out)
		squareModP(&temp, &temp)
		squareModP(&temp, &temp)
		squareModP(&temp, &temp)
		squareModP(&temp, &temp)
		squareModP(&temp, &temp)

		out = uint128.Zero
		mulAddModP(&out, &temp, &temp2)
	}

	reduceModP(&out)
	return byte((-out.Lo + 1) / 2)
}
