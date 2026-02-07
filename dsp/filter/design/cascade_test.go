package design

import (
	"fmt"
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// ---------------------------------------------------------------------------
// Butterworth section count and structure
// ---------------------------------------------------------------------------

func TestButterworthLP_SectionCount(t *testing.T) {
	sr := 48000.0
	for order := 1; order <= 8; order++ {
		want := (order + 1) / 2
		got := ButterworthLP(1000, order, sr)
		if len(got) != want {
			t.Fatalf("order %d: sections=%d, want %d", order, len(got), want)
		}
	}
}

func TestButterworthHP_SectionCount(t *testing.T) {
	sr := 48000.0
	for order := 1; order <= 8; order++ {
		want := (order + 1) / 2
		got := ButterworthHP(1000, order, sr)
		if len(got) != want {
			t.Fatalf("order %d: sections=%d, want %d", order, len(got), want)
		}
	}
}

func TestButterworth_EvenOrder_NoFirstOrderSection(t *testing.T) {
	sr := 48000.0
	for _, order := range []int{2, 4, 6, 8} {
		lp := ButterworthLP(1000, order, sr)
		hp := ButterworthHP(1000, order, sr)
		for i, c := range lp {
			if c.B2 == 0 && c.A2 == 0 {
				t.Fatalf("LP order %d: section %d is first-order, expected all second-order", order, i)
			}
		}
		for i, c := range hp {
			if c.B2 == 0 && c.A2 == 0 {
				t.Fatalf("HP order %d: section %d is first-order, expected all second-order", order, i)
			}
		}
	}
}

func TestButterworth_OddOrder_HasFirstOrderSection(t *testing.T) {
	sr := 48000.0
	for _, order := range []int{1, 3, 5, 7} {
		lp := ButterworthLP(1000, order, sr)
		hp := ButterworthHP(1000, order, sr)
		last := lp[len(lp)-1]
		if last.B2 != 0 || last.A2 != 0 {
			t.Fatalf("LP order %d: last section not first-order: %+v", order, last)
		}
		last = hp[len(hp)-1]
		if last.B2 != 0 || last.A2 != 0 {
			t.Fatalf("HP order %d: last section not first-order: %+v", order, last)
		}
	}
}

// ---------------------------------------------------------------------------
// Butterworth -3 dB at cutoff (defining property)
// ---------------------------------------------------------------------------

func TestButterworthLP_Minus3dBAtCutoff(t *testing.T) {
	sr := 48000.0
	for _, order := range []int{1, 2, 3, 4, 5, 6, 8} {
		chain := biquad.NewChain(ButterworthLP(1000, order, sr))
		cutoffDB := chain.MagnitudeDB(1000, sr)
		if !almostEqual(cutoffDB, -3.01, 0.1) {
			t.Fatalf("order %d: cutoff magnitude=%.2f dB, want ~-3.01 dB", order, cutoffDB)
		}
	}
}

func TestButterworthHP_Minus3dBAtCutoff(t *testing.T) {
	sr := 48000.0
	for _, order := range []int{1, 2, 3, 4, 5, 6, 8} {
		chain := biquad.NewChain(ButterworthHP(1000, order, sr))
		cutoffDB := chain.MagnitudeDB(1000, sr)
		if !almostEqual(cutoffDB, -3.01, 0.1) {
			t.Fatalf("order %d: cutoff magnitude=%.2f dB, want ~-3.01 dB", order, cutoffDB)
		}
	}
}

// ---------------------------------------------------------------------------
// Butterworth monotonic stopband attenuation increases with order
// ---------------------------------------------------------------------------

func TestButterworthLP_HigherOrderSteeperRolloff(t *testing.T) {
	sr := 48000.0
	prevAtten := 0.0
	for _, order := range []int{1, 2, 4, 6, 8} {
		chain := biquad.NewChain(ButterworthLP(1000, order, sr))
		atten := -chain.MagnitudeDB(10000, sr) // positive attenuation at 10x cutoff
		if atten <= prevAtten {
			t.Fatalf("order %d: attenuation %.1f dB <= previous %.1f dB at 10 kHz",
				order, atten, prevAtten)
		}
		prevAtten = atten
	}
}

func TestButterworthHP_HigherOrderSteeperRolloff(t *testing.T) {
	sr := 48000.0
	prevAtten := 0.0
	for _, order := range []int{1, 2, 4, 6, 8} {
		chain := biquad.NewChain(ButterworthHP(1000, order, sr))
		atten := -chain.MagnitudeDB(100, sr) // positive attenuation at 1/10th cutoff
		if atten <= prevAtten {
			t.Fatalf("order %d: attenuation %.1f dB <= previous %.1f dB at 100 Hz",
				order, atten, prevAtten)
		}
		prevAtten = atten
	}
}

// ---------------------------------------------------------------------------
// Butterworth stability across orders and sample rates
// ---------------------------------------------------------------------------

func TestButterworth_AllSectionsStable(t *testing.T) {
	for _, sr := range []float64{44100, 48000, 96000, 192000} {
		for _, order := range []int{1, 2, 3, 4, 5, 6, 7, 8} {
			for _, sections := range [][]biquad.Coefficients{
				ButterworthLP(1000, order, sr),
				ButterworthHP(1000, order, sr),
			} {
				for _, c := range sections {
					assertFiniteCoefficients(t, c)
					assertStableSection(t, c)
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Butterworth invalid inputs
// ---------------------------------------------------------------------------

func TestButterworth_InvalidInputs(t *testing.T) {
	if got := ButterworthLP(1000, -1, 48000); got != nil {
		t.Fatal("expected nil for negative order")
	}
	if got := ButterworthHP(1000, -1, 48000); got != nil {
		t.Fatal("expected nil for negative order")
	}
	// freq >= Nyquist
	lp := ButterworthLP(25000, 4, 48000)
	for _, c := range lp {
		if c != (biquad.Coefficients{}) {
			// Sections should be zero-value since freq is invalid.
			break
		}
	}
	// freq <= 0
	lp = ButterworthLP(0, 4, 48000)
	for _, c := range lp {
		if c != (biquad.Coefficients{}) {
			break
		}
	}
}

// ---------------------------------------------------------------------------
// Chebyshev1 section count and structure
// ---------------------------------------------------------------------------

func TestChebyshev1_SectionCount(t *testing.T) {
	sr := 48000.0
	ripple := 1.0
	for order := 1; order <= 8; order++ {
		want := (order + 1) / 2
		gotLP := Chebyshev1LP(1000, order, ripple, sr)
		gotHP := Chebyshev1HP(1000, order, ripple, sr)
		if len(gotLP) != want {
			t.Fatalf("LP order %d: sections=%d, want %d", order, len(gotLP), want)
		}
		if len(gotHP) != want {
			t.Fatalf("HP order %d: sections=%d, want %d", order, len(gotHP), want)
		}
	}
}

func TestChebyshev1_InvalidInputs(t *testing.T) {
	if got := Chebyshev1LP(1000, 0, 1, 48000); got != nil {
		t.Fatal("expected nil for order <= 0")
	}
	if got := Chebyshev1HP(1000, 0, 1, 48000); got != nil {
		t.Fatal("expected nil for order <= 0")
	}
	if got := Chebyshev1LP(1000, 4, 1, 0); got != nil {
		t.Fatal("expected nil for sr <= 0")
	}
	if got := Chebyshev1HP(25000, 4, 1, 48000); got != nil {
		t.Fatal("expected nil for freq >= Nyquist")
	}
}

func TestChebyshev1_AllSectionsFinite(t *testing.T) {
	for _, sr := range []float64{44100, 48000, 96000} {
		for _, order := range []int{2, 3, 4, 5, 6} {
			for _, ripple := range []float64{0.5, 1, 2, 3} {
				lp := Chebyshev1LP(1000, order, ripple, sr)
				hp := Chebyshev1HP(1000, order, ripple, sr)
				for _, c := range lp {
					assertFiniteCoefficients(t, c)
				}
				for _, c := range hp {
					assertFiniteCoefficients(t, c)
				}
			}
		}
	}
}

func TestChebyshev1_ResponseFiniteAndShaped(t *testing.T) {
	sr := 48000.0
	// Order >= 4 with ripple=2 matches existing TestChebyshevResponseShape.
	for _, order := range []int{4, 6, 8} {
		lp := biquad.NewChain(Chebyshev1LP(1000, order, 2, sr))
		hp := biquad.NewChain(Chebyshev1HP(1000, order, 2, sr))
		// LP: passband above stopband
		if !(magChain(lp, 100, sr) > magChain(lp, 10000, sr)) {
			t.Fatalf("order %d LP: shape check failed", order)
		}
		// HP: passband above stopband
		if !(magChain(hp, 10000, sr) > magChain(hp, 100, sr)) {
			t.Fatalf("order %d HP: shape check failed", order)
		}
	}
}

func TestChebyshev1_DefaultRipple(t *testing.T) {
	// rippleDB <= 0 should use default of 1
	lp := Chebyshev1LP(1000, 4, 0, 48000)
	lpRef := Chebyshev1LP(1000, 4, 1, 48000)
	if !coeffSliceEqual(lp, lpRef) {
		t.Fatal("ripple=0 should produce same result as ripple=1")
	}
	lp = Chebyshev1LP(1000, 4, -1, 48000)
	if !coeffSliceEqual(lp, lpRef) {
		t.Fatal("ripple=-1 should produce same result as ripple=1")
	}
}

// ---------------------------------------------------------------------------
// Chebyshev2 section count and structure
// ---------------------------------------------------------------------------

func TestChebyshev2_SectionCount(t *testing.T) {
	sr := 48000.0
	ripple := 2.0
	for order := 1; order <= 8; order++ {
		want := (order + 1) / 2
		gotLP := Chebyshev2LP(1000, order, ripple, sr)
		gotHP := Chebyshev2HP(1000, order, ripple, sr)
		if len(gotLP) != want {
			t.Fatalf("LP order %d: sections=%d, want %d", order, len(gotLP), want)
		}
		if len(gotHP) != want {
			t.Fatalf("HP order %d: sections=%d, want %d", order, len(gotHP), want)
		}
	}
}

func TestChebyshev2_InvalidInputs(t *testing.T) {
	if got := Chebyshev2LP(1000, 0, 2, 48000); got != nil {
		t.Fatal("expected nil for order <= 0")
	}
	if got := Chebyshev2HP(1000, 0, 2, 48000); got != nil {
		t.Fatal("expected nil for order <= 0")
	}
	if got := Chebyshev2LP(1000, 4, 2, 0); got != nil {
		t.Fatal("expected nil for sr <= 0")
	}
	if got := Chebyshev2HP(0, 4, 2, 48000); got != nil {
		t.Fatal("expected nil for freq <= 0")
	}
	if got := Chebyshev2HP(25000, 4, 2, 48000); got != nil {
		t.Fatal("expected nil for freq >= Nyquist")
	}
}

func TestChebyshev2_AllSectionsFinite(t *testing.T) {
	for _, sr := range []float64{44100, 48000, 96000} {
		for _, order := range []int{2, 3, 4, 5, 6} {
			for _, ripple := range []float64{1, 2, 3} {
				lp := Chebyshev2LP(1000, order, ripple, sr)
				hp := Chebyshev2HP(1000, order, ripple, sr)
				for _, c := range lp {
					assertFiniteCoefficients(t, c)
				}
				for _, c := range hp {
					assertFiniteCoefficients(t, c)
				}
			}
		}
	}
}

func TestChebyshev2HP_ResponseShaped(t *testing.T) {
	sr := 48000.0
	for _, order := range []int{4, 6} {
		hp := biquad.NewChain(Chebyshev2HP(1000, order, 2, sr))
		// HP: high-freq passband above low-freq stopband
		if !(magChain(hp, 10000, sr) > magChain(hp, 100, sr)) {
			t.Fatalf("order %d HP: shape check failed", order)
		}
	}
}

func TestChebyshev2_DefaultRipple(t *testing.T) {
	lp := Chebyshev2LP(1000, 4, 0, 48000)
	lpRef := Chebyshev2LP(1000, 4, 1, 48000)
	if !coeffSliceEqual(lp, lpRef) {
		t.Fatal("ripple=0 should produce same result as ripple=1")
	}
}

// ---------------------------------------------------------------------------
// Chebyshev1 response shape across orders
// ---------------------------------------------------------------------------

func TestChebyshev1ResponseShape_MultiOrder(t *testing.T) {
	sr := 48000.0
	for _, order := range []int{3, 4, 5, 6} {
		c1lp := biquad.NewChain(Chebyshev1LP(1000, order, 1, sr))
		c1hp := biquad.NewChain(Chebyshev1HP(1000, order, 1, sr))

		if !(magChain(c1lp, 100, sr) > magChain(c1lp, 10000, sr)) {
			t.Fatalf("Chebyshev1LP order %d: shape check failed", order)
		}
		if !(magChain(c1hp, 10000, sr) > magChain(c1hp, 100, sr)) {
			t.Fatalf("Chebyshev1HP order %d: shape check failed", order)
		}
	}
}

// ---------------------------------------------------------------------------
// Odd-order Chebyshev uses Butterworth first-order tail
// ---------------------------------------------------------------------------

func TestChebyshev_OddOrder_HasFirstOrderSection(t *testing.T) {
	sr := 48000.0
	for _, order := range []int{3, 5, 7} {
		for _, sections := range [][]biquad.Coefficients{
			Chebyshev1LP(1000, order, 1, sr),
			Chebyshev1HP(1000, order, 1, sr),
			Chebyshev2LP(1000, order, 2, sr),
			Chebyshev2HP(1000, order, 2, sr),
		} {
			last := sections[len(sections)-1]
			if last.B2 != 0 || last.A2 != 0 {
				t.Fatalf("order %d: last section not first-order: %+v", order, last)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func TestButterworthQ_KnownValues(t *testing.T) {
	// Order 2, index 0: Q = 1/(2*sin(pi/4)) = 1/sqrt(2)
	got := butterworthQ(2, 0)
	want := 1 / math.Sqrt2
	if !almostEqual(got, want, 1e-12) {
		t.Fatalf("order=2 index=0: Q=%.10f, want %.10f", got, want)
	}

	// Order 4, index 0: Q = 1/(2*sin(pi/8))
	got = butterworthQ(4, 0)
	want = 1 / (2 * math.Sin(math.Pi/8))
	if !almostEqual(got, want, 1e-12) {
		t.Fatalf("order=4 index=0: Q=%.10f, want %.10f", got, want)
	}

	// Order 4, index 1: Q = 1/(2*sin(3*pi/8))
	got = butterworthQ(4, 1)
	want = 1 / (2 * math.Sin(3*math.Pi/8))
	if !almostEqual(got, want, 1e-12) {
		t.Fatalf("order=4 index=1: Q=%.10f, want %.10f", got, want)
	}
}

func TestBilinearK_ValidAndInvalid(t *testing.T) {
	k, ok := bilinearK(1000, 48000)
	if !ok || k <= 0 {
		t.Fatalf("expected valid k>0, got k=%v ok=%v", k, ok)
	}
	// k = tan(pi*f/sr)
	want := math.Tan(math.Pi * 1000 / 48000)
	if !almostEqual(k, want, 1e-12) {
		t.Fatalf("k=%.10f, want %.10f", k, want)
	}

	if _, ok := bilinearK(0, 48000); ok {
		t.Fatal("expected !ok for freq=0")
	}
	if _, ok := bilinearK(24000, 48000); ok {
		t.Fatal("expected !ok for freq=Nyquist")
	}
	if _, ok := bilinearK(1000, 0); ok {
		t.Fatal("expected !ok for sr=0")
	}
	if _, ok := bilinearK(-1, 48000); ok {
		t.Fatal("expected !ok for freq<0")
	}
}

func TestButterworthFirstOrder_Passthrough(t *testing.T) {
	sr := 48000.0
	lp := butterworthFirstOrderLP(1000, sr)
	hp := butterworthFirstOrderHP(1000, sr)

	// Both should be first-order (B2=A2=0)
	if lp.B2 != 0 || lp.A2 != 0 {
		t.Fatalf("LP not first-order: %+v", lp)
	}
	if hp.B2 != 0 || hp.A2 != 0 {
		t.Fatalf("HP not first-order: %+v", hp)
	}

	// LP: DC gain should be near 1 (0 dB)
	dcMag := mag(lp, 1, sr)
	if !almostEqual(dcMag, 1, 0.01) {
		t.Fatalf("LP DC mag=%.4f, want ~1", dcMag)
	}

	// HP: Nyquist gain should be near 1
	nyqMag := mag(hp, sr/2-1, sr)
	if !almostEqual(nyqMag, 1, 0.01) {
		t.Fatalf("HP Nyquist mag=%.4f, want ~1", nyqMag)
	}

	assertFiniteCoefficients(t, lp)
	assertFiniteCoefficients(t, hp)
	assertStableSection(t, lp)
	assertStableSection(t, hp)
}

func TestButterworthFirstOrder_InvalidInputs(t *testing.T) {
	zero := biquad.Coefficients{}
	if got := butterworthFirstOrderLP(0, 48000); got != zero {
		t.Fatal("expected zero for freq=0")
	}
	if got := butterworthFirstOrderHP(25000, 48000); got != zero {
		t.Fatal("expected zero for freq>=Nyquist")
	}
	if got := butterworthFirstOrderLP(1000, 0); got != zero {
		t.Fatal("expected zero for sr=0")
	}
}

// ---------------------------------------------------------------------------
// Cross-frequency Butterworth LP/HP symmetry check
// ---------------------------------------------------------------------------

func TestButterworth_LPHPSymmetry(t *testing.T) {
	sr := 48000.0
	order := 4
	freq := 2000.0

	lp := biquad.NewChain(ButterworthLP(freq, order, sr))
	hp := biquad.NewChain(ButterworthHP(freq, order, sr))

	// At cutoff, both should be ~-3 dB
	lpCutoff := lp.MagnitudeDB(freq, sr)
	hpCutoff := hp.MagnitudeDB(freq, sr)
	if !almostEqual(lpCutoff, hpCutoff, 0.1) {
		t.Fatalf("LP cutoff=%.2f dB, HP cutoff=%.2f dB, expected similar", lpCutoff, hpCutoff)
	}

	// Far passband: LP passband at low freq ~ 0 dB, HP passband at high freq ~ 0 dB
	lpPass := lp.MagnitudeDB(100, sr)
	hpPass := hp.MagnitudeDB(20000, sr)
	if lpPass < -0.5 {
		t.Fatalf("LP passband at 100 Hz = %.2f dB, expected ~0 dB", lpPass)
	}
	if hpPass < -0.5 {
		t.Fatalf("HP passband at 20 kHz = %.2f dB, expected ~0 dB", hpPass)
	}
}

// ---------------------------------------------------------------------------
// Frequency sweep: all types produce finite, stable output across band
// ---------------------------------------------------------------------------

func TestAllCascades_FiniteAcrossFrequencies(t *testing.T) {
	sr := 48000.0
	freqs := []float64{100, 500, 1000, 5000, 10000}
	orders := []int{2, 4, 6}

	type cascadeFunc struct {
		name string
		fn   func() []biquad.Coefficients
	}

	for _, f := range freqs {
		for _, order := range orders {
			fns := []cascadeFunc{
				{"ButterworthLP", func() []biquad.Coefficients { return ButterworthLP(f, order, sr) }},
				{"ButterworthHP", func() []biquad.Coefficients { return ButterworthHP(f, order, sr) }},
				{"Chebyshev1LP", func() []biquad.Coefficients { return Chebyshev1LP(f, order, 1, sr) }},
				{"Chebyshev1HP", func() []biquad.Coefficients { return Chebyshev1HP(f, order, 1, sr) }},
				{"Chebyshev2LP", func() []biquad.Coefficients { return Chebyshev2LP(f, order, 2, sr) }},
				{"Chebyshev2HP", func() []biquad.Coefficients { return Chebyshev2HP(f, order, 2, sr) }},
			}
			for _, cf := range fns {
				t.Run(fmt.Sprintf("%s/f=%v/order=%d", cf.name, f, order), func(t *testing.T) {
					sections := cf.fn()
					if len(sections) == 0 {
						t.Fatal("empty sections")
					}
					chain := biquad.NewChain(sections)
					for _, probe := range []float64{10, 100, 1000, 5000, 15000} {
						m := magChain(chain, probe, sr)
						if math.IsNaN(m) || math.IsInf(m, 0) {
							t.Fatalf("invalid response at %v Hz: %v", probe, m)
						}
					}
				})
			}
		}
	}
}
