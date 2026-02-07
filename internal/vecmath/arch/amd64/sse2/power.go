//go:build !purego && amd64

package sse2

// Power computes power (magnitude squared) from separate real and imaginary parts: dst[i] = re[i]^2 + im[i]^2.
// Slices must have equal length. Panics if lengths differ.
// Uses SSE2 SIMD instructions when available, with scalar fallback.
func Power(dst, re, im []float64) {
	if len(re) != len(im) || len(dst) != len(re) {
		panic("vecmath: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	powerSSE2(dst, re, im)
}

// Assembly function declaration (implemented in power.s)

//go:noescape
func powerSSE2(dst, re, im []float64)
