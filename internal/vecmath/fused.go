//go:build amd64

package vecmath

import (
	"github.com/cwbudde/algo-dsp/internal/cpu"
	"github.com/cwbudde/algo-dsp/internal/vecmath/arch/amd64/avx2"
	"github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic"
)

// AddMulBlock performs fused add-multiply: dst[i] = (a[i] + b[i]) * scale.
// Slices must have equal length. Panics if lengths differ.
// Automatically selects the best implementation based on CPU features.
func AddMulBlock(dst, a, b []float64, scale float64) {
	if cpu.HasAVX2() {
		avx2.AddMulBlock(dst, a, b, scale)
	} else {
		generic.AddMulBlock(dst, a, b, scale)
	}
}

// MulAddBlock performs fused multiply-add: dst[i] = a[i] * b[i] + c[i].
// Slices must have equal length. Panics if lengths differ.
// Automatically selects the best implementation based on CPU features.
func MulAddBlock(dst, a, b, c []float64) {
	if cpu.HasAVX2() {
		avx2.MulAddBlock(dst, a, b, c)
	} else {
		generic.MulAddBlock(dst, a, b, c)
	}
}
