package fir

import (
	"math"
	"math/cmplx"
	"testing"
)

const eps = 1e-12

func almostEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func TestNew(t *testing.T) {
	coeffs := []float64{0.25, 0.5, 0.25}
	f := New(coeffs)
	if f.Order() != 2 {
		t.Fatalf("Order: got %d, want 2", f.Order())
	}
	got := f.Coefficients()
	for i := range coeffs {
		if got[i] != coeffs[i] {
			t.Errorf("coeffs[%d]: got %v, want %v", i, got[i], coeffs[i])
		}
	}
	// Verify it's a copy.
	coeffs[0] = 999
	if f.coeffs[0] == 999 {
		t.Error("New did not copy coefficients")
	}
}

func TestProcessSample_Impulse(t *testing.T) {
	// Impulse response of FIR should equal the coefficients.
	coeffs := []float64{0.25, 0.5, 0.25}
	f := New(coeffs)

	for i, want := range coeffs {
		var x float64
		if i == 0 {
			x = 1
		}
		y := f.ProcessSample(x)
		if !almostEqual(y, want, eps) {
			t.Errorf("sample %d: got %v, want %v", i, y, want)
		}
	}
	// After the impulse response, output should be zero.
	for i := range 5 {
		y := f.ProcessSample(0)
		if !almostEqual(y, 0, eps) {
			t.Errorf("post-IR sample %d: got %v, want 0", i, y)
		}
	}
}

func TestProcessSample_MovingAverage(t *testing.T) {
	// 3-tap moving average: h = [1/3, 1/3, 1/3]
	f := New([]float64{1.0 / 3, 1.0 / 3, 1.0 / 3})
	input := []float64{1, 1, 1, 1, 1}
	// y[0] = 1/3, y[1] = 2/3, y[2..4] = 1
	want := []float64{1.0 / 3, 2.0 / 3, 1, 1, 1}
	for i, x := range input {
		y := f.ProcessSample(x)
		if !almostEqual(y, want[i], eps) {
			t.Errorf("sample %d: got %v, want %v", i, y, want[i])
		}
	}
}

func TestProcessSample_Differentiator(t *testing.T) {
	// Simple differentiator: h = [1, -1]
	f := New([]float64{1, -1})
	input := []float64{0, 1, 3, 6, 10}
	// y[n] = x[n] - x[n-1], with x[-1] = 0
	want := []float64{0, 1, 2, 3, 4}
	for i, x := range input {
		y := f.ProcessSample(x)
		if !almostEqual(y, want[i], eps) {
			t.Errorf("sample %d: got %v, want %v", i, y, want[i])
		}
	}
}

func TestProcessBlock_MatchesSample(t *testing.T) {
	coeffs := []float64{0.25, 0.5, 0.25}
	input := []float64{1, 0.5, -0.3, 0.7, 0, -1, 0.2, 0.8}

	f1 := New(coeffs)
	ref := make([]float64, len(input))
	for i, x := range input {
		ref[i] = f1.ProcessSample(x)
	}

	f2 := New(coeffs)
	block := make([]float64, len(input))
	copy(block, input)
	f2.ProcessBlock(block)

	for i := range block {
		if !almostEqual(block[i], ref[i], eps) {
			t.Errorf("sample %d: block=%.15f, ref=%.15f", i, block[i], ref[i])
		}
	}
}

func TestProcessBlockTo_MatchesSample(t *testing.T) {
	coeffs := []float64{0.25, 0.5, 0.25}
	input := []float64{1, 0.5, -0.3, 0.7, 0, -1, 0.2, 0.8}

	f1 := New(coeffs)
	ref := make([]float64, len(input))
	for i, x := range input {
		ref[i] = f1.ProcessSample(x)
	}

	f2 := New(coeffs)
	dst := make([]float64, len(input))
	f2.ProcessBlockTo(dst, input)

	for i := range dst {
		if !almostEqual(dst[i], ref[i], eps) {
			t.Errorf("sample %d: dst=%.15f, ref=%.15f", i, dst[i], ref[i])
		}
	}
}

func TestReset(t *testing.T) {
	f := New([]float64{0.25, 0.5, 0.25})
	f.ProcessSample(1)
	f.ProcessSample(0.5)
	f.Reset()

	// After reset, impulse response should match coefficients again.
	for i, want := range f.coeffs {
		var x float64
		if i == 0 {
			x = 1
		}
		y := f.ProcessSample(x)
		if !almostEqual(y, want, eps) {
			t.Errorf("sample %d after reset: got %v, want %v", i, y, want)
		}
	}
}

func TestResponse_DCGain(t *testing.T) {
	// DC gain of FIR = sum of coefficients.
	coeffs := []float64{0.25, 0.5, 0.25}
	f := New(coeffs)
	h := f.Response(0, 48000)
	dcGain := cmplx.Abs(h)
	sum := 0.0
	for _, c := range coeffs {
		sum += c
	}
	if !almostEqual(dcGain, sum, 1e-12) {
		t.Errorf("DC gain: got %v, want %v", dcGain, sum)
	}
}

func TestResponse_Differentiator_DC(t *testing.T) {
	// Differentiator [1, -1] should have DC gain = 0.
	f := New([]float64{1, -1})
	h := f.Response(0, 48000)
	if !almostEqual(cmplx.Abs(h), 0, 1e-12) {
		t.Errorf("differentiator DC gain: got %v, want 0", cmplx.Abs(h))
	}
}

func TestMagnitudeDB_MatchesResponse(t *testing.T) {
	f := New([]float64{0.25, 0.5, 0.25})
	sr := 48000.0
	for _, freq := range []float64{100, 1000, 10000} {
		h := f.Response(freq, sr)
		fromResponse := 20 * math.Log10(cmplx.Abs(h))
		fromMethod := f.MagnitudeDB(freq, sr)
		if !almostEqual(fromMethod, fromResponse, 1e-10) {
			t.Errorf("freq=%v: MagnitudeDB=%.15f, ref=%.15f", freq, fromMethod, fromResponse)
		}
	}
}

func TestCoefficients_IsCopy(t *testing.T) {
	f := New([]float64{0.25, 0.5, 0.25})
	c := f.Coefficients()
	c[0] = 999
	if f.coeffs[0] == 999 {
		t.Error("Coefficients did not return a copy")
	}
}

func TestSingleTap(t *testing.T) {
	// Single-tap FIR (gain only).
	f := New([]float64{0.5})
	if f.Order() != 0 {
		t.Fatalf("Order: got %d, want 0", f.Order())
	}
	input := []float64{1, 2, 3}
	for i, x := range input {
		y := f.ProcessSample(x)
		if !almostEqual(y, x*0.5, eps) {
			t.Errorf("sample %d: got %v, want %v", i, y, x*0.5)
		}
	}
}
