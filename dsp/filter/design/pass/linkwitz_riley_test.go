package pass

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

func TestLinkwitzRileyLP_Basic(t *testing.T) {
	sr := 48000.0
	tests := []struct {
		order    int
		sections int
	}{
		{2, 2},  // LR2: two 1st-order Butterworth filters
		{3, 2},  // LR3: 1st + 2nd-order Butterworth
		{4, 2},  // LR4: two 2nd-order Butterworth filters
		{5, 3},  // LR5: 2nd + 3rd-order Butterworth
		{8, 4},  // LR8: two 4th-order Butterworth filters
		{12, 6}, // LR12: two 6th-order Butterworth filters
	}
	for _, tt := range tests {
		sections := LinkwitzRileyLP(1000, tt.order, sr)
		if len(sections) != tt.sections {
			t.Errorf("LR%d LP: expected %d sections, got %d", tt.order, tt.sections, len(sections))
			continue
		}
		for i, s := range sections {
			assertFiniteCoefficients(t, s)
			// First-order sections (B2=A2=0) don't have standard pole pairs.
			if s.B2 != 0 || s.A2 != 0 {
				assertStableSection(t, s)
			}
			_ = i
		}
	}
}

func TestLinkwitzRileyHP_Basic(t *testing.T) {
	sr := 48000.0
	tests := []struct {
		order    int
		sections int
	}{
		{2, 2},
		{3, 2},
		{4, 2},
		{5, 3},
		{8, 4},
		{12, 6},
	}
	for _, tt := range tests {
		sections := LinkwitzRileyHP(1000, tt.order, sr)
		if len(sections) != tt.sections {
			t.Errorf("LR%d HP: expected %d sections, got %d", tt.order, tt.sections, len(sections))
			continue
		}
		for _, s := range sections {
			assertFiniteCoefficients(t, s)
			if s.B2 != 0 || s.A2 != 0 {
				assertStableSection(t, s)
			}
		}
	}
}

func TestLinkwitzRileyLP_InvalidOrder(t *testing.T) {
	sr := 48000.0
	invalid := []int{0, -1, 1}
	for _, order := range invalid {
		if got := LinkwitzRileyLP(1000, order, sr); got != nil {
			t.Errorf("LR LP order %d: expected nil, got %d sections", order, len(got))
		}
	}
}

func TestLinkwitzRileyHP_InvalidOrder(t *testing.T) {
	sr := 48000.0
	invalid := []int{0, -1, 1}
	for _, order := range invalid {
		if got := LinkwitzRileyHP(1000, order, sr); got != nil {
			t.Errorf("LR HP order %d: expected nil, got %d sections", order, len(got))
		}
	}
}

func TestLinkwitzRileyLP_InvalidFrequency(t *testing.T) {
	sr := 48000.0
	invalid := []float64{0, -100, sr / 2, sr}
	for _, freq := range invalid {
		if got := LinkwitzRileyLP(freq, 4, sr); got != nil {
			t.Errorf("LR LP freq %v: expected nil, got %d sections", freq, len(got))
		}
	}
}

func TestLinkwitzRileyHP_InvalidFrequency(t *testing.T) {
	sr := 48000.0
	invalid := []float64{0, -100, sr / 2, sr}
	for _, freq := range invalid {
		if got := LinkwitzRileyHP(freq, 4, sr); got != nil {
			t.Errorf("LR HP freq %v: expected nil, got %d sections", freq, len(got))
		}
	}
}

func TestLinkwitzRileyLP_InvalidSampleRate(t *testing.T) {
	invalid := []float64{0, -48000}
	for _, sr := range invalid {
		if got := LinkwitzRileyLP(1000, 4, sr); got != nil {
			t.Errorf("LR LP sr %v: expected nil, got %d sections", sr, len(got))
		}
	}
}

// TestLinkwitzRiley_CrossoverMagnitude verifies -6.02 dB at the crossover frequency.
func TestLinkwitzRiley_CrossoverMagnitude(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	expectedDB := -6.02 // Linkwitz-Riley: -6 dB at crossover
	tolerance := 0.05   // dB

	orders := []int{2, 3, 4, 5, 8, 12, 16}
	for _, order := range orders {
		lpSections := LinkwitzRileyLP(fc, order, sr)
		hpSections := LinkwitzRileyHP(fc, order, sr)

		lpMag := cascadeMagDB(lpSections, fc, sr)
		hpMag := cascadeMagDB(hpSections, fc, sr)

		if math.Abs(lpMag-expectedDB) > tolerance {
			t.Errorf("LR%d LP at crossover: %.3f dB, want %.2f ±%.2f dB", order, lpMag, expectedDB, tolerance)
		}
		if math.Abs(hpMag-expectedDB) > tolerance {
			t.Errorf("LR%d HP at crossover: %.3f dB, want %.2f ±%.2f dB", order, hpMag, expectedDB, tolerance)
		}
	}
}

