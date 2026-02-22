package shelving

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// Chebyshev1LowShelf designs an M-th order Chebyshev Type I low-shelving filter.
//
// freqHz is the cutoff frequency in Hz. gainDB is the shelf gain in dB
// (positive for boost, negative for cut). rippleDB controls the passband
// ripple and must be > 0 (typical values 0.5–1.0 dB). order must be >= 1.
//
// Compared to Butterworth, Chebyshev I provides a steeper transition for
// the same order at the cost of ripple in the transition region.
func Chebyshev1LowShelf(sampleRate, freqHz, gainDB, rippleDB float64, order int) ([]biquad.Coefficients, error) {
	if err := validateParams(sampleRate, freqHz, order); err != nil {
		return nil, err
	}

	if rippleDB <= 0 {
		return nil, ErrInvalidParams
	}

	if gainDB == 0 {
		return passthroughSections(), nil
	}

	g := db2Lin(gainDB)
	P := math.Pow(g, 1.0/float64(order))
	K := math.Tan(math.Pi * freqHz / sampleRate)
	pairs, realSigma := chebyshev1Poles(order, rippleDB)

	return lowShelfSections(K, P, pairs, realSigma), nil
}

// Chebyshev1HighShelf designs an M-th order Chebyshev Type I high-shelving filter.
//
// freqHz is the cutoff frequency in Hz. gainDB is the shelf gain in dB
// (positive for boost, negative for cut). rippleDB controls the passband
// ripple and must be > 0 (typical values 0.5–1.0 dB). order must be >= 1.
func Chebyshev1HighShelf(sampleRate, freqHz, gainDB, rippleDB float64, order int) ([]biquad.Coefficients, error) {
	if err := validateParams(sampleRate, freqHz, order); err != nil {
		return nil, err
	}

	if rippleDB <= 0 {
		return nil, ErrInvalidParams
	}

	if gainDB == 0 {
		return passthroughSections(), nil
	}

	g := db2Lin(gainDB)
	P := math.Pow(g, 1.0/float64(order))
	K := 1.0 / math.Tan(math.Pi*freqHz/sampleRate)
	pairs, realSigma := chebyshev1Poles(order, rippleDB)

	sections := lowShelfSections(K, P, pairs, realSigma)
	negateOddPowers(sections)

	return sections, nil
}
