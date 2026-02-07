//go:build purego || !(amd64 || arm64)

package vecmath

import "github.com/cwbudde/algo-dsp/internal/vecmath/arch/generic"

// MaxAbs returns the maximum absolute value in x.
// Returns 0 for an empty slice.
// This is the pure Go fallback implementation.
func MaxAbs(x []float64) float64 {
	return generic.MaxAbs(x)
}
