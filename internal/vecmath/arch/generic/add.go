package generic

// AddBlock performs element-wise addition: dst[i] = a[i] + b[i].
// Slices must have equal length. Panics if lengths differ.
// This is the pure Go fallback implementation.
func AddBlock(dst, a, b []float64) {
	if len(a) != len(b) || len(dst) != len(a) {
		panic("vecmath: slice length mismatch")
	}
	for i := range dst {
		dst[i] = a[i] + b[i]
	}
}

// AddBlockInPlace performs in-place element-wise addition: dst[i] += src[i].
// Slices must have equal length. Panics if lengths differ.
// This is the pure Go fallback implementation.
func AddBlockInPlace(dst, src []float64) {
	if len(dst) != len(src) {
		panic("vecmath: slice length mismatch")
	}
	for i := range dst {
		dst[i] += src[i]
	}
}
