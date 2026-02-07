package generic

// AddMulBlock performs fused add-multiply: dst[i] = (a[i] + b[i]) * scale.
// Slices must have equal length. Panics if lengths differ.
// This is the pure Go fallback implementation.
func AddMulBlock(dst, a, b []float64, scale float64) {
	if len(a) != len(b) || len(dst) != len(a) {
		panic("vecmath: slice length mismatch")
	}
	for i := range dst {
		dst[i] = (a[i] + b[i]) * scale
	}
}

// MulAddBlock performs fused multiply-add: dst[i] = a[i] * b[i] + c[i].
// Slices must have equal length. Panics if lengths differ.
// This is the pure Go fallback implementation.
func MulAddBlock(dst, a, b, c []float64) {
	if len(a) != len(b) || len(dst) != len(a) || len(c) != len(a) {
		panic("vecmath: slice length mismatch")
	}
	for i := range dst {
		dst[i] = a[i]*b[i] + c[i]
	}
}
