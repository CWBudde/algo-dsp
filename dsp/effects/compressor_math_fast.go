//go:build fastmath

package effects

import (
	"math"

	"github.com/meko-christian/algo-approx"
)

// ln2 is the natural logarithm of 2, used for log base conversions.
const ln2 = 0.693147180559945309417232121458

// mathLog2 computes log2(x) using fast approximation.
// Uses the identity: log2(x) = ln(x) / ln(2)
func mathLog2(x float64) float64 {
	return approx.FastLog(x) / ln2
}

// mathPower2 computes 2^x using fast approximation.
// Uses the identity: 2^x = e^(x * ln(2))
func mathPower2(x float64) float64 {
	return approx.FastExp(x * ln2)
}

// mathPower10 computes 10^x using standard library.
// Note: algo-approx doesn't provide direct power-of-10, so we use math.Pow
// for makeup gain calculation (called once per parameter change, not in hot path).
func mathPower10(x float64) float64 {
	return math.Pow(10, x)
}

// mathSqrt computes sqrt(x) using fast approximation.
func mathSqrt(x float64) float64 {
	return approx.FastSqrt(x)
}
