//go:build !purego && amd64

package sse2

// MulBlock performs element-wise multiplication: dst[i] = a[i] * b[i].
// Slices must have equal length. Panics if lengths differ.
// Uses SSE2 SIMD instructions to process 2 float64 values at once.
func MulBlock(dst, a, b []float64) {
	if len(a) != len(b) || len(dst) != len(a) {
		panic("vecmath: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	mulBlockSSE2(dst, a, b)
}

// MulBlockInPlace performs in-place element-wise multiplication: dst[i] *= src[i].
// Slices must have equal length. Panics if lengths differ.
// Uses SSE2 SIMD instructions to process 2 float64 values at once.
func MulBlockInPlace(dst, src []float64) {
	if len(dst) != len(src) {
		panic("vecmath: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	mulBlockInPlaceSSE2(dst, src)
}

// Assembly function declarations (implemented in mul.s)

//go:noescape
func mulBlockSSE2(dst, a, b []float64)

//go:noescape
func mulBlockInPlaceSSE2(dst, src []float64)
