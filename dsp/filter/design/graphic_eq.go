package design

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design/shelving"
)

// GraphicEQ designs a graphic equalizer with complementary Butterworth shelf
// transitions between adjacent bands. Equal neighboring gains create no extra
// filter; equal gains across all bands produce a frequency-independent gain.
// The first and last gains extend to DC and Nyquist respectively.
//
// Centers must contain 1–64 strictly increasing frequencies between DC and
// Nyquist, with one finite gain in [-48, 48] dB per center. Order (1–32) controls
// the transition steepness. Each transition's half-dB-gain frequency is the
// geometric mean of its adjacent centers. Returned coefficients are suitable
// for a biquad.Chain; no sample processing occurs during design.
func GraphicEQ(sampleRate float64, centers, gainsDB []float64, order int) ([]biquad.Coefficients, error) {
	if math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) || sampleRate <= 0 ||
		len(centers) == 0 || len(centers) > 64 || len(centers) != len(gainsDB) || order < 1 || order > 32 {
		return nil, fmt.Errorf("graphic EQ: invalid geometry")
	}

	for i, hz := range centers {
		if math.IsNaN(hz) || hz <= 0 || hz >= sampleRate/2 || (i > 0 && hz <= centers[i-1]) ||
			math.IsNaN(gainsDB[i]) || math.IsInf(gainsDB[i], 0) || math.Abs(gainsDB[i]) > 48 {
			return nil, fmt.Errorf("graphic EQ: invalid band %d", i+1)
		}
	}

	coeffs := []biquad.Coefficients{{B0: math.Pow(10, gainsDB[0]/20)}}
	for i := 1; i < len(centers); i++ {
		delta := gainsDB[i] - gainsDB[i-1]
		if delta == 0 {
			continue
		}

		boundary := math.Sqrt(centers[i-1]) * math.Sqrt(centers[i])
		// The shelf's analog poles/zeros differ by P=10^(delta/(20*order)).
		// Center them around the prewarped boundary using sqrt(P), so boost
		// and cut have reciprocal responses and cross at exactly delta/2 dB.
		warped := math.Tan(math.Pi*boundary/sampleRate) * math.Pow(10, delta/(40*float64(order)))
		frequency := sampleRate / math.Pi * math.Atan(warped)

		sections, err := shelving.ButterworthHighShelf(sampleRate, frequency, delta, order)
		if err != nil {
			return nil, fmt.Errorf("graphic EQ: transition %d: %w", i, err)
		}

		coeffs = append(coeffs, sections...)
	}

	return coeffs, nil
}
