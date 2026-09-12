package time

import (
	"math"
	"testing"
)

// peakReference is an independent restatement of the peak scan, used to check
// Peak across the lengths where a vectorized implementation would change block
// and remainder handling.
func peakReference(signal []float64) float64 {
	if len(signal) == 0 {
		return 0
	}

	peak := math.Abs(signal[0])
	for _, x := range signal[1:] {
		a := math.Abs(x)
		if a > peak {
			peak = a
		}
	}

	return peak
}

// TestPeakMatchesScalarReference sweeps the lengths around the block and
// remainder boundaries a vectorized implementation would use. The comparison is
// == because max is exact: unlike a sum, a reduction over max returns the
// identical double however it is ordered.
func TestPeakMatchesScalarReference(t *testing.T) {
	lengths := []int{0, 1, 2, 3, 4, 5, 7, 8, 9, 15, 16, 17, 31, 32, 33, 63, 1025}

	for _, n := range lengths {
		signal := make([]float64, n)
		for i := range signal {
			signal[i] = math.Sin(float64(i)*0.37) * float64(1+i%13)
		}

		if got, want := Peak(signal), peakReference(signal); got != want {
			t.Errorf("n=%d: Peak = %v, reference = %v", n, got, want)
		}
	}
}

// TestPeakSpecialValues pins the special-value cases. NaN is covered separately
// by TestPeakNaNIsDeterministic.
func TestPeakSpecialValues(t *testing.T) {
	negZero := math.Copysign(0, -1)

	tests := []struct {
		name   string
		signal []float64
		want   float64
	}{
		{"empty", nil, 0},
		{"single negative", []float64{-3.5}, 3.5},
		{"negative zero", []float64{negZero, negZero, negZero, negZero}, 0},
		{"mixed zeros", []float64{0, negZero, 0, negZero, 0}, 0},
		{"negative infinity", []float64{-math.Inf(1), 1, 2, 3}, math.Inf(1)},
		{"positive infinity last", []float64{1, 2, 3, math.Inf(1)}, math.Inf(1)},
		{"all equal", []float64{-2, 2, -2, 2, -2}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Peak(tt.signal); got != tt.want {
				t.Errorf("Peak = %v, want %v", got, tt.want)
			}

			if got, want := math.Signbit(Peak(tt.signal)), false; got != want {
				t.Errorf("Peak returned a negative zero")
			}
		})
	}
}

// TestPeakNaNIsDeterministic guards against re-adopting vecmath.MaxAbs here.
//
// MaxAbs is roughly 3x faster on long signals, but on AVX2 a NaN can discard
// the true maximum and return a smaller finite value: MaxAbs([999, NaN, 0.5])
// returns 0.5 while the pure-Go kernel returns 999. That is not a NaN-policy
// difference, it is a silently wrong finite peak on some CPUs only, and no
// cheap check catches it -- an IsNaN test on the result does not fire, because
// the result is finite. If someone swaps the loop in Peak for MaxAbs, this test
// fails on an AVX2 machine.
func TestPeakNaNIsDeterministic(t *testing.T) {
	nan := math.NaN()

	tests := []struct {
		name   string
		signal []float64
		want   float64
	}{
		{"NaN hides a larger value", []float64{999, nan, 0.5}, 999},
		{"NaN hides a larger value, longer", []float64{999, nan, 0.5, 0.25}, 999},
		{"NaN last", []float64{1, 2, 3, nan}, 3},
		{"NaN in the middle", []float64{1, 2, nan, 4, 5, 6, 7, 8}, 8},
		{"NaN past a vector block", append(append(make([]float64, 64), 999), nan), 999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Peak(tt.signal); got != tt.want {
				t.Errorf("Peak = %v, want %v", got, tt.want)
			}
		})
	}

	// signal[0] == NaN is the one case that propagates, because the scan seeds
	// from it. Long-standing behavior, pinned so a future rewrite keeps it.
	if got := Peak([]float64{nan, 1, 2, 3}); !math.IsNaN(got) {
		t.Errorf("Peak with a leading NaN = %v, want NaN", got)
	}
}
