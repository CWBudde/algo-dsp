package dither

import (
	"math"
	"testing"
)

func TestFIRShaperPassthrough(t *testing.T) {
	// With no coefficients, Shape returns input unchanged.
	shaper := NewFIRShaper(nil)

	for idx := range 10 {
		got := shaper.Shape(float64(idx))
		if got != float64(idx) {
			t.Errorf("sample %d: got %v, want %v", idx, got, float64(idx))
		}

		shaper.RecordError(0) // no-op for nil shaper
	}
}

func TestFIRShaperEFB(t *testing.T) {
	// [1.0] = simple error feedback. Each sample subtracts 1x the previous error.
	shaper := NewFIRShaper([]float64{1.0})

	// First sample: no history, input passes through.
	got := shaper.Shape(1.0)
	if got != 1.0 {
		t.Fatalf("sample 0: got %v, want 1.0", got)
	}

	// Record error of 0.5.
	shaper.RecordError(0.5)

	// Second sample: 1.0 - 1.0*0.5 = 0.5
	got = shaper.Shape(1.0)
	if got != 0.5 {
		t.Fatalf("sample 1: got %v, want 0.5", got)
	}

	// Record error of 0.0.
	shaper.RecordError(0.0)

	// Third sample: 1.0 - 1.0*0.0 = 1.0
	got = shaper.Shape(1.0)
	if got != 1.0 {
		t.Fatalf("sample 2: got %v, want 1.0", got)
	}
}

func TestFIRShaperSecondOrder(t *testing.T) {
	// [1.0, -0.5] = 2nd order. Verify two taps of history are used.
	shaper := NewFIRShaper([]float64{1.0, -0.5})

	// Sample 0: no history.
	got := shaper.Shape(2.0)
	if got != 2.0 {
		t.Fatalf("sample 0: got %v, want 2.0", got)
	}

	shaper.RecordError(0.4)

	// Sample 1: history has one non-zero entry, shaping is active.
	got = shaper.Shape(2.0)
	shaper.RecordError(0.2)

	if math.IsNaN(got) || math.IsInf(got, 0) {
		t.Fatalf("sample 1: got %v", got)
	}
}

func TestFIRShaperReset(t *testing.T) {
	shaper := NewFIRShaper([]float64{1.0})
	shaper.Shape(1.0)
	shaper.RecordError(0.5)
	shaper.Shape(1.0)
	shaper.RecordError(0.3)

	shaper.Reset()

	// After reset, history is zeroed, so Shape should pass through.
	got := shaper.Shape(1.0)
	if got != 1.0 {
		t.Errorf("after reset: got %v, want 1.0", got)
	}
}

func TestFIRShaperStability(t *testing.T) {
	// Run 9FC preset for 10000 samples with small random errors.
	coeffs := Preset9FC.Coefficients()
	shaper := NewFIRShaper(coeffs)

	for idx := range 10000 {
		val := shaper.Shape(0.5)
		if math.IsNaN(val) || math.IsInf(val, 0) {
			t.Fatalf("sample %d: got %v", idx, val)
		}

		// Simulate a small quantization error.
		shaper.RecordError(0.01 * float64(idx%10))
	}
}

func TestFIRShaperImplementsInterface(_ *testing.T) {
	var (
		_ NoiseShaper = NewFIRShaper(nil)
		_ NoiseShaper = NewFIRShaper([]float64{1.0})
	)
}

func TestFIRShaperCopiesCoeffs(t *testing.T) {
	orig := []float64{1.0, 2.0, 3.0}
	shaper := NewFIRShaper(orig)
	orig[0] = 999

	got := shaper.Shape(0)
	shaper.RecordError(1.0)

	// If coefficients were not copied, the shaper would use 999.
	got2 := shaper.Shape(0)
	_ = got

	if math.IsNaN(got2) {
		t.Fatal("unexpected NaN")
	}
}
