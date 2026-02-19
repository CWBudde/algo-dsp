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
// Chebyshev II provides equiripple in the flat region while maintaining a
// monotonic shelf region, complementary to Chebyshev I which has ripple in
// the transition. Uses the Orfanidis parametric framework.
func Chebyshev2LowShelf(sampleRate, freqHz, gainDB, stopbandDB float64, order int) ([]biquad.Coefficients, error) {
	if err := validateParams(sampleRate, freqHz, order); err != nil {
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

	K := math.Tan(math.Pi * freqHz / sampleRate)

	// The stopband gain is near 0 dB: stopbandDB sets the Orfanidis stopband
	// depth parameter relative to the shelf gain.
	if gainDB < 0 {
		stopbandDB = -stopbandDB
	}

	return chebyshev2Sections(K, gainDB, stopbandDB, order)
}

// Chebyshev2HighShelf designs an M-th order Chebyshev Type II high-shelving filter.
//
// freqHz is the cutoff frequency in Hz. gainDB is the shelf gain in dB
// (positive for boost, negative for cut). stopbandDB (aka rippleDB) controls
// the stopband (flat region) ripple depth relative to the shelf gain and must
// be > 0 and < |gainDB| (typical values 0.1–1.0 dB).
// order must be >= 1.
func Chebyshev2HighShelf(sampleRate, freqHz, gainDB, stopbandDB float64, order int) ([]biquad.Coefficients, error) {
	if err := validateParams(sampleRate, freqHz, order); err != nil {
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

	K := 1.0 / math.Tan(math.Pi*freqHz/sampleRate)

	if gainDB < 0 {
		stopbandDB = -stopbandDB
	}

	sections, err := chebyshev2Sections(K, gainDB, stopbandDB, order)
	if err != nil {
		return nil, err
	}
	negateOddPowers(sections)
	return sections, nil
}
