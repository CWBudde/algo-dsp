package shelving

import (
	"math"
	"math/cmplx"
	"testing"
)

// ============================================================
// Butterworth shelving filter tests
// ============================================================

func TestButterworthLowShelf_InvalidParams(t *testing.T) {
	tests := []struct {
		name     string
		sr, freq float64
		gainDB   float64
		order    int
	}{
		{"zero sample rate", 0, 1000, 6, 2},
		{"negative freq", 48000, -1, 6, 2},
		{"freq at Nyquist", 48000, 24000, 6, 2},
		{"freq above Nyquist", 48000, 25000, 6, 2},
		{"zero order", 48000, 1000, 6, 0},
		{"negative order", 48000, 1000, 6, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ButterworthLowShelf(tt.sr, tt.freq, tt.gainDB, tt.order)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestButterworthHighShelf_InvalidParams(t *testing.T) {
	_, err := ButterworthHighShelf(0, 1000, 6, 2)
	if err == nil {
		t.Error("expected error for zero sample rate")
	}

	_, err = ButterworthHighShelf(48000, 1000, 6, 0)
	if err == nil {
		t.Error("expected error for zero order")
	}
}

// ============================================================
// Passthrough at zero gain
// ============================================================

func TestLowShelf_ZeroGain(t *testing.T) {
	sections, err := ButterworthLowShelf(testSR, 1000, 0, 4)
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

func TestHighShelf_ZeroGain(t *testing.T) {
	sections, err := ButterworthHighShelf(testSR, 1000, 0, 4)
	if err != nil {
		t.Fatal(err)
	}

	if len(sections) != 1 {
		t.Fatalf("expected 1 passthrough section, got %d", len(sections))
	}
}

// ============================================================
// Section count
// ============================================================

func TestLowShelf_SectionCount(t *testing.T) {
	for _, M := range []int{1, 2, 3, 4, 5, 6, 7, 8} {
		t.Run(orderName(M), func(t *testing.T) {
			sections, err := ButterworthLowShelf(testSR, 1000, 6, M)
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
// First-order section structure for odd M
// ============================================================

func TestLowShelf_Order1_FirstOrderSection(t *testing.T) {
	sections, err := ButterworthLowShelf(testSR, 1000, 6, 1)
	if err != nil {
		t.Fatal(err)
	}

	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}

	s := sections[0]
	if s.B2 != 0 || s.A2 != 0 {
		t.Errorf("M=1 should produce first-order section, but B2=%.6f A2=%.6f", s.B2, s.A2)
	}
}

func TestLowShelf_Order3_HasFirstOrder(t *testing.T) {
	sections, err := ButterworthLowShelf(testSR, 1000, 6, 3)
	if err != nil {
		t.Fatal(err)
	}

	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
	// Last section should be first-order.
	last := sections[len(sections)-1]
	if last.B2 != 0 || last.A2 != 0 {
		t.Errorf("last section of odd M should be first-order, but B2=%.6f A2=%.6f", last.B2, last.A2)
	}
}

// ============================================================
// Low-shelf frequency response
// ============================================================

func TestLowShelf_DCGain(t *testing.T) {
	for _, gainDB := range []float64{-12, -6, 6, 12, 20} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			sections, err := ButterworthLowShelf(testSR, 1000, gainDB, 4)
			if err != nil {
				t.Fatal(err)
			}

			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			if !almostEqual(dcMag, gainDB, 0.1) {
				t.Errorf("DC gain = %.4f dB, expected %.4f dB", dcMag, gainDB)
			}
		})
	}
}

func TestLowShelf_NyquistGain(t *testing.T) {
	for _, gainDB := range []float64{-12, -6, 6, 12} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			sections, err := ButterworthLowShelf(testSR, 1000, gainDB, 4)
			if err != nil {
				t.Fatal(err)
			}

			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
			if math.Abs(nyqMag) > 0.1 {
				t.Errorf("Nyquist gain = %.4f dB, expected ~0 dB", nyqMag)
			}
		})
	}
}

func TestLowShelf_CutoffGain(t *testing.T) {
	// At the cutoff frequency, |H|^2 = (g^2 + 1) / 2 (Eq. 5).
	gainDB := 12.0
	g := db2Lin(gainDB)
	expectedPower := (g*g + 1) / 2
	expectedDB := 10 * math.Log10(expectedPower)

	sections, err := ButterworthLowShelf(testSR, 1000, gainDB, 6)
	if err != nil {
		t.Fatal(err)
	}

	cutoffMag := cascadeMagnitudeDB(sections, 1000, testSR)
	if !almostEqual(cutoffMag, expectedDB, 0.2) {
		t.Errorf("cutoff gain = %.4f dB, expected %.4f dB", cutoffMag, expectedDB)
	}
}

// ============================================================
// High-shelf frequency response
// ============================================================

func TestHighShelf_NyquistGain(t *testing.T) {
	for _, gainDB := range []float64{-12, -6, 6, 12, 20} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			sections, err := ButterworthHighShelf(testSR, 1000, gainDB, 4)
			if err != nil {
				t.Fatal(err)
			}

			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
			if !almostEqual(nyqMag, gainDB, 0.2) {
				t.Errorf("Nyquist gain = %.4f dB, expected %.4f dB", nyqMag, gainDB)
			}
		})
	}
}

