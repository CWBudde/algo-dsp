//go:build !purego && arm64

package vecmath

import "github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic"

// MulBlock performs element-wise multiplication: dst[i] = a[i] * b[i].
// Slices must have equal length. Panics if lengths differ.
// This is the arm64 fallback implementation.
func MulBlock(dst, a, b []float64) {
	generic.MulBlock(dst, a, b)
}

// MulBlockInPlace performs in-place element-wise multiplication: dst[i] *= src[i].
// Slices must have equal length. Panics if lengths differ.
// This is the arm64 fallback implementation.
func MulBlockInPlace(dst, src []float64) {
	generic.MulBlockInPlace(dst, src)
}