// TestLinkwitzRiley_AllpassSum verifies LP + HP = allpass (flat magnitude)
// when using the correct polarity (inverted HP for orders ≡ 2 mod 4).
func TestLinkwitzRiley_AllpassSum(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	tolerance := 0.05 // dB

	orders := []int{2, 4, 6, 8, 12}
	for _, order := range orders {
		lpSections := LinkwitzRileyLP(fc, order, sr)
		var hpSections []biquad.Coefficients
		if LinkwitzRileyNeedsHPInvert(order) {
			hpSections = LinkwitzRileyHPInverted(fc, order, sr)
		} else {
			hpSections = LinkwitzRileyHP(fc, order, sr)
		}

		lpChain := biquad.NewChain(lpSections)
		hpChain := biquad.NewChain(hpSections)

		// Check sum magnitude at several frequencies.
		freqs := []float64{20, 100, 500, fc, 2000, 5000, 10000, 20000}
		for _, f := range freqs {
			if f >= sr/2 {
				continue
			}
			lpH := lpChain.Response(f, sr)
			hpH := hpChain.Response(f, sr)
			sumMag := 20 * math.Log10(cmplxAbs(lpH+hpH))

			if math.Abs(sumMag) > tolerance {
				t.Errorf("LR%d sum at %.0f Hz: %.4f dB (want 0 ±%.2f dB)", order, f, sumMag, tolerance)
			}
		}
	}
}

// TestLinkwitzRiley_NeedsHPInvert validates the polarity detection helper.
func TestLinkwitzRiley_NeedsHPInvert(t *testing.T) {
	tests := []struct {
		order int
		want  bool
	}{
		{0, false},
		{1, false},
		{2, true},   // LR2: half-order 1 is odd → needs invert
		{3, false},  // Odd order: simple polarity flip is insufficient
		{4, false},  // LR4: half-order 2 is even → no invert
		{5, false},  // Odd order: simple polarity flip is insufficient
		{6, true},   // LR6: half-order 3 is odd → needs invert
		{8, false},  // LR8: half-order 4 is even → no invert
		{10, true},  // LR10: half-order 5 is odd → needs invert
		{12, false}, // LR12: half-order 6 is even → no invert
	}
	for _, tt := range tests {
		got := LinkwitzRileyNeedsHPInvert(tt.order)
		if got != tt.want {
			t.Errorf("NeedsHPInvert(%d) = %v, want %v", tt.order, got, tt.want)
		}
	}
}

// TestLinkwitzRiley_FamilySignature validates Butterworth-squared passband flatness
// and monotonic stopband for the lowpass.
func TestLinkwitzRiley_FamilySignature(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	sections := LinkwitzRileyLP(fc, 4, sr)

	passband := measureBandSignature(sections, 10, 0.3*fc, 10, sr)
	stop := measureBandSignature(sections, 2*fc, 0.45*sr, 100, sr)

	if passband.spanDB > 0.1 {
		t.Fatalf("LR4 LP passband should be very flat: span=%.3f dB", passband.spanDB)
	}
	if passband.extrema > 0 {
		t.Fatalf("LR4 LP passband should be monotonic: extrema=%d", passband.extrema)
	}
	if stop.extrema != 0 {
		t.Fatalf("LR4 LP stopband should be monotonic: extrema=%d", stop.extrema)
	}
}

// TestLinkwitzRiley_DoubledSections verifies that even orders are exactly doubled Butterworth.
func TestLinkwitzRiley_DoubledSections(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	order := 8

	bwLP := ButterworthLP(fc, order/2, sr)
	lrLP := LinkwitzRileyLP(fc, order, sr)

	if len(lrLP) != 2*len(bwLP) {
		t.Fatalf("LR%d LP: expected %d sections, got %d", order, 2*len(bwLP), len(lrLP))
	}
	for i, bwCoeff := range bwLP {
		// First half should match.
		lrCoeff := lrLP[i]
		if !coeffEqual(bwCoeff, lrCoeff) {
			t.Errorf("section %d: Butterworth %+v != LR first half %+v", i, bwCoeff, lrCoeff)
		}
		// Second half should also match.
		lrCoeff2 := lrLP[len(bwLP)+i]
		if !coeffEqual(bwCoeff, lrCoeff2) {
			t.Errorf("section %d: Butterworth %+v != LR second half %+v", i, bwCoeff, lrCoeff2)
		}
	}
}

// TestLinkwitzRiley_OddOrderSections verifies odd orders are built from
// adjacent Butterworth orders.
func TestLinkwitzRiley_OddOrderSections(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	order := 5

	bwLow := ButterworthLP(fc, order/2, sr)
	bwHigh := ButterworthLP(fc, (order+1)/2, sr)
	lrLP := LinkwitzRileyLP(fc, order, sr)

	if len(lrLP) != len(bwLow)+len(bwHigh) {
		t.Fatalf("LR%d LP: expected %d sections, got %d", order, len(bwLow)+len(bwHigh), len(lrLP))
	}
	for i, bwCoeff := range bwLow {
		if !coeffEqual(bwCoeff, lrLP[i]) {
			t.Errorf("section %d: Butterworth-low %+v != LR %+v", i, bwCoeff, lrLP[i])
		}
	}
	for i, bwCoeff := range bwHigh {
		j := len(bwLow) + i
		if !coeffEqual(bwCoeff, lrLP[j]) {
			t.Errorf("section %d: Butterworth-high %+v != LR %+v", j, bwCoeff, lrLP[j])
		}
	}
}

