package shelving

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// ============================================================
// Chebyshev Type II shelving filter tests
// ============================================================

func TestChebyshev2LowShelf_InvalidParams(t *testing.T) {
	tests := []struct {
		name             string
		sr, freq, gainDB float64
		rippleDB         float64
		order            int
	}{
		{"zero sample rate", 0, 1000, 6, 0.5, 2},
		{"negative freq", 48000, -1, 6, 0.5, 2},
		{"freq at Nyquist", 48000, 24000, 6, 0.5, 2},
		{"zero order", 48000, 1000, 6, 0.5, 0},
		{"zero ripple", 48000, 1000, 6, 0, 2},
		{"negative ripple", 48000, 1000, 6, -1, 2},
		{"ripple >= boost", 48000, 1000, 1, 1.0, 4},
		{"ripple > boost", 48000, 1000, 1, 1.5, 4},
		{"ripple >= cut magnitude", 48000, 1000, -1, 1.0, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Chebyshev2LowShelf(tt.sr, tt.freq, tt.gainDB, tt.rippleDB, tt.order)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestChebyshev2HighShelf_InvalidParams(t *testing.T) {
	_, err := Chebyshev2HighShelf(0, 1000, 6, 0.5, 2)
	if err == nil {
		t.Error("expected error for zero sample rate")
	}
	_, err = Chebyshev2HighShelf(48000, 1000, 6, 0, 2)
	if err == nil {
		t.Error("expected error for zero ripple")
	}
	_, err = Chebyshev2HighShelf(48000, 1000, 1, 1.0, 4)
	if err == nil {
		t.Error("expected error for ripple >= gain magnitude")
	}
}

// ============================================================
// Chebyshev Type II: passthrough at zero gain
// ============================================================

func TestChebyshev2LowShelf_ZeroGain(t *testing.T) {
	sections, err := Chebyshev2LowShelf(testSR, 1000, 0, 0.5, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 {
		t.Fatalf("expected 1 passthrough section, got %d", len(sections))
	}
	mag := cascadeMagnitudeDB(sections, 1000, testSR)
	if !almostEqual(mag, 0, 1e-10) {
		t.Errorf("zero gain: mag = %v dB, expected 0", mag)
	}
}

// ============================================================
// Chebyshev Type II: section count (same as Butterworth/Cheby1)
// ============================================================

func TestChebyshev2LowShelf_SectionCount(t *testing.T) {
	for _, M := range []int{1, 2, 3, 4, 5, 6, 7, 8} {
		t.Run(orderName(M), func(t *testing.T) {
			sections, err := Chebyshev2LowShelf(testSR, 1000, 6, 0.5, M)
			if err != nil {
				t.Fatal(err)
			}
			expected := (M + 1) / 2
			if len(sections) != expected {
				t.Errorf("order %d: got %d sections, expected %d", M, len(sections), expected)
			}
		})
	}
}

// ============================================================
// Chebyshev Type II: DC and Nyquist gain accuracy
// ============================================================

func TestChebyshev2LowShelf_DCGain(t *testing.T) {
	for _, gainDB := range []float64{-12, -6, 6, 12, 20} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			sections, err := Chebyshev2LowShelf(testSR, 1000, gainDB, 0.5, 4)
			if err != nil {
				t.Fatal(err)
			}
			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			if !almostEqual(dcMag, gainDB, 0.2) {
				t.Errorf("DC gain = %.4f dB, expected %.4f dB", dcMag, gainDB)
			}
		})
	}
}

func TestChebyshev2LowShelf_NyquistGain(t *testing.T) {
	// For this implementation, after DC normalization the far-stopband anchor
	// is shifted by approximately gainDB-rippleDB.
	for _, gainDB := range []float64{-12, -6, 6, 12} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			rippleDB := 0.5
			sections, err := Chebyshev2LowShelf(testSR, 1000, gainDB, rippleDB, 4)
			if err != nil {
				t.Fatal(err)
			}
			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
			expected := gainDB - math.Copysign(rippleDB, gainDB)
			if !almostEqual(nyqMag, expected, 0.2) {
				t.Errorf("Nyquist gain = %.4f dB, expected ~%.4f dB (ripple=%.1f)", nyqMag, expected, rippleDB)
			}
		})
	}
}

