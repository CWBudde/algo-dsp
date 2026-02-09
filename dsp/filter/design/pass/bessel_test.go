package pass

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// --- Bessel Lowpass ---

func TestBesselLP_Basic(t *testing.T) {
	sr := 48000.0
	sections := BesselLP(1000, 4, sr)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections for order 4, got %d", len(sections))
	}
	for _, s := range sections {
		assertFiniteCoefficients(t, s)
		assertStableSection(t, s)
	}
}

func TestBesselLP_PassbandFlat(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for _, order := range []int{2, 4, 6, 8} {
		sections := BesselLP(fc, order, sr)
		// Bessel optimizes group delay, not magnitude. Its passband rolls off
		// more gently and earlier than Butterworth, so measure up to 0.5*fc
		// and allow up to 1 dB variation in that range.
		maxPB, minPB := -1000.0, 1000.0
		for f := 10.0; f <= fc*0.5; f += 5 {
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

func TestBesselLP_Rolloff(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	// Bessel should roll off more gently than Butterworth at the same order.
	for _, order := range []int{4, 6, 8} {
		bessel := BesselLP(fc, order, sr)
		bw := ButterworthLP(fc, order, sr)

		// At 2x cutoff, Bessel attenuation should be less than Butterworth.
		besselAtten := cascadeMagDB(bessel, 2*fc, sr)
		bwAtten := cascadeMagDB(bw, 2*fc, sr)
		if besselAtten <= bwAtten {
			t.Errorf("order %d: Bessel at 2fc (%.2f dB) should be less attenuated than Butterworth (%.2f dB)",
				order, besselAtten, bwAtten)
		}
	}
}

func TestBesselLP_GroupDelayFlat(t *testing.T) {
	sr := 48000.0
	fc := 2000.0

	for _, order := range []int{4, 6} {
		sections := BesselLP(fc, order, sr)

		// Measure group delay at several passband frequencies.
		// Group delay = -d(phase)/d(omega), approximated by finite difference.
		df := 1.0 // Hz step for finite difference
		var delays []float64
		for f := 100.0; f <= fc*0.5; f += 50 {
			phase1 := cascadePhase(sections, f-df/2, sr)
			phase2 := cascadePhase(sections, f+df/2, sr)
			gd := -(phase2 - phase1) / (2 * math.Pi * df)
			delays = append(delays, gd)
		}

		if len(delays) < 2 {
			continue
		}

		// Find min and max group delay.
		minGD, maxGD := delays[0], delays[0]
		for _, gd := range delays[1:] {
			if gd < minGD {
				minGD = gd
			}
			if gd > maxGD {
				maxGD = gd
			}
		}

		// Bessel: group delay variation should be very small in the passband.
		// Allow 20% variation relative to mean.
		meanGD := (minGD + maxGD) / 2
		if meanGD > 0 {
			variation := (maxGD - minGD) / meanGD
			if variation > 0.2 {
				t.Errorf("order %d: group delay variation = %.1f%% (min=%.6f max=%.6f), expected < 20%%",
					order, variation*100, minGD, maxGD)
			}
		}
	}
}

func TestBesselLP_CutoffAttenuation(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for _, order := range []int{2, 4, 6, 8} {
		sections := BesselLP(fc, order, sr)
		atCutoff := cascadeMagDB(sections, fc, sr)
		// Bessel -3 dB normalized: should be near -3 dB at cutoff.
		if atCutoff > -1 || atCutoff < -6 {
			t.Errorf("order %d: gain at cutoff = %.2f dB, expected near -3 dB", order, atCutoff)
		}
	}
}

func TestBesselLP_OddOrder(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for _, order := range []int{1, 3, 5, 7, 9} {
		sections := BesselLP(fc, order, sr)
		expected := (order + 1) / 2
		if len(sections) != expected {
			t.Errorf("order %d: expected %d sections, got %d", order, expected, len(sections))
		}
		for _, s := range sections {
			assertFiniteCoefficients(t, s)
			assertStableSection(t, s)
		}
		dcGain := cascadeMagDB(sections, 10, sr)
		if dcGain < -1 {
			t.Errorf("order %d: DC gain too low: %.2f dB", order, dcGain)
		}
	}
}

func TestBesselLP_Stability_AllOrders(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for order := 1; order <= 10; order++ {
		sections := BesselLP(fc, order, sr)
		if sections == nil {
			t.Errorf("order %d: returned nil", order)
			continue
		}
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

func TestBesselLP_EdgeCases(t *testing.T) {
	if sections := BesselLP(1000, 0, 48000); sections != nil {
		t.Error("order 0 should return nil")
	}
	if sections := BesselLP(1000, -1, 48000); sections != nil {
		t.Error("negative order should return nil")
	}
	if sections := BesselLP(1000, 11, 48000); sections != nil {
		t.Error("order > 10 should return nil")
	}
	if sections := BesselLP(0, 4, 48000); sections != nil {
		t.Error("zero freq should return nil")
	}
	if sections := BesselLP(24000, 4, 48000); sections != nil {
		t.Error("freq at Nyquist should return nil")
	}
	if sections := BesselLP(1000, 4, 0); sections != nil {
		t.Error("zero sample rate should return nil")
	}
}

func TestBesselLP_SampleRates(t *testing.T) {
	for _, sr := range []float64{8000, 22050, 44100, 48000, 96000, 192000} {
		fc := sr * 0.1
		sections := BesselLP(fc, 4, sr)
		if len(sections) != 2 {
			t.Errorf("sr=%.0f: expected 2 sections, got %d", sr, len(sections))
		}
		dcGain := cascadeMagDB(sections, fc*0.01, sr)
		if math.Abs(dcGain) > 1 {
			t.Errorf("sr=%.0f: DC gain = %.2f dB, expected near 0 dB", sr, dcGain)
		}
	}
}

func TestBesselLP_FrequencyRange(t *testing.T) {
	sr := 48000.0
	for _, fc := range []float64{50, 100, 500, 1000, 5000, 10000, 20000} {
		sections := BesselLP(fc, 4, sr)
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

func TestBesselLP_ImpulseResponse_Bounded(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	sections := BesselLP(fc, 4, sr)
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

// --- Bessel Highpass ---

func TestBesselHP_Basic(t *testing.T) {
	sr := 48000.0
	sections := BesselHP(1000, 4, sr)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections for order 4, got %d", len(sections))
	}
	for _, s := range sections {
		assertFiniteCoefficients(t, s)
		assertStableSection(t, s)
	}
}

func TestBesselHP_HighFreqGain(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for _, order := range []int{2, 4, 6, 8} {
		sections := BesselHP(fc, order, sr)
		highGain := cascadeMagDB(sections, sr*0.4, sr)
		if math.Abs(highGain) > 1 {
			t.Errorf("order %d: high-freq gain = %.2f dB, expected near 0 dB", order, highGain)
		}
	}
}

func TestBesselHP_OddOrder(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for _, order := range []int{1, 3, 5, 7, 9} {
		sections := BesselHP(fc, order, sr)
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

func TestBesselHP_Stability_AllOrders(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for order := 1; order <= 10; order++ {
		sections := BesselHP(fc, order, sr)
		if sections == nil {
			t.Errorf("order %d: returned nil", order)
			continue
		}
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

func TestBesselHP_EdgeCases(t *testing.T) {
	if sections := BesselHP(1000, 0, 48000); sections != nil {
		t.Error("order 0 should return nil")
	}
	if sections := BesselHP(1000, 11, 48000); sections != nil {
		t.Error("order > 10 should return nil")
	}
	if sections := BesselHP(0, 4, 48000); sections != nil {
		t.Error("zero freq should return nil")
	}
	if sections := BesselHP(24000, 4, 48000); sections != nil {
		t.Error("freq at Nyquist should return nil")
	}
}

func TestBesselHP_ImpulseResponse_Bounded(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	sections := BesselHP(fc, 4, sr)
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

func TestBesselHP_SampleRates(t *testing.T) {
	for _, sr := range []float64{8000, 22050, 44100, 48000, 96000, 192000} {
		fc := sr * 0.1
		sections := BesselHP(fc, 4, sr)
		if len(sections) != 2 {
			t.Errorf("sr=%.0f: expected 2 sections, got %d", sr, len(sections))
		}
		highGain := cascadeMagDB(sections, sr*0.4, sr)
		if math.Abs(highGain) > 1 {
			t.Errorf("sr=%.0f: high-freq gain = %.2f dB, expected near 0 dB", sr, highGain)
		}
	}
}

// --- LP/HP Symmetry ---

func TestBessel_LP_HP_Symmetry(t *testing.T) {
	sr := 48000.0
	fc := 2000.0
	order := 4

	lp := BesselLP(fc, order, sr)
	hp := BesselHP(fc, order, sr)

	// LP passband and HP passband should both be near 0 dB.
	lpLow := cascadeMagDB(lp, 100, sr)
	hpHigh := cascadeMagDB(hp, sr*0.4, sr)

	if math.Abs(lpLow-hpHigh) > 2 {
		t.Errorf("LP passband (%.2f dB) and HP passband (%.2f dB) should be comparable", lpLow, hpHigh)
	}
}

// cascadePhase computes the total phase response of a biquad cascade at the given frequency.
func cascadePhase(sections []biquad.Coefficients, freq, sr float64) float64 {
	h := complex(1, 0)
	for _, c := range sections {
		h *= c.Response(freq, sr)
	}
	return cmplx.Phase(h)
}
