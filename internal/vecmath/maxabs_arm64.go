//go:build !purego && arm64

package vecmath

import (
	"github.com/cwbudde/algo-dsp/internal/cpu"
	"github.com/cwbudde/algo-dsp/internal/vecmath/arch/arm64/neon"
	"github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic"
)

// MaxAbs returns the maximum absolute value in x.
// Returns 0 for an empty slice.
// Automatically selects the best implementation based on CPU features.
func MaxAbs(x []float64) float64 {
	if cpu.HasNEON() {
		return neon.MaxAbs(x)
	}
	return generic.MaxAbs(x)
}
