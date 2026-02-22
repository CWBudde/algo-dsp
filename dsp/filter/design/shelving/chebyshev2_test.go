package shelving

import (
	"fmt"
	"math"
	"math/cmplx"
	"strings"
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
		stopbandDB       float64
		order            int
	}{
		{"zero sample rate", 0, 1000, 6, 0.5, 2},
		{"negative freq", 48000, -1, 6, 0.5, 2},
		{"freq at Nyquist", 48000, 24000, 6, 0.5, 2},
		{"zero order", 48000, 1000, 6, 0.5, 0},
		{"zero stopband", 48000, 1000, 6, 0, 2},
		{"negative stopband", 48000, 1000, 6, -1, 2},
		{"stopband >= boost", 48000, 1000, 1, 1.0, 4},
		{"stopband > boost", 48000, 1000, 1, 1.5, 4},
		{"stopband >= cut magnitude", 48000, 1000, -1, 1.0, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Chebyshev2LowShelf(tt.sr, tt.freq, tt.gainDB, tt.stopbandDB, tt.order)
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
		t.Error("expected error for zero stopband")
	}

	_, err = Chebyshev2HighShelf(48000, 1000, 1, 1.0, 4)
	if err == nil {
		t.Error("expected error for stopband >= gain magnitude")
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
			stopbandDB := 0.5

			sections, err := Chebyshev2LowShelf(testSR, 1000, gainDB, stopbandDB, 4)
			if err != nil {
				t.Fatal(err)
			}

			dcMag := cascadeMagnitudeDB(sections, 1, testSR)

			expected := gainDB - math.Copysign(stopbandDB, gainDB)
			if !almostEqual(dcMag, expected, 0.2) {
				t.Errorf("DC gain = %.4f dB, expected %.4f dB", dcMag, expected)
			}
		})
	}
}

func TestChebyshev2LowShelf_NyquistGain(t *testing.T) {
	for _, gainDB := range []float64{-12, -6, 6, 12} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			stopbandDB := 0.5

			sections, err := Chebyshev2LowShelf(testSR, 1000, gainDB, stopbandDB, 4)
			if err != nil {
				t.Fatal(err)
			}

			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
			if !almostEqual(nyqMag, 0, 0.2) {
				t.Errorf("Nyquist gain = %.4f dB, expected ~0 dB", nyqMag)
			}
		})
	}
}

func TestChebyshev2HighShelf_NyquistGain(t *testing.T) {
	for _, gainDB := range []float64{-12, -6, 6, 12, 20} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			stopbandDB := 0.5

			sections, err := Chebyshev2HighShelf(testSR, 1000, gainDB, stopbandDB, 4)
			if err != nil {
				t.Fatal(err)
			}

			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)

			expected := gainDB - math.Copysign(stopbandDB, gainDB)
			if !almostEqual(nyqMag, expected, 0.3) {
				t.Errorf("Nyquist gain = %.4f dB, expected %.4f dB", nyqMag, expected)
			}
		})
	}
}

func TestChebyshev2HighShelf_DCGain(t *testing.T) {
	for _, gainDB := range []float64{-12, -6, 6, 12} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			stopbandDB := 0.5

			sections, err := Chebyshev2HighShelf(testSR, 1000, gainDB, stopbandDB, 4)
			if err != nil {
				t.Fatal(err)
			}

			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			if !almostEqual(dcMag, 0, 0.2) {
				t.Errorf("DC gain = %.4f dB, expected ~0 dB", dcMag)
			}
		})
	}
}

// ============================================================
// Chebyshev Type II: endpoint anchor model verification
// ============================================================

