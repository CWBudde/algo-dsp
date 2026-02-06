//go:build !purego && amd64

package simd

// MulBlock performs element-wise multiplication: dst[i] = a[i] * b[i].
// Slices must have equal length. Panics if lengths differ.
// Uses AVX2 SIMD instructions when available, with scalar fallback.
func MulBlock(dst, a, b []float64) {
	if len(a) != len(b) || len(dst) != len(a) {
		panic("simd: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	mulBlockAVX2(dst, a, b)
}

// MulBlockInPlace performs in-place element-wise multiplication: dst[i] *= src[i].
// Slices must have equal length. Panics if lengths differ.
// Uses AVX2 SIMD instructions when available, with scalar fallback.
func MulBlockInPlace(dst, src []float64) {
	if len(dst) != len(src) {
		panic("simd: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	mulBlockInPlaceAVX2(dst, src)
}

// ScaleBlock multiplies each element by a scalar: dst[i] = src[i] * scale.
// Slices must have equal length. Panics if lengths differ.
// Uses AVX2 SIMD instructions when available, with scalar fallback.
func ScaleBlock(dst, src []float64, scale float64) {
	if len(dst) != len(src) {
		panic("simd: slice length mismatch")
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

// AddMulBlock performs fused add-multiply: dst[i] = (a[i] + b[i]) * scale.
// Slices must have equal length. Panics if lengths differ.
// Uses AVX2 SIMD instructions when available, with scalar fallback.
func AddMulBlock(dst, a, b []float64, scale float64) {
	if len(a) != len(b) || len(dst) != len(a) {
		panic("simd: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	addMulBlockAVX2(dst, a, b, scale)
}

// AddBlock performs element-wise addition: dst[i] = a[i] + b[i].
// Slices must have equal length. Panics if lengths differ.
// Uses AVX2 SIMD instructions when available, with scalar fallback.
func AddBlock(dst, a, b []float64) {
	if len(a) != len(b) || len(dst) != len(a) {
		panic("simd: slice length mismatch")
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
		panic("simd: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	addBlockInPlaceAVX2(dst, src)
}

// MulAddBlock performs fused multiply-add: dst[i] = a[i] * b[i] + c[i].
// Slices must have equal length. Panics if lengths differ.
// Uses AVX2 SIMD instructions when available, with scalar fallback.
func MulAddBlock(dst, a, b, c []float64) {
	if len(a) != len(b) || len(dst) != len(a) || len(c) != len(a) {
		panic("simd: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	mulAddBlockAVX2(dst, a, b, c)
}

// Assembly function declarations (implemented in mul_amd64.s)

//go:noescape
func mulBlockAVX2(dst, a, b []float64)

//go:noescape
func mulBlockInPlaceAVX2(dst, src []float64)

//go:noescape
func scaleBlockAVX2(dst, src []float64, scale float64)

//go:noescape
func scaleBlockInPlaceAVX2(dst []float64, scale float64)

//go:noescape
func addMulBlockAVX2(dst, a, b []float64, scale float64)

//go:noescape
func addBlockAVX2(dst, a, b []float64)

//go:noescape
func addBlockInPlaceAVX2(dst, src []float64)

//go:noescape
func mulAddBlockAVX2(dst, a, b, c []float64)
