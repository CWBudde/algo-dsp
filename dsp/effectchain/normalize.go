package effectchain

import (
	"strings"

	"github.com/cwbudde/algo-dsp/dsp/effects"
	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
)

const (
	familyRBJ         = "rbj"
	familyButterworth = "butterworth"
	familyBessel      = "bessel"
	familyChebyshev1  = "chebyshev1"
	familyChebyshev2  = "chebyshev2"
	familyElliptic    = "elliptic"
	familyMoog        = "moog"
	kindLowpass       = "lowpass"
	kindHighpass      = "highpass"
	kindBandpass      = "bandpass"
	kindPeak          = "peak"
)

func normalizeFilterFamily(raw, nodeType string) string {
	if nodeType == "filter-moog" {
		return familyMoog
	}

	family := strings.ToLower(strings.TrimSpace(raw))
	if family == "" {
		return familyRBJ
	}

	switch family {
	case familyRBJ, familyButterworth, familyBessel, familyChebyshev1, familyChebyshev2, familyElliptic, familyMoog:
		return family
	default:
		return familyRBJ
	}
}

func normalizeFilterKind(nodeType, raw string) string {
	if nodeType == "filter-moog" {
		return kindLowpass
	}

	kind := normalizeEQTypeForChain(raw)
	if strings.TrimSpace(raw) != "" {
		return kind
	}

	switch nodeType {
	case "filter-highpass":
		return kindHighpass
	case "filter-bandpass":
		return kindBandpass
	case "filter-notch":
		return "notch"
	case "filter-allpass":
		return "allpass"
	case "filter-peak":
		return "peak"
	case "filter-lowshelf":
		return "lowshelf"
	case "filter-highshelf":
		return "highshelf"
	default:
		return kindLowpass
	}
}

// normalizeEQTypeForChain normalizes a filter kind string.
func normalizeEQTypeForChain(kind string) string {
	normalized := strings.ToLower(strings.TrimSpace(kind))
	switch normalized {
	case "bandeq", "band-eq", "bandeqpeak", "bell", "bandbell":
		normalized = "peak"
	}

	switch normalized {
	case kindHighpass, kindLowpass, kindBandpass, "notch", "allpass", kindPeak, "highshelf", "lowshelf":
		return normalized
	default:
		return kindPeak
	}
}

func moogOversamplingFromOrder(order int) int {
	switch {
	case order >= 12:
		return 8
	case order >= 8:
		return 4
	case order >= 4:
		return 2
	default:
		return 1
	}
}

//nolint:cyclop
func normalizeDistortionMode(raw string) effects.DistortionMode {
	switch raw {
	case "hardclip":
		return effects.DistortionModeHardClip
	case "tanh":
		return effects.DistortionModeTanh
	case "waveshaper1":
		return effects.DistortionModeWaveshaper1
	case "waveshaper2":
		return effects.DistortionModeWaveshaper2
	case "waveshaper3":
		return effects.DistortionModeWaveshaper3
	case "waveshaper4":
		return effects.DistortionModeWaveshaper4
	case "waveshaper5":
		return effects.DistortionModeWaveshaper5
	case "waveshaper6":
		return effects.DistortionModeWaveshaper6
	case "waveshaper7":
		return effects.DistortionModeWaveshaper7
	case "waveshaper8":
		return effects.DistortionModeWaveshaper8
	case "saturate":
		return effects.DistortionModeSaturate
	case "saturate2":
		return effects.DistortionModeSaturate2
	case "softsat":
		return effects.DistortionModeSoftSat
	case "chebyshev":
		return effects.DistortionModeChebyshev
	case "softclip":
		fallthrough
	default:
		return effects.DistortionModeSoftClip
	}
}

func normalizeDistortionApproxMode(raw string) effects.DistortionApproxMode {
	switch raw {
	case "polynomial":
		return effects.DistortionApproxPolynomial
	case "exact":
		fallthrough
	default:
		return effects.DistortionApproxExact
	}
}

func normalizeChebyshevHarmonicMode(raw string) effects.ChebyshevHarmonicMode {
	switch raw {
	case "odd":
		return effects.ChebyshevHarmonicOdd
	case "even":
		return effects.ChebyshevHarmonicEven
	case "all":
		fallthrough
	default:
		return effects.ChebyshevHarmonicAll
	}
}

func normalizeTransformerQuality(raw string) effects.TransformerQuality {
	switch raw {
	case "lightweight":
		return effects.TransformerQualityLightweight
	case "high":
		fallthrough
	default:
		return effects.TransformerQualityHigh
	}
}

func normalizeSpectralFreezePhaseMode(raw string) effects.SpectralFreezePhaseMode {
	switch raw {
	case "hold":
		return effects.SpectralFreezePhaseHold
	case "advance":
		fallthrough
	default:
		return effects.SpectralFreezePhaseAdvance
	}
}

func normalizeDynamicsTopology(raw string) dynamics.DynamicsTopology {
	switch raw {
	case "feedback":
		return dynamics.DynamicsTopologyFeedback
	case "feedforward":
		fallthrough
	default:
		return dynamics.DynamicsTopologyFeedforward
	}
}

func normalizeDynamicsDetectorMode(raw string) dynamics.DetectorMode {
	switch raw {
	case "rms":
		return dynamics.DetectorModeRMS
	case "peak":
		fallthrough
	default:
		return dynamics.DetectorModePeak
	}
}

func normalizeDeesserMode(raw string) dynamics.DeEsserMode {
	switch raw {
	case "wideband":
		return dynamics.DeEsserWideband
	case "splitband":
		fallthrough
	default:
		return dynamics.DeEsserSplitBand
	}
}

func normalizeDeesserDetector(raw string) dynamics.DeEsserDetector {
	switch raw {
	case kindHighpass:
		return dynamics.DeEsserDetectHighpass
	case kindBandpass:
		fallthrough
	default:
		return dynamics.DeEsserDetectBandpass
	}
}
