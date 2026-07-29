package shelving

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/internal/orfanidis"
)

// shelfRippleDB is the ripple allowed on the shelf side of the transition for
// the equiripple designers, matching the 0.05 dB convention used by the band
// elliptic designer. Only the reference-side ripple is user controlled.
const shelfRippleDB = 0.05

// EllipticLowShelf designs an M-th order elliptic (Cauer) low-shelving filter.
//
// freqHz is the cutoff frequency in Hz, defined as in the Butterworth and
// Chebyshev I designers by |H(freqHz)|² = (G² + 1)/2, i.e. roughly 3 dB below
// the shelf gain. gainDB is the shelf gain in dB (positive for boost, negative
// for cut). stopbandDB is the ripple bound in the flat 0 dB region and must be
// > 0 and small enough to leave room below the shelf (typical 0.1–1.0 dB).
// order must be >= 1.
//
// The design is equiripple on both sides of the transition, giving the steepest
// transition of the four shelving families at the cost of ripple in both bands.
func EllipticLowShelf(sampleRate, freqHz, gainDB, stopbandDB float64, order int) ([]biquad.Coefficients, error) {
	return ellipticShelf(sampleRate, freqHz, gainDB, stopbandDB, order, false)
}

// EllipticHighShelf designs an M-th order elliptic (Cauer) high-shelving filter.
//
// Parameters follow EllipticLowShelf; the shelf occupies the band above freqHz.
func EllipticHighShelf(sampleRate, freqHz, gainDB, stopbandDB float64, order int) ([]biquad.Coefficients, error) {
	return ellipticShelf(sampleRate, freqHz, gainDB, stopbandDB, order, true)
}

func ellipticShelf(sampleRate, freqHz, gainDB, stopbandDB float64, order int, high bool) ([]biquad.Coefficients, error) {
	if err := validateParams(sampleRate, freqHz, order); err != nil {
		return nil, err
	}

	if stopbandDB <= 0 {
		return nil, ErrInvalidParams
	}

	if gainDB == 0 {
		return passthroughSections(), nil
	}

	// The shelf-side ripple bound must sit strictly between the reference-side
	// bound and the shelf gain.
	gbDB := gainDB - math.Copysign(shelfRippleDB, gainDB)
	if stopbandDB >= math.Abs(gbDB) {
		return nil, ErrInvalidParams
	}

	G := db2Lin(gainDB)

	spec := orfanidis.Spec{
		Order: order,
		G0:    1.0,
		G:     G,
		Gb:    db2Lin(gbDB),
		Gs:    db2Lin(math.Copysign(stopbandDB, gainDB)),
	}

	return buildShelf(spec, sampleRate, freqHz, G, high, orfanidis.EllipticPrototype)
}

// buildShelf turns an Orfanidis analog prototype into a shelving cascade.
//
// The prototype is first designed at WB = 1 to locate the analog frequency
// where it reaches the package's cutoff level |H|² = (G² + 1)/2, then rebuilt
// with WB scaled so that level lands exactly on freqHz. A high shelf uses the
// reciprocal prewarped frequency and the substitution z → −z, since
// H_HS(z) = H_LS(−z) maps the corner to π − ω_c.
func buildShelf(
	spec orfanidis.Spec,
	sampleRate, freqHz, G float64,
	high bool,
	prototype func(orfanidis.Spec) ([]orfanidis.Section, error),
) ([]biquad.Coefficients, error) {
	build := func(wb float64) ([]orfanidis.Section, error) {
		scaled := spec
		scaled.WB = wb

		return prototype(scaled)
	}

	target := math.Sqrt((G*G + 1) * 0.5)

	omega, err := orfanidis.EdgeOmega(build, target)
	if err != nil {
		return nil, ErrInvalidParams
	}

	K := math.Tan(math.Pi * freqHz / sampleRate)
	if high {
		if K == 0 {
			return nil, ErrInvalidParams
		}

		K = 1.0 / K
	}

	secs, err := build(K / omega)
	if err != nil {
		return nil, ErrInvalidParams
	}

	sections, err := orfanidis.LowpassBLT(secs)
	if err != nil {
		return nil, ErrInvalidParams
	}

	sections = foldGainSections(sections)
	if len(sections) == 0 {
		return nil, ErrInvalidParams
	}

	for _, s := range sections {
		if !coeffsAreFinite(s) {
			return nil, ErrInvalidParams
		}
	}

	if high {
		negateOddPowers(sections)
	}

	return sections, nil
}

// foldGainSections merges plain gain stages into the numerator of the following
// section, so an even-order cascade reports M/2 biquads rather than M/2+1 and
// the section count matches the other shelving families.
func foldGainSections(sections []biquad.Coefficients) []biquad.Coefficients {
	out := make([]biquad.Coefficients, 0, len(sections))

	pending := 1.0

	for _, s := range sections {
		if s.B1 == 0 && s.B2 == 0 && s.A1 == 0 && s.A2 == 0 {
			pending *= s.B0
			continue
		}

		s.B0 *= pending
		s.B1 *= pending
		s.B2 *= pending
		pending = 1.0

		out = append(out, s)
	}

	if pending != 1.0 {
		if len(out) == 0 {
			return []biquad.Coefficients{{B0: pending}}
		}

		out[len(out)-1].B0 *= pending
		out[len(out)-1].B1 *= pending
		out[len(out)-1].B2 *= pending
	}

	return out
}
