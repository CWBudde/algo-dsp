package vecmath

import (
	"sync"

	"github.com/cwbudde/algo-dsp/internal/cpu"
	"github.com/cwbudde/algo-dsp/internal/vecmath/registry"
)

var (
	// Cached function pointer for magnitude operation (initialized once, used many times)
	magnitudeImpl func([]float64, []float64, []float64)
	magnitudeOnce sync.Once
)

// initMagnitudeOperation performs one-time initialization of magnitude operation function pointer.
// This function selects the best implementation based on detected CPU features and
// caches the function pointer for subsequent calls.
func initMagnitudeOperation() {
	features := cpu.DetectFeatures()
	entry := registry.Global.Lookup(features)

	if entry == nil {
		panic("vecmath: no magnitude implementation registered (missing generic fallback?)")
	}

	if entry.Magnitude == nil {
		panic("vecmath: selected implementation missing magnitude operation")
	}

	magnitudeImpl = entry.Magnitude
}

// Magnitude computes magnitude from separate real and imaginary parts: dst[i] = sqrt(re[i]^2 + im[i]^2).
//
// All slices must have equal length. Panics if lengths differ.
//
// The implementation is automatically selected based on CPU features:
//   - AVX2 on x86-64 CPUs with AVX2 support (Haswell 2013+) - processes 4 values at once
//   - SSE2 on x86-64 CPUs with SSE2 support - processes 2 values at once
//   - NEON on ARM64 CPUs - processes 2 values at once
//   - Generic pure Go fallback otherwise
//
// After the first call, subsequent calls have zero dispatch overhead
// (direct function pointer call, equivalent to hand-written dispatch).
func Magnitude(dst, re, im []float64) {
	magnitudeOnce.Do(initMagnitudeOperation)
	magnitudeImpl(dst, re, im)
}
