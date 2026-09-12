package time

import (
	"math"
	"testing"
)

// peakReference is the scalar loop Peak used before it was routed through
// vecmath.MaxAbs. Peak must stay exactly equal to it for every NaN-free input.
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

// TestPeakMatchesScalarReference sweeps the lengths around the vector kernel's
// block and remainder boundaries. The comparison is == because max is exact:
// unlike a sum, a tree reduction over max returns the identical double.
func TestPeakMatchesScalarReference(t *testing.T) {
	lengths := []int{0, 1, 2, 3, 4, 5, 7, 8, 9, 15, 16, 17, 31, 32, 33, 63, 1025}

	for _, n := range lengths {
		signal := make([]float64, n)
		for i := range signal {
			signal[i] = math.Sin(float64(i)*0.37) * float64(1+i%13)
		}

		if got, want := Peak(signal), peakReference(signal); got != want {
			t.Errorf("n=%d: Peak = %v, scalar reference = %v", n, got, want)
		}
	}
}

// TestPeakSpecialValues pins the cases that must behave identically on every
// dispatch path. NaN is deliberately absent -- see TestPeakNaNDoesNotPanic.
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

// TestPeakNaNDoesNotPanic covers the one input class where Peak's result is
// documented as unspecified. The vectorized kernel compares with MAXPD, which
// propagates or discards NaN depending on the lane it lands in, so the answer
// differs between CPUs and between the SIMD and purego builds. Assert only that
// the call is safe and total -- pinning a value here would fail on some other
// machine, which is exactly the property being documented.
func TestPeakNaNDoesNotPanic(t *testing.T) {
	nan := math.NaN()

	inputs := [][]float64{
		{nan},
		{nan, 1, 2, 3},
		{1, 2, 3, nan},
		{1, 2, nan, 4, 5, 6, 7, 8},
		append(make([]float64, 16), nan),
	}

	for i, signal := range inputs {
		got := Peak(signal)
		if !math.IsNaN(got) && got < 0 {
			t.Errorf("input %d: Peak = %v, want NaN or a non-negative value", i, got)
		}
	}
}
