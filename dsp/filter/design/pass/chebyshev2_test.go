package pass

import (
	"math"
	"math/cmplx"
	"testing"
)

// --- Chebyshev Type II Lowpass ---

func TestChebyshev2LP_Basic(t *testing.T) {
	sr := 48000.0

	sections := Chebyshev2LP(1000, 4, 2.0, sr)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections for order 4, got %d", len(sections))
	}

	for _, s := range sections {
		assertFiniteCoefficients(t, s)
		assertStableSection(t, s)
	}
}

func TestChebyshev2LP_PassbandFlat(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for _, order := range []int{4, 6, 8} {
		sections := Chebyshev2LP(fc, order, 2.0, sr)
		// Passband should be maximally flat (monotonically decreasing)
		// Check that the passband variation is small (< 1 dB up to 80% of cutoff)
		maxPB, minPB := -1000.0, 1000.0

		for f := 10.0; f <= fc*0.8; f += 5 {
			g := cascadeMagDB(sections, f, sr)
			if g > maxPB {
				maxPB = g
			}

			if g < minPB {
				minPB = g
			}
		}

		if maxPB-minPB > 1.0 {
			t.Errorf("order %d: passband variation = %.4f dB, expected < 1 dB", order, maxPB-minPB)
		}

		if math.Abs(maxPB) > 0.5 {
			t.Errorf("order %d: passband max = %.4f dB, expected near 0 dB", order, maxPB)
		}
	}
}

func TestChebyshev2LP_StopbandFloor(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	// With ripple=2.0, stopband attenuation is ~-7 dB.
	// Use ripple=3.0 for deeper stopband (~-10 dB).
	for _, tc := range []struct {
		ripple, minAtten float64
	}{
		{2.0, -7.0},
		{3.0, -10.0},
	} {
		for _, order := range []int{4, 6, 8} {
			sections := Chebyshev2LP(fc, order, tc.ripple, sr)
			// Measure deep in stopband
			maxSB := -1000.0

			for f := fc * 2; f < sr*0.45; f += 100 {
				g := cascadeMagDB(sections, f, sr)
				if g > maxSB {
					maxSB = g
				}
			}

			if maxSB > tc.minAtten+1 {
				t.Errorf("order %d ripple=%.1f: stopband max = %.2f dB, expected < %.0f dB",
					order, tc.ripple, maxSB, tc.minAtten+1)
			}
		}
	}
}

func TestChebyshev2LP_StopbandEquiripple(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	order := 6

	sections := Chebyshev2LP(fc, order, 2.0, sr)

	// Type II stopband should have equiripple behavior:
	// multiple local minima (notches) with maxima bounded by the stopband level
	var minStop, maxStop float64

	minStop = 0
	maxStop = -200

	for f := 2 * fc; f < sr/2; f += 50 {
		g := cascadeMagDB(sections, f, sr)
		if g < minStop {
			minStop = g
		}

		if g > maxStop {
			maxStop = g
		}
	}
	// All stopband values should be negative (attenuating)
	if maxStop > 0 {
		t.Errorf("stopband gain exceeds 0 dB: max=%.2f dB", maxStop)
	}
	// There should be significant variation (equiripple notches)
	if maxStop-minStop < 3 {
		t.Errorf("stopband has insufficient variation: max=%.2f min=%.2f (expected equiripple)",
			maxStop, minStop)
	}
}

func TestChebyshev2LP_DeeperStopbandWithHigherRipple(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	order := 6

	// Higher ripple parameter should produce deeper stopband
	ripples := []float64{1.0, 2.0, 3.0}

	var prevMax float64

	for i, ripple := range ripples {
		sections := Chebyshev2LP(fc, order, ripple, sr)
		maxSB := -1000.0

		for f := fc * 2; f < sr*0.45; f += 50 {
			g := cascadeMagDB(sections, f, sr)
			if g > maxSB {
				maxSB = g
			}
		}

		if i > 0 && maxSB >= prevMax {
			t.Errorf("ripple=%.1f: stopband max %.2f dB not deeper than ripple=%.1f (%.2f dB)",
				ripple, maxSB, ripples[i-1], prevMax)
		}

		prevMax = maxSB
	}
}

