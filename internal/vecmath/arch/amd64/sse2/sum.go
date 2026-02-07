//go:build !purego && amd64

package sse2

// Sum returns the sum of all elements in x.
// Returns 0 for an empty slice.
// Uses SSE2 SIMD instructions.
func Sum(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}
	return sumSSE2(x)
}

//go:noescape
func sumSSE2(x []float64) float64