// TestLinkwitzRiley_OddOrderSumNotAllpass verifies odd-order LP/HP pairs do
// not form an exact allpass response via polarity inversion alone.
func TestLinkwitzRiley_OddOrderSumNotAllpass(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	for _, order := range []int{3, 5, 7} {
		lp := biquad.NewChain(LinkwitzRileyLP(fc, order, sr))
		hp := biquad.NewChain(LinkwitzRileyHP(fc, order, sr))
		hpInv := biquad.NewChain(LinkwitzRileyHPInverted(fc, order, sr))

		lpH := lp.Response(fc, sr)
		sum := 20 * math.Log10(cmplxAbs(lpH+hp.Response(fc, sr)))
		sumInv := 20 * math.Log10(cmplxAbs(lpH+hpInv.Response(fc, sr)))

		if math.Abs(sum) < 0.5 {
			t.Errorf("LR%d odd-order sum unexpectedly near allpass at crossover: %.3f dB", order, sum)
		}
		if math.Abs(sumInv) < 0.5 {
			t.Errorf("LR%d odd-order inverted sum unexpectedly near allpass at crossover: %.3f dB", order, sumInv)
		}
	}
}

// TestLinkwitzRiley_HighOrders verifies that very high orders work.
func TestLinkwitzRiley_HighOrders(t *testing.T) {
	sr := 48000.0
	fc := 1000.0

	for _, order := range []int{20, 21, 24, 33, 48} {
		lp := LinkwitzRileyLP(fc, order, sr)
		hp := LinkwitzRileyHP(fc, order, sr)
		if lp == nil {
			t.Errorf("LR%d LP: got nil", order)
			continue
		}
		if hp == nil {
			t.Errorf("LR%d HP: got nil", order)
			continue
		}

		lowOrder := order / 2
		highOrder := (order + 1) / 2
		expectedSections := len(ButterworthLP(fc, lowOrder, sr)) + len(ButterworthLP(fc, highOrder, sr))
		if len(lp) != expectedSections {
			t.Errorf("LR%d LP: expected %d sections, got %d", order, expectedSections, len(lp))
		}

		// Verify crossover magnitude.
		lpMag := cascadeMagDB(lp, fc, sr)
		hpMag := cascadeMagDB(hp, fc, sr)
		if math.Abs(lpMag-(-6.02)) > 0.1 {
			t.Errorf("LR%d LP at crossover: %.3f dB, want -6.02 dB", order, lpMag)
		}
		if math.Abs(hpMag-(-6.02)) > 0.1 {
			t.Errorf("LR%d HP at crossover: %.3f dB, want -6.02 dB", order, hpMag)
		}
	}
}

// TestLinkwitzRileyHPInverted_Polarity verifies inverted HP negates B coefficients.
func TestLinkwitzRileyHPInverted_Polarity(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	order := 2

	hp := LinkwitzRileyHP(fc, order, sr)
	hpInv := LinkwitzRileyHPInverted(fc, order, sr)

	if len(hp) != len(hpInv) {
		t.Fatalf("section count mismatch: %d vs %d", len(hp), len(hpInv))
	}

	// First section should have negated B coefficients.
	if math.Abs(hp[0].B0+hpInv[0].B0) > 1e-15 {
		t.Errorf("B0: %v vs %v (should be negated)", hp[0].B0, hpInv[0].B0)
	}
	if math.Abs(hp[0].B1+hpInv[0].B1) > 1e-15 {
		t.Errorf("B1: %v vs %v (should be negated)", hp[0].B1, hpInv[0].B1)
	}

	// A coefficients should be identical.
	if math.Abs(hp[0].A1-hpInv[0].A1) > 1e-15 {
		t.Errorf("A1 should be identical: %v vs %v", hp[0].A1, hpInv[0].A1)
	}

	// Remaining sections should be unchanged.
	for i := 1; i < len(hp); i++ {
		if !coeffEqual(hp[i], hpInv[i]) {
			t.Errorf("section %d should be unchanged", i)
		}
	}
}

// helpers

func cmplxAbs(c complex128) float64 {
	return math.Sqrt(real(c)*real(c) + imag(c)*imag(c))
}

func coeffEqual(a, b biquad.Coefficients) bool {
	const eps = 1e-15
	return math.Abs(a.B0-b.B0) < eps &&
		math.Abs(a.B1-b.B1) < eps &&
		math.Abs(a.B2-b.B2) < eps &&
		math.Abs(a.A1-b.A1) < eps &&
		math.Abs(a.A2-b.A2) < eps
}
