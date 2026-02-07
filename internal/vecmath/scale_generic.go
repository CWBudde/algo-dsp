//go:build purego || !(amd64 || arm64)

package vecmath

import "github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic"

// ScaleBlock multiplies each element by a scalar: dst[i] = src[i] * scale.
// Slices must have equal length. Panics if lengths differ.
// This is the pure Go fallback implementation.
func ScaleBlock(dst, src []float64, scale float64) {
	generic.ScaleBlock(dst, src, scale)
}

// ScaleBlockInPlace multiplies each element by a scalar in-place: dst[i] *= scale.
// This is the pure Go fallback implementation.
func ScaleBlockInPlace(dst []float64, scale float64) {
	generic.ScaleBlockInPlace(dst, scale)
}
