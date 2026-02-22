package webdemo

import (
	"math"
	"strings"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
	"github.com/cwbudde/algo-dsp/dsp/filter/design/band"
	"github.com/cwbudde/algo-dsp/dsp/filter/design/shelving"
)

const eqEllipticStopbandDB = 40.0

// SetEQ updates EQ parameters and rebuilds the filters.
func (e *Engine) SetEQ(eq EQParams) error {
	eq.HPFreq = clamp(eq.HPFreq, 20, e.sampleRate*0.49)
	eq.HPType = normalizeEQType("hp", eq.HPType)
	eq.HPFamily = normalizeEQFamily(eq.HPFamily)
	eq.HPFamily = normalizeEQFamilyForType(eq.HPType, eq.HPFamily)
	eq.HPOrder = normalizeEQOrder(eq.HPType, eq.HPFamily, eq.HPOrder)
	eq.LowFreq = clamp(eq.LowFreq, 20, e.sampleRate*0.49)
	eq.LowType = normalizeEQType("low", eq.LowType)
	eq.LowFamily = normalizeEQFamily(eq.LowFamily)
	eq.LowFamily = normalizeEQFamilyForType(eq.LowType, eq.LowFamily)
	eq.LowOrder = normalizeEQOrder(eq.LowType, eq.LowFamily, eq.LowOrder)
	eq.MidFreq = clamp(eq.MidFreq, 20, e.sampleRate*0.49)
	eq.MidType = normalizeEQType("mid", eq.MidType)
	eq.MidFamily = normalizeEQFamily(eq.MidFamily)
	eq.MidFamily = normalizeEQFamilyForType(eq.MidType, eq.MidFamily)
	eq.MidOrder = normalizeEQOrder(eq.MidType, eq.MidFamily, eq.MidOrder)
	eq.HighFreq = clamp(eq.HighFreq, 20, e.sampleRate*0.49)
	eq.HighType = normalizeEQType("high", eq.HighType)
	eq.HighFamily = normalizeEQFamily(eq.HighFamily)
	eq.HighFamily = normalizeEQFamilyForType(eq.HighType, eq.HighFamily)
	eq.HighOrder = normalizeEQOrder(eq.HighType, eq.HighFamily, eq.HighOrder)
	eq.LPFreq = clamp(eq.LPFreq, 20, e.sampleRate*0.49)
	eq.LPType = normalizeEQType("lp", eq.LPType)
	eq.LPFamily = normalizeEQFamily(eq.LPFamily)
	eq.LPFamily = normalizeEQFamilyForType(eq.LPType, eq.LPFamily)
	eq.LPOrder = normalizeEQOrder(eq.LPType, eq.LPFamily, eq.LPOrder)
	eq.LowGain = clamp(eq.LowGain, -24, 24)
	eq.HPGain = clamp(eq.HPGain, -24, 24)
	eq.MidGain = clamp(eq.MidGain, -24, 24)
	eq.HighGain = clamp(eq.HighGain, -24, 24)
	eq.LPGain = clamp(eq.LPGain, -24, 24)
	eq.HPQ = clampEQShape(eq.HPType, eq.HPFamily, eq.HPFreq, e.sampleRate, eq.HPQ)
	eq.LowQ = clampEQShape(eq.LowType, eq.LowFamily, eq.LowFreq, e.sampleRate, eq.LowQ)
	eq.MidQ = clampEQShape(eq.MidType, eq.MidFamily, eq.MidFreq, e.sampleRate, eq.MidQ)
	eq.HighQ = clampEQShape(eq.HighType, eq.HighFamily, eq.HighFreq, e.sampleRate, eq.HighQ)
	eq.LPQ = clampEQShape(eq.LPType, eq.LPFamily, eq.LPFreq, e.sampleRate, eq.LPQ)

	eq.Master = clamp(eq.Master, 0, 1)
	e.eq = eq

	return e.rebuildEQ()
}

