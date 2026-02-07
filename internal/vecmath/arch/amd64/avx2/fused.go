//go:build !purego && amd64

package avx2

// AddMulBlock performs fused add-multiply: dst[i] = (a[i] + b[i]) * scale.
// Slices must have equal length. Panics if lengths differ.
// Uses AVX2 SIMD instructions when available, with scalar fallback.
func AddMulBlock(dst, a, b []float64, scale float64) {
	if len(a) != len(b) || len(dst) != len(a) {
		panic("vecmath: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	addMulBlockAVX2(dst, a, b, scale)
}

// MulAddBlock performs fused multiply-add: dst[i] = a[i] * b[i] + c[i].
// Slices must have equal length. Panics if lengths differ.
// Uses AVX2 SIMD instructions when available, with scalar fallback.
func MulAddBlock(dst, a, b, c []float64) {
	if len(a) != len(b) || len(dst) != len(a) || len(c) != len(a) {
		panic("vecmath: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	mulAddBlockAVX2(dst, a, b, c)
}

// Assembly function declarations (implemented in fused.s)

//go:noescape
func addMulBlockAVX2(dst, a, b []float64, scale float64)

//go:noescape
func mulAddBlockAVX2(dst, a, b, c []float64)