// The current realization anchors the stopband near 0 dB and places the shelf
// endpoint near gainDB-sign(gainDB)*stopbandDB.
func TestChebyshev2LowShelf_StopbandAnchorModel(t *testing.T) {
	testCases := []struct {
		name       string
		gainDB     float64
		stopbandDB float64
		order      int
	}{
		{"boost_6dB_stopband_0.5", 6, 0.5, 4},
		{"boost_12dB_stopband_0.5", 12, 0.5, 4},
		{"boost_18dB_stopband_0.5", 18, 0.5, 4},
		{"boost_24dB_stopband_0.5", 24, 0.5, 4},
		{"cut_6dB_stopband_0.5", -6, 0.5, 4},
		{"cut_12dB_stopband_0.5", -12, 0.5, 4},
		{"boost_12dB_stopband_0.1", 12, 0.1, 4},
		{"boost_12dB_stopband_0.25", 12, 0.25, 4},
		{"boost_12dB_stopband_1.0", 12, 1.0, 4},
		{"boost_12dB_stopband_2.0", 12, 2.0, 4},
		{"boost_12dB_stopband_3.0", 12, 3.0, 4},
		{"boost_12dB_order_2", 12, 0.5, 2},
		{"boost_12dB_order_6", 12, 0.5, 6},
		{"boost_12dB_order_8", 12, 0.5, 8},
		{"boost_12dB_order_10", 12, 0.5, 10},
		{"small_boost_3dB", 3, 0.5, 4},
		{"large_boost_30dB", 30, 0.5, 4},
		{"small_stopband_0.05", 12, 0.05, 4},
		{"large_stopband_5.0", 12, 5.0, 4},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sections, err := Chebyshev2LowShelf(testSR, 1000, tc.gainDB, tc.stopbandDB, tc.order)
			if err != nil {
				t.Fatal(err)
			}

			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
			farStopband := cascadeMagnitudeDB(sections, 10000, testSR)
			expectedShelf := tc.gainDB - math.Copysign(tc.stopbandDB, tc.gainDB)
			tolStop := tc.stopbandDB + 0.2

			if !almostEqual(dcMag, expectedShelf, 0.25) {
				t.Errorf("DC gain = %.4f dB, expected %.4f dB", dcMag, expectedShelf)
			}

			if !almostEqual(nyqMag, 0, tolStop) {
				t.Errorf("Nyquist gain = %.4f dB, expected ~0 dB", nyqMag)
			}

			if !almostEqual(farStopband, 0, tolStop) {
				t.Errorf("far stopband gain = %.4f dB, expected ~0 dB", farStopband)
			}
		})
	}
}

