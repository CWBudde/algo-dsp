package shelving

import (
	"errors"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// ErrInvalidParams is returned when filter parameters are out of range.
var ErrInvalidParams = errors.New("shelving: invalid parameters")

// ButterworthLowShelf designs an M-th order Butterworth low-shelving filter.
//
// freqHz is the cutoff frequency in Hz. gainDB is the shelf gain in dB
// (positive for boost, negative for cut). order must be >= 1.
// Returns a cascade of biquad sections.
func ButterworthLowShelf(sampleRate, freqHz, gainDB float64, order int) ([]biquad.Coefficients, error) {
	if err := validateParams(sampleRate, freqHz, order); err != nil {
		return nil, err
	}
	if gainDB == 0 {
		return passthroughSections(), nil
	}

	g := db2Lin(gainDB)
	P := math.Pow(g, 1.0/float64(order))
	K := math.Tan(math.Pi * freqHz / sampleRate)

	return lowShelfSections(K, P, order), nil
}

// ButterworthHighShelf designs an M-th order Butterworth high-shelving filter.
//
// freqHz is the cutoff frequency in Hz. gainDB is the shelf gain in dB
// (positive for boost, negative for cut). order must be >= 1.
// Returns a cascade of biquad sections.
func ButterworthHighShelf(sampleRate, freqHz, gainDB float64, order int) ([]biquad.Coefficients, error) {
	if err := validateParams(sampleRate, freqHz, order); err != nil {
		return nil, err
	}
	if gainDB == 0 {
		return passthroughSections(), nil
	}

	g := db2Lin(gainDB)
	P := math.Pow(g, 1.0/float64(order))
	K := 1.0 / math.Tan(math.Pi*freqHz/sampleRate)

	sections := lowShelfSections(K, P, order)

	// H_HS(z) = H_LS(-z): negate odd-power z^{-1} coefficients.
	for i := range sections {
		sections[i].B1 = -sections[i].B1
		sections[i].A1 = -sections[i].A1
	}

	return sections, nil
}

func validateParams(sampleRate, freqHz float64, order int) error {
	if sampleRate <= 0 || freqHz <= 0 || order < 1 {
		return ErrInvalidParams
	}
	if freqHz >= sampleRate*0.5 {
		return ErrInvalidParams
	}
	return nil
}

func passthroughSections() []biquad.Coefficients {
	return []biquad.Coefficients{{B0: 1, B1: 0, B2: 0, A1: 0, A2: 0}}
}
