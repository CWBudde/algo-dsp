package band

import (
	"testing"
)

// ============================================================
// Chebyshev Type II band filter tests
// ============================================================

func TestChebyshev2Band_Boost(t *testing.T) {
	testBandDesign(t, "Chebyshev2 +12dB", Chebyshev2Band, 1000, 500, 12, 4, 0.5)
}

func TestChebyshev2Band_Cut(t *testing.T) {
	testBandDesign(t, "Chebyshev2 -12dB", Chebyshev2Band, 1000, 500, -12, 4, 0.5)
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