func TestHighShelf_DCGain(t *testing.T) {
	for _, gainDB := range []float64{-12, -6, 6, 12} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			sections, err := ButterworthHighShelf(testSR, 1000, gainDB, 4)
			if err != nil {
				t.Fatal(err)
			}

			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			if math.Abs(dcMag) > 0.1 {
				t.Errorf("DC gain = %.4f dB, expected ~0 dB", dcMag)
			}
		})
	}
}

// ============================================================
// Pole stability
// ============================================================

func TestLowShelf_Stability(t *testing.T) {
	for _, M := range []int{1, 2, 3, 4, 5, 6, 8, 10, 12} {
		t.Run(orderName(M), func(t *testing.T) {
			sections, err := ButterworthLowShelf(testSR, 1000, 12, M)
			if err != nil {
				t.Fatal(err)
			}

			allPolesStable(t, sections)
		})
	}
}

func TestHighShelf_Stability(t *testing.T) {
	for _, M := range []int{1, 2, 3, 4, 5, 6, 8, 10, 12} {
		t.Run(orderName(M), func(t *testing.T) {
			sections, err := ButterworthHighShelf(testSR, 1000, 12, M)
			if err != nil {
				t.Fatal(err)
			}

			allPolesStable(t, sections)
		})
	}
}

// ============================================================
// Boost/cut inversion
// ============================================================

func TestLowShelf_BoostCutInversion(t *testing.T) {
	// The Holters/Zölzer Butterworth shelving design has a known asymmetry
	// between boost and cut (paper Section 2.1), so exact inversion only
	// holds at DC and Nyquist. We test those plus well-separated frequencies.
	boost, err := ButterworthLowShelf(testSR, 1000, 12, 6)
	if err != nil {
		t.Fatal(err)
	}

	cut, err := ButterworthLowShelf(testSR, 1000, -12, 6)
	if err != nil {
		t.Fatal(err)
	}

	// DC and Nyquist: exact inversion.
	for _, freq := range []float64{1, testSR/2 - 1} {
		hBoost := cascadeResponse(boost, freq, testSR)
		hCut := cascadeResponse(cut, freq, testSR)
		hCombined := hBoost * hCut

		magDB := 20 * math.Log10(cmplx.Abs(hCombined))
		if math.Abs(magDB) > 0.1 {
			t.Errorf("freq=%v: boost*cut = %.4f dB, expected ~0 dB", freq, magDB)
		}
	}

	// Well away from transition: inversion is close.
	for _, freq := range []float64{50, 15000} {
		hBoost := cascadeResponse(boost, freq, testSR)
		hCut := cascadeResponse(cut, freq, testSR)
		hCombined := hBoost * hCut

		magDB := 20 * math.Log10(cmplx.Abs(hCombined))
		if math.Abs(magDB) > 0.5 {
			t.Errorf("freq=%v: boost*cut = %.4f dB, expected ~0 dB", freq, magDB)
		}
	}
}

func TestHighShelf_BoostCutInversion(t *testing.T) {
	boost, err := ButterworthHighShelf(testSR, 1000, 12, 6)
	if err != nil {
		t.Fatal(err)
	}

	cut, err := ButterworthHighShelf(testSR, 1000, -12, 6)
	if err != nil {
		t.Fatal(err)
	}

	for _, freq := range []float64{1, 50, 15000, testSR/2 - 1} {
		hBoost := cascadeResponse(boost, freq, testSR)
		hCut := cascadeResponse(cut, freq, testSR)
		hCombined := hBoost * hCut

		magDB := 20 * math.Log10(cmplx.Abs(hCombined))
		if math.Abs(magDB) > 0.5 {
			t.Errorf("freq=%v: boost*cut = %.4f dB, expected ~0 dB", freq, magDB)
		}
	}
}

// ============================================================
// Order sweep
// ============================================================

func TestLowShelf_VariousOrders(t *testing.T) {
	for _, M := range []int{1, 2, 3, 4, 5, 6, 8, 10, 12} {
		t.Run(orderName(M), func(t *testing.T) {
			sections, err := ButterworthLowShelf(testSR, 1000, 12, M)
			if err != nil {
				t.Fatal(err)
			}

			allPolesStable(t, sections)

			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			if !almostEqual(dcMag, 12, 0.1) {
				t.Errorf("M=%d: DC gain = %.4f dB, expected ~12 dB", M, dcMag)
			}

			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
			if math.Abs(nyqMag) > 0.1 {
				t.Errorf("M=%d: Nyquist gain = %.4f dB, expected ~0 dB", M, nyqMag)
			}
		})
	}
}