func (e *Engine) rebuildEQ() error {
	e.hp = buildEQChain(e.eq.HPFamily, e.eq.HPType, e.eq.HPOrder, e.eq.HPFreq, e.eq.HPGain, e.eq.HPQ, e.sampleRate)
	e.low = buildEQChain(e.eq.LowFamily, e.eq.LowType, e.eq.LowOrder, e.eq.LowFreq, e.eq.LowGain, e.eq.LowQ, e.sampleRate)
	e.mid = buildEQChain(e.eq.MidFamily, e.eq.MidType, e.eq.MidOrder, e.eq.MidFreq, e.eq.MidGain, e.eq.MidQ, e.sampleRate)
	e.high = buildEQChain(e.eq.HighFamily, e.eq.HighType, e.eq.HighOrder, e.eq.HighFreq, e.eq.HighGain, e.eq.HighQ, e.sampleRate)
	e.lp = buildEQChain(e.eq.LPFamily, e.eq.LPType, e.eq.LPOrder, e.eq.LPFreq, e.eq.LPGain, e.eq.LPQ, e.sampleRate)

	return nil
}

func buildEQChain(family, kind string, order int, freq, gainDB, q, sampleRate float64) *biquad.Chain {
	family = normalizeEQFamilyForType(kind, normalizeEQFamily(family))
	order = normalizeEQOrder(kind, family, order)
	q = clampEQShape(kind, family, freq, sampleRate, q)
	linGain := nodeLinearGain(family, kind, gainDB)
	ripple := chebyshevRippleFromShape(q)

	switch family {
	case "butterworth":
		switch kind {
		case "highpass":
			return chainFromCoeffs(design.ButterworthHP(freq, order, sampleRate), linGain)
		case "lowpass":
			return chainFromCoeffs(design.ButterworthLP(freq, order, sampleRate), linGain)
		case "peak":
			bw := peakBandwidthHz(kind, family, freq, sampleRate, q)

			coeffs, err := band.ButterworthBand(sampleRate, freq, bw, gainDB, order)
			if err == nil {
				return chainFromCoeffs(coeffs, linGain)
			}
		case "highshelf":
			coeffs, err := shelving.ButterworthHighShelf(sampleRate, freq, gainDB, order)
			if err == nil {
				return chainFromCoeffs(coeffs, linGain)
			}
		case "lowshelf":
			coeffs, err := shelving.ButterworthLowShelf(sampleRate, freq, gainDB, order)
			if err == nil {
				return chainFromCoeffs(coeffs, linGain)
			}
		}
	case "chebyshev1":
		switch kind {
		case "highpass":
			return chainFromCoeffs(design.Chebyshev1HP(freq, order, ripple, sampleRate), linGain)
		case "lowpass":
			return chainFromCoeffs(design.Chebyshev1LP(freq, order, ripple, sampleRate), linGain)
		case "peak":
			bw := peakBandwidthHz(kind, family, freq, sampleRate, q)

			coeffs, err := band.Chebyshev1Band(sampleRate, freq, bw, gainDB, order)
			if err == nil {
				return chainFromCoeffs(coeffs, linGain)
			}
		case "highshelf":
			coeffs, err := shelving.Chebyshev1HighShelf(sampleRate, freq, gainDB, ripple, order)
			if err == nil {
				return chainFromCoeffs(coeffs, linGain)
			}
		case "lowshelf":
			coeffs, err := shelving.Chebyshev1LowShelf(sampleRate, freq, gainDB, ripple, order)
			if err == nil {
				return chainFromCoeffs(coeffs, linGain)
			}
		}
	case "chebyshev2":
		switch kind {
		case "highpass":
			return chainFromCoeffs(design.Chebyshev2HP(freq, order, ripple, sampleRate), linGain)
		case "lowpass":
			return chainFromCoeffs(design.Chebyshev2LP(freq, order, ripple, sampleRate), linGain)
		case "peak":
			bw := peakBandwidthHz(kind, family, freq, sampleRate, q)

			coeffs, err := band.Chebyshev2Band(sampleRate, freq, bw, gainDB, order)
			if err == nil {
				return chainFromCoeffs(coeffs, linGain)
			}
		case "highshelf":
			coeffs, err := shelving.Chebyshev2HighShelf(sampleRate, freq, gainDB, ripple, order)
			if err == nil {
				return chainFromCoeffs(coeffs, linGain)
			}
		case "lowshelf":
			coeffs, err := shelving.Chebyshev2LowShelf(sampleRate, freq, gainDB, ripple, order)
			if err == nil {
				return chainFromCoeffs(coeffs, linGain)
			}
		}
	case "bessel":
		switch kind {
		case "highpass":
			return chainFromCoeffs(design.BesselHP(freq, order, sampleRate), linGain)
		case "lowpass":
			return chainFromCoeffs(design.BesselLP(freq, order, sampleRate), linGain)
		}
	case "elliptic":
		switch kind {
		case "highpass":
			return chainFromCoeffs(design.EllipticHP(freq, order, ripple, eqEllipticStopbandDB, sampleRate), linGain)
		case "lowpass":
			return chainFromCoeffs(design.EllipticLP(freq, order, ripple, eqEllipticStopbandDB, sampleRate), linGain)
		case "peak":
			bw := peakBandwidthHz(kind, family, freq, sampleRate, q)

			coeffs, err := band.EllipticBand(sampleRate, freq, bw, gainDB, order)
			if err == nil {
				return chainFromCoeffs(coeffs, linGain)
			}
		}
	}

	switch kind {
	case "highpass":
		return chainFromCoeffs([]biquad.Coefficients{design.Highpass(freq, q, sampleRate)}, linGain)
	case "bandpass":
		return chainFromCoeffs([]biquad.Coefficients{design.Bandpass(freq, q, sampleRate)}, linGain)
	case "notch":
		return chainFromCoeffs([]biquad.Coefficients{design.Notch(freq, q, sampleRate)}, linGain)
	case "allpass":
		return chainFromCoeffs([]biquad.Coefficients{design.Allpass(freq, q, sampleRate)}, linGain)
	case "peak":
		return chainFromCoeffs([]biquad.Coefficients{design.Peak(freq, gainDB, rbjQFromShape(kind, family, freq, q), sampleRate)}, linGain)
	case "highshelf":
		return chainFromCoeffs([]biquad.Coefficients{design.HighShelf(freq, gainDB, q, sampleRate)}, linGain)
	case "lowshelf":
		return chainFromCoeffs([]biquad.Coefficients{design.LowShelf(freq, gainDB, q, sampleRate)}, linGain)
	default:
		return chainFromCoeffs([]biquad.Coefficients{design.Lowpass(freq, q, sampleRate)}, linGain)
	}
}

