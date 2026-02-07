//go:build !purego && arm64

package neon

// AddMulBlock performs fused add-multiply: dst[i] = (a[i] + b[i]) * scale.
// Slices must have equal length. Panics if lengths differ.
// Uses ARM NEON SIMD instructions to process 2 float64 values at once.
func AddMulBlock(dst, a, b []float64, scale float64) {
	if len(a) != len(b) || len(dst) != len(a) {
		panic("vecmath: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	addMulBlockNEON(dst, a, b, scale)
}

// MulAddBlock performs fused multiply-add: dst[i] = a[i] * b[i] + c[i].
// Slices must have equal length. Panics if lengths differ.
// Uses ARM NEON SIMD instructions to process 2 float64 values at once.
func MulAddBlock(dst, a, b, c []float64) {
	if len(a) != len(b) || len(dst) != len(a) || len(c) != len(a) {
		panic("vecmath: slice length mismatch")
	}
	if len(dst) == 0 {
		return
	}
	mulAddBlockNEON(dst, a, b, c)
}

// Assembly function declarations (implemented in fused.s)

//go:noescape
func addMulBlockNEON(dst, a, b []float64, scale float64)

//go:noescape
func mulAddBlockNEON(dst, a, b, c []float64)