func TestChebyshev2HighShelf_StopbandAnchorModel(t *testing.T) {
	testCases := []struct {
		name       string
		gainDB     float64
		stopbandDB float64
		order      int
	}{
		{"boost_6dB_stopband_0.5", 6, 0.5, 4},
		{"boost_12dB_stopband_0.5", 12, 0.5, 4},
		{"boost_18dB_stopband_0.5", 18, 0.5, 4},
		{"cut_6dB_stopband_0.5", -6, 0.5, 4},
		{"cut_12dB_stopband_0.5", -12, 0.5, 4},
		{"boost_12dB_stopband_0.1", 12, 0.1, 4},
		{"boost_12dB_stopband_1.0", 12, 1.0, 4},
		{"boost_12dB_stopband_2.0", 12, 2.0, 4},
		{"boost_12dB_order_2", 12, 0.5, 2},
		{"boost_12dB_order_6", 12, 0.5, 6},
		{"boost_12dB_order_8", 12, 0.5, 8},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sections, err := Chebyshev2HighShelf(testSR, 1000, tc.gainDB, tc.stopbandDB, tc.order)
			if err != nil {
				t.Fatal(err)
			}

			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			farStopband := cascadeMagnitudeDB(sections, 50, testSR)
			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
			expectedShelf := tc.gainDB - math.Copysign(tc.stopbandDB, tc.gainDB)
			tolStop := tc.stopbandDB + 0.2

			if !almostEqual(nyqMag, expectedShelf, 0.25) {
				t.Errorf("Nyquist gain = %.4f dB, expected %.4f dB", nyqMag, expectedShelf)
			}

			if !almostEqual(dcMag, 0, tolStop) {
				t.Errorf("DC gain = %.4f dB, expected ~0 dB", dcMag)
			}

			if !almostEqual(farStopband, 0, tolStop) {
				t.Errorf("far stopband gain = %.4f dB, expected ~0 dB", farStopband)
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

func TestChebyshev2LowShelf_PoleZeroPairs(t *testing.T) {
	for _, M := range []int{1, 3, 5, 8} {
		t.Run(orderName(M), func(t *testing.T) {
			sections, err := Chebyshev2LowShelf(testSR, 1000, 12, 0.5, M)
			if err != nil {
				t.Fatal(err)
			}

			pairs := biquad.PoleZeroPairs(sections)
			if len(pairs) != len(sections) {
				t.Fatalf("got %d pole/zero pairs, expected %d", len(pairs), len(sections))
			}

			for i, pair := range pairs {
				for j, pole := range pair.Poles {
					if math.IsNaN(real(pole)) || math.IsNaN(imag(pole)) {
						t.Fatalf("section %d pole %d is NaN: %v", i, j, pole)
					}

					if cmplx.Abs(pole) >= 1.0+1e-9 {
						t.Fatalf("section %d pole %d unstable: |p|=%.8f p=%v", i, j, cmplx.Abs(pole), pole)
					}
				}
			}
		})
	}
}

func TestChebyshev2HighShelf_PoleZeroPairs(t *testing.T) {
	for _, M := range []int{1, 3, 5, 8} {
		t.Run(orderName(M), func(t *testing.T) {
			sections, err := Chebyshev2HighShelf(testSR, 1000, 12, 0.5, M)
			if err != nil {
				t.Fatal(err)
			}

			pairs := biquad.PoleZeroPairs(sections)
			if len(pairs) != len(sections) {
				t.Fatalf("got %d pole/zero pairs, expected %d", len(pairs), len(sections))
			}

			for i, pair := range pairs {
				for j, pole := range pair.Poles {
					if math.IsNaN(real(pole)) || math.IsNaN(imag(pole)) {
						t.Fatalf("section %d pole %d is NaN: %v", i, j, pole)
					}

					if cmplx.Abs(pole) >= 1.0+1e-9 {
						t.Fatalf("section %d pole %d unstable: |p|=%.8f p=%v", i, j, cmplx.Abs(pole), pole)
					}
				}
			}
		})
	}
}

// ============================================================
// Chebyshev Type II: order sweep
// ============================================================

func TestChebyshev2LowShelf_VariousOrders(t *testing.T) {
	for _, M := range []int{1, 2, 3, 4, 5, 6, 8, 10, 12} {
		t.Run(orderName(M), func(t *testing.T) {
			stopbandDB := 0.5

			sections, err := Chebyshev2LowShelf(testSR, 1000, 12, stopbandDB, M)
			if err != nil {
				t.Fatal(err)
			}

			allPolesStable(t, sections)
			dcMag := cascadeMagnitudeDB(sections, 1, testSR)

			expectedShelf := 12.0 - stopbandDB
			if !almostEqual(dcMag, expectedShelf, 0.2) {
				t.Errorf("M=%d: DC gain = %.4f dB, expected ~%.4f dB", M, dcMag, expectedShelf)
			}

			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
			if !almostEqual(nyqMag, 0, 0.2) {
				t.Errorf("M=%d: Nyquist gain = %.4f dB, expected ~0 dB", M, nyqMag)
			}
		})
	}
}

// ============================================================
// Chebyshev Type II: stopband ripple bounded by stopbandDB
// ============================================================

func TestChebyshev2LowShelf_StopbandRipple(t *testing.T) {
	// For a low-shelf boost, the "stopband" (flat region) is the high-frequency
	// portion above the cutoff. The ripple there should be bounded by stopbandDB.
	stopbandDB := 0.5

	sections, err := Chebyshev2LowShelf(testSR, 1000, 12, stopbandDB, 6)
	if err != nil {
		t.Fatal(err)
	}

	// Sample the flat region (well above cutoff).
	expected := 0.0

	for f := 5000.0; f < testSR/2-100; f += 500 {
		mag := cascadeMagnitudeDB(sections, f, testSR)
		if math.Abs(mag-expected) > stopbandDB+0.2 {
			t.Errorf("stopband at %.0f Hz: %.4f dB exceeds expected %.4f dB by > ±%.1f dB", f, mag, expected, stopbandDB)
		}
	}
}

func TestChebyshev2HighShelf_StopbandRipple(t *testing.T) {
	// For a high-shelf boost, the stopband is the low-frequency portion below cutoff.
	stopbandDB := 0.5

	sections, err := Chebyshev2HighShelf(testSR, 1000, 12, stopbandDB, 6)
	if err != nil {
		t.Fatal(err)
	}

	expected := 0.0

	for f := 10.0; f < 200; f += 10 {
		mag := cascadeMagnitudeDB(sections, f, testSR)
		if math.Abs(mag-expected) > stopbandDB+0.2 {
			t.Errorf("stopband at %.0f Hz: %.4f dB exceeds expected %.4f dB by > ±%.1f dB", f, mag, expected, stopbandDB)
		}
	}
}

// ============================================================
// Chebyshev Type II: extreme gains
// ============================================================

func TestChebyshev2LowShelf_ExtremeGains(t *testing.T) {
	for _, gainDB := range []float64{-30, -20, -6, -1, 1, 6, 20, 30} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			stopbandDB := 0.5

			sections, err := Chebyshev2LowShelf(testSR, 1000, gainDB, stopbandDB, 4)
			if err != nil {
				t.Fatal(err)
			}

			allPolesStable(t, sections)
			dcMag := cascadeMagnitudeDB(sections, 1, testSR)

			expected := gainDB - math.Copysign(stopbandDB, gainDB)
			if !almostEqual(dcMag, expected, 0.3) {
				t.Errorf("DC gain = %.4f dB, expected %.4f dB", dcMag, expected)
			}
		})
	}
}

