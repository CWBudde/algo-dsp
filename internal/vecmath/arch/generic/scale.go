package generic

// ScaleBlock multiplies each element by a scalar: dst[i] = src[i] * scale.
// Slices must have equal length. Panics if lengths differ.
// This is the pure Go fallback implementation.
func ScaleBlock(dst, src []float64, scale float64) {
	if len(dst) != len(src) {
		panic("vecmath: slice length mismatch")
	}
	for i := range dst {
		dst[i] = src[i] * scale
	}
}

// ScaleBlockInPlace multiplies each element by a scalar in-place: dst[i] *= scale.
// This is the pure Go fallback implementation.
func ScaleBlockInPlace(dst []float64, scale float64) {
	for i := range dst {
		dst[i] *= scale
	}
}
