// Package generic provides pure Go fallback implementations of vector math operations.
package generic

// MulBlock performs element-wise multiplication: dst[i] = a[i] * b[i].
// Slices must have equal length. Panics if lengths differ.
// This is the pure Go fallback implementation.
func MulBlock(dst, a, b []float64) {
	if len(a) != len(b) || len(dst) != len(a) {
		panic("vecmath: slice length mismatch")
	}
	for i := range dst {
		dst[i] = a[i] * b[i]
	}
}

// MulBlockInPlace performs in-place element-wise multiplication: dst[i] *= src[i].
// Slices must have equal length. Panics if lengths differ.
// This is the pure Go fallback implementation.
func MulBlockInPlace(dst, src []float64) {
	if len(dst) != len(src) {
		panic("vecmath: slice length mismatch")
	}
	for i := range dst {
		dst[i] *= src[i]
	}
}
