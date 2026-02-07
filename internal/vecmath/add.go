//go:build amd64

package vecmath

import (
	"github.com/cwbudde/algo-dsp/internal/cpu"
	"github.com/cwbudde/algo-dsp/internal/vecmath/arch/amd64/avx2"
	"github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic"
)

// AddBlock performs element-wise addition: dst[i] = a[i] + b[i].
// Slices must have equal length. Panics if lengths differ.
// Automatically selects the best implementation based on CPU features.
func AddBlock(dst, a, b []float64) {
	if cpu.HasAVX2() {
		avx2.AddBlock(dst, a, b)
	} else {
		generic.AddBlock(dst, a, b)
	}
}

// AddBlockInPlace performs in-place element-wise addition: dst[i] += src[i].
// Slices must have equal length. Panics if lengths differ.
// Automatically selects the best implementation based on CPU features.
func AddBlockInPlace(dst, src []float64) {
	if cpu.HasAVX2() {
		avx2.AddBlockInPlace(dst, src)
	} else {
		generic.AddBlockInPlace(dst, src)
	}
}
