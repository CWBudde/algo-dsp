//go:build !purego && amd64

package avx2

// ScaleBlock multiplies each element by a scalar: dst[i] = src[i] * scale.
// Slices must have equal length. Panics if lengths differ.
// Uses AVX2 SIMD instructions when available, with scalar fallback.
func ScaleBlock(dst, src []float64, scale float64) {
	if len(dst) != len(src) {
		panic("vecmath: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	scaleBlockAVX2(dst, src, scale)
}

// ScaleBlockInPlace multiplies each element by a scalar in-place: dst[i] *= scale.
// Uses AVX2 SIMD instructions when available, with scalar fallback.
func ScaleBlockInPlace(dst []float64, scale float64) {
	if len(dst) == 0 {
		return
	}
	scaleBlockInPlaceAVX2(dst, scale)
}

// Assembly function declarations (implemented in scale.s)

//go:noescape
func scaleBlockAVX2(dst, src []float64, scale float64)

//go:noescape
func scaleBlockInPlaceAVX2(dst []float64, scale float64)
