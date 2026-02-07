//go:build !purego && arm64

package neon

// MulBlock performs element-wise multiplication: dst[i] = a[i] * b[i].
// Slices must have equal length. Panics if lengths differ.
// Uses ARM NEON SIMD instructions to process 2 float64 values at once.
func MulBlock(dst, a, b []float64) {
	if len(a) != len(b) || len(dst) != len(a) {
		panic("vecmath: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	mulBlockNEON(dst, a, b)
}

// MulBlockInPlace performs in-place element-wise multiplication: dst[i] *= src[i].
// Slices must have equal length. Panics if lengths differ.
// Uses ARM NEON SIMD instructions to process 2 float64 values at once.
func MulBlockInPlace(dst, src []float64) {
	if len(dst) != len(src) {
		panic("vecmath: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	mulBlockInPlaceNEON(dst, src)
}

// Assembly function declarations (implemented in mul.s)

//go:noescape
func mulBlockNEON(dst, a, b []float64)

//go:noescape
func mulBlockInPlaceNEON(dst, src []float64)