func TestChebyshev2HighShelf_NyquistGain(t *testing.T) {
	for _, gainDB := range []float64{-12, -6, 6, 12, 20} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			sections, err := Chebyshev2HighShelf(testSR, 1000, gainDB, 0.5, 4)
			if err != nil {
				t.Fatal(err)
			}
			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
			if !almostEqual(nyqMag, gainDB, 0.3) {
				t.Errorf("Nyquist gain = %.4f dB, expected %.4f dB", nyqMag, gainDB)
			}
		})
	}
}

func TestChebyshev2HighShelf_DCGain(t *testing.T) {
	// For this implementation, after Nyquist normalization the far-stopband
	// anchor is shifted by approximately gainDB-rippleDB.
	for _, gainDB := range []float64{-12, -6, 6, 12} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			rippleDB := 0.5
			sections, err := Chebyshev2HighShelf(testSR, 1000, gainDB, rippleDB, 4)
			if err != nil {
				t.Fatal(err)
			}
			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			expected := gainDB - math.Copysign(rippleDB, gainDB)
			if !almostEqual(dcMag, expected, 0.2) {
				t.Errorf("DC gain = %.4f dB, expected ~%.4f dB (ripple=%.1f)", dcMag, expected, rippleDB)
			}
		})
	}
}

// ============================================================
// Chebyshev Type II: endpoint anchor model verification
// ============================================================

// The current realization anchors one shelf endpoint exactly and shifts the
// opposite endpoint by approximately gainDB-sign(gainDB)*rippleDB.
func TestChebyshev2LowShelf_StopbandAnchorModel(t *testing.T) {
	testCases := []struct {
		name     string
		gainDB   float64
		rippleDB float64
		order    int
	}{
		{"boost_6dB_ripple_0.5", 6, 0.5, 4},
		{"boost_12dB_ripple_0.5", 12, 0.5, 4},
		{"boost_18dB_ripple_0.5", 18, 0.5, 4},
		{"boost_24dB_ripple_0.5", 24, 0.5, 4},
		{"cut_6dB_ripple_0.5", -6, 0.5, 4},
		{"cut_12dB_ripple_0.5", -12, 0.5, 4},
		{"boost_12dB_ripple_0.1", 12, 0.1, 4},
		{"boost_12dB_ripple_0.25", 12, 0.25, 4},
		{"boost_12dB_ripple_1.0", 12, 1.0, 4},
		{"boost_12dB_ripple_2.0", 12, 2.0, 4},
		{"boost_12dB_ripple_3.0", 12, 3.0, 4},
		{"boost_12dB_order_2", 12, 0.5, 2},
		{"boost_12dB_order_6", 12, 0.5, 6},
		{"boost_12dB_order_8", 12, 0.5, 8},
		{"boost_12dB_order_10", 12, 0.5, 10},
		{"small_boost_3dB", 3, 0.5, 4},
		{"large_boost_30dB", 30, 0.5, 4},
		{"small_ripple_0.05", 12, 0.05, 4},
		{"large_ripple_5.0", 12, 5.0, 4},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sections, err := Chebyshev2LowShelf(testSR, 1000, tc.gainDB, tc.rippleDB, tc.order)
			if err != nil {
				t.Fatal(err)
			}

			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
			farStopband := cascadeMagnitudeDB(sections, 10000, testSR)
			expectedStop := tc.gainDB - math.Copysign(tc.rippleDB, tc.gainDB)

			if !almostEqual(dcMag, tc.gainDB, 0.25) {
				t.Errorf("DC gain = %.4f dB, expected %.4f dB", dcMag, tc.gainDB)
			}
			if !almostEqual(nyqMag, expectedStop, 0.25) {
				t.Errorf("Nyquist gain = %.4f dB, expected %.4f dB", nyqMag, expectedStop)
			}
			if !almostEqual(farStopband, expectedStop, 0.25) {
				t.Errorf("far stopband gain = %.4f dB, expected %.4f dB", farStopband, expectedStop)
			}
		})
	}
}

