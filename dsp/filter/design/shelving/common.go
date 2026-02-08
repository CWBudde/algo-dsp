package shelving

import (
	"errors"

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
