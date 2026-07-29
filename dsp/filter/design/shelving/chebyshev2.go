package shelving

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/internal/orfanidis"
)

// Chebyshev2LowShelf designs an M-th order Chebyshev Type II low-shelving filter.
//
// freqHz is the cutoff frequency in Hz, defined as in the Butterworth and
// Chebyshev I designers by |H(freqHz)|² = (G² + 1)/2, i.e. roughly 3 dB below
// the shelf gain. gainDB is the shelf gain in dB (positive for boost, negative
// for cut). stopbandDB is the ripple bound in the flat 0 dB region and must be
// > 0 and < |gainDB| (typical values 0.1–1.0 dB). order must be >= 1.
//
// Type II is equiripple in the flat region and monotonic across the shelf, so
// the response reaches gainDB at DC and then oscillates within stopbandDB of
// 0 dB above the transition. Compared with the Butterworth shelf this buys a
// steeper transition at the cost of that ripple.
func Chebyshev2LowShelf(sampleRate, freqHz, gainDB, stopbandDB float64, order int) ([]biquad.Coefficients, error) {
	return chebyshev2Shelf(sampleRate, freqHz, gainDB, stopbandDB, order, false)
}

// Chebyshev2HighShelf designs an M-th order Chebyshev Type II high-shelving filter.
//
// Parameters follow Chebyshev2LowShelf; the shelf occupies the band above freqHz.
func Chebyshev2HighShelf(sampleRate, freqHz, gainDB, stopbandDB float64, order int) ([]biquad.Coefficients, error) {
	return chebyshev2Shelf(sampleRate, freqHz, gainDB, stopbandDB, order, true)
}

func chebyshev2Shelf(sampleRate, freqHz, gainDB, stopbandDB float64, order int, high bool) ([]biquad.Coefficients, error) {
	if err := validateParams(sampleRate, freqHz, order); err != nil {
		return nil, err
	}

	if stopbandDB <= 0 {
		return nil, ErrInvalidParams
	}

	if gainDB == 0 {
		return passthroughSections(), nil
	}

	if stopbandDB >= math.Abs(gainDB) {
		return nil, ErrInvalidParams
	}

	G := db2Lin(gainDB)

	spec := orfanidis.Spec{
		Order: order,
		G0:    1.0,
		G:     G,
		Gs:    db2Lin(math.Copysign(stopbandDB, gainDB)),
	}

	return buildShelf(spec, sampleRate, freqHz, G, high, orfanidis.Chebyshev2Prototype)
}
