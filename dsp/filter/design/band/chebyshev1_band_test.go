package band

import (
	"testing"
)

// ============================================================
// Chebyshev Type I band filter tests
// ============================================================

func TestChebyshev1Band_Boost(t *testing.T) {
	testBandDesign(t, "Chebyshev1 +12dB", Chebyshev1Band, 1000, 500, 12, 4, 0.5)
}

func TestChebyshev1Band_Cut(t *testing.T) {
	testBandDesign(t, "Chebyshev1 -12dB", Chebyshev1Band, 1000, 500, -12, 4, 0.5)
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
