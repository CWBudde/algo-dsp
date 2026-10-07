//nolint:funlen,gocognit,gocyclo,cyclop
package effectchain

import (
	"math"
	"strings"

	"github.com/cwbudde/algo-dsp/dsp/core"
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
	"github.com/cwbudde/algo-dsp/dsp/filter/design/band"
	"github.com/cwbudde/algo-dsp/dsp/filter/design/shelving"
)

const eqEllipticStopbandDB = 40.0

// eqShelfSideRippleDB mirrors the fixed shelf-side ripple the elliptic shelving
// designers reserve below the nominal gain, and eqShelfRippleHeadroom keeps the
// bounded ripple strictly inside the designers' admissible range.
const (
	eqShelfSideRippleDB   = 0.05
	eqShelfRippleHeadroom = 0.9
)

// equirippleShelfRipple bounds the node's ripple control against the shelf gain.
//
// The Chebyshev II and elliptic shelving designers place the cutoff at
// |H(freqHz)|² = (G² + 1)/2, so the reference-side ripple must stay below that
// level — otherwise the cutoff falls inside the ripple band and the design is
// rejected. The elliptic family additionally reserves eqShelfSideRippleDB below
// the nominal gain. The node's shape control is clamped to a fixed dB range
// independently of gain, so without this bound the designer rejects common
// small-gain settings and buildEQChain silently falls back to a one-section RBJ
// shelf while still reporting the selected family and order.
//
// The second result is false only when the gain is too small to host any
// stopband at all, where the shelf is inaudible and the fallback is harmless.
func equirippleShelfRipple(family string, gainDB, ripple float64) (float64, bool) {
	// The cutoff level in dB, always strictly between 0 dB and gainDB.
	g := math.Pow(10, gainDB/20)
	limit := math.Abs(10 * math.Log10((g*g+1)*0.5))

	if family == eqFamilyElliptic {
		limit = math.Min(limit, math.Abs(gainDB)-eqShelfSideRippleDB)
	}

	if limit <= 0 {
		return 0, false
	}

	return math.Min(ripple, limit*eqShelfRippleHeadroom), true
}

const (
	eqDefaultOrder       = 2
	eqKindHighpass       = "highpass"
	eqKindLowpass        = "lowpass"
	eqKindBandpass       = "bandpass"
	eqKindNotch          = "notch"
	eqKindAllpass        = "allpass"
	eqKindPeak           = "peak"
	eqKindHighShelf      = "highshelf"
	eqKindLowShelf       = "lowshelf"
	eqFamilyRBJ          = "rbj"
	eqFamilyButterworth  = "butterworth"
	eqFamilyBessel       = "bessel"
	eqFamilyChebyshev1   = "chebyshev1"
	eqFamilyChebyshev2   = "chebyshev2"
	eqFamilyElliptic     = "elliptic"
	eqFamilyMoog         = "moog"
	eqShapeModeQ         = "q"
	eqShapeModeBandwidth = "bandwidth"
	eqShapeModeRipple    = "ripple"
)

