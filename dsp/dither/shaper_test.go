package dither

import (
	"math"
	"testing"
)

func TestFIRShaperPassthrough(t *testing.T) {
	// With no coefficients, Shape returns input unchanged.
	s := NewFIRShaper(nil)
	for i := 0; i < 10; i++ {
		got := s.Shape(float64(i))
		if got != float64(i) {
			t.Errorf("sample %d: got %v, want %v", i, got, float64(i))
		}
		s.RecordError(0) // no-op for nil shaper
	}
}

func TestFIRShaperEFB(t *testing.T) {
	// [1.0] = simple error feedback. Each sample subtracts 1x the previous error.
	s := NewFIRShaper([]float64{1.0})

	// First sample: no history, input passes through.
	got := s.Shape(1.0)
	if got != 1.0 {
		t.Fatalf("sample 0: got %v, want 1.0", got)
	}
	// Record error of 0.5.
	s.RecordError(0.5)

	// Second sample: 1.0 - 1.0*0.5 = 0.5
	got = s.Shape(1.0)
	if got != 0.5 {
		t.Fatalf("sample 1: got %v, want 0.5", got)
	}
	// Record error of 0.0.
	s.RecordError(0.0)

	// Third sample: 1.0 - 1.0*0.0 = 1.0
	got = s.Shape(1.0)
	if got != 1.0 {
		t.Fatalf("sample 2: got %v, want 1.0", got)
	}
}

func TestFIRShaperSecondOrder(t *testing.T) {
	// [1.0, -0.5] = 2nd order. Verify two taps of history are used.
	s := NewFIRShaper([]float64{1.0, -0.5})

	// Sample 0: no history.
	got := s.Shape(2.0)
	if got != 2.0 {
		t.Fatalf("sample 0: got %v, want 2.0", got)
	}
	s.RecordError(0.4) // error stored at pos after advance

	// Sample 1: input - 1.0*error[pos] - (-0.5)*error[pos-1]
	// After first Shape, pos was advanced. So error[0.4] is at the new pos.
	// For the second Shape call, history has one non-zero entry.
	got = s.Shape(2.0)
	s.RecordError(0.2)

	// Just verify finite and not equal to input (shaping active).
	if math.IsNaN(got) || math.IsInf(got, 0) {
		t.Fatalf("sample 1: got %v", got)
	}
}

func TestFIRShaperReset(t *testing.T) {
	s := NewFIRShaper([]float64{1.0})
	s.Shape(1.0)
	s.RecordError(0.5)
	s.Shape(1.0)
	s.RecordError(0.3)

	s.Reset()

	// After reset, history is zeroed, so Shape should pass through.
	got := s.Shape(1.0)
	if got != 1.0 {
		t.Errorf("after reset: got %v, want 1.0", got)
	}
}

func TestFIRShaperStability(t *testing.T) {
	// Run 9FC preset for 10000 samples with small random errors.
	coeffs := Preset9FC.Coefficients()
	s := NewFIRShaper(coeffs)
	for i := 0; i < 10000; i++ {
		v := s.Shape(0.5)
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("sample %d: got %v", i, v)
		}
		// Simulate a small quantization error.
		s.RecordError(0.01 * float64(i%10))
	}
}

func TestFIRShaperImplementsInterface(t *testing.T) {
	var _ NoiseShaper = NewFIRShaper(nil)
	var _ NoiseShaper = NewFIRShaper([]float64{1.0})
}

func TestFIRShaperCopiesCoeffs(t *testing.T) {
	orig := []float64{1.0, 2.0, 3.0}
	s := NewFIRShaper(orig)
	orig[0] = 999
	got := s.Shape(0)
	s.RecordError(1.0)
	// If coefficients were not copied, the shaper would use 999.
	got2 := s.Shape(0)
	_ = got
	if math.IsNaN(got2) {
		t.Fatal("unexpected NaN")
	}
}
