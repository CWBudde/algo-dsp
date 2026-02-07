//go:build amd64

package vecmath

import (
	"github.com/cwbudde/algo-dsp/internal/cpu"
	"github.com/cwbudde/algo-dsp/internal/vecmath/arch/amd64/avx2"
	"github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic"
)

// ScaleBlock multiplies each element by a scalar: dst[i] = src[i] * scale.
// Slices must have equal length. Panics if lengths differ.
// Automatically selects the best implementation based on CPU features.
func ScaleBlock(dst, src []float64, scale float64) {
	if cpu.HasAVX2() {
		avx2.ScaleBlock(dst, src, scale)
	} else {
		generic.ScaleBlock(dst, src, scale)
	}
}

// ScaleBlockInPlace multiplies each element by a scalar in-place: dst[i] *= scale.
// Automatically selects the best implementation based on CPU features.
func ScaleBlockInPlace(dst []float64, scale float64) {
	if cpu.HasAVX2() {
		avx2.ScaleBlockInPlace(dst, scale)
	} else {
		generic.ScaleBlockInPlace(dst, scale)
	}
}
