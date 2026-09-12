package ir

import (
	"math"
	"testing"
)

// TestFindImpulseStartNaNIsDeterministic guards against re-adopting
// vecmath.MaxAbs for the peak scan in findImpulseStart.
//
// MaxAbs is roughly 5.6x faster here, but on AVX2 a NaN can discard the true
// maximum and return a smaller finite value: for the first layout below it
// returns 0.5 instead of 999, while the pure-Go kernel returns 999. A peak
// under-reported by that much leaves the threshold orders of magnitude too low,
// so the quiet run before the impulse clears it and the onset is reported at
// index 0 instead of 64 -- silently corrupting every metric derived from it,
// and on some CPUs only. An IsNaN check on the result does not catch this,
// because the result is finite.
func TestFindImpulseStartNaNIsDeterministic(t *testing.T) {
	const (
		quiet   = 64  // samples of low-level noise before the impulse
		level   = 0.5 // their amplitude
		impulse = 999
	)

	nan := math.NaN()
	a := &Analyzer{}

	build := func(tail int) []float64 {
		ir := make([]float64, 0, quiet+2+tail)
		for range quiet {
			ir = append(ir, level)
		}

		ir = append(ir, impulse, nan)
		for range tail {
			ir = append(ir, level)
		}

		return ir
	}

	tests := []struct {
		name string
		ir   []float64
	}{
		// MaxAbs returns a finite 0.5 here: the NaN discards the 999.
		{"NaN discards the peak", build(1)},
		{"NaN discards the peak, longer tail", build(5)},
		// MaxAbs returns NaN here, which makes every comparison false.
		{"NaN propagates", build(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := a.findImpulseStart(tt.ir, 0.1); got != quiet {
				t.Errorf("findImpulseStart = %d, want %d (the impulse index)", got, quiet)
			}
		})
	}
}