func chainFromCoeffs(coeffs []biquad.Coefficients, gain float64) *biquad.Chain {
	if len(coeffs) == 0 {
		coeffs = []biquad.Coefficients{{B0: 1}}
	}

	return biquad.NewChain(coeffs, biquad.WithGain(gain))
}

func typeUsesEmbeddedGain(family, kind string) bool {
	if kind == "peak" || kind == "lowshelf" || kind == "highshelf" {
		return true
	}

	return kind == "bandpass" && family != "rbj"
}

func nodeLinearGain(family, kind string, gainDB float64) float64 {
	if typeUsesEmbeddedGain(family, kind) {
		return 1
	}

	return math.Pow(10, gainDB/20)
}

func chebyshevRippleFromShape(shape float64) float64 {
	// Reuse the node's shape control as Chebyshev ripple (dB-like control).
	return clamp(shape, 0.05, 24)
}

func eqShapeMode(kind, family string) string {
	if kind == "peak" && family != "rbj" {
		return "bandwidth"
	}

	if (family == "chebyshev1" || family == "chebyshev2") &&
		(kind == "highpass" || kind == "lowpass" || kind == "highshelf" || kind == "lowshelf") {
		return "ripple"
	}

	if family == "elliptic" && (kind == "highpass" || kind == "lowpass") {
		return "ripple"
	}

	return "q"
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
	case "bandwidth":
		return clamp(value, 1, maxPeakBandwidth(freq, sampleRate))
	case "ripple":
		if family == "chebyshev2" {
			return clamp(value, 0.05, 24)
		}

		return clamp(value, 0.05, 12)
	default:
		return clamp(value, 0.2, 8)
	}
}

