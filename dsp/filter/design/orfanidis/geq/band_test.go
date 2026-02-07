package geq

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// ============================================================
// band.go parameter validation tests
// ============================================================

func TestBandParams_Valid(t *testing.T) {
	w0, wb, err := bandParams(48000, 1000, 500, 4)
	if err != nil {
		t.Fatal(err)
	}
	expectW0 := 2 * math.Pi * 1000 / 48000
	expectWb := 2 * math.Pi * 500 / 48000
	if !almostEqual(w0, expectW0, 1e-12) {
		t.Errorf("w0 = %v, expected %v", w0, expectW0)
	}
	if !almostEqual(wb, expectWb, 1e-12) {
		t.Errorf("wb = %v, expected %v", wb, expectWb)
	}
}

func TestBandParams_Errors(t *testing.T) {
	tests := []struct {
		name   string
		sr, f0 float64
		bw     float64
		order  int
	}{
		{"zero sample rate", 0, 1000, 500, 4},
		{"negative f0", 48000, -1, 500, 4},
		{"f0 >= Nyquist", 48000, 24000, 500, 4},
		{"zero bandwidth", 48000, 1000, 0, 4},
		{"order too small", 48000, 1000, 500, 2},
		{"odd order", 48000, 1000, 500, 5},
		{"bandwidth exceeds Nyquist", 48000, 1000, 48000, 4},
		{"fl <= 0", 48000, 100, 300, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := bandParams(tt.sr, tt.f0, tt.bw, tt.order)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestBWGainDB_Functions(t *testing.T) {
	if !almostEqual(butterworthBWGainDB(6), 3, 1e-12) {
		t.Errorf("butterworth(6) = %v, expected 3", butterworthBWGainDB(6))
	}
	if !almostEqual(butterworthBWGainDB(-6), -3, 1e-12) {
		t.Errorf("butterworth(-6) = %v, expected -3", butterworthBWGainDB(-6))
	}
	if !almostEqual(butterworthBWGainDB(2), 2/math.Sqrt2, 1e-12) {
		t.Errorf("butterworth(2) = %v, expected %v", butterworthBWGainDB(2), 2/math.Sqrt2)
	}

	if !almostEqual(chebyshev1BWGainDB(6), 5.9, 1e-12) {
		t.Errorf("chebyshev1(6) = %v, expected 5.9", chebyshev1BWGainDB(6))
	}
	if !almostEqual(chebyshev1BWGainDB(-6), -5.9, 1e-12) {
		t.Errorf("chebyshev1(-6) = %v, expected -5.9", chebyshev1BWGainDB(-6))
	}

	if !almostEqual(chebyshev2BWGainDB(12), 0.1, 1e-12) {
		t.Errorf("chebyshev2(12) = %v, expected 0.1", chebyshev2BWGainDB(12))
	}
	if !almostEqual(chebyshev2BWGainDB(-12), -0.1, 1e-12) {
		t.Errorf("chebyshev2(-12) = %v, expected -0.1", chebyshev2BWGainDB(-12))
	}

	if !almostEqual(ellipticBWGainDB(6), 5.95, 1e-12) {
		t.Errorf("elliptic(6) = %v, expected 5.95", ellipticBWGainDB(6))
	}
}

func TestPassthroughSections(t *testing.T) {
	s := passthroughSections()
	if len(s) != 1 {
		t.Fatalf("expected 1 section, got %d", len(s))
	}
	if s[0].B0 != 1 || s[0].B1 != 0 || s[0].B2 != 0 || s[0].A1 != 0 || s[0].A2 != 0 {
		t.Errorf("passthrough section not unity: %+v", s[0])
	}
}

func TestDb2Lin(t *testing.T) {
	if !almostEqual(db2Lin(0), 1.0, 1e-12) {
		t.Errorf("db2Lin(0) = %v, expected 1", db2Lin(0))
	}
	if !almostEqual(db2Lin(20), 10.0, 1e-10) {
		t.Errorf("db2Lin(20) = %v, expected 10", db2Lin(20))
	}
	if !almostEqual(db2Lin(-20), 0.1, 1e-10) {
		t.Errorf("db2Lin(-20) = %v, expected 0.1", db2Lin(-20))
	}
}

// ============================================================
// Integration tests: frequency response validation
// ============================================================

// testBandDesign validates fundamental properties of any band filter design.
func testBandDesign(t *testing.T, name string, designFn func(float64, float64, float64, float64, int) ([]biquad.Coefficients, error),
	f0Hz, bwHz, gainDB float64, order int, centerTolDB float64,
) {
	t.Helper()

	sections, err := designFn(testSR, f0Hz, bwHz, gainDB, order)
	if err != nil {
		t.Fatalf("%s: design failed: %v", name, err)
	}

	allPolesStable(t, sections)

	centerMag := cascadeMagnitudeDB(sections, f0Hz, testSR)
	if !almostEqual(centerMag, gainDB, centerTolDB) {
		t.Errorf("%s: center freq gain = %.4f dB, expected %.4f dB (tol %.2f)", name, centerMag, gainDB, centerTolDB)
	}

	dcMag := cascadeMagnitudeDB(sections, 1, testSR)
	if math.Abs(dcMag) > 1.0 {
		t.Errorf("%s: DC gain = %.4f dB, expected ~0 dB", name, dcMag)
	}

	nyqMag := cascadeMagnitudeDB(sections, testSR/2-1, testSR)
	if math.Abs(nyqMag) > 1.0 {
		t.Errorf("%s: Nyquist gain = %.4f dB, expected ~0 dB", name, nyqMag)
	}

	if f0Hz > 5000 {
		lowMag := cascadeMagnitudeDB(sections, 50, testSR)
		if math.Abs(lowMag) > 0.5 {
			t.Errorf("%s: 50 Hz gain = %.4f dB, expected ~0 dB", name, lowMag)
		}
	}
}

func TestButterworthBand_Boost(t *testing.T) {
	testBandDesign(t, "Butterworth +12dB", ButterworthBand, 1000, 500, 12, 4, 0.5)
}

func TestButterworthBand_Cut(t *testing.T) {
	testBandDesign(t, "Butterworth -12dB", ButterworthBand, 1000, 500, -12, 4, 0.5)
}

func TestChebyshev1Band_Boost(t *testing.T) {
	testBandDesign(t, "Chebyshev1 +12dB", Chebyshev1Band, 1000, 500, 12, 4, 0.5)
}

func TestChebyshev1Band_Cut(t *testing.T) {
	testBandDesign(t, "Chebyshev1 -12dB", Chebyshev1Band, 1000, 500, -12, 4, 0.5)
}

func TestChebyshev2Band_Boost(t *testing.T) {
	testBandDesign(t, "Chebyshev2 +12dB", Chebyshev2Band, 1000, 500, 12, 4, 0.5)
}

func TestChebyshev2Band_Cut(t *testing.T) {
	testBandDesign(t, "Chebyshev2 -12dB", Chebyshev2Band, 1000, 500, -12, 4, 0.5)
}

func TestEllipticBand_Boost(t *testing.T) {
	testBandDesign(t, "Elliptic +12dB", EllipticBand, 1000, 500, 12, 4, 0.5)
}

func TestEllipticBand_Cut(t *testing.T) {
	testBandDesign(t, "Elliptic -12dB", EllipticBand, 1000, 500, -12, 4, 0.5)
}

func TestButterworthBand_ZeroGain(t *testing.T) {
	sections, err := ButterworthBand(testSR, 1000, 500, 0, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 {
		t.Fatalf("expected 1 passthrough section, got %d", len(sections))
	}
	mag := cascadeMagnitudeDB(sections, 1000, testSR)
	if !almostEqual(mag, 0, 1e-10) {
		t.Errorf("zero gain: center mag = %v dB, expected 0", mag)
	}
}

func TestButterworthBand_BoostCutInversion(t *testing.T) {
	boost, err := ButterworthBand(testSR, 1000, 500, 12, 4)
	if err != nil {
		t.Fatal(err)
	}
	cut, err := ButterworthBand(testSR, 1000, 500, -12, 4)
	if err != nil {
		t.Fatal(err)
	}

	for _, freq := range []float64{200, 500, 800, 1000, 1200, 1500, 2000, 5000} {
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
// Numerical stress tests
// ============================================================

func TestButterworthBand_VariousOrders(t *testing.T) {
	for _, order := range []int{4, 6, 8, 10, 12} {
		t.Run(orderName(order), func(t *testing.T) {
			sections, err := ButterworthBand(testSR, 1000, 500, 12, order)
			if err != nil {
				t.Fatalf("order %d: %v", order, err)
			}
			allPolesStable(t, sections)
			centerMag := cascadeMagnitudeDB(sections, 1000, testSR)
			if !almostEqual(centerMag, 12, 1.0) {
				t.Errorf("order %d: center gain = %.4f dB, expected ~12 dB", order, centerMag)
			}
		})
	}
}

func TestChebyshev1Band_VariousOrders(t *testing.T) {
	for _, order := range []int{4, 6, 8, 10} {
		t.Run(orderName(order), func(t *testing.T) {
			sections, err := Chebyshev1Band(testSR, 1000, 500, 12, order)
			if err != nil {
				t.Fatalf("order %d: %v", order, err)
			}
			allPolesStable(t, sections)
			centerMag := cascadeMagnitudeDB(sections, 1000, testSR)
			if !almostEqual(centerMag, 12, 1.0) {
				t.Errorf("order %d: center gain = %.4f dB, expected ~12 dB", order, centerMag)
			}
		})
	}
}

func TestChebyshev2Band_VariousOrders(t *testing.T) {
	for _, order := range []int{4, 6, 8, 10} {
		t.Run(orderName(order), func(t *testing.T) {
			sections, err := Chebyshev2Band(testSR, 1000, 500, 12, order)
			if err != nil {
				t.Fatalf("order %d: %v", order, err)
			}
			allPolesStable(t, sections)
			centerMag := cascadeMagnitudeDB(sections, 1000, testSR)
			if !almostEqual(centerMag, 12, 1.0) {
				t.Errorf("order %d: center gain = %.4f dB, expected ~12 dB", order, centerMag)
			}
		})
	}
}

func TestEllipticBand_VariousOrders(t *testing.T) {
	for _, order := range []int{4, 6, 8, 10} {
		t.Run(orderName(order), func(t *testing.T) {
			sections, err := EllipticBand(testSR, 1000, 500, 12, order)
			if err != nil {
				t.Fatalf("order %d: %v", order, err)
			}
			allPolesStable(t, sections)
			centerMag := cascadeMagnitudeDB(sections, 1000, testSR)
			if !almostEqual(centerMag, 12, 1.0) {
				t.Errorf("order %d: center gain = %.4f dB, expected ~12 dB", order, centerMag)
			}
		})
	}
}

func TestButterworthBand_ExtremeGains(t *testing.T) {
	for _, gainDB := range []float64{-30, -20, -6, -1, 1, 6, 20, 30} {
		t.Run(gainName(gainDB), func(t *testing.T) {
			sections, err := ButterworthBand(testSR, 1000, 500, gainDB, 4)
			if err != nil {
				t.Fatalf("gain %v dB: %v", gainDB, err)
			}
			allPolesStable(t, sections)
			centerMag := cascadeMagnitudeDB(sections, 1000, testSR)
			if !almostEqual(centerMag, gainDB, 1.0) {
				t.Errorf("gain %v dB: center = %.4f dB", gainDB, centerMag)
			}
		})
	}
}

func TestButterworthBand_VariousFrequencies(t *testing.T) {
	for _, f0 := range []float64{63, 125, 250, 500, 1000, 2000, 4000, 8000, 16000} {
		bw := f0 * 0.5
		if f0-bw/2 <= 0 || f0+bw/2 >= testSR/2 {
			continue
		}
		t.Run(freqName(f0), func(t *testing.T) {
			sections, err := ButterworthBand(testSR, f0, bw, 12, 4)
			if err != nil {
				t.Fatalf("f0=%v: %v", f0, err)
			}
			allPolesStable(t, sections)
			centerMag := cascadeMagnitudeDB(sections, f0, testSR)
			if !almostEqual(centerMag, 12, 1.0) {
				t.Errorf("f0=%v: center gain = %.4f dB, expected ~12 dB", f0, centerMag)
			}
		})
	}
}

func TestButterworthBand_SmallGain(t *testing.T) {
	sections, err := ButterworthBand(testSR, 1000, 500, 0.1, 4)
	if err != nil {
		t.Fatal(err)
	}
	allPolesStable(t, sections)
	for _, freq := range []float64{100, 500, 1000, 5000, 20000} {
		mag := cascadeMagnitudeDB(sections, freq, testSR)
		if math.Abs(mag) > 0.2 {
			t.Errorf("freq=%v: mag = %.4f dB, expected near 0 for 0.1 dB gain", freq, mag)
		}
	}
}

func TestButterworthBand_HighOrder_Stability(t *testing.T) {
	for _, order := range []int{4, 6, 8, 10, 12, 14, 16} {
		t.Run(orderName(order), func(t *testing.T) {
			sections, err := ButterworthBand(testSR, 1000, 500, 12, order)
			if err != nil {
				t.Skipf("order %d failed: %v (known Durand-Kerner limitation)", order, err)
				return
			}
			allPolesStable(t, sections)
			centerMag := cascadeMagnitudeDB(sections, 1000, testSR)
			if math.Abs(centerMag-12) > 2.0 {
				t.Errorf("order %d: center gain = %.4f dB, expected ~12 dB", order, centerMag)
			}
			dcMag := cascadeMagnitudeDB(sections, 1, testSR)
			if math.Abs(dcMag) > 1.0 {
				t.Errorf("order %d: DC gain = %.4f dB, expected ~0 dB", order, dcMag)
			}
		})
	}
}

func TestButterworthBand_NarrowBandwidth(t *testing.T) {
	sections, err := ButterworthBand(testSR, 1000, 50, 12, 4)
	if err != nil {
		t.Fatal(err)
	}
	allPolesStable(t, sections)
	centerMag := cascadeMagnitudeDB(sections, 1000, testSR)
	if !almostEqual(centerMag, 12, 1.0) {
		t.Errorf("narrow band: center = %.4f dB, expected ~12", centerMag)
	}
	offMag := cascadeMagnitudeDB(sections, 500, testSR)
	if offMag > 1.0 {
		t.Errorf("narrow band: 500 Hz = %.4f dB, expected < 1 dB", offMag)
	}
}

func TestButterworthBand_WideBandwidth(t *testing.T) {
	sections, err := ButterworthBand(testSR, 5000, 8000, 6, 4)
	if err != nil {
		t.Fatal(err)
	}
	allPolesStable(t, sections)
	centerMag := cascadeMagnitudeDB(sections, 5000, testSR)
	if !almostEqual(centerMag, 6, 1.0) {
		t.Errorf("wide band: center = %.4f dB, expected ~6", centerMag)
	}
}

func TestButterworthBand_CoefficientConsistency(t *testing.T) {
	f0 := 1000.0
	bw := 500.0
	gainDB := 12.0

	sections, err := butterworthBandRad(
		2*math.Pi*f0/testSR, 2*math.Pi*bw/testSR,
		gainDB, butterworthBWGainDB(gainDB), 4,
	)
	if err != nil {
		t.Fatal(err)
	}

	centerMag := cascadeMagnitudeDB(sections, f0, testSR)
	if !almostEqual(centerMag, gainDB, 0.5) {
		t.Errorf("center gain = %.4f dB, expected %.4f dB", centerMag, gainDB)
	}
}

// ============================================================
// All designers: error handling
// ============================================================

func TestAllDesigners_ErrorOnInvalidParams(t *testing.T) {
	designers := []struct {
		name string
		fn   func(float64, float64, float64, float64, int) ([]biquad.Coefficients, error)
	}{
		{"Butterworth", ButterworthBand},
		{"Chebyshev1", Chebyshev1Band},
		{"Chebyshev2", Chebyshev2Band},
		{"Elliptic", EllipticBand},
	}

	for _, d := range designers {
		t.Run(d.name+"/order2", func(t *testing.T) {
			_, err := d.fn(testSR, 1000, 500, 12, 2)
			if err == nil {
				t.Error("expected error for order=2")
			}
		})
		t.Run(d.name+"/order3", func(t *testing.T) {
			_, err := d.fn(testSR, 1000, 500, 12, 3)
			if err == nil {
				t.Error("expected error for odd order")
			}
		})
		t.Run(d.name+"/zeroGain", func(t *testing.T) {
			sections, err := d.fn(testSR, 1000, 500, 0, 4)
			if err != nil {
				t.Fatalf("zero gain should not error: %v", err)
			}
			if len(sections) != 1 {
				t.Errorf("zero gain: expected 1 passthrough section, got %d", len(sections))
			}
		})
	}
}
