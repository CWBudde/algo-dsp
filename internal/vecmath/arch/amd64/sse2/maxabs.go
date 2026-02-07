//go:build !purego && amd64

package sse2

// MaxAbs returns the maximum absolute value in x.
// Returns 0 for an empty slice.
// Uses SSE2 SIMD instructions.
func MaxAbs(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}
	return maxAbsSSE2(x)
}

//go:noescape
func maxAbsSSE2(x []float64) float64