func TestChebyshev2HighShelf_StopbandAnchorModel(t *testing.T) {
	testCases := []struct {
		name     string
		gainDB   float64
		rippleDB float64
		order    int
	}{
		{"boost_6dB_ripple_0.5", 6, 0.5, 4},
		{"boost_12dB_ripple_0.5", 12, 0.5, 4},
		{"boost_18dB_ripple_0.5", 18, 0.5, 4},
		{"cut_6dB_ripple_0.5", -6, 0.5, 4},
		{"cut_12dB_ripple_0.5", -12, 0.5, 4},
		{"boost_12dB_ripple_0.1", 12, 0.1, 4},
		{"boost_12dB_ripple_1.0", 12, 1.0, 4},
		{"boost_12dB_ripple_2.0", 12, 2.0, 4},
		{"boost_12dB_order_2", 12, 0.5, 2},
		{"boost_12dB_order_6", 12, 0.5, 6},
		{"boost_12dB_order_8", 12, 0.5, 8},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sections, err := Chebyshev2HighShelf(testSR, 1000, tc.gainDB, tc.rippleDB, tc.order)
			if err != nil {
				t.Fatal(err)
			}

			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			farStopband := cascadeMagnitudeDB(sections, 50, testSR)
			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
			expectedStop := tc.gainDB - math.Copysign(tc.rippleDB, tc.gainDB)

			if !almostEqual(nyqMag, tc.gainDB, 0.25) {
				t.Errorf("Nyquist gain = %.4f dB, expected %.4f dB", nyqMag, tc.gainDB)
			}
			if !almostEqual(dcMag, expectedStop, 0.25) {
				t.Errorf("DC gain = %.4f dB, expected %.4f dB", dcMag, expectedStop)
			}
			if !almostEqual(farStopband, expectedStop, 0.25) {
				t.Errorf("far stopband gain = %.4f dB, expected %.4f dB", farStopband, expectedStop)
			}
		})
	}
}

// ============================================================
// Chebyshev Type II: pole stability
// ============================================================

func TestChebyshev2LowShelf_Stability(t *testing.T) {
	for _, M := range []int{1, 2, 3, 4, 5, 6, 8, 10, 12} {
		t.Run(orderName(M), func(t *testing.T) {
			sections, err := Chebyshev2LowShelf(testSR, 1000, 12, 0.5, M)
			if err != nil {
				t.Fatal(err)
			}
			allPolesStable(t, sections)
		})
	}
}

func TestChebyshev2HighShelf_Stability(t *testing.T) {
	for _, M := range []int{1, 2, 3, 4, 5, 6, 8, 10, 12} {
		t.Run(orderName(M), func(t *testing.T) {
			sections, err := Chebyshev2HighShelf(testSR, 1000, 12, 0.5, M)
			if err != nil {
				t.Fatal(err)
			}
			allPolesStable(t, sections)
		})
	}
}

// ============================================================
// Chebyshev Type II: order sweep
// ============================================================

func TestChebyshev2LowShelf_VariousOrders(t *testing.T) {
	for _, M := range []int{1, 2, 3, 4, 5, 6, 8, 10, 12} {
		t.Run(orderName(M), func(t *testing.T) {
			rippleDB := 0.5
			sections, err := Chebyshev2LowShelf(testSR, 1000, 12, rippleDB, M)
			if err != nil {
				t.Fatal(err)
			}
			allPolesStable(t, sections)
			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			if !almostEqual(dcMag, 12, 0.2) {
				t.Errorf("M=%d: DC gain = %.4f dB, expected ~12 dB", M, dcMag)
			}
			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
			expected := 12.0 - rippleDB
			if M%2 == 1 {
				expected = 12.0
			}
			if !almostEqual(nyqMag, expected, 0.2) {
				t.Errorf("M=%d: Nyquist gain = %.4f dB, expected ~%.4f dB", M, nyqMag, expected)
			}
		})
	}
}

// ============================================================
// Chebyshev Type II: stopband ripple bounded by rippleDB
// ============================================================

func TestChebyshev2LowShelf_StopbandRipple(t *testing.T) {
	// For a low-shelf boost, the "stopband" (flat region) is the high-frequency
	// portion above the cutoff. The ripple there should be bounded by rippleDB.
	rippleDB := 0.5
	sections, err := Chebyshev2LowShelf(testSR, 1000, 12, rippleDB, 6)
	if err != nil {
		t.Fatal(err)
	}

	// Sample the flat region (well above cutoff).
	expected := 12.0 - rippleDB
	for f := 5000.0; f < testSR/2-100; f += 500 {
		mag := cascadeMagnitudeDB(sections, f, testSR)
		if math.Abs(mag-expected) > rippleDB+0.2 {
			t.Errorf("stopband at %.0f Hz: %.4f dB exceeds expected %.4f dB by > ±%.1f dB", f, mag, expected, rippleDB)
		}
	}
}

