package shelving

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// ButterworthLowShelf designs an M-th order Butterworth low-shelving filter.
//
// freqHz is the cutoff frequency in Hz. gainDB is the shelf gain in dB
// (positive for boost, negative for cut). order must be >= 1.
// Returns a cascade of biquad sections.
func ButterworthLowShelf(sampleRate, freqHz, gainDB float64, order int) ([]biquad.Coefficients, error) {
	err := validateParams(sampleRate, freqHz, order)
	if err != nil {
		return nil, err
	}

	if gainDB == 0 {
		return passthroughSections(), nil
	}

	g := db2Lin(gainDB)
	P := math.Pow(g, 1.0/float64(order))
	K := math.Tan(math.Pi * freqHz / sampleRate)
	pairs, realSigma := butterworthPoles(order)

	return lowShelfSections(K, P, pairs, realSigma), nil
}

// ButterworthHighShelf designs an M-th order Butterworth high-shelving filter.
//
// freqHz is the cutoff frequency in Hz. gainDB is the shelf gain in dB
// (positive for boost, negative for cut). order must be >= 1.
// Returns a cascade of biquad sections.
func ButterworthHighShelf(sampleRate, freqHz, gainDB float64, order int) ([]biquad.Coefficients, error) {
	err := validateParams(sampleRate, freqHz, order)
	if err != nil {
		return nil, err
	}

	if gainDB == 0 {
		return passthroughSections(), nil
	}

	g := db2Lin(gainDB)
	P := math.Pow(g, 1.0/float64(order))
	K := 1.0 / math.Tan(math.Pi*freqHz/sampleRate)
	pairs, realSigma := butterworthPoles(order)

	sections := lowShelfSections(K, P, pairs, realSigma)
	negateOddPowers(sections)

	return sections, nil
}
