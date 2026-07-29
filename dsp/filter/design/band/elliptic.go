package band

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/internal/orfanidis"
)

// ellipticBandRad designs an elliptic bandpass filter in the digital domain.
// w0 is the center frequency in radians, wb the bandwidth in radians,
// gainDB the peak gain, gbDB the bandwidth-edge gain, and order the filter order (must be even and > 2).
// It returns a cascade of biquad sections implementing the filter.
//
// The analog prototype and the bandpass bilinear transform live in
// internal/orfanidis, shared with the shelving designers; this function only
// adds the fourth-order factoring step.
func ellipticBandRad(w0, wb, gainDB, gbDB float64, order int) ([]biquad.Coefficients, error) {
	if order <= 2 || order%2 != 0 {
		return nil, ErrInvalidParams
	}

	// Convert dB parameters to linear amplitude scale. Gs is the ripple bound
	// on the reference side of the transition.
	spec := orfanidis.Spec{
		Order: order,
		G0:    1.0, // db2Lin(0) is always exactly 1
		G:     db2Lin(gainDB),
		Gb:    db2Lin(gbDB),
		Gs:    db2Lin(gainDB - gbDB),
		WB:    math.Tan(wb * 0.5),
	}

	aSections, err := orfanidis.EllipticPrototype(spec)
	if err != nil {
		return nil, ErrInvalidParams
	}

	foSections, err := orfanidis.BandpassBLT(aSections, w0)
	if err != nil {
		return nil, ErrInvalidParams
	}

	// Factor each fourth-order digital section into a pair of biquads.
	// Gain-only and second-order sections are handled as special cases.
	out := make([]biquad.Coefficients, 0, len(foSections)*2)

	for _, s := range foSections {
		// Detect gain-only or first/second-order sections that don't need
		// 4th-order root-finding. These arise from the zeroth-order gain
		// section (even order) or first-order section (odd order).
		if isZero(s.B[1]) && isZero(s.B[2]) && isZero(s.B[3]) && isZero(s.B[4]) &&
			isZero(s.A[1]) && isZero(s.A[2]) && isZero(s.A[3]) && isZero(s.A[4]) {
			// Gain-only: single passthrough biquad with gain.
			gain := s.B[0] / s.A[0]
			out = append(out, biquad.Coefficients{B0: gain, B1: 0, B2: 0, A1: 0, A2: 0})

			continue
		}

		if isZero(s.B[3]) && isZero(s.B[4]) && isZero(s.A[3]) && isZero(s.A[4]) {
			// Second-order section: directly map to a single biquad.
			a0 := s.A[0]
			out = append(out, biquad.Coefficients{
				B0: s.B[0] / a0, B1: s.B[1] / a0, B2: s.B[2] / a0,
				A1: s.A[1] / a0, A2: s.A[2] / a0,
			})

			continue
		}
		// Full fourth-order section: factor into two cascaded biquads
		// by finding roots of the numerator and denominator polynomials.
		biquads, err := splitFOSection(s.B, s.A)
		if err != nil {
			return nil, err
		}

		out = append(out, biquads...)
	}

	return out, nil
}

// isZero returns true if the absolute value of v is below a small threshold,
// used throughout to detect numerically negligible coefficients.
func isZero(v float64) bool {
	return orfanidis.IsZero(v)
}
