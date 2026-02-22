package shelving

import (
	"errors"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// ErrInvalidParams is returned when filter parameters are out of range.
var ErrInvalidParams = errors.New("shelving: invalid parameters")

// validateParams checks common parameter constraints for shelving filters.
func validateParams(sampleRate, freqHz float64, order int) error {
	if sampleRate <= 0 || freqHz <= 0 || order < 1 {
		return ErrInvalidParams
	}

	if freqHz >= sampleRate*0.5 {
		return ErrInvalidParams
	}

	return nil
}

// passthroughSections returns a single passthrough section (unity gain).
func passthroughSections() []biquad.Coefficients {
	return []biquad.Coefficients{{B0: 1, B1: 0, B2: 0, A1: 0, A2: 0}}
}

// negateOddPowers converts a low-shelf cascade to high-shelf by applying
// H_HS(z) = H_LS(-z): negate odd-power z^{-1} coefficients.
func negateOddPowers(sections []biquad.Coefficients) {
	for i := range sections {
		sections[i].B1 = -sections[i].B1
		sections[i].A1 = -sections[i].A1
	}
}

// invertSections builds a cascade whose transfer function is the exact
// reciprocal of the input cascade, section-by-section.
func invertSections(sections []biquad.Coefficients) ([]biquad.Coefficients, error) {
	if len(sections) == 0 {
		return nil, ErrInvalidParams
	}

	out := make([]biquad.Coefficients, len(sections))
	for i, s := range sections {
		if s.B0 == 0 || math.IsNaN(s.B0) || math.IsInf(s.B0, 0) {
			return nil, ErrInvalidParams
		}

		invB0 := 1.0 / s.B0

		inv := biquad.Coefficients{
			B0: invB0,
			B1: s.A1 * invB0,
			B2: s.A2 * invB0,
			A1: s.B1 * invB0,
			A2: s.B2 * invB0,
		}
		if !coeffsAreFinite(inv) {
			return nil, ErrInvalidParams
		}

		out[i] = inv
	}

	return out, nil
}
