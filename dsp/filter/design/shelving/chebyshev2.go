package shelving

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// Chebyshev2LowShelf designs an M-th order Chebyshev Type II low-shelving filter.
//
// freqHz is the cutoff frequency in Hz. gainDB is the shelf gain in dB
// (positive for boost, negative for cut). rippleDB controls the stopband
// (flat region) ripple and must be > 0 (typical values 0.1–1.0 dB).
// order must be >= 1.
//
// Chebyshev II provides equiripple in the flat region while maintaining a
// monotonic shelf region, complementary to Chebyshev I which has ripple in
// the transition. Uses the Orfanidis parametric framework.
func Chebyshev2LowShelf(sampleRate, freqHz, gainDB, rippleDB float64, order int) ([]biquad.Coefficients, error) {
	if err := validateParams(sampleRate, freqHz, order); err != nil {
		return nil, err
	}
	if rippleDB <= 0 {
		return nil, ErrInvalidParams
	}
	if gainDB == 0 {
		return passthroughSections(), nil
	}

	K := math.Tan(math.Pi * freqHz / sampleRate)

	// The stopband gain is near 0 dB: rippleDB controls how far from unity
	// the flat region is allowed to deviate.
	stopbandDB := rippleDB
	if gainDB < 0 {
		stopbandDB = -rippleDB
	}

	return chebyshev2Sections(K, gainDB, stopbandDB, order)
}

// Chebyshev2HighShelf designs an M-th order Chebyshev Type II high-shelving filter.
//
// freqHz is the cutoff frequency in Hz. gainDB is the shelf gain in dB
// (positive for boost, negative for cut). rippleDB controls the stopband
// (flat region) ripple and must be > 0 (typical values 0.1–1.0 dB).
// order must be >= 1.
func Chebyshev2HighShelf(sampleRate, freqHz, gainDB, rippleDB float64, order int) ([]biquad.Coefficients, error) {
	if err := validateParams(sampleRate, freqHz, order); err != nil {
		return nil, err
	}
	if rippleDB <= 0 {
		return nil, ErrInvalidParams
	}
	if gainDB == 0 {
		return passthroughSections(), nil
	}

	K := 1.0 / math.Tan(math.Pi*freqHz/sampleRate)

	stopbandDB := rippleDB
	if gainDB < 0 {
		stopbandDB = -rippleDB
	}

	sections, err := chebyshev2Sections(K, gainDB, stopbandDB, order)
	if err != nil {
		return nil, err
	}
	negateOddPowers(sections)
	return sections, nil
}
