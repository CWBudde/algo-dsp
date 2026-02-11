package band

import (
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/internal/ellipticmath"
)

// soSection represents a second-order analog prototype section
// with numerator coefficients b0, b1, b2 and denominator a0, a1, a2.
type soSection struct {
	b0, b1, b2 float64
	a0, a1, a2 float64
}

// foSection represents a fourth-order digital section after the
// bilinear bandpass transform, stored as length-5 coefficient arrays.
type foSection struct {
	b [5]float64
	a [5]float64
}

// ellipticBandRad designs an elliptic bandpass filter in the digital domain.
// w0 is the center frequency in radians, wb the bandwidth in radians,
// gainDB the peak gain, gbDB the bandwidth-edge gain, and order the filter order (must be even and > 2).
// It returns a cascade of biquad sections implementing the filter.
func ellipticBandRad(w0, wb, gainDB, gbDB float64, order int) ([]biquad.Coefficients, error) {
	if order <= 2 || order%2 != 0 {
		return nil, ErrInvalidParams
	}

	// Convert dB parameters to linear amplitude scale.
	G0 := 1.0 // db2Lin(0) is always exactly 1
	G := db2Lin(gainDB)
	Gb := db2Lin(gbDB)
	Gs := db2Lin(gainDB - gbDB)

	// Compute bandwidth parameter and selectivity ratios for the elliptic design.
	// WB is the pre-warped bandwidth, e and es control passband/stopband ripple.
	WB := math.Tan(wb * 0.5)
	e := math.Sqrt((G*G - Gb*Gb) / (Gb*Gb - G0*G0))
	es := math.Sqrt((G*G - Gs*Gs) / (Gs*Gs - G0*G0))
	k1 := e / es
	k := ellipdeg(order, k1, 2.2e-16)

	// Compute Jacobi elliptic function arguments for pole/zero placement.
	// ju0 determines zeros, jv0 determines poles in the analog prototype.
	ju0 := asne(complex(0, 1)*complex(G/(e*G0), 0), k1, 2.2e-16) / complex(float64(order), 0)
	jv0 := asne(complex(0, 1)/complex(e, 0), k1, 2.2e-16) / complex(float64(order), 0)

	// Determine if order is odd (r=1) or even (r=0);
	// L = number of conjugate pole/zero pairs.
	r := order % 2
	L := (order - r) / 2

	// Build analog prototype sections. For even order, the first section
	// is a pure gain stage at the bandwidth-edge level.
	var aSections []soSection
	if r == 0 {
		aSections = append(aSections, soSection{b0: Gb, b1: 0, b2: 0, a0: 1, a1: 0, a2: 0})
	} else {
		// Odd order: compute the real zero and pole for the first-order section
		// using the cd elliptic function evaluated at the prototype arguments.
		z0 := real(complex(0, 1) * cde(-1.0+ju0, k, 2.2e-16))
		B00 := G * WB
		B01 := -G / z0
		A00 := WB
		A01 := -1 / real(complex(0, 1)*cde(-1.0+jv0, k, 2.2e-16))
		aSections = append(aSections, soSection{b0: B00, b1: B01, b2: 0, a0: A00, a1: A01, a2: 0})
	}

	// Build second-order sections for each conjugate pole/zero pair.
	// Each pair i uses uniformly spaced sample points ui on the elliptic function.
	if L > 0 {
		for i := 1; i <= L; i++ {
			ui := (2.0*float64(i) - 1.0) / float64(order)

			// Evaluate cd function to get the i-th zero and pole in the s-plane.
			zeros := complex(0, 1) * cde(complex(ui, 0)-ju0, k, 2.2e-16)
			poles := complex(0, 1) * cde(complex(ui, 0)-jv0, k, 2.2e-16)

			// Invert and extract real parts and magnitudes to form the
			// second-order section coefficients from the pole/zero locations.
			invZero := 1.0 / zeros
			invPole := 1.0 / poles
			zre := real(invZero)
			pre := real(invPole)
			zabs := cmplx.Abs(invZero)
			pabs := cmplx.Abs(invPole)
			sa := soSection{
				b0: WB * WB,
				b1: -2 * WB * zre,
				b2: zabs * zabs,
				a0: WB * WB,
				a1: -2 * WB * pre,
				a2: pabs * pabs,
			}
			aSections = append(aSections, sa)
		}
	}

	// Apply bilinear bandpass transform to convert analog prototype
	// sections into digital fourth-order sections centered at w0.
	foSections := blt(aSections, w0)

	// Factor each fourth-order digital section into a pair of biquads.
	// Gain-only and second-order sections are handled as special cases.
	out := make([]biquad.Coefficients, 0, len(foSections)*2)
	for _, s := range foSections {
		// Detect gain-only or first/second-order sections that don't need
		// 4th-order root-finding. These arise from the zeroth-order gain
		// section (even order) or first-order section (odd order).
		if isZero(s.b[1]) && isZero(s.b[2]) && isZero(s.b[3]) && isZero(s.b[4]) &&
			isZero(s.a[1]) && isZero(s.a[2]) && isZero(s.a[3]) && isZero(s.a[4]) {
			// Gain-only: single passthrough biquad with gain.
			gain := s.b[0] / s.a[0]
			out = append(out, biquad.Coefficients{B0: gain, B1: 0, B2: 0, A1: 0, A2: 0})
			continue
		}
		if isZero(s.b[3]) && isZero(s.b[4]) && isZero(s.a[3]) && isZero(s.a[4]) {
			// Second-order section: directly map to a single biquad.
			a0 := s.a[0]
			out = append(out, biquad.Coefficients{
				B0: s.b[0] / a0, B1: s.b[1] / a0, B2: s.b[2] / a0,
				A1: s.a[1] / a0, A2: s.a[2] / a0,
			})
			continue
		}
		// Full fourth-order section: factor into two cascaded biquads
		// by finding roots of the numerator and denominator polynomials.
		biquads, err := splitFOSection(s.b, s.a)
		if err != nil {
			return nil, err
		}
		out = append(out, biquads...)
	}

	return out, nil
}

