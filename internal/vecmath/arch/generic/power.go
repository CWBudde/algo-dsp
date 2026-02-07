package generic

// Power computes power (magnitude squared) from separate real and imaginary parts: dst[i] = re[i]^2 + im[i]^2.
// Slices must have equal length. Panics if lengths differ.
// This is the pure Go fallback implementation.
func Power(dst, re, im []float64) {
	if len(re) != len(im) || len(dst) != len(re) {
		panic("vecmath: slice length mismatch")
	}
	for i := range dst {
		r := re[i]
		m := im[i]
		dst[i] = r*r + m*m
	}
}