func TestChebyshev2HighShelf_StopbandRipple(t *testing.T) {
	// For a high-shelf boost, the stopband is the low-frequency portion below cutoff.
	rippleDB := 0.5
	sections, err := Chebyshev2HighShelf(testSR, 1000, 12, rippleDB, 6)
	if err != nil {
		t.Fatal(err)
	}

	expected := 12.0 - rippleDB
	for f := 10.0; f < 200; f += 10 {
		mag := cascadeMagnitudeDB(sections, f, testSR)
		if math.Abs(mag-expected) > rippleDB+0.2 {
			t.Errorf("stopband at %.0f Hz: %.4f dB exceeds expected %.4f dB by > ±%.1f dB", f, mag, expected, rippleDB)
		}
	}
}

// ============================================================
// Chebyshev Type II: extreme gains
// ============================================================

func TestChebyshev2LowShelf_ExtremeGains(t *testing.T) {
	for _, gainDB := range []float64{-30, -20, -6, -1, 1, 6, 20, 30} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			sections, err := Chebyshev2LowShelf(testSR, 1000, gainDB, 0.5, 4)
			if err != nil {
				t.Fatal(err)
			}
			allPolesStable(t, sections)
			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			if !almostEqual(dcMag, gainDB, 0.3) {
				t.Errorf("DC gain = %.4f dB, expected %.4f dB", dcMag, gainDB)
			}
		})
	}
}

// ============================================================
// Chebyshev Type II: various ripple values
// ============================================================

func TestChebyshev2LowShelf_VariousRipple(t *testing.T) {
	ripples := []float64{0.1, 0.25, 0.5, 1.0, 2.0, 3.0}
	for _, rip := range ripples {
		name := ftoa(rip) + "dBripple"
		t.Run(name, func(t *testing.T) {
			sections, err := Chebyshev2LowShelf(testSR, 1000, 12, rip, 6)
			if err != nil {
				t.Fatal(err)
			}
			allPolesStable(t, sections)
			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			if !almostEqual(dcMag, 12, 0.5) {
				t.Errorf("ripple=%.1f: DC gain = %.4f dB, expected ~12 dB", rip, dcMag)
			}
			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
			expected := 12.0 - rip
			if !almostEqual(nyqMag, expected, 0.2) {
				t.Errorf("ripple=%.1f: Nyquist gain = %.4f dB, expected ~%.4f dB", rip, nyqMag, expected)
			}
		})
	}
}

// ============================================================
// Chebyshev Type II: frequency sweep
// ============================================================

func TestChebyshev2LowShelf_VariousFrequencies(t *testing.T) {
	for _, freq := range []float64{100, 300, 500, 1000, 2000, 5000, 10000} {
		t.Run(freqName(freq), func(t *testing.T) {
			sections, err := Chebyshev2LowShelf(testSR, freq, 12, 0.5, 4)
			if err != nil {
				t.Fatal(err)
			}
			allPolesStable(t, sections)
			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			if !almostEqual(dcMag, 12, 0.2) {
				t.Errorf("freq=%v: DC gain = %.4f dB, expected ~12 dB", freq, dcMag)
			}
		})
	}
}

func TestChebyshev2HighShelf_VariousFrequencies(t *testing.T) {
	for _, freq := range []float64{100, 300, 500, 1000, 2000, 5000, 10000} {
		t.Run(freqName(freq), func(t *testing.T) {
			sections, err := Chebyshev2HighShelf(testSR, freq, 12, 0.5, 4)
			if err != nil {
				t.Fatal(err)
			}
			allPolesStable(t, sections)
			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
			if !almostEqual(nyqMag, 12, 0.3) {
				t.Errorf("freq=%v: Nyquist gain = %.4f dB, expected ~12 dB", freq, nyqMag)
			}
		})
	}
}

// ============================================================
// Chebyshev Type II: boost/cut inversion
// ============================================================

