//go:build !purego && amd64

package avx2

// AddBlock performs element-wise addition: dst[i] = a[i] + b[i].
// Slices must have equal length. Panics if lengths differ.
// Uses AVX2 SIMD instructions when available, with scalar fallback.
func AddBlock(dst, a, b []float64) {
	if len(a) != len(b) || len(dst) != len(a) {
		panic("vecmath: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	addBlockAVX2(dst, a, b)
}

// AddBlockInPlace performs in-place element-wise addition: dst[i] += src[i].
// Slices must have equal length. Panics if lengths differ.
// Uses AVX2 SIMD instructions when available, with scalar fallback.
func AddBlockInPlace(dst, src []float64) {
	if len(dst) != len(src) {
		panic("vecmath: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	addBlockInPlaceAVX2(dst, src)
}

// Assembly function declarations (implemented in add.s)

//go:noescape
func addBlockAVX2(dst, a, b []float64)

//go:noescape
func addBlockInPlaceAVX2(dst, src []float64)
