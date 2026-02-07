//go:build !purego && arm64

package neon

// Magnitude computes magnitude from separate real and imaginary parts: dst[i] = sqrt(re[i]^2 + im[i]^2).
// Slices must have equal length. Panics if lengths differ.
// Uses ARM64 NEON SIMD instructions when available, with scalar fallback.
func Magnitude(dst, re, im []float64) {
	if len(re) != len(im) || len(dst) != len(re) {
		panic("vecmath: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	magnitudeNEON(dst, re, im)
}

// Assembly function declaration (implemented in magnitude.s)

//go:noescape
func magnitudeNEON(dst, re, im []float64)
