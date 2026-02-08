package pass

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

func cascadeMagDB(sections []biquad.Coefficients, freq, sr float64) float64 {
	h := complex(1, 0)
	for _, c := range sections {
		h *= c.Response(freq, sr)
	}
	return 20 * math.Log10(cmplx.Abs(h))
}

// --- Chebyshev Type I Lowpass ---

func TestChebyshev1LP_Basic(t *testing.T) {
	sr := 48000.0
	sections := Chebyshev1LP(1000, 4, 1.0, sr)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections for order 4, got %d", len(sections))
	}
	for i, s := range sections {
		assertFiniteCoefficients(t, s)
		assertStableSection(t, s)
		if s.B0 <= 0 {
			t.Errorf("section %d: B0 should be positive, got %v", i, s.B0)
		}
	}
}

func TestChebyshev1LP_PassbandGain(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for _, order := range []int{2, 4, 6, 8} {
		sections := Chebyshev1LP(fc, order, 1.0, sr)
		dcGain := cascadeMagDB(sections, 10, sr)
		if dcGain < -1 || dcGain > 1 {
			t.Errorf("order %d: DC gain = %.2f dB, expected near 0 dB", order, dcGain)
		}
	}
}

func TestChebyshev1LP_CutoffAttenuation(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	// At cutoff, Chebyshev Type I should have gain near 0 dB (within ripple)
	for _, order := range []int{2, 4, 6} {
		sections := Chebyshev1LP(fc, order, 1.0, sr)
		atCutoff := cascadeMagDB(sections, fc, sr)
		if atCutoff > 1 || atCutoff < -6 {
			t.Errorf("order %d: gain at cutoff = %.2f dB, expected within [-6, 1] dB", order, atCutoff)
		}
	}
}

func TestChebyshev1LP_StopbandAttenuation(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for _, order := range []int{2, 4, 6, 8} {
		sections := Chebyshev1LP(fc, order, 1.0, sr)
		// At 5x cutoff, should have significant attenuation
		at5x := cascadeMagDB(sections, 5*fc, sr)
		if at5x > -20 {
			t.Errorf("order %d: gain at 5x cutoff = %.2f dB, expected < -20 dB", order, at5x)
		}
	}
}

func TestChebyshev1LP_SteepnessVsButterworth(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	order := 4

	cheby := Chebyshev1LP(fc, order, 1.0, sr)
	butter := ButterworthLP(fc, order, sr)

	// At 2x cutoff, Chebyshev should be steeper
	chebyAt2x := cascadeMagDB(cheby, 2*fc, sr)
	butterAt2x := cascadeMagDB(butter, 2*fc, sr)

	if chebyAt2x > butterAt2x {
		t.Errorf("Chebyshev (%.2f dB) should have more attenuation at 2x cutoff than Butterworth (%.2f dB)",
			chebyAt2x, butterAt2x)
	}
}

func TestChebyshev1LP_RippleEffect(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	order := 4

	// Higher ripple = steeper transition
	lowRipple := Chebyshev1LP(fc, order, 0.5, sr)
	highRipple := Chebyshev1LP(fc, order, 2.0, sr)

	lowAt2x := cascadeMagDB(lowRipple, 2*fc, sr)
	highAt2x := cascadeMagDB(highRipple, 2*fc, sr)

	// Higher ripple should give less attenuation at transition (wider passband)
	if highAt2x < lowAt2x-1 {
		t.Logf("low ripple at 2x: %.2f dB, high ripple at 2x: %.2f dB", lowAt2x, highAt2x)
	}

	// Both should still be lowpass (attenuating at 2x cutoff)
	if lowAt2x > -5 {
		t.Errorf("low ripple: insufficient stopband attenuation at 2x: %.2f dB", lowAt2x)
	}
	if highAt2x > -5 {
		t.Errorf("high ripple: insufficient stopband attenuation at 2x: %.2f dB", highAt2x)
	}
}

func TestChebyshev1LP_OddOrder(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for _, order := range []int{1, 3, 5, 7} {
		sections := Chebyshev1LP(fc, order, 1.0, sr)
		expected := (order + 1) / 2
		if len(sections) != expected {
			t.Errorf("order %d: expected %d sections, got %d", order, expected, len(sections))
		}
		for i, s := range sections {
			assertFiniteCoefficients(t, s)
			assertStableSection(t, s)
			_ = i
		}
		// Should still be a lowpass
		dcGain := cascadeMagDB(sections, 10, sr)
		highGain := cascadeMagDB(sections, 10*fc, sr)
		if dcGain < -3 {
			t.Errorf("order %d: DC gain too low: %.2f dB", order, dcGain)
		}
		if highGain > dcGain-10 {
			t.Errorf("order %d: high freq (%.2f dB) should be well below DC (%.2f dB)", order, highGain, dcGain)
		}
	}
}

