//go:build !purego && arm64

package neon

// AddBlock performs element-wise addition: dst[i] = a[i] + b[i].
// Slices must have equal length. Panics if lengths differ.
// Uses ARM NEON SIMD instructions to process 2 float64 values at once.
func AddBlock(dst, a, b []float64) {
	if len(a) != len(b) || len(dst) != len(a) {
		panic("vecmath: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	addBlockNEON(dst, a, b)
}

// AddBlockInPlace performs in-place element-wise addition: dst[i] += src[i].
// Slices must have equal length. Panics if lengths differ.
// Uses ARM NEON SIMD instructions to process 2 float64 values at once.
func AddBlockInPlace(dst, src []float64) {
	if len(dst) != len(src) {
		panic("vecmath: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	addBlockInPlaceNEON(dst, src)
}

// Assembly function declarations (implemented in add.s)

//go:noescape
func addBlockNEON(dst, a, b []float64)

//go:noescape
func addBlockInPlaceNEON(dst, src []float64)
