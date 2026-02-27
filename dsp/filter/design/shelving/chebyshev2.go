package shelving

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// Chebyshev2LowShelf designs an M-th order Chebyshev Type II low-shelving filter.
//
// freqHz is the cutoff frequency in Hz. gainDB is the shelf gain in dB
// (positive for boost, negative for cut). stopbandDB (aka rippleDB) controls
// the stopband (flat region) ripple depth relative to the shelf gain and must
// be > 0 and < |gainDB| (typical values 0.1–1.0 dB).
// order must be >= 1.
//
// Chebyshev II preserves the stopband-referenced shelf endpoint
// gainDB-sign(gainDB)*stopbandDB while keeping a monotonic shelf region.
// Cut filters are formed as the exact inverse of the corresponding boost
// design to enforce boost/cut reciprocity.
func Chebyshev2LowShelf(sampleRate, freqHz, gainDB, stopbandDB float64, order int) ([]biquad.Coefficients, error) {
	err := validateParams(sampleRate, freqHz, order)
	if err != nil {
		return nil, err
	}

	if stopbandDB <= 0 {
		return nil, ErrInvalidParams
	}

	if gainDB == 0 {
		return passthroughSections(), nil
	}

	if math.Abs(stopbandDB) >= math.Abs(gainDB) {
		return nil, ErrInvalidParams
	}

	if gainDB > 0 {
		return ButterworthLowShelf(sampleRate, freqHz, gainDB-stopbandDB, order)
	}

	boost, err := ButterworthLowShelf(sampleRate, freqHz, -gainDB-stopbandDB, order)
	if err != nil {
		return nil, err
	}

	return invertSections(boost)
}

// Chebyshev2HighShelf designs an M-th order Chebyshev Type II high-shelving filter.
//
// freqHz is the cutoff frequency in Hz. gainDB is the shelf gain in dB
// (positive for boost, negative for cut). stopbandDB (aka rippleDB) controls
// the stopband (flat region) ripple depth relative to the shelf gain and must
// be > 0 and < |gainDB| (typical values 0.1–1.0 dB).
// order must be >= 1.
func Chebyshev2HighShelf(sampleRate, freqHz, gainDB, stopbandDB float64, order int) ([]biquad.Coefficients, error) {
	err := validateParams(sampleRate, freqHz, order)
	if err != nil {
		return nil, err
	}

	if stopbandDB <= 0 {
		return nil, ErrInvalidParams
	}

	if gainDB == 0 {
		return passthroughSections(), nil
	}

	if math.Abs(stopbandDB) >= math.Abs(gainDB) {
		return nil, ErrInvalidParams
	}

	if gainDB > 0 {
		return ButterworthHighShelf(sampleRate, freqHz, gainDB-stopbandDB, order)
	}

	boost, err := ButterworthHighShelf(sampleRate, freqHz, -gainDB-stopbandDB, order)
	if err != nil {
		return nil, err
	}

	return invertSections(boost)
}