func TestChebyshev1LP_Monotonic_Stopband(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	order := 4

	sections := Chebyshev1LP(fc, order, 1.0, sr)

	// Chebyshev Type I should be monotonically decreasing in the stopband
	prev := cascadeMagDB(sections, 2*fc, sr)
	for _, f := range []float64{3 * fc, 5 * fc, 8 * fc, 10 * fc} {
		cur := cascadeMagDB(sections, f, sr)
		if cur > prev+0.5 { // small tolerance for numerical
			t.Errorf("stopband not monotonic: %.0f Hz = %.2f dB > %.2f dB", f, cur, prev)
		}
		prev = cur
	}
}

func TestChebyshev1LP_Stability_AllOrders(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for order := 1; order <= 12; order++ {
		sections := Chebyshev1LP(fc, order, 1.0, sr)
		for i, s := range sections {
			assertFiniteCoefficients(t, s)
			r1, r2 := sectionRoots(s)
			if cmplx.Abs(r1) >= 1 || cmplx.Abs(r2) >= 1 {
				t.Errorf("order %d section %d: unstable poles |r1|=%.6f |r2|=%.6f",
					order, i, cmplx.Abs(r1), cmplx.Abs(r2))
			}
		}
	}
}

func TestChebyshev1LP_SampleRates(t *testing.T) {
	for _, sr := range []float64{8000, 22050, 44100, 48000, 96000, 192000} {
		fc := sr * 0.1 // 10% of sample rate
		sections := Chebyshev1LP(fc, 4, 1.0, sr)
		if len(sections) != 2 {
			t.Errorf("sr=%.0f: expected 2 sections, got %d", sr, len(sections))
		}
		dcGain := cascadeMagDB(sections, fc*0.1, sr)
		if dcGain < -2 || dcGain > 2 {
			t.Errorf("sr=%.0f: DC gain = %.2f dB, expected near 0 dB", sr, dcGain)
		}
	}
}

func TestChebyshev1LP_EdgeCases(t *testing.T) {
	// Order 0
	if sections := Chebyshev1LP(1000, 0, 1.0, 48000); sections != nil {
		t.Error("order 0 should return nil")
	}
	// Negative order
	if sections := Chebyshev1LP(1000, -1, 1.0, 48000); sections != nil {
		t.Error("negative order should return nil")
	}
	// Invalid freq
	if sections := Chebyshev1LP(0, 4, 1.0, 48000); sections != nil {
		t.Error("zero freq should return nil")
	}
	// Freq at Nyquist
	if sections := Chebyshev1LP(24000, 4, 1.0, 48000); sections != nil {
		t.Error("freq at Nyquist should return nil")
	}
	// Invalid sample rate
	if sections := Chebyshev1LP(1000, 4, 1.0, 0); sections != nil {
		t.Error("zero sample rate should return nil")
	}
}

func TestChebyshev1LP_RippleEdgeCases(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	// Zero ripple should still work (uses default)
	sections := Chebyshev1LP(fc, 4, 0, sr)
	if len(sections) != 2 {
		t.Fatalf("zero ripple: expected 2 sections, got %d", len(sections))
	}
	for _, s := range sections {
		assertFiniteCoefficients(t, s)
		assertStableSection(t, s)
	}

	// Negative ripple should still work (uses default)
	sections = Chebyshev1LP(fc, 4, -1, sr)
	if len(sections) != 2 {
		t.Fatalf("negative ripple: expected 2 sections, got %d", len(sections))
	}
	for _, s := range sections {
		assertFiniteCoefficients(t, s)
		assertStableSection(t, s)
	}
}

// --- Chebyshev Type I Highpass ---

func TestChebyshev1HP_Basic(t *testing.T) {
	sr := 48000.0
	sections := Chebyshev1HP(1000, 4, 1.0, sr)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections for order 4, got %d", len(sections))
	}
	for i, s := range sections {
		assertFiniteCoefficients(t, s)
		assertStableSection(t, s)
		_ = i
	}
}

func TestChebyshev1HP_HighFreqGain(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for _, order := range []int{2, 4, 6, 8} {
		sections := Chebyshev1HP(fc, order, 1.0, sr)
		// Far above cutoff, gain should be near 0 dB
		highGain := cascadeMagDB(sections, sr*0.4, sr)
		if highGain < -3 || highGain > 3 {
			t.Errorf("order %d: high-freq gain = %.2f dB, expected near 0 dB", order, highGain)
		}
	}
}

