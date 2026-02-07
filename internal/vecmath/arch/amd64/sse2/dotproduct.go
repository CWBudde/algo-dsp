//go:build !purego && amd64

package sse2

// DotProduct returns the dot product of a and b: sum(a[i] * b[i]).
// Returns 0 if slices are empty or have different lengths.
// Only the minimum length of the two slices is used.
// Uses SSE2 SIMD instructions.
func DotProduct(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return 0
	}
	return dotProductSSE2(a[:n], b[:n])
}

//go:noescape
func dotProductSSE2(a, b []float64) float64