// blt performs the bilinear bandpass transform on a set of analog prototype
// second-order sections, producing digital fourth-order sections centered at w0.
// This combines the LP-to-BP frequency transformation with the bilinear z-transform.
func blt(aSections []soSection, w0 float64) []foSection {
	c0 := math.Cos(w0)
	degenerate := isZero(math.Abs(c0) - 1)
	c0c0 := c0 * c0

	out := make([]foSection, len(aSections))
	for j, s := range aSections {
		b0, b1, b2 := s.b0, s.b1, s.b2
		a0, a1, a2 := s.a0, s.a1, s.a2

		// Classify and transform each section based on its order.
		hasFirst := !isZero(b1) || !isZero(a1)
		hasSecond := !isZero(b2) || !isZero(a2)

		// Intermediate bilinear-transformed lowpass coefficients.
		var bh, ah [3]float64

		if !hasFirst && !hasSecond {
			// Zeroth-order (gain-only): scalar gain ratio.
			bh[0] = b0 / a0
			ah[0] = 1
		} else if !hasSecond {
			// First-order: bilinear transform s -> (z-1)/(z+1).
			D := a0 + a1
			bh[0] = (b0 + b1) / D
			bh[1] = (b0 - b1) / D
			ah[0] = 1
			ah[1] = (a0 - a1) / D
		} else {
			// Second-order: bilinear transform normalized at z=1.
			D := a0 + a1 + a2
			bh[0] = (b0 + b1 + b2) / D
			bh[1] = 2 * (b0 - b2) / D
			bh[2] = (b0 - b1 + b2) / D
			ah[0] = 1
			ah[1] = 2 * (a0 - a2) / D
			ah[2] = (a0 - a1 + a2) / D
		}

		// Edge case: when w0 is at DC or Nyquist the bandpass transform
		// degenerates; use direct lowpass coefficients with sign correction.
		if degenerate {
			out[j].b = [5]float64{bh[0], bh[1] * c0, bh[2]}
			out[j].a = [5]float64{ah[0], ah[1] * c0, ah[2]}
			continue
		}

		// LP-to-BP frequency mapping via cos(w0).
		if !hasFirst && !hasSecond {
			// Gain-only passthrough.
			out[j].b[0] = bh[0]
			out[j].a[0] = 1
		} else if !hasSecond {
			// First-order -> second-order bandpass.
			out[j].b = [5]float64{
				bh[0],
				c0 * (bh[1] - bh[0]),
				-bh[1],
			}
			out[j].a = [5]float64{
				1,
				c0 * (ah[1] - 1),
				-ah[1],
			}
		} else {
			// Second-order -> fourth-order bandpass.
			out[j].b = [5]float64{
				bh[0],
				c0 * (bh[1] - 2*bh[0]),
				(bh[0]-bh[1]+bh[2])*c0c0 - bh[1],
				c0 * (bh[1] - 2*bh[2]),
				bh[2],
			}
			out[j].a = [5]float64{
				1,
				c0 * (ah[1] - 2),
				(1-ah[1]+ah[2])*c0c0 - ah[1],
				c0 * (ah[1] - 2*ah[2]),
				ah[2],
			}
		}
	}

	return out
}