func TestChebyshev2LowShelf_BoostCutInversion(t *testing.T) {
	boost, err := Chebyshev2LowShelf(testSR, 1000, 12, 0.5, 6)
	if err != nil {
		t.Fatal(err)
	}
	cut, err := Chebyshev2LowShelf(testSR, 1000, -12, 0.5, 6)
	if err != nil {
		t.Fatal(err)
	}

	// DC and Nyquist: expect near-exact inversion.
	for _, freq := range []float64{1, testSR/2 - 1} {
		hBoost := cascadeResponse(boost, freq, testSR)
		hCut := cascadeResponse(cut, freq, testSR)
		hCombined := hBoost * hCut
		magDB := 20 * math.Log10(cmplx.Abs(hCombined))
		if math.Abs(magDB) > 0.5 {
			t.Errorf("freq=%v: boost*cut = %.4f dB, expected ~0 dB", freq, magDB)
		}
	}

	// Well away from transition: inversion is close.
	for _, freq := range []float64{50, 15000} {
		hBoost := cascadeResponse(boost, freq, testSR)
		hCut := cascadeResponse(cut, freq, testSR)
		hCombined := hBoost * hCut
		magDB := 20 * math.Log10(cmplx.Abs(hCombined))
		if math.Abs(magDB) > 1.0 {
			t.Errorf("freq=%v: boost*cut = %.4f dB, expected ~0 dB", freq, magDB)
		}
	}
}

func TestChebyshev2HighShelf_BoostCutInversion(t *testing.T) {
	boost, err := Chebyshev2HighShelf(testSR, 1000, 12, 0.5, 6)
	if err != nil {
		t.Fatal(err)
	}
	cut, err := Chebyshev2HighShelf(testSR, 1000, -12, 0.5, 6)
	if err != nil {
		t.Fatal(err)
	}

	for _, freq := range []float64{1, 50, 15000, testSR/2 - 1} {
		hBoost := cascadeResponse(boost, freq, testSR)
		hCut := cascadeResponse(cut, freq, testSR)
		hCombined := hBoost * hCut
		magDB := 20 * math.Log10(cmplx.Abs(hCombined))
		if math.Abs(magDB) > 1.0 {
			t.Errorf("freq=%v: boost*cut = %.4f dB, expected ~0 dB", freq, magDB)
		}
	}
}

// ============================================================
// Chebyshev Type II: monotonic shelf region
// ============================================================

func TestChebyshev2LowShelf_MonotonicShelfRegion(t *testing.T) {
	// Chebyshev II should be monotonic in the passband (shelf region).
	// For a boost low-shelf, magnitude should decrease monotonically
	// from DC toward the cutoff.
	sections, err := Chebyshev2LowShelf(testSR, 1000, 12, 0.5, 6)
	if err != nil {
		t.Fatal(err)
	}

	// Check monotonicity from DC up through the shelf region.
	prevMag := cascadeMagnitudeDB(sections, 1, testSR)
	for f := 10.0; f <= 800; f += 10 {
		mag := cascadeMagnitudeDB(sections, f, testSR)
		if mag > prevMag+0.12 {
			t.Errorf("non-monotonic at %.0f Hz: %.4f dB > %.4f dB (prev)", f, mag, prevMag)
			break
		}
		prevMag = mag
	}
}

// ============================================================
// Chebyshev Type II: flat stopband verification
// ============================================================

func TestChebyshev2_FlatStopband(t *testing.T) {
	// Chebyshev II should have a well-controlled stopband (flat region).
	// The maximum deviation in the stopband should be bounded by rippleDB.
	order := 6
	gainDB := 12.0
	rippleDB := 0.5

	sections, err := Chebyshev2LowShelf(testSR, 1000, gainDB, rippleDB, order)
	if err != nil {
		t.Fatal(err)
	}

	// In the far stopband (well above cutoff), magnitudes should stay close to
	// the implementation's stopband anchor.
	expected := gainDB - rippleDB
	maxDev := 0.0
	for f := 8000.0; f < testSR/2-100; f += 100 {
		mag := cascadeMagnitudeDB(sections, f, testSR)
		dev := math.Abs(mag - expected)
		if dev > maxDev {
			maxDev = dev
		}
	}
	if maxDev > rippleDB+0.2 {
		t.Errorf("stopband max deviation = %.4f dB from %.4f dB, exceeds bound %.1f dB", maxDev, expected, rippleDB)
	}
}

// ============================================================
// Chebyshev Type II: math verification tests
// ============================================================

