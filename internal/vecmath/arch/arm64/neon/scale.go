//go:build !purego && arm64

package neon

// ScaleBlock multiplies each element by a scalar: dst[i] = src[i] * scale.
// Slices must have equal length. Panics if lengths differ.
// Uses ARM NEON SIMD instructions to process 2 float64 values at once.
func ScaleBlock(dst, src []float64, scale float64) {
	if len(dst) != len(src) {
		panic("vecmath: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	scaleBlockNEON(dst, src, scale)
}

// ScaleBlockInPlace multiplies each element by a scalar in-place: dst[i] *= scale.
// Uses ARM NEON SIMD instructions to process 2 float64 values at once.
func ScaleBlockInPlace(dst []float64, scale float64) {
	if len(dst) == 0 {
		return
	}
	scaleBlockInPlaceNEON(dst, scale)
}

// Assembly function declarations (implemented in scale.s)

//go:noescape
func scaleBlockNEON(dst, src []float64, scale float64)

//go:noescape
func scaleBlockInPlaceNEON(dst []float64, scale float64)
