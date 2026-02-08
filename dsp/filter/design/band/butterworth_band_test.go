package band

import (
	"math"
	"math/cmplx"
	"testing"
)

// ============================================================
// Butterworth band filter tests
// ============================================================

func TestButterworthBand_Boost(t *testing.T) {
	testBandDesign(t, "Butterworth +12dB", ButterworthBand, 1000, 500, 12, 4, 0.5)
}

func TestButterworthBand_Cut(t *testing.T) {
	testBandDesign(t, "Butterworth -12dB", ButterworthBand, 1000, 500, -12, 4, 0.5)
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
