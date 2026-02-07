//go:build !purego && arm64

package neon

// DotProduct returns the dot product of a and b: sum(a[i] * b[i]).
// Returns 0 if slices are empty or have different lengths.
// Only the minimum length of the two slices is used.
// Uses NEON SIMD instructions.
func DotProduct(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return 0
	}
	return dotProductNEON(a[:n], b[:n])
}

//go:noescape
func dotProductNEON(a, b []float64) float64
