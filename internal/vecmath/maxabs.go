//go:build amd64

package vecmath

import (
	"github.com/cwbudde/algo-dsp/internal/cpu"
	"github.com/cwbudde/algo-dsp/internal/vecmath/arch/amd64/avx2"
	"github.com/cwbudde/algo-dsp/internal/vecmath/arch/amd64/sse2"
	"github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic"
)

// MaxAbs returns the maximum absolute value in x.
// Returns 0 for an empty slice.
// Automatically selects the best implementation based on CPU features.
func MaxAbs(x []float64) float64 {
	if cpu.HasAVX2() {
		return avx2.MaxAbs(x)
	}
	if cpu.HasSSE2() {
		return sse2.MaxAbs(x)
	}
	return generic.MaxAbs(x)
}
