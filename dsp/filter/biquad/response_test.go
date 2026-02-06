package biquad

import (
	"math"
	"math/cmplx"
	"testing"
)

func TestMagnitudeSquared_MatchesResponse(t *testing.T) {
	// Verify closed-form MagnitudeSquared matches |Response|^2 across frequencies.
	c := Coefficients{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04}
	sr := 48000.0

	for _, freq := range []float64{100, 500, 1000, 5000, 10000, 20000} {
		h := c.Response(freq, sr)
		fromResponse := real(h)*real(h) + imag(h)*imag(h)
		fromClosed := c.MagnitudeSquared(freq, sr)
		if !almostEqual(fromClosed, fromResponse, 1e-10) {
			t.Errorf("freq=%v: MagnitudeSquared=%.15f, |Response|²=%.15f", freq, fromClosed, fromResponse)
		}
	}
}

func TestMagnitudeDB_MatchesMagnitudeSquared(t *testing.T) {
	c := Coefficients{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04}
	sr := 48000.0

	for _, freq := range []float64{100, 1000, 10000} {
		db := c.MagnitudeDB(freq, sr)
		fromSq := 10 * math.Log10(c.MagnitudeSquared(freq, sr))
		if !almostEqual(db, fromSq, 1e-12) {
			t.Errorf("freq=%v: MagnitudeDB=%.15f, 10*log10(MagSq)=%.15f", freq, db, fromSq)
		}
	}
}

func TestPhase_MatchesResponse(t *testing.T) {
	c := Coefficients{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04}
	sr := 48000.0

	for _, freq := range []float64{100, 500, 1000, 5000, 10000} {
		h := c.Response(freq, sr)
		fromResponse := cmplx.Phase(h)
		fromClosed := c.Phase(freq, sr)
		if !almostEqual(fromClosed, fromResponse, 1e-10) {
			t.Errorf("freq=%v: Phase=%.15f, arg(Response)=%.15f", freq, fromClosed, fromResponse)
		}
	}
}

func TestResponse_Passthrough(t *testing.T) {
	// Passthrough (B0=1) should have magnitude 1 and phase 0 at all frequencies.
	c := passthrough()
	sr := 48000.0
	for _, freq := range []float64{0, 100, 1000, 10000, 24000} {
		h := c.Response(freq, sr)
		mag := cmplx.Abs(h)
		if !almostEqual(mag, 1, 1e-12) {
			t.Errorf("freq=%v: |H|=%v, want 1", freq, mag)
		}
	}
}

func TestResponse_Allpass(t *testing.T) {
	// First-order allpass: B0=A2, B1=A1, B2=1, A1=A1, A2=A2
	// |H(f)| = 1 for all f.
	a1, a2 := -0.5, 0.3
	c := Coefficients{B0: a2, B1: a1, B2: 1, A1: a1, A2: a2}
	sr := 48000.0
	for _, freq := range []float64{100, 500, 1000, 5000, 10000, 20000} {
		h := c.Response(freq, sr)
		mag := cmplx.Abs(h)
		if !almostEqual(mag, 1, 1e-10) {
			t.Errorf("freq=%v: |H|=%.15f, want 1", freq, mag)
		}
	}
}

func TestChain_Response_ProductOfSections(t *testing.T) {
	coeffs := twoSectionCoeffs()
	chain := NewChain(coeffs)
	sr := 48000.0

	for _, freq := range []float64{100, 1000, 10000} {
		h1 := coeffs[0].Response(freq, sr)
		h2 := coeffs[1].Response(freq, sr)
		ref := h1 * h2
		got := chain.Response(freq, sr)
		if !almostEqual(real(got), real(ref), 1e-10) || !almostEqual(imag(got), imag(ref), 1e-10) {
			t.Errorf("freq=%v: chain=%v, product=%v", freq, got, ref)
		}
	}
}

func TestChain_Response_WithGain(t *testing.T) {
	coeffs := twoSectionCoeffs()
	gain := 0.5
	chain := NewChain(coeffs, WithGain(gain))
	chainNoGain := NewChain(coeffs)
	sr := 48000.0

	for _, freq := range []float64{100, 1000, 10000} {
		ref := chainNoGain.Response(freq, sr) * complex(gain, 0)
		got := chain.Response(freq, sr)
		if !almostEqual(real(got), real(ref), 1e-10) || !almostEqual(imag(got), imag(ref), 1e-10) {
			t.Errorf("freq=%v: chain=%v, ref=%v", freq, got, ref)
		}
	}
}

func TestChain_MagnitudeDB_MatchesResponse(t *testing.T) {
	chain := NewChain(twoSectionCoeffs())
	sr := 48000.0

	for _, freq := range []float64{100, 1000, 10000} {
		h := chain.Response(freq, sr)
		fromResponse := 20 * math.Log10(cmplx.Abs(h))
		fromMethod := chain.MagnitudeDB(freq, sr)
		if !almostEqual(fromMethod, fromResponse, 1e-10) {
			t.Errorf("freq=%v: MagnitudeDB=%.15f, 20*log10(|H|)=%.15f", freq, fromMethod, fromResponse)
		}
	}
}

func TestSection_ImpulseResponse(t *testing.T) {
	c := Coefficients{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04}
	s := NewSection(c)

	// Process some samples to build state.
	s.ProcessSample(0.5)
	s.ProcessSample(0.3)
	savedState := s.State()

	ir := s.ImpulseResponse(8)

	// State must be unchanged after ImpulseResponse.
	if s.State() != savedState {
		t.Fatal("ImpulseResponse modified section state")
	}

	// Verify IR by computing manually.
	ref := NewSection(c)
	for i, want := range ir {
		var x float64
		if i == 0 {
			x = 1
		}
		got := ref.ProcessSample(x)
		if !almostEqual(got, want, eps) {
			t.Errorf("ir[%d]: got %.15f, want %.15f", i, got, want)
		}
	}
}

func TestSection_ImpulseResponse_Zero(t *testing.T) {
	s := NewSection(passthrough())
	ir := s.ImpulseResponse(0)
	if ir != nil {
		t.Errorf("ImpulseResponse(0) should return nil, got %v", ir)
	}
	ir = s.ImpulseResponse(-1)
	if ir != nil {
		t.Errorf("ImpulseResponse(-1) should return nil, got %v", ir)
	}
}

func TestChain_ImpulseResponse(t *testing.T) {
	coeffs := twoSectionCoeffs()
	chain := NewChain(coeffs)

	chain.ProcessSample(0.5)
	chain.ProcessSample(0.3)
	savedState := chain.State()

	ir := chain.ImpulseResponse(16)

	// State must be unchanged.
	restoredState := chain.State()
	for i, s := range savedState {
		if s != restoredState[i] {
			t.Fatalf("ImpulseResponse modified chain state at section %d", i)
		}
	}

	// Verify by computing manually.
	ref := NewChain(coeffs)
	for i, want := range ir {
		var x float64
		if i == 0 {
			x = 1
		}
		got := ref.ProcessSample(x)
		if !almostEqual(got, want, eps) {
			t.Errorf("ir[%d]: got %.15f, want %.15f", i, got, want)
		}
	}
}

func TestSection_ImpulseResponse_Passthrough(t *testing.T) {
	s := NewSection(passthrough())
	ir := s.ImpulseResponse(5)
	want := []float64{1, 0, 0, 0, 0}
	for i := range ir {
		if !almostEqual(ir[i], want[i], eps) {
			t.Errorf("ir[%d]: got %v, want %v", i, ir[i], want[i])
		}
	}
}