func TestChebyshev1HP_StopbandAttenuation(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for _, order := range []int{2, 4, 6} {
		sections := Chebyshev1HP(fc, order, 1.0, sr)
		// Well below cutoff should be highly attenuated
		lowGain := cascadeMagDB(sections, fc/5, sr)
		if lowGain > -20 {
			t.Errorf("order %d: gain at fc/5 = %.2f dB, expected < -20 dB", order, lowGain)
		}
	}
}

func TestChebyshev1HP_SteepnessVsButterworth(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	order := 4

	cheby := Chebyshev1HP(fc, order, 1.0, sr)
	butter := ButterworthHP(fc, order, sr)

	// At fc/2, Chebyshev should have more attenuation
	chebyAtHalf := cascadeMagDB(cheby, fc/2, sr)
	butterAtHalf := cascadeMagDB(butter, fc/2, sr)

	if chebyAtHalf > butterAtHalf {
		t.Errorf("Chebyshev (%.2f dB) should attenuate more at fc/2 than Butterworth (%.2f dB)",
			chebyAtHalf, butterAtHalf)
	}
}

func TestChebyshev1HP_OddOrder(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for _, order := range []int{1, 3, 5, 7} {
		sections := Chebyshev1HP(fc, order, 1.0, sr)
		expected := (order + 1) / 2
		if len(sections) != expected {
			t.Errorf("order %d: expected %d sections, got %d", order, expected, len(sections))
		}
		for _, s := range sections {
			assertFiniteCoefficients(t, s)
			assertStableSection(t, s)
		}
	}
}

func TestChebyshev1HP_Stability_AllOrders(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for order := 1; order <= 12; order++ {
		sections := Chebyshev1HP(fc, order, 1.0, sr)
		for i, s := range sections {
			assertFiniteCoefficients(t, s)
			r1, r2 := sectionRoots(s)
			if cmplx.Abs(r1) >= 1 || cmplx.Abs(r2) >= 1 {
				t.Errorf("order %d section %d: unstable poles |r1|=%.6f |r2|=%.6f",
					order, i, cmplx.Abs(r1), cmplx.Abs(r2))
			}
		}
	}
}

func TestChebyshev1HP_EdgeCases(t *testing.T) {
	if sections := Chebyshev1HP(1000, 0, 1.0, 48000); sections != nil {
		t.Error("order 0 should return nil")
	}
	if sections := Chebyshev1HP(0, 4, 1.0, 48000); sections != nil {
		t.Error("zero freq should return nil")
	}
	if sections := Chebyshev1HP(24000, 4, 1.0, 48000); sections != nil {
		t.Error("freq at Nyquist should return nil")
	}
}

func TestChebyshev1HP_CutoffAttenuation(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for _, order := range []int{2, 4, 6} {
		sections := Chebyshev1HP(fc, order, 1.0, sr)
		atCutoff := cascadeMagDB(sections, fc, sr)
		if atCutoff > 1 || atCutoff < -6 {
			t.Errorf("order %d: gain at cutoff = %.2f dB, expected within [-6, 1] dB", order, atCutoff)
		}
	}
}

func TestChebyshev1_LP_HP_Symmetry(t *testing.T) {
	sr := 48000.0
	fc := 2000.0
	order := 4
	ripple := 1.0

	lp := Chebyshev1LP(fc, order, ripple, sr)
	hp := Chebyshev1HP(fc, order, ripple, sr)

	// LP at low freq should match HP at high freq (both near 0 dB)
	lpLow := cascadeMagDB(lp, 100, sr)
	hpHigh := cascadeMagDB(hp, sr*0.4, sr)

	if math.Abs(lpLow-hpHigh) > 3 {
		t.Errorf("LP passband (%.2f dB) and HP passband (%.2f dB) should be comparable", lpLow, hpHigh)
	}

	// LP at cutoff should be similar to HP at cutoff
	lpCut := cascadeMagDB(lp, fc, sr)
	hpCut := cascadeMagDB(hp, fc, sr)

	if math.Abs(lpCut-hpCut) > 3 {
		t.Errorf("LP at cutoff (%.2f dB) and HP at cutoff (%.2f dB) should be comparable", lpCut, hpCut)
	}
}

func TestChebyshev1LP_FrequencyRange(t *testing.T) {
	sr := 48000.0
	// Test with various cutoff frequencies
	for _, fc := range []float64{50, 100, 500, 1000, 5000, 10000, 20000} {
		sections := Chebyshev1LP(fc, 4, 1.0, sr)
		if sections == nil {
			t.Errorf("fc=%.0f: returned nil", fc)
			continue
		}
		dcGain := cascadeMagDB(sections, fc*0.01, sr)
		if dcGain < -2 || dcGain > 2 {
			t.Errorf("fc=%.0f: DC gain = %.2f dB, expected near 0 dB", fc, dcGain)
		}
	}
}
