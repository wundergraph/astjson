package fastfloat

// This file implements the Eisel-Lemire ParseFloat algorithm for float64.
// Adapted from Go standard library (src/strconv/eisel_lemire.go).
//
// Reference: https://nigeltao.github.io/blog/2020/eisel-lemire.html

import (
	"math"
	"math/bits"
)

// eiselLemire64 attempts to compute the float64 for mantissa * 10^exp10.
// Returns (result, true) on success, or (0, false) if fallback is needed.
//
// mantissa is the integer mantissa (all significant digits concatenated).
// exp10 is the power-of-10 exponent adjustment.
// neg indicates if the number is negative.
//
// For example, the number -1.234e5 would be represented as:
//   mantissa=1234, exp10=5-3=2, neg=true
func eiselLemire64(neg bool, mantissa uint64, exp10 int) (float64, bool) {
	// The terse comments in this function body refer to sections of the
	// https://nigeltao.github.io/blog/2020/eisel-lemire.html blog post.

	// Exp10 Range.
	if mantissa == 0 {
		if neg {
			return math.Float64frombits(0x8000000000000000), true // Negative zero.
		}
		return 0, true
	}
	if exp10 < detailedPowersOfTenMinExp10 || detailedPowersOfTenMaxExp10 < exp10 {
		return 0, false
	}

	// Normalization.
	clz := bits.LeadingZeros64(mantissa)
	mantissa <<= uint(clz)
	const float64ExponentBias = 1023
	retExp2 := uint64(217706*exp10>>16+64+float64ExponentBias) - uint64(clz)

	// Multiplication.
	xHi, xLo := bits.Mul64(mantissa, detailedPowersOfTen[exp10-detailedPowersOfTenMinExp10][1])

	// Wider Approximation.
	if xHi&0x1FF == 0x1FF && xLo+mantissa < mantissa {
		yHi, yLo := bits.Mul64(mantissa, detailedPowersOfTen[exp10-detailedPowersOfTenMinExp10][0])
		mergedHi, mergedLo := xHi, xLo+yHi
		if mergedLo < xLo {
			mergedHi++
		}
		if mergedHi&0x1FF == 0x1FF && mergedLo+1 == 0 && yLo+mantissa < mantissa {
			return 0, false
		}
		xHi, xLo = mergedHi, mergedLo
	}

	// Shifting to 54 Bits.
	msb := xHi >> 63
	retMantissa := xHi >> (msb + 9)
	retExp2 -= 1 ^ msb

	// Half-way Ambiguity.
	if xLo == 0 && xHi&0x1FF == 0 && retMantissa&3 == 1 {
		return 0, false
	}

	// From 54 to 53 Bits.
	retMantissa += retMantissa & 1
	retMantissa >>= 1
	if retMantissa>>53 > 0 {
		retMantissa >>= 1
		retExp2 += 1
	}
	// retExp2 is a uint64. Zero or underflow means that we're in subnormal
	// float64 space. 0x7FF or above means that we're in Inf/NaN float64 space.
	if retExp2-1 >= 0x7FF-1 {
		return 0, false
	}
	retBits := retExp2<<52 | retMantissa&0x000FFFFFFFFFFFFF
	if neg {
		retBits |= 0x8000000000000000
	}
	return math.Float64frombits(retBits), true
}
