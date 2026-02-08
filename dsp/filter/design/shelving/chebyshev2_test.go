package shelving

import (
	"math"
	"math/cmplx"
	"testing"
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
	// Chebyshev II has equiripple in the stopband (flat region).
	// Nyquist gain should be near 0 dB, bounded by the ripple parameter.
	for _, gainDB := range []float64{-12, -6, 6, 12} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			rippleDB := 0.5
			sections, err := Chebyshev2LowShelf(testSR, 1000, gainDB, rippleDB, 4)
			if err != nil {
				t.Fatal(err)
			}
			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
			if math.Abs(nyqMag) > rippleDB+0.1 {
				t.Errorf("Nyquist gain = %.4f dB, expected ~0 dB (ripple=%.1f)", nyqMag, rippleDB)
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
	// DC should be near 0 dB for high-shelf, bounded by ripple.
	for _, gainDB := range []float64{-12, -6, 6, 12} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			rippleDB := 0.5
			sections, err := Chebyshev2HighShelf(testSR, 1000, gainDB, rippleDB, 4)
			if err != nil {
				t.Fatal(err)
			}
			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			if math.Abs(dcMag) > rippleDB+0.1 {
				t.Errorf("DC gain = %.4f dB, expected ~0 dB (ripple=%.1f)", dcMag, rippleDB)
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
			if math.Abs(nyqMag) > rippleDB+0.1 {
				t.Errorf("M=%d: Nyquist gain = %.4f dB, expected ~0 dB", M, nyqMag)
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
	for f := 5000.0; f < testSR/2-100; f += 500 {
		mag := cascadeMagnitudeDB(sections, f, testSR)
		if math.Abs(mag) > rippleDB+0.2 {
			t.Errorf("stopband at %.0f Hz: %.4f dB exceeds ripple bound (±%.1f dB)", f, mag, rippleDB)
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

	for f := 10.0; f < 200; f += 10 {
		mag := cascadeMagnitudeDB(sections, f, testSR)
		if math.Abs(mag) > rippleDB+0.2 {
			t.Errorf("stopband at %.0f Hz: %.4f dB exceeds ripple bound (±%.1f dB)", f, mag, rippleDB)
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
			if math.Abs(nyqMag) > rip+0.2 {
				t.Errorf("ripple=%.1f: Nyquist gain = %.4f dB, expected ~0 dB (bound=±%.1f)", rip, nyqMag, rip)
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
		if mag > prevMag+0.01 {
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

	// In the far stopband (well above cutoff), all magnitudes should be
	// within the ripple bound.
	maxDev := 0.0
	for f := 8000.0; f < testSR/2-100; f += 100 {
		mag := math.Abs(cascadeMagnitudeDB(sections, f, testSR))
		if mag > maxDev {
			maxDev = mag
		}
	}
	if maxDev > rippleDB+0.2 {
		t.Errorf("stopband max deviation = %.4f dB, exceeds ripple bound %.1f dB", maxDev, rippleDB)
	}
}


