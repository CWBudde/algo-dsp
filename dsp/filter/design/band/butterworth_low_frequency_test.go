package band

import (
	"math"
	"testing"
)

func TestButterworthBandLowFrequencyAtHighRate(t *testing.T) {
	for _, order := range []int{4, 6, 8, 10, 12} {
		for _, gain := range []float64{-24, -6, 0.1, 6, 24} {
			sections, err := ButterworthBand(384000, 31.5, 31.5*0.707, gain, order)
			if err != nil {
				t.Fatal(err)
			}

			allPolesStable(t, sections)

			center := cascadeMagnitudeDB(sections, 31.5, 384000)
			if math.Abs(center-gain) > 1e-5 {
				t.Errorf("order%d gain%g center%g", order, gain, center)
			}

			for _, frequency := range []float64{0, 192000} {
				if db := cascadeMagnitudeDB(sections, frequency, 384000); math.Abs(db) > 1e-5 {
					t.Errorf("endpoint%gHz response%g dB", frequency, db)
				}
			}
		}
	}
}
