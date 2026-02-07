package generic

import "math"

// Magnitude computes magnitude from separate real and imaginary parts: dst[i] = sqrt(re[i]^2 + im[i]^2).
// Slices must have equal length. Panics if lengths differ.
// This is the pure Go fallback implementation.
func Magnitude(dst, re, im []float64) {
	if len(re) != len(im) || len(dst) != len(re) {
		panic("vecmath: slice length mismatch")
	}
	for i := range dst {
		r := re[i]
		m := im[i]
		dst[i] = math.Sqrt(r*r + m*m)
	}
}
