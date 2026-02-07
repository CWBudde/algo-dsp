//go:build !purego && arm64

package vecmath

import "github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic"

// AddMulBlock performs fused add-multiply: dst[i] = (a[i] + b[i]) * scale.
// Slices must have equal length. Panics if lengths differ.
// This is the arm64 fallback implementation.
func AddMulBlock(dst, a, b []float64, scale float64) {
	generic.AddMulBlock(dst, a, b, scale)
}

// MulAddBlock performs fused multiply-add: dst[i] = a[i] * b[i] + c[i].
// Slices must have equal length. Panics if lengths differ.
// This is the arm64 fallback implementation.
func MulAddBlock(dst, a, b, c []float64) {
	generic.MulAddBlock(dst, a, b, c)
}