func clamp(v, lo, hi float64) float64 { return core.Clamp(v, lo, hi) }
func buildEQChain(family, kind string, order int, freq, gainDB, q, sampleRate float64) *biquad.Chain {
	family = normalizeEQFamilyForType(kind, normalizeEQFamily(family))
	order = normalizeEQOrder(kind, family, order)
	q = clampEQShape(kind, family, freq, sampleRate, q)
	linGain := nodeLinearGain(family, kind, gainDB)
	ripple := chebyshevRippleFromShape(q)

	switch family {
	case eqFamilyButterworth:
		switch kind {
		case eqKindHighpass:
			return chainFromCoeffs(design.ButterworthHP(freq, order, sampleRate), linGain)
		case eqKindLowpass:
			return chainFromCoeffs(design.ButterworthLP(freq, order, sampleRate), linGain)
		case eqKindPeak:
			bw := peakBandwidthHz(kind, family, freq, sampleRate, q)

			coeffs, err := band.ButterworthBand(sampleRate, freq, bw, gainDB, order)
			if err == nil {
				return chainFromCoeffs(coeffs, linGain)
			}
		case eqKindHighShelf:
			coeffs, err := shelving.ButterworthHighShelf(sampleRate, freq, gainDB, order)
			if err == nil {
				return chainFromCoeffs(coeffs, linGain)
			}
		case eqKindLowShelf:
			coeffs, err := shelving.ButterworthLowShelf(sampleRate, freq, gainDB, order)
			if err == nil {
				return chainFromCoeffs(coeffs, linGain)
			}
		}
	case eqFamilyChebyshev1:
		switch kind {
		case eqKindHighpass:
			return chainFromCoeffs(design.Chebyshev1HP(freq, order, ripple, sampleRate), linGain)
		case eqKindLowpass:
			return chainFromCoeffs(design.Chebyshev1LP(freq, order, ripple, sampleRate), linGain)
		case eqKindPeak:
			bw := peakBandwidthHz(kind, family, freq, sampleRate, q)

			coeffs, err := band.Chebyshev1Band(sampleRate, freq, bw, gainDB, order)
			if err == nil {
				return chainFromCoeffs(coeffs, linGain)
			}
		case eqKindHighShelf:
			coeffs, err := shelving.Chebyshev1HighShelf(sampleRate, freq, gainDB, ripple, order)
			if err == nil {
				return chainFromCoeffs(coeffs, linGain)
			}
		case eqKindLowShelf:
			coeffs, err := shelving.Chebyshev1LowShelf(sampleRate, freq, gainDB, ripple, order)
			if err == nil {
				return chainFromCoeffs(coeffs, linGain)
			}
		}
	case eqFamilyChebyshev2:
		switch kind {
		case eqKindHighpass:
			return chainFromCoeffs(design.Chebyshev2HP(freq, order, ripple, sampleRate), linGain)
		case eqKindLowpass:
			return chainFromCoeffs(design.Chebyshev2LP(freq, order, ripple, sampleRate), linGain)
		case eqKindPeak:
			bw := peakBandwidthHz(kind, family, freq, sampleRate, q)

			coeffs, err := band.Chebyshev2Band(sampleRate, freq, bw, gainDB, order)
			if err == nil {
				return chainFromCoeffs(coeffs, linGain)
			}
		case eqKindHighShelf:
			if stopband, ok := equirippleShelfRipple(family, gainDB, ripple); ok {
				coeffs, err := shelving.Chebyshev2HighShelf(sampleRate, freq, gainDB, stopband, order)
				if err == nil {
					return chainFromCoeffs(coeffs, linGain)
				}
			}
		case eqKindLowShelf:
			if stopband, ok := equirippleShelfRipple(family, gainDB, ripple); ok {
				coeffs, err := shelving.Chebyshev2LowShelf(sampleRate, freq, gainDB, stopband, order)
				if err == nil {
					return chainFromCoeffs(coeffs, linGain)
				}
			}
		}
	case eqFamilyBessel:
		switch kind {
		case eqKindHighpass:
			return chainFromCoeffs(design.BesselHP(freq, order, sampleRate), linGain)
		case eqKindLowpass:
			return chainFromCoeffs(design.BesselLP(freq, order, sampleRate), linGain)
		}
	case eqFamilyElliptic:
		switch kind {
		case eqKindHighpass:
			return chainFromCoeffs(design.EllipticHP(freq, order, ripple, eqEllipticStopbandDB, sampleRate), linGain)
		case eqKindLowpass:
			return chainFromCoeffs(design.EllipticLP(freq, order, ripple, eqEllipticStopbandDB, sampleRate), linGain)
		case eqKindPeak:
			bw := peakBandwidthHz(kind, family, freq, sampleRate, q)

			coeffs, err := band.EllipticBand(sampleRate, freq, bw, gainDB, order)
			if err == nil {
				return chainFromCoeffs(coeffs, linGain)
			}
		case eqKindHighShelf:
			// The shelving designers take the reference-side ripple bound, which
			// the node's shape control supplies; the fixed eqEllipticStopbandDB
			// used by the high/lowpass designers would exceed any usable gain.
			if stopband, ok := equirippleShelfRipple(family, gainDB, ripple); ok {
				coeffs, err := shelving.EllipticHighShelf(sampleRate, freq, gainDB, stopband, order)
				if err == nil {
					return chainFromCoeffs(coeffs, linGain)
				}
			}
		case eqKindLowShelf:
			if stopband, ok := equirippleShelfRipple(family, gainDB, ripple); ok {
				coeffs, err := shelving.EllipticLowShelf(sampleRate, freq, gainDB, stopband, order)
				if err == nil {
					return chainFromCoeffs(coeffs, linGain)
				}
			}
		}
	}

	fq := rbjFallbackQ(kind, family, freq, q)

	switch kind {
	case eqKindHighpass:
		return chainFromCoeffs([]biquad.Coefficients{design.Highpass(freq, fq, sampleRate)}, linGain)
	case eqKindBandpass:
		return chainFromCoeffs([]biquad.Coefficients{design.Bandpass(freq, fq, sampleRate)}, linGain)
	case eqKindNotch:
		return chainFromCoeffs([]biquad.Coefficients{design.Notch(freq, fq, sampleRate)}, linGain)
	case eqKindAllpass:
		return chainFromCoeffs([]biquad.Coefficients{design.Allpass(freq, fq, sampleRate)}, linGain)
	case eqKindPeak:
		return chainFromCoeffs([]biquad.Coefficients{design.Peak(freq, gainDB, fq, sampleRate)}, linGain)
	case eqKindHighShelf:
		return chainFromCoeffs([]biquad.Coefficients{design.HighShelf(freq, gainDB, fq, sampleRate)}, linGain)
	case eqKindLowShelf:
		return chainFromCoeffs([]biquad.Coefficients{design.LowShelf(freq, gainDB, fq, sampleRate)}, linGain)
	default:
		return chainFromCoeffs([]biquad.Coefficients{design.Lowpass(freq, fq, sampleRate)}, linGain)
	}
}

