package shelving

import (
	"math"
	"testing"
)

// ============================================================
// Chebyshev Type I shelving filter tests
// ============================================================

func TestChebyshev1LowShelf_InvalidParams(t *testing.T) {
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
			_, err := Chebyshev1LowShelf(tt.sr, tt.freq, tt.gainDB, tt.rippleDB, tt.order)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}


func TestChebyshev1HighShelf_InvalidParams(t *testing.T) {
	_, err := Chebyshev1HighShelf(0, 1000, 6, 0.5, 2)
	if err == nil {
		t.Error("expected error for zero sample rate")
	}
	_, err = Chebyshev1HighShelf(48000, 1000, 6, 0, 2)
	if err == nil {
		t.Error("expected error for zero ripple")
	}
}

// ============================================================
// Chebyshev Type I: passthrough at zero gain
// ============================================================


func TestChebyshev1LowShelf_ZeroGain(t *testing.T) {
	sections, err := Chebyshev1LowShelf(testSR, 1000, 0, 0.5, 4)
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
// Chebyshev Type I: section count (same as Butterworth)
// ============================================================


func TestChebyshev1LowShelf_SectionCount(t *testing.T) {
	for _, M := range []int{1, 2, 3, 4, 5, 6, 7, 8} {
		t.Run(orderName(M), func(t *testing.T) {
			sections, err := Chebyshev1LowShelf(testSR, 1000, 6, 0.5, M)
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
// Chebyshev Type I: DC and Nyquist gain accuracy
// ============================================================


func TestChebyshev1LowShelf_DCGain(t *testing.T) {
	for _, gainDB := range []float64{-12, -6, 6, 12, 20} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			sections, err := Chebyshev1LowShelf(testSR, 1000, gainDB, 0.5, 4)
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


func TestChebyshev1LowShelf_NyquistGain(t *testing.T) {
	for _, gainDB := range []float64{-12, -6, 6, 12} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			sections, err := Chebyshev1LowShelf(testSR, 1000, gainDB, 0.5, 4)
			if err != nil {
				t.Fatal(err)
			}
			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
			if math.Abs(nyqMag) > 0.2 {
				t.Errorf("Nyquist gain = %.4f dB, expected ~0 dB", nyqMag)
			}
		})
	}
}


func TestChebyshev1HighShelf_NyquistGain(t *testing.T) {
	for _, gainDB := range []float64{-12, -6, 6, 12, 20} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			sections, err := Chebyshev1HighShelf(testSR, 1000, gainDB, 0.5, 4)
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


func TestChebyshev1HighShelf_DCGain(t *testing.T) {
	for _, gainDB := range []float64{-12, -6, 6, 12} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			sections, err := Chebyshev1HighShelf(testSR, 1000, gainDB, 0.5, 4)
			if err != nil {
				t.Fatal(err)
			}
			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			if math.Abs(dcMag) > 0.2 {
				t.Errorf("DC gain = %.4f dB, expected ~0 dB", dcMag)
			}
		})
	}
}

// ============================================================
// Chebyshev Type I: pole stability
// ============================================================


func TestChebyshev1LowShelf_Stability(t *testing.T) {
	for _, M := range []int{1, 2, 3, 4, 5, 6, 8, 10, 12} {
		t.Run(orderName(M), func(t *testing.T) {
			sections, err := Chebyshev1LowShelf(testSR, 1000, 12, 0.5, M)
			if err != nil {
				t.Fatal(err)
			}
			allPolesStable(t, sections)
		})
	}
}


func TestChebyshev1HighShelf_Stability(t *testing.T) {
	for _, M := range []int{1, 2, 3, 4, 5, 6, 8, 10, 12} {
		t.Run(orderName(M), func(t *testing.T) {
			sections, err := Chebyshev1HighShelf(testSR, 1000, 12, 0.5, M)
			if err != nil {
				t.Fatal(err)
			}
			allPolesStable(t, sections)
		})
	}
}

// ============================================================
// Chebyshev Type I: order sweep
// ============================================================


func TestChebyshev1LowShelf_VariousOrders(t *testing.T) {
	for _, M := range []int{1, 2, 3, 4, 5, 6, 8, 10, 12} {
		t.Run(orderName(M), func(t *testing.T) {
			sections, err := Chebyshev1LowShelf(testSR, 1000, 12, 0.5, M)
			if err != nil {
				t.Fatal(err)
			}
			allPolesStable(t, sections)
			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			if !almostEqual(dcMag, 12, 0.2) {
				t.Errorf("M=%d: DC gain = %.4f dB, expected ~12 dB", M, dcMag)
			}
			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
			if math.Abs(nyqMag) > 0.2 {
				t.Errorf("M=%d: Nyquist gain = %.4f dB, expected ~0 dB", M, nyqMag)
			}
		})
	}
}

// ============================================================
// Chebyshev Type I: steeper transition than Butterworth
// ============================================================


func TestChebyshev1_SteeperTransition(t *testing.T) {
	// For the same order, Chebyshev I should have a steeper transition.
	// We verify this by comparing magnitude at a frequency in the transition
	// region: the Chebyshev I filter should be closer to the shelf gain
	// (for boost low-shelf, higher magnitude in the transition).
	order := 6
	gainDB := 12.0
	freq := 1000.0

	bw, err := ButterworthLowShelf(testSR, freq, gainDB, order)
	if err != nil {
		t.Fatal(err)
	}
	ch, err := Chebyshev1LowShelf(testSR, freq, gainDB, 1.0, order)
	if err != nil {
		t.Fatal(err)
	}

	// Check at a frequency slightly below cutoff (still in the shelf region).
	// Chebyshev should be closer to the full shelf gain.
	fTest := freq * 0.7
	bwMag := cascadeMagnitudeDB(bw, fTest, testSR)
	chMag := cascadeMagnitudeDB(ch, fTest, testSR)

	// For a boost low-shelf, both should be positive and Chebyshev should
	// be higher (closer to 12 dB) in the transition region.
	if chMag <= bwMag {
		t.Errorf("expected Chebyshev steeper: cheby=%.4f dB, butter=%.4f dB at %.0f Hz",
			chMag, bwMag, fTest)
	}
}

// ============================================================
// Chebyshev Type I: extreme gains
// ============================================================


func TestChebyshev1LowShelf_ExtremeGains(t *testing.T) {
	for _, gainDB := range []float64{-30, -20, -6, -1, 1, 6, 20, 30} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			sections, err := Chebyshev1LowShelf(testSR, 1000, gainDB, 0.5, 4)
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
// Chebyshev Type I: various ripple values
// ============================================================


func TestChebyshev1LowShelf_VariousRipple(t *testing.T) {
	ripples := []float64{0.1, 0.25, 0.5, 1.0, 2.0, 3.0}
	for _, rip := range ripples {
		name := ftoa(rip) + "dBripple"
		t.Run(name, func(t *testing.T) {
			sections, err := Chebyshev1LowShelf(testSR, 1000, 12, rip, 6)
			if err != nil {
				t.Fatal(err)
			}
			allPolesStable(t, sections)
			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			if !almostEqual(dcMag, 12, 0.5) {
				t.Errorf("ripple=%.1f: DC gain = %.4f dB, expected ~12 dB", rip, dcMag)
			}
			nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
			if math.Abs(nyqMag) > 0.5 {
				t.Errorf("ripple=%.1f: Nyquist gain = %.4f dB, expected ~0 dB", rip, nyqMag)
			}
		})
	}
}

// ============================================================
// Chebyshev Type I: frequency sweep
// ============================================================


func TestChebyshev1LowShelf_VariousFrequencies(t *testing.T) {
	for _, freq := range []float64{100, 300, 500, 1000, 2000, 5000, 10000} {
		t.Run(freqName(freq), func(t *testing.T) {
			sections, err := Chebyshev1LowShelf(testSR, freq, 12, 0.5, 4)
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

// ============================================================
// Paper design example verification (Section 5)
// ============================================================


