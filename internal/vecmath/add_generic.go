//go:build purego || !(amd64 || arm64)

package vecmath

import "github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic"

// AddBlock performs element-wise addition: dst[i] = a[i] + b[i].
// Slices must have equal length. Panics if lengths differ.
// This is the pure Go fallback implementation.
func AddBlock(dst, a, b []float64) {
	generic.AddBlock(dst, a, b)
}

// AddBlockInPlace performs in-place element-wise addition: dst[i] += src[i].
// Slices must have equal length. Panics if lengths differ.
// This is the pure Go fallback implementation.
func AddBlockInPlace(dst, src []float64) {
	generic.AddBlockInPlace(dst, src)
}
