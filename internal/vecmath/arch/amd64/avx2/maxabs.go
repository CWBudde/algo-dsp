//go:build !purego && amd64

package avx2

// MaxAbs returns the maximum absolute value in x.
// Returns 0 for an empty slice.
// Uses AVX2 SIMD instructions.
func MaxAbs(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}
	return maxAbsAVX2(x)
}

//go:noescape
func maxAbsAVX2(x []float64) float64
