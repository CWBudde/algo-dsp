// Package orfanidis implements the analog prototype and bilinear-transform
// machinery for high-order digital parametric equalizers following
//
//	S. J. Orfanidis, "High-Order Digital Parametric Equalizer Design",
//	J. Audio Eng. Soc., vol. 53, no. 11, pp. 1026-1046, November 2005.
//
// The prototypes are frequency-normalized analog shelving/lowpass sections that
// become peaking (band) equalizers via BandpassBLT and shelving filters via
// LowpassBLT. Both the band and shelving designers in dsp/filter/design build on
// this package so the pole/zero placement exists in exactly one place.
package orfanidis

import (
	"errors"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// ErrInvalidParams is returned when prototype parameters are out of range or
// produce a non-finite design.
var ErrInvalidParams = errors.New("orfanidis: invalid parameters")

// Tol is the convergence tolerance handed to the elliptic-function helpers.
const Tol = 2.2e-16

// Section is an analog prototype section
//
//	H(s) = (B0 + B1·s + B2·s²) / (A0 + A1·s + A2·s²)
//
// B0 and A0 are the s⁰ terms. This ordering is what the bilinear transforms in
// this package assume; a section with B2 == A2 == 0 is first order, and one with
// B1 == B2 == A1 == A2 == 0 is a plain gain stage.
type Section struct {
	B0, B1, B2 float64
	A0, A1, A2 float64
}

// Spec describes a high-order parametric-EQ prototype. All gains are linear
// amplitudes, not decibels.
type Spec struct {
	// Order is the filter order M (>= 1).
	Order int
	// G0 is the reference gain outside the active band (normally 1.0).
	G0 float64
	// G is the peak (band) or shelf gain.
	G float64
	// Gb is the ripple bound on the G side of the transition.
	Gb float64
	// Gs is the ripple bound on the G0 side of the transition.
	Gs float64
	// WB is the prewarped band edge, tan(Δω/2).
	WB float64
}

// FourthOrder is a digital section of up to fourth order, stored as length-5
// coefficient arrays in ascending powers of z⁻¹ with A[0] normalized to 1.
type FourthOrder struct {
	B [5]float64
	A [5]float64
}

// IsZero reports whether v is numerically negligible.
func IsZero(v float64) bool {
	return math.Abs(v) < 1e-12
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// sectionOrder classifies a section as gain-only (0), first order (1) or second
// order (2).
func sectionOrder(s Section) int {
	switch {
	case !IsZero(s.B2) || !IsZero(s.A2):
		return 2
	case !IsZero(s.B1) || !IsZero(s.A1):
		return 1
	default:
		return 0
	}
}

// bilinear applies s = (1 − z⁻¹)/(1 + z⁻¹) to a single analog section, yielding
// digital numerator/denominator coefficients normalized so that ah[0] == 1.
func bilinear(s Section) (bh, ah [3]float64, err error) {
	switch sectionOrder(s) {
	case 0:
		if IsZero(s.A0) {
			return bh, ah, ErrInvalidParams
		}

		bh[0] = s.B0 / s.A0
		ah[0] = 1
	case 1:
		d := s.A0 + s.A1
		if IsZero(d) {
			return bh, ah, ErrInvalidParams
		}

		bh[0] = (s.B0 + s.B1) / d
		bh[1] = (s.B0 - s.B1) / d
		ah[0] = 1
		ah[1] = (s.A0 - s.A1) / d
	default:
		d := s.A0 + s.A1 + s.A2
		if IsZero(d) {
			return bh, ah, ErrInvalidParams
		}

		bh[0] = (s.B0 + s.B1 + s.B2) / d
		bh[1] = 2 * (s.B0 - s.B2) / d
		bh[2] = (s.B0 - s.B1 + s.B2) / d
		ah[0] = 1
		ah[1] = 2 * (s.A0 - s.A2) / d
		ah[2] = (s.A0 - s.A1 + s.A2) / d
	}

	for i := range bh {
		if !isFinite(bh[i]) || !isFinite(ah[i]) {
			return bh, ah, ErrInvalidParams
		}
	}

	return bh, ah, nil
}

// LowpassBLT converts analog prototype sections into digital biquads via the
// plain bilinear transform, i.e. the ω0 = 0 degenerate case of BandpassBLT.
// Analog DC maps to digital DC and analog s → ∞ maps to Nyquist, so a
// lowpass-shaped prototype becomes a low-shelving filter.
//
// A high-shelving filter is obtained from the result by the substitution
// z → −z, i.e. by negating the odd-power coefficients.
func LowpassBLT(secs []Section) ([]biquad.Coefficients, error) {
	if len(secs) == 0 {
		return nil, ErrInvalidParams
	}

	out := make([]biquad.Coefficients, 0, len(secs))

	for _, s := range secs {
		bh, ah, err := bilinear(s)
		if err != nil {
			return nil, err
		}

		out = append(out, biquad.Coefficients{
			B0: bh[0], B1: bh[1], B2: bh[2],
			A1: ah[1], A2: ah[2],
		})
	}

	return out, nil
}

// BandpassBLT performs the combined lowpass-to-bandpass frequency mapping and
// bilinear transform on analog prototype sections, producing digital sections
// of up to fourth order centered at ω0 (radians per sample).
//
// When |cos ω0| == 1 the bandpass mapping degenerates to the shelving case and
// the plain lowpass coefficients are returned with the odd-power sign
// correction that turns a low shelf into a high shelf at ω0 = π.
func BandpassBLT(secs []Section, w0 float64) ([]FourthOrder, error) {
	c0 := math.Cos(w0)
	degenerate := IsZero(math.Abs(c0) - 1)
	c0c0 := c0 * c0

	out := make([]FourthOrder, len(secs))

	for j, s := range secs {
		bh, ah, err := bilinear(s)
		if err != nil {
			return nil, err
		}

		if degenerate {
			out[j].B = [5]float64{bh[0], bh[1] * c0, bh[2]}
			out[j].A = [5]float64{ah[0], ah[1] * c0, ah[2]}

			continue
		}

		switch sectionOrder(s) {
		case 0:
			out[j].B[0] = bh[0]
			out[j].A[0] = 1
		case 1:
			out[j].B = [5]float64{
				bh[0],
				c0 * (bh[1] - bh[0]),
				-bh[1],
			}
			out[j].A = [5]float64{
				1,
				c0 * (ah[1] - 1),
				-ah[1],
			}
		default:
			out[j].B = [5]float64{
				bh[0],
				c0 * (bh[1] - 2*bh[0]),
				(bh[0]-bh[1]+bh[2])*c0c0 - bh[1],
				c0 * (bh[1] - 2*bh[2]),
				bh[2],
			}
			out[j].A = [5]float64{
				1,
				c0 * (ah[1] - 2),
				(1-ah[1]+ah[2])*c0c0 - ah[1],
				c0 * (ah[1] - 2*ah[2]),
				ah[2],
			}
		}
	}

	return out, nil
}

// AnalogMagnitude evaluates |H(jΩ)| for a cascade of analog prototype sections.
func AnalogMagnitude(secs []Section, omega float64) float64 {
	w2 := omega * omega

	mag := 1.0

	for _, s := range secs {
		nRe := s.B0 - s.B2*w2
		nIm := s.B1 * omega
		dRe := s.A0 - s.A2*w2
		dIm := s.A1 * omega

		den := math.Hypot(dRe, dIm)
		if den == 0 {
			return math.Inf(1)
		}

		mag *= math.Hypot(nRe, nIm) / den
	}

	return mag
}

// analogLimit returns |H(jΩ)| as Ω → ∞ for a cascade of analog sections,
// evaluated per section from its highest non-negligible coefficient pair so no
// infinities are formed.
func analogLimit(secs []Section) float64 {
	mag := 1.0

	for _, s := range secs {
		var num, den float64

		switch sectionOrder(s) {
		case 0:
			num, den = s.B0, s.A0
		case 1:
			num, den = s.B1, s.A1
		default:
			num, den = s.B2, s.A2
		}

		if den == 0 {
			return math.Inf(1)
		}

		mag *= math.Abs(num / den)
	}

	return mag
}

// EdgeOmega locates the analog frequency at which a prototype built with
// WB == 1 reaches the magnitude target.
//
// Orfanidis places the band edge at the ripple bound Gb, but callers generally
// want the edge specified at some other level (for shelving filters this
// library uses |H|² = (G² + G0²)/2, matching the Butterworth shelf of
// Holters & Zölzer). Because every prototype in this package scales purely in
// frequency with WB, designing at WB == 1 and then rescaling by
// WB_target / EdgeOmega places the edge exactly.
//
// The target must lie strictly between the two ripple bounds, where the
// magnitude is monotonic, so bisection converges unconditionally.
func EdgeOmega(build func(wb float64) ([]Section, error), target float64) (float64, error) {
	if !isFinite(target) || target <= 0 {
		return 0, ErrInvalidParams
	}

	secs, err := build(1.0)
	if err != nil {
		return 0, err
	}

	at := func(omega float64) float64 { return AnalogMagnitude(secs, omega) }

	lo, hi := 1.0, 1.0

	// The magnitude runs monotonically from the G side at Ω = 0 to the G0 side
	// as Ω → ∞ (or the reverse for a cut). Bracket the crossing first.
	rising := at(0) < analogLimit(secs)
	crossed := func(omega float64) bool {
		if rising {
			return at(omega) >= target
		}

		return at(omega) <= target
	}

	if crossed(1.0) {
		for i := 0; crossed(lo); i++ {
			if i > 200 {
				return 0, ErrInvalidParams
			}

			hi = lo
			lo /= 2
		}
	} else {
		for i := 0; !crossed(hi); i++ {
			if i > 200 {
				return 0, ErrInvalidParams
			}

			lo = hi
			hi *= 2
		}
	}

	for range 200 {
		mid := 0.5 * (lo + hi)
		if mid == lo || mid == hi {
			break
		}

		if crossed(mid) {
			hi = mid
		} else {
			lo = mid
		}
	}

	omega := 0.5 * (lo + hi)
	if !isFinite(omega) || omega <= 0 {
		return 0, ErrInvalidParams
	}

	return omega, nil
}