// ============================================================
// Chebyshev Type II: various stopband values
// ============================================================

func TestChebyshev2LowShelf_VariousStopband(t *testing.T) {
	stopbands := []float64{0.1, 0.25, 0.5, 1.0, 2.0, 3.0}
	for _, sb := range stopbands {
		name := ftoa(sb) + "dBstopband"
		t.Run(name, func(t *testing.T) {
			sections, err := Chebyshev2LowShelf(testSR, 1000, 12, sb, 6)
			if err != nil {
				t.Fatal(err)
			}

			allPolesStable(t, sections)
			dcMag := cascadeMagnitudeDB(sections, 1, testSR)

			expectedShelf := 12.0 - sb
			if !almostEqual(dcMag, expectedShelf, 0.5) {
				t.Errorf("stopband=%.1f: DC gain = %.4f dB, expected ~%.4f dB", sb, dcMag, expectedShelf)
			}

			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
			if !almostEqual(nyqMag, 0, 0.2) {
				t.Errorf("stopband=%.1f: Nyquist gain = %.4f dB, expected ~0 dB", sb, nyqMag)
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
			stopbandDB := 0.5

			sections, err := Chebyshev2LowShelf(testSR, freq, 12, stopbandDB, 4)
			if err != nil {
				t.Fatal(err)
			}

			allPolesStable(t, sections)
			dcMag := cascadeMagnitudeDB(sections, 1, testSR)

			expectedShelf := 12.0 - stopbandDB
			if !almostEqual(dcMag, expectedShelf, 0.2) {
				t.Errorf("freq=%v: DC gain = %.4f dB, expected ~%.4f dB", freq, dcMag, expectedShelf)
			}
		})
	}
}

func TestChebyshev2HighShelf_VariousFrequencies(t *testing.T) {
	for _, freq := range []float64{100, 300, 500, 1000, 2000, 5000, 10000} {
		t.Run(freqName(freq), func(t *testing.T) {
			stopbandDB := 0.5

			sections, err := Chebyshev2HighShelf(testSR, freq, 12, stopbandDB, 4)
			if err != nil {
				t.Fatal(err)
			}

			allPolesStable(t, sections)
			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)

			expectedShelf := 12.0 - stopbandDB
			if !almostEqual(nyqMag, expectedShelf, 0.3) {
				t.Errorf("freq=%v: Nyquist gain = %.4f dB, expected ~%.4f dB", freq, nyqMag, expectedShelf)
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
	// The maximum deviation in the stopband should be bounded by stopbandDB.
	order := 6
	gainDB := 12.0
	stopbandDB := 0.5

	sections, err := Chebyshev2LowShelf(testSR, 1000, gainDB, stopbandDB, order)
	if err != nil {
		t.Fatal(err)
	}

	// In the far stopband (well above cutoff), magnitudes should stay close to
	// the implementation's stopband anchor (0 dB).
	expected := 0.0
	maxDev := 0.0

	for f := 8000.0; f < testSR/2-100; f += 100 {
		mag := cascadeMagnitudeDB(sections, f, testSR)

		dev := math.Abs(mag - expected)
		if dev > maxDev {
			maxDev = dev
		}
	}

	if maxDev > stopbandDB+0.2 {
		t.Errorf("stopband max deviation = %.4f dB from %.4f dB, exceeds bound %.1f dB", maxDev, expected, stopbandDB)
	}
}

// ============================================================
// Chebyshev Type II: math verification tests
// ============================================================

// chebyshev2SectionsNoStopbandNormalization mirrors chebyshev2Sections but
// intentionally skips the final stopband normalization so we can test where
// endpoint behavior diverges.
func chebyshev2SectionsNoStopbandNormalization(K float64, gainDB, stopbandDB float64, order int) []biquad.Coefficients {
	G0 := 1.0
	G := db2Lin(gainDB)
	Gb := db2Lin(gainDB - stopbandDB)
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

func TestChebyshev2Math_StopbandNormalizationScalesAllFrequencies(t *testing.T) {
	sr := 48000.0
	fc := 1000.0
	order := 6
	gainDB := 12.0
	stopbandDB := 0.5
	K := math.Tan(math.Pi * fc / sr)

	raw := chebyshev2SectionsNoStopbandNormalization(K, gainDB, stopbandDB, order)

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
	stopbandDB := 0.5
	K := math.Tan(math.Pi * fc / sr)

	raw := chebyshev2SectionsNoStopbandNormalization(K, gainDB, stopbandDB, order)

	corrected, err := chebyshev2Sections(K, gainDB, stopbandDB, order)
	if err != nil {
		t.Fatal(err)
	}

	rawDC := cascadeMagnitudeDB(raw, 1, sr)
	rawNyq := cascadeMagnitudeDB(raw, sr/2-1, sr)
	corrDC := cascadeMagnitudeDB(corrected, 1, sr)
	corrNyq := cascadeMagnitudeDB(corrected, sr/2-1, sr)

	// Diagnostic expectations for current implementation behavior:
	// 1) Raw sections are anchored near gainDB-stopbandDB at DC.
	// 2) Raw sections are anchored near 0 dB at Nyquist.
	// 3) Stopband normalization keeps Nyquist at 0 dB.
	expectedShelf := gainDB - stopbandDB
	if !almostEqual(rawDC, expectedShelf, 0.2) {
		t.Fatalf("raw DC = %.4f dB, expected near shelf %.4f dB", rawDC, expectedShelf)
	}

	if math.Abs(rawNyq) > 0.2 {
		t.Fatalf("raw Nyquist = %.4f dB, expected near 0 dB", rawNyq)
	}

	if !almostEqual(corrDC, expectedShelf, 0.2) {
		t.Fatalf("corrected DC = %.4f dB, expected near shelf %.4f dB", corrDC, expectedShelf)
	}

	if math.Abs(corrNyq) > 0.2 {
		t.Fatalf("corrected Nyquist = %.4f dB, expected near 0 dB", corrNyq)
	}
}

const cheby2GridMaxExamples = 8

// cheby2GridCaseName formats a deterministic identifier for grid sweeps.
func cheby2GridCaseName(gainDB, stopbandDB float64, order int, cutoffHz float64) string {
	return fmt.Sprintf("G%+.1f_SB%.2f_M%d_F%.0f", gainDB, stopbandDB, order, cutoffHz)
}

func cheby2AppendFailure(examples *[]string, msg string) {
	if len(*examples) < cheby2GridMaxExamples {
		*examples = append(*examples, msg)
	}
}

// ============================================================
// Chebyshev Type II: low-shelf grid/property tests
// ============================================================

func TestChebyshev2LowShelf_EndpointAnchorsGrid(t *testing.T) {
	orders := []int{1, 2, 4, 6}
	cutoffs := []float64{300, 1000, 3000}
	stopbands := []float64{0.1, 0.5, 1.0}
	gains := []float64{-24, -12, -6, -3, 3, 6, 12, 24}

	total := 0
	failed := 0
	examples := make([]string, 0, cheby2GridMaxExamples)

	for _, order := range orders {
		for _, cutoffHz := range cutoffs {
			for _, stopbandDB := range stopbands {
				for _, gainDB := range gains {
					if stopbandDB >= math.Abs(gainDB) {
						continue
					}

					total++
					caseName := cheby2GridCaseName(gainDB, stopbandDB, order, cutoffHz)

					sections, err := Chebyshev2LowShelf(testSR, cutoffHz, gainDB, stopbandDB, order)
					if err != nil {
						failed++

						cheby2AppendFailure(&examples, fmt.Sprintf("%s: design error: %v", caseName, err))

						continue
					}

					expectedDC := gainDB - math.Copysign(stopbandDB, gainDB)
					dcMag := cascadeMagnitudeDB(sections, 1, testSR)
					nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)

					if !almostEqual(dcMag, expectedDC, 0.35) {
						failed++

						cheby2AppendFailure(&examples, fmt.Sprintf("%s: DC gain %.4f dB, expected %.4f dB", caseName, dcMag, expectedDC))

						continue
					}

					if !almostEqual(nyqMag, 0, stopbandDB+0.25) {
						failed++

						cheby2AppendFailure(&examples, fmt.Sprintf("%s: Nyquist gain %.4f dB, expected ~0 dB", caseName, nyqMag))
					}
				}
			}
		}
	}

	if failed > 0 {
		t.Fatalf("endpoint-anchor grid failures: %d/%d cases failed. First %d:\n%s", failed, total, len(examples), strings.Join(examples, "\n"))
	}
}

func TestChebyshev2LowShelf_MonotonicGrid_Boost(t *testing.T) {
	orders := []int{2, 3, 4, 6, 8}
	cutoffs := []float64{300, 1000, 3000}
	stopbands := []float64{0.1, 0.5, 1.0}
	gains := []float64{3, 6, 12, 24}

	total := 0
	failed := 0
	examples := make([]string, 0, cheby2GridMaxExamples)

	for _, order := range orders {
		for _, cutoffHz := range cutoffs {
			upperHz := math.Min(cutoffHz*0.8, 800)
			if upperHz <= 20 {
				continue
			}

			stepHz := math.Max(5, upperHz/80)

			for _, stopbandDB := range stopbands {
				for _, gainDB := range gains {
					if stopbandDB >= math.Abs(gainDB) {
						continue
					}

					total++
					caseName := cheby2GridCaseName(gainDB, stopbandDB, order, cutoffHz)

					sections, err := Chebyshev2LowShelf(testSR, cutoffHz, gainDB, stopbandDB, order)
					if err != nil {
						failed++

						cheby2AppendFailure(&examples, fmt.Sprintf("%s: design error: %v", caseName, err))

						continue
					}

					prevMag := cascadeMagnitudeDB(sections, 1, testSR)
					for f := stepHz; f <= upperHz; f += stepHz {
						mag := cascadeMagnitudeDB(sections, f, testSR)
						if mag > prevMag+0.12 {
							failed++

							cheby2AppendFailure(&examples, fmt.Sprintf("%s: non-monotonic at %.2f Hz: %.4f dB > %.4f dB", caseName, f, mag, prevMag))

							break
						}

						prevMag = mag
					}
				}
			}
		}
	}

	if failed > 0 {
		t.Fatalf("monotonic-grid failures: %d/%d cases failed. First %d:\n%s", failed, total, len(examples), strings.Join(examples, "\n"))
	}
}

func TestChebyshev2LowShelf_BoostCutInversionGrid(t *testing.T) {
	orders := []int{2, 4, 6, 8}
	cutoffs := []float64{300, 1000, 3000}
	stopbands := []float64{0.1, 0.5, 1.0}
	gains := []float64{3, 6, 12, 24}

	total := 0
	failed := 0
	examples := make([]string, 0, cheby2GridMaxExamples)

	for _, order := range orders {
		for _, cutoffHz := range cutoffs {
			for _, stopbandDB := range stopbands {
				for _, gainDB := range gains {
					if stopbandDB >= gainDB {
						continue
					}

					total++
					caseName := cheby2GridCaseName(gainDB, stopbandDB, order, cutoffHz)

					boost, err := Chebyshev2LowShelf(testSR, cutoffHz, gainDB, stopbandDB, order)
					if err != nil {
						failed++

						cheby2AppendFailure(&examples, fmt.Sprintf("%s: boost design error: %v", caseName, err))

						continue
					}

					cut, err := Chebyshev2LowShelf(testSR, cutoffHz, -gainDB, stopbandDB, order)
					if err != nil {
						failed++

						cheby2AppendFailure(&examples, fmt.Sprintf("%s: cut design error: %v", caseName, err))

						continue
					}

					probeFreqs := []float64{
						1,
						math.Max(10, cutoffHz*0.1),
						cutoffHz,
						math.Min(15000, testSR/2-200),
						testSR/2 - 1,
					}

					for _, freq := range probeFreqs {
						hBoost := cascadeResponse(boost, freq, testSR)
						hCut := cascadeResponse(cut, freq, testSR)
						errDB := math.Abs(20 * math.Log10(cmplx.Abs(hBoost*hCut)))

						tolDB := 1.0
						if freq <= 1.5 || math.Abs(freq-(testSR/2-1)) < 1e-9 {
							tolDB = 0.6
						}

						if errDB > tolDB {
							failed++

							cheby2AppendFailure(&examples, fmt.Sprintf("%s: freq=%.2f Hz inversion error %.4f dB > %.2f dB", caseName, freq, errDB, tolDB))

							break
						}
					}
				}
			}
		}
	}

	if failed > 0 {
		t.Fatalf("boost/cut inversion-grid failures: %d/%d cases failed. First %d:\n%s", failed, total, len(examples), strings.Join(examples, "\n"))
	}
}

// ============================================================
// Chebyshev Type II: high-shelf grid/property tests
// ============================================================

func TestChebyshev2HighShelf_EndpointAnchorsGrid(t *testing.T) {
	orders := []int{1, 2, 4, 6}
	cutoffs := []float64{300, 1000, 3000}
	stopbands := []float64{0.1, 0.5, 1.0}
	gains := []float64{-24, -12, -6, -3, 3, 6, 12, 24}

	total := 0
	failed := 0
	examples := make([]string, 0, cheby2GridMaxExamples)

	for _, order := range orders {
		for _, cutoffHz := range cutoffs {
			for _, stopbandDB := range stopbands {
				for _, gainDB := range gains {
					if stopbandDB >= math.Abs(gainDB) {
						continue
					}

					total++
					caseName := cheby2GridCaseName(gainDB, stopbandDB, order, cutoffHz)

					sections, err := Chebyshev2HighShelf(testSR, cutoffHz, gainDB, stopbandDB, order)
					if err != nil {
						failed++

						cheby2AppendFailure(&examples, fmt.Sprintf("%s: design error: %v", caseName, err))

						continue
					}

					expectedNyq := gainDB - math.Copysign(stopbandDB, gainDB)
					dcMag := cascadeMagnitudeDB(sections, 1, testSR)
					nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)

					if !almostEqual(dcMag, 0, stopbandDB+0.25) {
						failed++

						cheby2AppendFailure(&examples, fmt.Sprintf("%s: DC gain %.4f dB, expected ~0 dB", caseName, dcMag))

						continue
					}

					if !almostEqual(nyqMag, expectedNyq, 0.35) {
						failed++

						cheby2AppendFailure(&examples, fmt.Sprintf("%s: Nyquist gain %.4f dB, expected %.4f dB", caseName, nyqMag, expectedNyq))
					}
				}
			}
		}
	}

	if failed > 0 {
		t.Fatalf("high-shelf endpoint-anchor grid failures: %d/%d cases failed. First %d:\n%s", failed, total, len(examples), strings.Join(examples, "\n"))
	}
}

func TestChebyshev2HighShelf_MonotonicGrid_Boost(t *testing.T) {
	orders := []int{2, 3, 4, 6, 8}
	cutoffs := []float64{300, 1000, 3000}
	stopbands := []float64{0.1, 0.5, 1.0}
	gains := []float64{3, 6, 12, 24}

	total := 0
	failed := 0
	examples := make([]string, 0, cheby2GridMaxExamples)

	for _, order := range orders {
		for _, cutoffHz := range cutoffs {
			upperHz := math.Min(cutoffHz*0.8, 800)
			if upperHz <= 20 {
				continue
			}

			stepHz := math.Max(5, upperHz/80)

			for _, stopbandDB := range stopbands {
				for _, gainDB := range gains {
					if stopbandDB >= math.Abs(gainDB) {
						continue
					}

					total++
					caseName := cheby2GridCaseName(gainDB, stopbandDB, order, cutoffHz)

					sections, err := Chebyshev2HighShelf(testSR, cutoffHz, gainDB, stopbandDB, order)
					if err != nil {
						failed++

						cheby2AppendFailure(&examples, fmt.Sprintf("%s: design error: %v", caseName, err))

						continue
					}

					prevMag := cascadeMagnitudeDB(sections, 1, testSR)
					for f := stepHz; f <= upperHz; f += stepHz {
						mag := cascadeMagnitudeDB(sections, f, testSR)
						if mag < prevMag-0.12 {
							failed++

							cheby2AppendFailure(&examples, fmt.Sprintf("%s: non-monotonic at %.2f Hz: %.4f dB < %.4f dB", caseName, f, mag, prevMag))

							break
						}

						prevMag = mag
					}
				}
			}
		}
	}

	if failed > 0 {
		t.Fatalf("high-shelf monotonic-grid failures: %d/%d cases failed. First %d:\n%s", failed, total, len(examples), strings.Join(examples, "\n"))
	}
}

func TestChebyshev2HighShelf_BoostCutInversionGrid(t *testing.T) {
	orders := []int{2, 4, 6, 8}
	cutoffs := []float64{300, 1000, 3000}
	stopbands := []float64{0.1, 0.5, 1.0}
	gains := []float64{3, 6, 12, 24}

	total := 0
	failed := 0
	examples := make([]string, 0, cheby2GridMaxExamples)

	for _, order := range orders {
		for _, cutoffHz := range cutoffs {
			for _, stopbandDB := range stopbands {
				for _, gainDB := range gains {
					if stopbandDB >= gainDB {
						continue
					}

					total++
					caseName := cheby2GridCaseName(gainDB, stopbandDB, order, cutoffHz)

					boost, err := Chebyshev2HighShelf(testSR, cutoffHz, gainDB, stopbandDB, order)
					if err != nil {
						failed++

						cheby2AppendFailure(&examples, fmt.Sprintf("%s: boost design error: %v", caseName, err))

						continue
					}

					cut, err := Chebyshev2HighShelf(testSR, cutoffHz, -gainDB, stopbandDB, order)
					if err != nil {
						failed++

						cheby2AppendFailure(&examples, fmt.Sprintf("%s: cut design error: %v", caseName, err))

						continue
					}

					probeFreqs := []float64{
						1,
						math.Max(10, cutoffHz*0.1),
						cutoffHz,
						math.Min(15000, testSR/2-200),
						testSR/2 - 1,
					}

					for _, freq := range probeFreqs {
						hBoost := cascadeResponse(boost, freq, testSR)
						hCut := cascadeResponse(cut, freq, testSR)
						errDB := math.Abs(20 * math.Log10(cmplx.Abs(hBoost*hCut)))

						tolDB := 1.0
						if freq <= 1.5 || math.Abs(freq-(testSR/2-1)) < 1e-9 {
							tolDB = 0.6
						}

						if errDB > tolDB {
							failed++

							cheby2AppendFailure(&examples, fmt.Sprintf("%s: freq=%.2f Hz inversion error %.4f dB > %.2f dB", caseName, freq, errDB, tolDB))

							break
						}
					}
				}
			}
		}
	}

	if failed > 0 {
		t.Fatalf("high-shelf boost/cut inversion-grid failures: %d/%d cases failed. First %d:\n%s", failed, total, len(examples), strings.Join(examples, "\n"))
	}
}
