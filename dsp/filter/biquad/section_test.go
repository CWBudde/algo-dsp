package biquad

import (
	"math"
	"testing"
)

// tolerance for floating-point comparisons.
const eps = 1e-12

func almostEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

// passthrough returns coefficients for a unity gain passthrough (B0=1, all else 0).
func passthrough() Coefficients {
	return Coefficients{B0: 1}
}

// simpleLowpass returns a simple first-order-ish lowpass biquad.
// H(z) = 0.5*(1 + z^-1) / (1 + 0*z^-1 + 0*z^-2) — two-tap average.
func simpleLowpass() Coefficients {
	return Coefficients{B0: 0.5, B1: 0.5}
}

func TestNewSection(t *testing.T) {
	c := Coefficients{B0: 1, B1: 2, B2: 3, A1: 4, A2: 5}
	s := NewSection(c)
	if s.Coefficients != c {
		t.Fatalf("coefficients mismatch: got %v, want %v", s.Coefficients, c)
	}
	st := s.State()
	if st != [2]float64{0, 0} {
		t.Fatalf("initial state not zero: %v", st)
	}
}

func TestProcessSample_Passthrough(t *testing.T) {
	s := NewSection(passthrough())
	input := []float64{1, 0, -1, 0.5, 0.25}
	for i, x := range input {
		y := s.ProcessSample(x)
		if !almostEqual(y, x, eps) {
			t.Errorf("sample %d: got %v, want %v", i, y, x)
		}
	}
}

func TestProcessSample_DFIIT(t *testing.T) {
	// Hand-traced DF-II-T with specific coefficients:
	// B0=0.25, B1=0.5, B2=0.25, A1=-0.2, A2=0.04
	//
	// Step through with x = [1, 0, 0, 0]:
	//
	// n=0: y=0.25*1+0 = 0.25
	//      d0=0.5*1-(-0.2)*0.25+0 = 0.5+0.05 = 0.55
	//      d1=0.25*1-0.04*0.25 = 0.25-0.01 = 0.24
	//
	// n=1: y=0.25*0+0.55 = 0.55
	//      d0=0.5*0-(-0.2)*0.55+0.24 = 0.11+0.24 = 0.35
	//      d1=0.25*0-0.04*0.55 = -0.022
	//
	// n=2: y=0.25*0+0.35 = 0.35
	//      d0=0.5*0-(-0.2)*0.35+(-0.022) = 0.07-0.022 = 0.048
	//      d1=0.25*0-0.04*0.35 = -0.014
	//
	// n=3: y=0.25*0+0.048 = 0.048
	//      d0=0.5*0-(-0.2)*0.048+(-0.014) = 0.0096-0.014 = -0.0044
	//      d1=0.25*0-0.04*0.048 = -0.00192

	c := Coefficients{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04}
	s := NewSection(c)

	want := []float64{0.25, 0.55, 0.35, 0.048}
	for i, w := range want {
		var x float64
		if i == 0 {
			x = 1
		}
		y := s.ProcessSample(x)
		if !almostEqual(y, w, eps) {
			t.Errorf("sample %d: got %.15f, want %.15f", i, y, w)
		}
	}
}

func TestProcessBlock_MatchesSample(t *testing.T) {
	c := Coefficients{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04}

	// ProcessSample reference
	s1 := NewSection(c)
	input := []float64{1, 0.5, -0.3, 0.7, 0, -1, 0.2, 0.8}
	ref := make([]float64, len(input))
	for i, x := range input {
		ref[i] = s1.ProcessSample(x)
	}

	// ProcessBlock
	s2 := NewSection(c)
	block := make([]float64, len(input))
	copy(block, input)
	s2.ProcessBlock(block)

	for i := range block {
		if !almostEqual(block[i], ref[i], eps) {
			t.Errorf("sample %d: ProcessBlock=%.15f, ProcessSample=%.15f", i, block[i], ref[i])
		}
	}
}

func TestProcessBlockTo_MatchesSample(t *testing.T) {
	c := Coefficients{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04}

	s1 := NewSection(c)
	input := []float64{1, 0.5, -0.3, 0.7, 0, -1, 0.2, 0.8}
	ref := make([]float64, len(input))
	for i, x := range input {
		ref[i] = s1.ProcessSample(x)
	}

	s2 := NewSection(c)
	dst := make([]float64, len(input))
	s2.ProcessBlockTo(dst, input)

	for i := range dst {
		if !almostEqual(dst[i], ref[i], eps) {
			t.Errorf("sample %d: ProcessBlockTo=%.15f, ProcessSample=%.15f", i, dst[i], ref[i])
		}
	}

	// Verify src was not modified.
	for i := range input {
		orig := []float64{1, 0.5, -0.3, 0.7, 0, -1, 0.2, 0.8}
		if input[i] != orig[i] {
			t.Errorf("src modified at index %d", i)
		}
	}
}