// isZero returns true if the absolute value of v is below a small threshold,
// used throughout to detect numerically negligible coefficients.
func isZero(v float64) bool {
	return math.Abs(v) < 1e-12
}

// landen computes the Landen sequence of descending moduli for the given
// elliptic modulus k. If tol < 1 it is used as convergence tolerance;
// otherwise it is interpreted as the fixed number of iterations M.
// The sequence converges to zero and is used by ellipk, cde, sne, and acde.
func landen(k, tol float64) []float64 {
	return ellipticmath.Landen(k, tol)
}

// landenK computes K from a precomputed Landen sequence using the product formula
// K(k) = (pi/2) * product(1 + v[i]). The sequence is not modified.
func landenK(v []float64) float64 {
	return ellipticmath.LandenK(v)
}

// ellipk computes the complete elliptic integral of the first kind K(k)
// and its complement K'(k) = K(k') where k' = sqrt(1 - k^2).
// Uses the Landen transformation for the general case, with asymptotic
// approximations near k=0 and k=1 where the transform is ill-conditioned.
func ellipk(k, tol float64) (float64, float64) {
	return ellipticmath.EllipK(k, tol)
}

// ellipkReuse is like ellipk but accepts an optional precomputed Landen sequence
// for k (used for the K half). If vk is nil, it computes the sequence internally.
// The slice is consumed and must not be reused by the caller.
func ellipkReuse(k, tol float64, vk []float64) (float64, float64) {
	return ellipticmath.EllipKReuse(k, tol, vk)
}

// ellipdeg2 computes the elliptic degree equation k1 = ellipdeg2(n, k)
// using the nome q and a truncated theta-function series (M=7 terms).
// This is the fallback when k1 is very small and the direct sne-based
// method in ellipdeg would lose precision.
func ellipdeg2(n, k, tol float64) float64 {
	return ellipticmath.EllipDeg2(n, k, tol)
}

// srem computes a symmetric remainder of x modulo y, adjusting the standard
// math.Remainder result so the output lies in [-y/2, y/2]. This is needed
// for normalizing elliptic function arguments to their fundamental period.
func srem(x, y float64) float64 {
	return ellipticmath.SymmetricRemainder(x, y)
}

// acde computes the inverse cd elliptic function acd(w, k) using the
// descending Landen transformation. The result is normalized to the
// quarter-period rectangle using srem to keep real and imaginary parts
// within the fundamental domain.
func acde(w complex128, k, tol float64) complex128 {
	return ellipticmath.ACDE(w, k, tol)
}

// asne computes the inverse sn elliptic function asn(w, k) = 1 - acd(w, k).
// This identity relates the Jacobi sn and cd functions via their quarter-period shift.
func asne(w complex128, k, tol float64) complex128 {
	return ellipticmath.ASNE(w, k, tol)
}

// cde evaluates the Jacobi cd elliptic function cd(u, k) using the
// ascending Landen transformation. Starting from cos(u*pi/2) at the
// smallest modulus, it iterates back up through the Landen sequence.
func cde(u complex128, k, tol float64) complex128 {
	return ellipticmath.CDE(u, k, tol)
}

// sne evaluates the Jacobi sn elliptic function for a vector of real arguments u.
// Uses the ascending Landen transformation starting from sin(u*pi/2).
func sne(u []float64, k, tol float64) []float64 {
	return ellipticmath.SNE(u, k, tol)
}

// ellipdeg solves the degree equation for elliptic filter design:
// given order N and selectivity k1, compute the discrimination parameter k.
// Uses sne evaluation at uniformly spaced points on the complementary modulus,
// falling back to ellipdeg2 when k1 is very small.
func ellipdeg(N int, k1, tol float64) float64 {
	return ellipticmath.EllipDeg(N, k1, tol)
}
