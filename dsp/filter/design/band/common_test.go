package band

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// ============================================================
// Common band filter test helpers
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

func TestBandDesign_NonDefaultCenterFrequency(t *testing.T) {
	testBandDesign(t, "Butterworth +6dB @2k", ButterworthBand, 2000, 700, 6, 6, 0.7)
}