func TestProcessBlockUnrolled2_MatchesSample(t *testing.T) {
	c := Coefficients{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04}

	s1 := NewSection(c)
	input := []float64{1, 0.5, -0.3, 0.7, 0, -1, 0.2, 0.8, -0.1}
	ref := make([]float64, len(input))
	for i, x := range input {
		ref[i] = s1.ProcessSample(x)
	}

	s2 := NewSection(c)
	block := make([]float64, len(input))
	copy(block, input)
	s2.processBlockUnrolled2(block)

	for i := range block {
		if !almostEqual(block[i], ref[i], eps) {
			t.Errorf("sample %d: processBlockUnrolled2=%.15f, ProcessSample=%.15f", i, block[i], ref[i])
		}
	}
}

func TestProcessSample_ZeroCoefficients(t *testing.T) {
	// All-zero coefficients should produce silence.
	s := NewSection(Coefficients{})
	for i := range 10 {
		y := s.ProcessSample(1.0)
		if y != 0 {
			t.Errorf("sample %d: got %v, want 0", i, y)
		}
	}
}

func TestProcessSample_PureDelay(t *testing.T) {
	// B0=0, B1=1, all A=0: output = d0 = previous B1*x = x[n-1]
	s := NewSection(Coefficients{B1: 1})
	input := []float64{1, 2, 3, 4, 5}
	want := []float64{0, 1, 2, 3, 4}
	for i, x := range input {
		y := s.ProcessSample(x)
		if !almostEqual(y, want[i], eps) {
			t.Errorf("sample %d: got %v, want %v", i, y, want[i])
		}
	}
}

func TestReset(t *testing.T) {
	c := Coefficients{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04}
	s := NewSection(c)

	// Process some samples to build up state.
	s.ProcessSample(1)
	s.ProcessSample(0.5)

	st := s.State()
	if st == [2]float64{0, 0} {
		t.Fatal("state should be non-zero after processing")
	}

	s.Reset()
	st = s.State()
	if st != [2]float64{0, 0} {
		t.Fatalf("state not zero after reset: %v", st)
	}
}

func TestState_SaveRestore(t *testing.T) {
	c := Coefficients{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04}
	s := NewSection(c)

	// Process two samples.
	s.ProcessSample(1)
	s.ProcessSample(0.5)
	saved := s.State()

	// Process more samples.
	y3 := s.ProcessSample(-0.3)
	y4 := s.ProcessSample(0.7)

	// Restore state and reprocess — should get same results.
	s.SetState(saved)
	y3b := s.ProcessSample(-0.3)
	y4b := s.ProcessSample(0.7)

	if !almostEqual(y3, y3b, eps) {
		t.Errorf("sample 3: got %v after restore, want %v", y3b, y3)
	}
	if !almostEqual(y4, y4b, eps) {
		t.Errorf("sample 4: got %v after restore, want %v", y4b, y4)
	}
}

func TestProcessSample_StabilityLongRun(t *testing.T) {
	// Stable lowpass-like filter: process 10000 zero-input samples after
	// an impulse, verify output decays and doesn't diverge.
	c := Coefficients{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04}
	s := NewSection(c)
	s.ProcessSample(1)

	var maxAbs float64
	for range 10000 {
		y := s.ProcessSample(0)
		if a := math.Abs(y); a > maxAbs {
			maxAbs = a
		}
	}
	// After 10000 zero-input samples, state should have decayed to near zero.
	st := s.State()
	if math.Abs(st[0]) > 1e-100 || math.Abs(st[1]) > 1e-100 {
		t.Errorf("state did not decay: %v", st)
	}
}

func TestProcessSample_SimpleLowpass(t *testing.T) {
	// Two-tap average: y[n] = 0.5*x[n] + 0.5*x[n-1]
	s := NewSection(simpleLowpass())
	input := []float64{1, 1, 1, 1}
	want := []float64{0.5, 1, 1, 1}
	for i, x := range input {
		y := s.ProcessSample(x)
		if !almostEqual(y, want[i], eps) {
			t.Errorf("sample %d: got %v, want %v", i, y, want[i])
		}
	}
}
