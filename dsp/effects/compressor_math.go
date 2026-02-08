//go:build !fastmath

package effects

import "math"

// mathLog2 computes log2(x) using standard library math.
func mathLog2(x float64) float64 {
	return math.Log2(x)
}

// mathPower2 computes 2^x using standard library math.
func mathPower2(x float64) float64 {
	return math.Pow(2, x)
}

// mathPower10 computes 10^x using standard library math.
func mathPower10(x float64) float64 {
	return math.Pow(10, x)
}

// mathSqrt computes sqrt(x) using standard library math.
func mathSqrt(x float64) float64 {
	return math.Sqrt(x)
}
