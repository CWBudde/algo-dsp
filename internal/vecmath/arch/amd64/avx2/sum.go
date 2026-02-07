//go:build !purego && amd64

package avx2

// Sum returns the sum of all elements in x.
// Returns 0 for an empty slice.
// Uses AVX2 SIMD instructions.
func Sum(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}
	return sumAVX2(x)
}

//go:noescape
func sumAVX2(x []float64) float64