// rbjDefaultQ is the Butterworth-flat Q the RBJ cookbook filters default to.
const rbjDefaultQ = math.Sqrt2 / 2

// rbjFallbackQ supplies the Q for the single-section RBJ fallback.
//
// When the node's shape control is in ripple mode its value is a dB ripple
// bound, not a Q, so a high-order design that bails out must not hand that
// number to an RBJ filter — a 12 dB ripple would silently become a Q of 12.
// There is no meaningful Q to recover in that case, so use the RBJ default.
func rbjFallbackQ(kind, family string, freq, shape float64) float64 {
	if eqShapeMode(kind, family) == eqShapeModeRipple {
		return rbjDefaultQ
	}

	return rbjQFromShape(kind, family, freq, shape)
}

func chainFromCoeffs(coeffs []biquad.Coefficients, gain float64) *biquad.Chain {
	if len(coeffs) == 0 {
		coeffs = []biquad.Coefficients{{B0: 1}}
	}

	return biquad.NewChain(coeffs, biquad.WithGain(gain))
}

func typeUsesEmbeddedGain(family, kind string) bool {
	if kind == eqKindPeak || kind == eqKindLowShelf || kind == eqKindHighShelf {
		return true
	}

	return kind == eqKindBandpass && family != eqFamilyRBJ
}

func nodeLinearGain(family, kind string, gainDB float64) float64 {
	if typeUsesEmbeddedGain(family, kind) {
		return 1
	}

	return math.Pow(10, gainDB/20)
}

func chebyshevRippleFromShape(shape float64) float64 {
	// Reuse the node's shape control as Chebyshev ripple (dB-like control).
	return clamp(shape, 0.05, 120)
}

func eqShapeMode(kind, family string) string {
	if kind == eqKindPeak && family != eqFamilyRBJ {
		return eqShapeModeBandwidth
	}

	if (family == eqFamilyChebyshev1 || family == eqFamilyChebyshev2 || family == eqFamilyElliptic) &&
		(kind == eqKindHighpass || kind == eqKindLowpass || kind == eqKindHighShelf || kind == eqKindLowShelf) {
		return eqShapeModeRipple
	}

	return eqShapeModeQ
}

func maxPeakBandwidth(freq, sampleRate float64) float64 {
	nyquist := sampleRate * 0.5

	maxBW := 2 * math.Min(math.Max(freq, 1), math.Max(nyquist-freq, 1))
	if maxBW < 1 {
		maxBW = 1
	}

	return maxBW
}

func clampEQShape(kind, family string, freq, sampleRate, value float64) float64 {
	switch eqShapeMode(kind, family) {
	case eqShapeModeBandwidth:
		return clamp(value, 1, maxPeakBandwidth(freq, sampleRate))
	case eqShapeModeRipple:
		if family == eqFamilyChebyshev2 {
			return clamp(value, 0.05, 120)
		}

		return clamp(value, 0.05, 12)
	default:
		return clamp(value, 0.2, 8)
	}
}

func ellipticPassChain(kind string, order int, freq, gain, ripple, stopband, sampleRate float64) *biquad.Chain {
	if kind == kindHighpass {
		return chainFromCoeffs(design.EllipticHP(freq, order, ripple, stopband, sampleRate), math.Pow(10, gain/20))
	}

	return chainFromCoeffs(design.EllipticLP(freq, order, ripple, stopband, sampleRate), math.Pow(10, gain/20))
}

