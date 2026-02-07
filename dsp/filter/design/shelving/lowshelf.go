package shelving

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// lowShelfSections computes the Butterworth low-shelf biquad cascade.
//
// K is the pre-warped frequency parameter tan(Omega_B / 2).
// P is the per-section gain factor g^(1/M), where g is the linear gain.
// M is the filter order (>= 1).
//
// For even M, the result contains M/2 second-order sections.
// For odd M, it contains (M-1)/2 second-order sections plus one first-order section.
func lowShelfSections(K, P float64, M int) []biquad.Coefficients {
	L := M / 2
	hasFirstOrder := M%2 == 1
	n := L
	if hasFirstOrder {
		n++
	}
	sections := make([]biquad.Coefficients, 0, n)

	KP := K * P
	K2 := K * K
	KP2 := KP * KP

	// Second-order sections for each conjugate pair.
	for m := 1; m <= L; m++ {
		cm := math.Cos((0.5 - (2.0*float64(m)-1.0)/(2.0*float64(M))) * math.Pi)

		D := 1.0 + 2.0*K*cm + K2
		invD := 1.0 / D

		sections = append(sections, biquad.Coefficients{
			B0: (1.0 + 2.0*KP*cm + KP2) * invD,
			B1: (2.0*KP2 - 2.0) * invD,
			B2: (1.0 - 2.0*KP*cm + KP2) * invD,
			A1: (2.0*K2 - 2.0) * invD,
			A2: (1.0 - 2.0*K*cm + K2) * invD,
		})
	}

	// First-order section for odd order.
	if hasFirstOrder {
		D := 1.0 + K
		invD := 1.0 / D

		sections = append(sections, biquad.Coefficients{
			B0: (1.0 + KP) * invD,
			B1: (KP - 1.0) * invD,
			B2: 0,
			A1: (K - 1.0) * invD,
			A2: 0,
		})
	}

	return sections
}

// ln10over20 is the precomputed constant ln(10)/20.
const ln10over20 = 0.11512925464970228

func db2Lin(db float64) float64 {
	return math.Exp(db * ln10over20)
}
