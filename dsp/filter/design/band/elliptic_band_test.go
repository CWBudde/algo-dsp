package band

import (
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// ============================================================
// Elliptic band filter tests
// ============================================================

func TestEllipticBand_Boost(t *testing.T) {
	testBandDesign(t, "Elliptic +12dB", EllipticBand, 1000, 500, 12, 4, 0.5)
}

func TestEllipticBand_Cut(t *testing.T) {
	testBandDesign(t, "Elliptic -12dB", EllipticBand, 1000, 500, -12, 4, 0.5)
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
