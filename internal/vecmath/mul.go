//go:build amd64

package vecmath

import (
	"github.com/cwbudde/algo-dsp/internal/cpu"
	"github.com/cwbudde/algo-dsp/internal/vecmath/arch/amd64/avx2"
	"github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic"
)

// MulBlock performs element-wise multiplication: dst[i] = a[i] * b[i].
// Slices must have equal length. Panics if lengths differ.
// Automatically selects the best implementation based on CPU features.
func MulBlock(dst, a, b []float64) {
	if cpu.HasAVX2() {
		avx2.MulBlock(dst, a, b)
	} else {
		generic.MulBlock(dst, a, b)
	}
}

// MulBlockInPlace performs in-place element-wise multiplication: dst[i] *= src[i].
// Slices must have equal length. Panics if lengths differ.
// Automatically selects the best implementation based on CPU features.
func MulBlockInPlace(dst, src []float64) {
	if cpu.HasAVX2() {
		avx2.MulBlockInPlace(dst, src)
	} else {
		generic.MulBlockInPlace(dst, src)
	}
}
