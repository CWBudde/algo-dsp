//go:build !purego && arm64

package neon

// Sum returns the sum of all elements in x.
// Returns 0 for an empty slice.
// Uses NEON SIMD instructions.
func Sum(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}
	return sumNEON(x)
}

//go:noescape
func sumNEON(x []float64) float64