// ============================================================
// Frequency sweep
// ============================================================

func TestLowShelf_VariousFrequencies(t *testing.T) {
	for _, freq := range []float64{100, 300, 500, 1000, 2000, 5000, 10000} {
		t.Run(freqName(freq), func(t *testing.T) {
			sections, err := ButterworthLowShelf(testSR, freq, 12, 4)
			if err != nil {
				t.Fatal(err)
			}

			allPolesStable(t, sections)

			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			if !almostEqual(dcMag, 12, 0.1) {
				t.Errorf("freq=%v: DC gain = %.4f dB, expected ~12 dB", freq, dcMag)
			}
		})
	}
}

func TestHighShelf_VariousFrequencies(t *testing.T) {
	for _, freq := range []float64{100, 300, 500, 1000, 2000, 5000, 10000} {
		t.Run(freqName(freq), func(t *testing.T) {
			sections, err := ButterworthHighShelf(testSR, freq, 12, 4)
			if err != nil {
				t.Fatal(err)
			}

			allPolesStable(t, sections)

			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
			if !almostEqual(nyqMag, 12, 0.2) {
				t.Errorf("freq=%v: Nyquist gain = %.4f dB, expected ~12 dB", freq, nyqMag)
			}
		})
	}
}

// ============================================================
// Extreme gains
// ============================================================

func TestLowShelf_ExtremeGains(t *testing.T) {
	for _, gainDB := range []float64{-30, -20, -6, -1, 1, 6, 20, 30} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			sections, err := ButterworthLowShelf(testSR, 1000, gainDB, 4)
			if err != nil {
				t.Fatal(err)
			}

			allPolesStable(t, sections)

			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			if !almostEqual(dcMag, gainDB, 0.2) {
				t.Errorf("DC gain = %.4f dB, expected %.4f dB", dcMag, gainDB)
			}
		})
	}
}

// ============================================================
// Monotonicity (Butterworth property)
// ============================================================

func TestLowShelf_Monotonic(t *testing.T) {
	sections, err := ButterworthLowShelf(testSR, 1000, 12, 6)
	if err != nil {
		t.Fatal(err)
	}

	// For a boost low-shelf, magnitude should decrease monotonically
	// from DC to Nyquist.
	prevMag := cascadeMagnitudeDB(sections, 1, testSR)
	for f := 10.0; f < testSR/2-10; f += 10 {
		mag := cascadeMagnitudeDB(sections, f, testSR)
		if mag > prevMag+0.01 {
			t.Errorf("non-monotonic at %.0f Hz: %.4f dB > %.4f dB (prev)", f, mag, prevMag)
			break
		}

		prevMag = mag
	}
}

func TestHighShelf_Monotonic(t *testing.T) {
	sections, err := ButterworthHighShelf(testSR, 1000, 12, 6)
	if err != nil {
		t.Fatal(err)
	}

	// For a boost high-shelf, magnitude should increase monotonically
	// from DC to Nyquist.
	prevMag := cascadeMagnitudeDB(sections, 1, testSR)
	for f := 10.0; f < testSR/2-10; f += 10 {
		mag := cascadeMagnitudeDB(sections, f, testSR)
		if mag < prevMag-0.01 {
			t.Errorf("non-monotonic at %.0f Hz: %.4f dB < %.4f dB (prev)", f, mag, prevMag)
			break
		}

		prevMag = mag
	}
}

// ============================================================
// Chebyshev Type I: parameter validation
// ============================================================

func TestPaperDesignExample(t *testing.T) {
	// The paper uses fs=48kHz, f_B=300Hz, G=-5dB for a low-shelf.
	// K = tan(pi * 300 / 48000) = tan(0.019635) ~ 0.019637
	// Verify K computation.
	K := math.Tan(math.Pi * 300 / 48000)
	if !almostEqual(K, 0.019637, 1e-4) {
		t.Errorf("K = %v, expected ~0.019637", K)
	}

	// Verify M=1 and M=6 produce valid filters.
	for _, M := range []int{1, 2, 6} {
		t.Run(orderName(M), func(t *testing.T) {
			sections, err := ButterworthLowShelf(48000, 300, -5, M)
			if err != nil {
				t.Fatal(err)
			}

			allPolesStable(t, sections)

			dcMag := cascadeMagnitudeDB(sections, 1, 48000)
			if !almostEqual(dcMag, -5, 0.1) {
				t.Errorf("DC gain = %.4f dB, expected -5 dB", dcMag)
			}
		})
	}
}

// ============================================================
// Chebyshev Type II: parameter validation
// ============================================================