func peakBandwidthHz(kind, family string, freq, sampleRate, shape float64) float64 {
	if eqShapeMode(kind, family) == "bandwidth" {
		return clamp(shape, 1, maxPeakBandwidth(freq, sampleRate))
	}

	return clamp(freq/math.Max(shape, 1e-6), 1, maxPeakBandwidth(freq, sampleRate))
}

func rbjQFromShape(kind, family string, freq, shape float64) float64 {
	if eqShapeMode(kind, family) == "bandwidth" {
		return clamp(freq/math.Max(shape, 1e-6), 0.2, 8)
	}

	return clamp(shape, 0.2, 8)
}

func normalizeEQFamily(family string) string {
	switch strings.ToLower(strings.TrimSpace(family)) {
	case "rbj", "butterworth", "bessel", "chebyshev1", "chebyshev2", "elliptic":
		return strings.ToLower(strings.TrimSpace(family))
	default:
		return "rbj"
	}
}

func supportsEQFamily(kind, family string) bool {
	switch family {
	case "rbj":
		return true
	case "bessel":
		return kind == "highpass" || kind == "lowpass"
	case "butterworth", "chebyshev1", "chebyshev2":
		return kind == "highpass" || kind == "lowpass" || kind == "peak" || kind == "lowshelf" || kind == "highshelf"
	case "elliptic":
		return kind == "highpass" || kind == "lowpass" || kind == "peak"
	default:
		return false
	}
}

func normalizeEQFamilyForType(kind, family string) string {
	if supportsEQFamily(kind, family) {
		return family
	}

	return "rbj"
}

func supportsEQOrder(kind, family string) bool {
	if family == "rbj" {
		return false
	}

	if family == "bessel" {
		return kind == "highpass" || kind == "lowpass"
	}

	if family == "elliptic" {
		return kind == "highpass" || kind == "lowpass" || kind == "peak"
	}

	if family == "butterworth" || family == "chebyshev1" || family == "chebyshev2" {
		return kind == "highpass" || kind == "lowpass" || kind == "peak" || kind == "lowshelf" || kind == "highshelf"
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

	maxOrder := 12.0
	if family == "bessel" {
		maxOrder = 10
	}

	if kind == "peak" {
		order = int(clamp(float64(order), 4, maxOrder))
		if order%2 != 0 {
			order++
		}

		return order
	}

	return int(clamp(float64(order), 1, maxOrder))
}

func normalizeEQType(node, kind string) string {
	normalized := strings.ToLower(strings.TrimSpace(kind))
	switch normalized {
	case "bandeq", "band-eq", "bandeqpeak", "bell", "bandbell":
		normalized = "peak"
	}

	switch normalized {
	case "highpass", "lowpass", "bandpass", "notch", "allpass", "peak", "highshelf", "lowshelf":
	default:
		normalized = ""
	}

	switch node {
	case "hp":
		switch normalized {
		case "highpass", "lowpass", "bandpass", "notch", "allpass", "peak", "lowshelf", "highshelf":
			return normalized
		default:
			return "highpass"
		}
	case "low":
		switch normalized {
		case "highpass", "lowpass", "bandpass", "notch", "allpass", "peak", "lowshelf", "highshelf":
			return normalized
		default:
			return "lowshelf"
		}
	case "mid":
		switch normalized {
		case "highpass", "lowpass", "bandpass", "notch", "allpass", "peak", "lowshelf", "highshelf":
			return normalized
		default:
			return "peak"
		}
	case "high":
		switch normalized {
		case "highpass", "lowpass", "bandpass", "notch", "allpass", "peak", "lowshelf", "highshelf":
			return normalized
		default:
			return "highshelf"
		}
	case "lp":
		switch normalized {
		case "highpass", "lowpass", "bandpass", "notch", "allpass", "peak", "lowshelf", "highshelf":
			return normalized
		default:
			return "lowpass"
		}
	default:
		return "peak"
	}
}