func peakBandwidthHz(kind, family string, freq, sampleRate, shape float64) float64 {
	if eqShapeMode(kind, family) == eqShapeModeBandwidth {
		return clamp(shape, 1, maxPeakBandwidth(freq, sampleRate))
	}

	return clamp(freq/math.Max(shape, 1e-6), 1, maxPeakBandwidth(freq, sampleRate))
}

func rbjQFromShape(kind, family string, freq, shape float64) float64 {
	if eqShapeMode(kind, family) == eqShapeModeBandwidth {
		return clamp(freq/math.Max(shape, 1e-6), 0.2, 8)
	}

	return clamp(shape, 0.2, 8)
}

func normalizeEQFamily(family string) string {
	switch strings.ToLower(strings.TrimSpace(family)) {
	case eqFamilyRBJ, eqFamilyButterworth, eqFamilyBessel, eqFamilyChebyshev1, eqFamilyChebyshev2, eqFamilyElliptic:
		return strings.ToLower(strings.TrimSpace(family))
	default:
		return eqFamilyRBJ
	}
}

func supportsEQFamily(kind, family string) bool {
	switch family {
	case eqFamilyRBJ:
		return true
	case eqFamilyBessel:
		return kind == eqKindHighpass || kind == eqKindLowpass
	case eqFamilyButterworth, eqFamilyChebyshev1, eqFamilyChebyshev2, eqFamilyElliptic:
		return kind == eqKindHighpass || kind == eqKindLowpass || kind == eqKindPeak || kind == eqKindLowShelf || kind == eqKindHighShelf
	default:
		return false
	}
}

func normalizeEQFamilyForType(kind, family string) string {
	if supportsEQFamily(kind, family) {
		return family
	}

	return eqFamilyRBJ
}

func supportsEQOrder(kind, family string) bool {
	if family == eqFamilyRBJ {
		return false
	}

	if family == eqFamilyBessel {
		return kind == eqKindHighpass || kind == eqKindLowpass
	}

	if family == eqFamilyButterworth || family == eqFamilyChebyshev1 ||
		family == eqFamilyChebyshev2 || family == eqFamilyElliptic {
		return kind == eqKindHighpass || kind == eqKindLowpass || kind == eqKindPeak || kind == eqKindLowShelf || kind == eqKindHighShelf
	}

	return false
}

func normalizeEQOrder(kind, family string, order int) int {
	if !supportsEQOrder(kind, family) {
		return 1
	}

	if order <= 0 {
		order = eqDefaultOrder
	}

	maxOrder := 20.0
	if family == eqFamilyBessel {
		maxOrder = 10
	}

	if family == eqFamilyElliptic {
		// Higher orders are ill-conditioned at the fixed pass/stopband bounds.
		maxOrder = 12
	}

	if kind == eqKindPeak {
		order = int(clamp(float64(order), 4, maxOrder))
		if order%2 != 0 {
			order++
		}

		return order
	}

	return int(clamp(float64(order), 1, maxOrder))
}

// BuiltInFilterDesigner designs RBJ, Butterworth, Bessel, Chebyshev and elliptic
// filters using the library's pure coefficient designers. Shape means Q for RBJ,
// bandwidth in Hz for high-order peaks, and ripple in dB for equiripple passes.
type BuiltInFilterDesigner struct{}

// NormalizeFamily returns a supported coefficient family.
func (BuiltInFilterDesigner) NormalizeFamily(v string) string { return normalizeEQFamily(v) }

// NormalizeFamilyForType selects RBJ when a family cannot design the kind.
func (BuiltInFilterDesigner) NormalizeFamilyForType(k, f string) string {
	return normalizeEQFamilyForType(k, f)
}

// NormalizeOrder bounds the supported prototype order.
func (BuiltInFilterDesigner) NormalizeOrder(k, f string, n int) int { return normalizeEQOrder(k, f, n) }

// ClampShape bounds Q, bandwidth or ripple according to kind and family.
func (BuiltInFilterDesigner) ClampShape(k, f string, hz, sr, v float64) float64 {
	return clampEQShape(k, f, hz, sr, v)
}

// BuildChain constructs a normalized filter cascade.
func (BuiltInFilterDesigner) BuildChain(f, k string, n int, hz, g, q, sr float64) *biquad.Chain {
	return buildEQChain(f, k, n, hz, g, q, sr)
}