// chebyshev2SectionsNoDCCorrection mirrors chebyshev2Sections but intentionally
// skips the final DC gain correction so we can test where endpoint behavior
// diverges.
func chebyshev2SectionsNoDCCorrection(K float64, gainDB, stopbandDB float64, order int) []biquad.Coefficients {
	G0 := 1.0
	G := db2Lin(gainDB)
	Gb := db2Lin(stopbandDB)
	g := math.Pow(G, 1.0/float64(order))

	e := math.Sqrt((G*G - Gb*Gb) / (Gb*Gb - G0*G0))
	eu := math.Pow(e+math.Sqrt(1+e*e), 1.0/float64(order))
	ew := math.Pow(G0*e+Gb*math.Sqrt(1.0+e*e), 1.0/float64(order))
	A := (eu - 1.0/eu) * 0.5
	B := (ew - g*g/ew) * 0.5

	L := order / 2
	hasFirstOrder := order%2 == 1
	n := L
	if hasFirstOrder {
		n++
	}
	sections := make([]biquad.Coefficients, 0, n)

	for m := 1; m <= L; m++ {
		theta := float64(2*m-1) / float64(2*order) * math.Pi
		si := math.Sin(theta)
		ci := math.Cos(theta)
		sp := sosParams{
			den: poleParams{sigma: A * si, r2: A*A + ci*ci},
			num: poleParams{sigma: B * si, r2: B*B + g*g*ci*ci},
		}
		sections = append(sections, bilinearSOS(K, sp))
	}

	if hasFirstOrder {
		sections = append(sections, bilinearFOS(K, fosParams{denSigma: A, numSigma: B}))
	}

	return sections
}

func TestChebyshev2Math_DCCorrectionScalesAllFrequencies(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	order := 6
	gainDB := 12.0
	rippleDB := 0.5
	K := math.Tan(math.Pi * fc / sr)

	stopbandDB := rippleDB
	raw := chebyshev2SectionsNoDCCorrection(K, gainDB, stopbandDB, order)
	corrected, err := chebyshev2Sections(K, gainDB, stopbandDB, order)
	if err != nil {
		t.Fatal(err)
	}

	rawDC := cmplx.Abs(cascadeResponse(raw, 1, sr))
	corrDC := cmplx.Abs(cascadeResponse(corrected, 1, sr))
	rawNyq := cmplx.Abs(cascadeResponse(raw, sr/2-1, sr))
	corrNyq := cmplx.Abs(cascadeResponse(corrected, sr/2-1, sr))

	dcScale := corrDC / rawDC
	nyqScale := corrNyq / rawNyq

	if !almostEqual(dcScale, nyqScale, 1e-6) {
		t.Fatalf("expected uniform scale from correction: dcScale=%.8f nyqScale=%.8f", dcScale, nyqScale)
	}
}

func TestChebyshev2Math_RawAndCorrectedEndpointAnchors(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	order := 6
	gainDB := 12.0
	rippleDB := 0.5
	K := math.Tan(math.Pi * fc / sr)
	stopbandDB := rippleDB

	raw := chebyshev2SectionsNoDCCorrection(K, gainDB, stopbandDB, order)
	corrected, err := chebyshev2Sections(K, gainDB, stopbandDB, order)
	if err != nil {
		t.Fatal(err)
	}

	rawDC := cascadeMagnitudeDB(raw, 1, sr)
	rawNyq := cascadeMagnitudeDB(raw, sr/2-1, sr)
	corrDC := cascadeMagnitudeDB(corrected, 1, sr)
	corrNyq := cascadeMagnitudeDB(corrected, sr/2-1, sr)

	// Diagnostic expectations for current implementation behavior:
	// 1) Raw sections are anchored near stopbandDB at DC.
	// 2) Raw sections are anchored near 0 dB at Nyquist.
	// 3) After correction, DC is moved to gainDB and Nyquist is shifted by
	//    roughly gainDB-stopbandDB.
	if !almostEqual(rawDC, stopbandDB, 0.2) {
		t.Fatalf("raw DC = %.4f dB, expected near stopband %.4f dB", rawDC, stopbandDB)
	}
	if math.Abs(rawNyq) > 0.2 {
		t.Fatalf("raw Nyquist = %.4f dB, expected near 0 dB", rawNyq)
	}
	if !almostEqual(corrDC, gainDB, 0.2) {
		t.Fatalf("corrected DC = %.4f dB, expected near gain %.4f dB", corrDC, gainDB)
	}
	if !almostEqual(corrNyq, gainDB-stopbandDB, 0.2) {
		t.Fatalf("corrected Nyquist = %.4f dB, expected near gain-stopband %.4f dB", corrNyq, gainDB-stopbandDB)
	}
}