func TestChebyshev2LP_CutoffAttenuation(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for _, order := range []int{2, 4, 6} {
		sections := Chebyshev2LP(fc, order, 2.0, sr)
		atCutoff := cascadeMagDB(sections, fc, sr)
		// Type II has its stopband edge near the specified frequency
		if atCutoff > 1 || atCutoff < -20 {
			t.Errorf("order %d: gain at cutoff = %.2f dB, expected within [-20, 1] dB", order, atCutoff)
		}
	}
}

func TestChebyshev2LP_OddOrder(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for _, order := range []int{1, 3, 5, 7} {
		sections := Chebyshev2LP(fc, order, 2.0, sr)

		expected := (order + 1) / 2
		if len(sections) != expected {
			t.Errorf("order %d: expected %d sections, got %d", order, expected, len(sections))
		}

		for _, s := range sections {
			assertFiniteCoefficients(t, s)
			assertStableSection(t, s)
		}
		// Should still be lowpass
		dcGain := cascadeMagDB(sections, 10, sr)
		if dcGain < -3 {
			t.Errorf("order %d: DC gain too low: %.2f dB", order, dcGain)
		}
	}
}

func TestChebyshev2LP_Stability_AllOrders(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for order := 1; order <= 12; order++ {
		sections := Chebyshev2LP(fc, order, 2.0, sr)
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

func TestChebyshev2LP_RippleEffect(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	order := 4

	// Different ripple values should affect the stopband rejection level
	sections1 := Chebyshev2LP(fc, order, 0.5, sr)
	sections2 := Chebyshev2LP(fc, order, 2.0, sr)

	// Both should be valid lowpass filters with flat passband
	dc1 := cascadeMagDB(sections1, 10, sr)
	dc2 := cascadeMagDB(sections2, 10, sr)

	if math.Abs(dc1) > 1 {
		t.Errorf("ripple=0.5: DC gain = %.2f dB, expected near 0 dB", dc1)
	}

	if math.Abs(dc2) > 1 {
		t.Errorf("ripple=2.0: DC gain = %.2f dB, expected near 0 dB", dc2)
	}

	// Higher ripple should give deeper stopband
	sb1 := cascadeMagDB(sections1, fc*5, sr)

	sb2 := cascadeMagDB(sections2, fc*5, sr)
	if sb2 >= sb1 {
		t.Errorf("higher ripple should give deeper stopband: sb(0.5)=%.2f, sb(2.0)=%.2f", sb1, sb2)
	}
}

func TestChebyshev2LP_SampleRates(t *testing.T) {
	for _, sr := range []float64{8000, 22050, 44100, 48000, 96000, 192000} {
		fc := sr * 0.1

		sections := Chebyshev2LP(fc, 4, 2.0, sr)
		if len(sections) != 2 {
			t.Errorf("sr=%.0f: expected 2 sections, got %d", sr, len(sections))
		}

		dcGain := cascadeMagDB(sections, fc*0.1, sr)
		if math.Abs(dcGain) > 1 {
			t.Errorf("sr=%.0f: DC gain = %.2f dB, expected near 0 dB", sr, dcGain)
		}
	}
}

func TestChebyshev2LP_EdgeCases(t *testing.T) {
	if sections := Chebyshev2LP(1000, 0, 2.0, 48000); sections != nil {
		t.Error("order 0 should return nil")
	}

	if sections := Chebyshev2LP(1000, -1, 2.0, 48000); sections != nil {
		t.Error("negative order should return nil")
	}

	if sections := Chebyshev2LP(0, 4, 2.0, 48000); sections != nil {
		t.Error("zero freq should return nil")
	}

	if sections := Chebyshev2LP(24000, 4, 2.0, 48000); sections != nil {
		t.Error("freq at Nyquist should return nil")
	}

	if sections := Chebyshev2LP(1000, 4, 2.0, 0); sections != nil {
		t.Error("zero sample rate should return nil")
	}
}

func TestChebyshev2LP_FrequencyRange(t *testing.T) {
	sr := 48000.0
	for _, fc := range []float64{50, 100, 500, 1000, 5000, 10000, 20000} {
		sections := Chebyshev2LP(fc, 4, 2.0, sr)
		if sections == nil {
			t.Errorf("fc=%.0f: returned nil", fc)
			continue
		}

		dcGain := cascadeMagDB(sections, fc*0.01, sr)
		if math.Abs(dcGain) > 1 {
			t.Errorf("fc=%.0f: DC gain = %.2f dB, expected near 0 dB", fc, dcGain)
		}
	}
}

// --- Chebyshev Type II Highpass ---

func TestChebyshev2HP_Basic(t *testing.T) {
	sr := 48000.0

	sections := Chebyshev2HP(1000, 4, 2.0, sr)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections for order 4, got %d", len(sections))
	}

	for _, s := range sections {
		assertFiniteCoefficients(t, s)
		assertStableSection(t, s)
	}
}

func TestChebyshev2HP_PassbandFlat(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for _, order := range []int{4, 6, 8} {
		sections := Chebyshev2HP(fc, order, 2.0, sr)
		// Passband (above cutoff) should be maximally flat
		maxPB, minPB := -1000.0, 1000.0

		for f := fc * 1.5; f <= sr*0.45; f += 50 {
			g := cascadeMagDB(sections, f, sr)
			if g > maxPB {
				maxPB = g
			}

			if g < minPB {
				minPB = g
			}
		}

		if maxPB-minPB > 1.0 {
			t.Errorf("order %d: passband variation = %.4f dB, expected < 1 dB", order, maxPB-minPB)
		}

		if math.Abs(maxPB) > 0.5 {
			t.Errorf("order %d: passband max = %.4f dB, expected near 0 dB", order, maxPB)
		}
	}
}

func TestChebyshev2HP_StopbandAttenuation(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	// With ripple=2.0, stopband is ~-7 dB. Use ripple=3.0 for ~-10 dB.
	for _, tc := range []struct {
		ripple, minAtten float64
	}{
		{2.0, -7.0},
		{3.0, -10.0},
	} {
		for _, order := range []int{4, 6, 8} {
			sections := Chebyshev2HP(fc, order, tc.ripple, sr)
			maxSB := -1000.0

			for f := 10.0; f <= fc*0.5; f += 5 {
				g := cascadeMagDB(sections, f, sr)
				if g > maxSB {
					maxSB = g
				}
			}

			if maxSB > tc.minAtten+1 {
				t.Errorf("order %d ripple=%.1f: stopband max = %.2f dB, expected < %.0f dB",
					order, tc.ripple, maxSB, tc.minAtten+1)
			}
		}
	}
}

func TestChebyshev2HP_HighFreqGain(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for _, order := range []int{2, 4, 6, 8} {
		sections := Chebyshev2HP(fc, order, 2.0, sr)

		highGain := cascadeMagDB(sections, sr*0.4, sr)
		if math.Abs(highGain) > 1 {
			t.Errorf("order %d: high-freq gain = %.2f dB, expected near 0 dB", order, highGain)
		}
	}
}

func TestChebyshev2HP_OddOrder(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for _, order := range []int{1, 3, 5, 7} {
		sections := Chebyshev2HP(fc, order, 2.0, sr)

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

func TestChebyshev2HP_Stability_AllOrders(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for order := 1; order <= 12; order++ {
		sections := Chebyshev2HP(fc, order, 2.0, sr)
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

func TestChebyshev2HP_EdgeCases(t *testing.T) {
	if sections := Chebyshev2HP(1000, 0, 2.0, 48000); sections != nil {
		t.Error("order 0 should return nil")
	}

	if sections := Chebyshev2HP(0, 4, 2.0, 48000); sections != nil {
		t.Error("zero freq should return nil")
	}

	if sections := Chebyshev2HP(24000, 4, 2.0, 48000); sections != nil {
		t.Error("freq at Nyquist should return nil")
	}
}

func TestChebyshev2HP_CutoffAttenuation(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for _, order := range []int{2, 4, 6} {
		sections := Chebyshev2HP(fc, order, 2.0, sr)

		atCutoff := cascadeMagDB(sections, fc, sr)
		if atCutoff > 1 || atCutoff < -20 {
			t.Errorf("order %d: gain at cutoff = %.2f dB, expected within [-20, 1] dB", order, atCutoff)
		}
	}
}

func TestChebyshev2_LP_HP_Symmetry(t *testing.T) {
	sr := 48000.0
	fc := 2000.0
	order := 4
	ripple := 2.0

	lp := Chebyshev2LP(fc, order, ripple, sr)
	hp := Chebyshev2HP(fc, order, ripple, sr)

	// LP passband and HP passband should both be near 0 dB
	lpLow := cascadeMagDB(lp, 100, sr)
	hpHigh := cascadeMagDB(hp, sr*0.4, sr)

	if math.Abs(lpLow-hpHigh) > 2 {
		t.Errorf("LP passband (%.2f dB) and HP passband (%.2f dB) should be comparable", lpLow, hpHigh)
	}
}

func TestChebyshev2HP_SampleRates(t *testing.T) {
	for _, sr := range []float64{8000, 22050, 44100, 48000, 96000, 192000} {
		fc := sr * 0.1

		sections := Chebyshev2HP(fc, 4, 2.0, sr)
		if len(sections) != 2 {
			t.Errorf("sr=%.0f: expected 2 sections, got %d", sr, len(sections))
		}

		highGain := cascadeMagDB(sections, sr*0.4, sr)
		if math.Abs(highGain) > 1 {
			t.Errorf("sr=%.0f: high-freq gain = %.2f dB, expected near 0 dB", sr, highGain)
		}
	}
}

func TestChebyshev2HP_FrequencyRange(t *testing.T) {
	sr := 48000.0
	for _, fc := range []float64{50, 100, 500, 1000, 5000, 10000} {
		sections := Chebyshev2HP(fc, 4, 2.0, sr)
		if sections == nil {
			t.Errorf("fc=%.0f: returned nil", fc)
			continue
		}
		// Measure well above cutoff but away from Nyquist
		measFreq := math.Min(fc*10, sr*0.4)

		highGain := cascadeMagDB(sections, measFreq, sr)
		if math.Abs(highGain) > 2 {
			t.Errorf("fc=%.0f: high-freq gain at %.0f Hz = %.2f dB, expected near 0 dB",
				fc, measFreq, highGain)
		}
	}
}

func TestChebyshev2LP_RippleEdgeCases(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	// Zero ripple should still work (uses default)
	sections := Chebyshev2LP(fc, 4, 0, sr)
	if len(sections) != 2 {
		t.Fatalf("zero ripple: expected 2 sections, got %d", len(sections))
	}

	for _, s := range sections {
		assertFiniteCoefficients(t, s)
		assertStableSection(t, s)
	}

	// Negative ripple should still work (uses default)
	sections = Chebyshev2LP(fc, 4, -1, sr)
	if len(sections) != 2 {
		t.Fatalf("negative ripple: expected 2 sections, got %d", len(sections))
	}

	for _, s := range sections {
		assertFiniteCoefficients(t, s)
		assertStableSection(t, s)
	}
}

// --- Time-domain impulse response tests ---

func TestChebyshev2LP_ImpulseResponse_Bounded(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	sections := Chebyshev2LP(fc, 4, 2.0, sr)
	chain := chainForTest(sections)

	out := chain.ProcessSample(1.0)

	maxVal := math.Abs(out)
	for range 1000 {
		out = chain.ProcessSample(0.0)
		if v := math.Abs(out); v > maxVal {
			maxVal = v
		}
	}

	if maxVal > 10 || math.IsNaN(maxVal) || math.IsInf(maxVal, 0) {
		t.Errorf("impulse response unbounded or NaN: max=%.6f", maxVal)
	}
}

func TestChebyshev1LP_ImpulseResponse_Bounded(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	sections := Chebyshev1LP(fc, 4, 1.0, sr)
	chain := chainForTest(sections)

	out := chain.ProcessSample(1.0)

	maxVal := math.Abs(out)
	for range 1000 {
		out = chain.ProcessSample(0.0)
		if v := math.Abs(out); v > maxVal {
			maxVal = v
		}
	}

	if maxVal > 10 || math.IsNaN(maxVal) || math.IsInf(maxVal, 0) {
		t.Errorf("impulse response unbounded or NaN: max=%.6f", maxVal)
	}
}

func TestChebyshev1HP_ImpulseResponse_Bounded(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	sections := Chebyshev1HP(fc, 4, 1.0, sr)
	chain := chainForTest(sections)

	out := chain.ProcessSample(1.0)

	maxVal := math.Abs(out)
	for range 1000 {
		out = chain.ProcessSample(0.0)
		if v := math.Abs(out); v > maxVal {
			maxVal = v
		}
	}

	if maxVal > 10 || math.IsNaN(maxVal) || math.IsInf(maxVal, 0) {
		t.Errorf("impulse response unbounded or NaN: max=%.6f", maxVal)
	}
}

func TestChebyshev2HP_ImpulseResponse_Bounded(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	sections := Chebyshev2HP(fc, 4, 2.0, sr)
	chain := chainForTest(sections)

	out := chain.ProcessSample(1.0)

	maxVal := math.Abs(out)
	for range 1000 {
		out = chain.ProcessSample(0.0)
		if v := math.Abs(out); v > maxVal {
			maxVal = v
		}
	}

	if maxVal > 10 || math.IsNaN(maxVal) || math.IsInf(maxVal, 0) {
		t.Errorf("impulse response unbounded or NaN: max=%.6f", maxVal)
	}
}

// --- Sign convention regression tests ---

func TestChebyshev1LP_A1_Negative(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	sections := Chebyshev1LP(fc, 4, 1.0, sr)
	for i, s := range sections {
		if s.A1 >= 0 {
			t.Errorf("section %d: A1=%.10f should be negative (sign convention)", i, s.A1)
		}

		if s.A2 <= 0 {
			t.Errorf("section %d: A2=%.10f should be positive (sign convention)", i, s.A2)
		}
	}
}

func TestChebyshev2LP_A1_Negative(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	sections := Chebyshev2LP(fc, 4, 2.0, sr)
	for i, s := range sections {
		if s.A1 >= 0 {
			t.Errorf("section %d: A1=%.10f should be negative (sign convention)", i, s.A1)
		}

		if s.A2 <= 0 {
			t.Errorf("section %d: A2=%.10f should be positive (sign convention)", i, s.A2)
		}
	}
}

func TestChebyshev1HP_A1_Negative(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	sections := Chebyshev1HP(fc, 4, 1.0, sr)
	for i, s := range sections {
		if s.A1 >= 0 {
			t.Errorf("section %d: A1=%.10f should be negative for HP (sign convention)", i, s.A1)
		}
	}
}

func TestChebyshev2HP_A1_Negative(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	sections := Chebyshev2HP(fc, 4, 2.0, sr)
	for i, s := range sections {
		if s.A1 >= 0 {
			t.Errorf("section %d: A1=%.10f should be negative for HP (sign convention)", i, s.A1)
		}
	}
}
