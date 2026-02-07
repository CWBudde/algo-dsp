//go:build !purego && amd64

package sse2

// ScaleBlock multiplies each element by a scalar: dst[i] = src[i] * scale.
// Slices must have equal length. Panics if lengths differ.
// Uses SSE2 SIMD instructions to process 2 float64 values at once.
func ScaleBlock(dst, src []float64, scale float64) {
	if len(dst) != len(src) {
		panic("vecmath: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	scaleBlockSSE2(dst, src, scale)
}

// ScaleBlockInPlace multiplies each element by a scalar in-place: dst[i] *= scale.
// Uses SSE2 SIMD instructions to process 2 float64 values at once.
func ScaleBlockInPlace(dst []float64, scale float64) {
	if len(dst) == 0 {
		return
	}
	scaleBlockInPlaceSSE2(dst, scale)
}

// Assembly function declarations (implemented in scale.s)

//go:noescape
func scaleBlockSSE2(dst, src []float64, scale float64)

//go:noescape
func scaleBlockInPlaceSSE2(dst []float64, scale float64)
