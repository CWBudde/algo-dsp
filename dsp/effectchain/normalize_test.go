package effectchain

import (
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/effects"
	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
)

func TestNormalizeFilterFamily(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		nodeType string
		want     string
	}{
		{"moog node always returns moog", "", "filter-moog", familyMoog},
		{"moog node ignores raw", "rbj", "filter-moog", familyMoog},
		{"empty defaults to rbj", "", "filter", familyRBJ},
		{"rbj passthrough", "rbj", "filter", familyRBJ},
		{"butterworth", "butterworth", "filter", familyButterworth},
		{"bessel", "bessel", "filter", familyBessel},
		{"chebyshev1", "chebyshev1", "filter", familyChebyshev1},
		{"chebyshev2", "chebyshev2", "filter", familyChebyshev2},
		{"elliptic", "elliptic", "filter", familyElliptic},
		{"moog", "moog", "filter", familyMoog},
		{"unknown defaults to rbj", "invalid", "filter", familyRBJ},
		{"case insensitive", "Butterworth", "filter", familyButterworth},
		{"trims whitespace", " rbj ", "filter", familyRBJ},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := normalizeFilterFamily(tt.raw, tt.nodeType)
			if got != tt.want {
				t.Errorf("normalizeFilterFamily(%q, %q) = %q, want %q", tt.raw, tt.nodeType, got, tt.want)
			}
		})
	}
}

func TestNormalizeFilterKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		nodeType string
		raw      string
		want     string
	}{
		{"moog always lowpass", "filter-moog", "highpass", "lowpass"},
		{"filter-highpass default", "filter-highpass", "", "highpass"},
		{"filter-bandpass default", "filter-bandpass", "", "bandpass"},
		{"filter-notch default", "filter-notch", "", "notch"},
		{"filter-allpass default", "filter-allpass", "", "allpass"},
		{"filter-peak default", "filter-peak", "", "peak"},
		{"filter-lowshelf default", "filter-lowshelf", "", "lowshelf"},
		{"filter-highshelf default", "filter-highshelf", "", "highshelf"},
		{"generic filter defaults to lowpass", "filter", "", "lowpass"},
		{"explicit kind overrides", "filter", "highpass", "highpass"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := normalizeFilterKind(tt.nodeType, tt.raw)
			if got != tt.want {
				t.Errorf("normalizeFilterKind(%q, %q) = %q, want %q", tt.nodeType, tt.raw, got, tt.want)
			}
		})
	}
}

func TestNormalizeEQTypeForChain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"highpass", "highpass"},
		{"lowpass", "lowpass"},
		{"bandpass", "bandpass"},
		{"notch", "notch"},
		{"allpass", "allpass"},
		{"peak", "peak"},
		{"highshelf", "highshelf"},
		{"lowshelf", "lowshelf"},
		{"bandeq", "peak"},
		{"band-eq", "peak"},
		{"bandeqpeak", "peak"},
		{"bell", "peak"},
		{"bandbell", "peak"},
		{"unknown", "peak"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := normalizeEQTypeForChain(tt.input)
			if got != tt.want {
				t.Errorf("normalizeEQTypeForChain(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMoogOversamplingFromOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		order int
		want  int
	}{
		{1, 1},
		{3, 1},
		{4, 2},
		{7, 2},
		{8, 4},
		{11, 4},
		{12, 8},
		{16, 8},
	}

	for _, tt := range tests {
		got := moogOversamplingFromOrder(tt.order)
		if got != tt.want {
			t.Errorf("moogOversamplingFromOrder(%d) = %d, want %d", tt.order, got, tt.want)
		}
	}
}

func TestNormalizeDistortionMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw  string
		want effects.DistortionMode
	}{
		{"hardclip", effects.DistortionModeHardClip},
		{"tanh", effects.DistortionModeTanh},
		{"softclip", effects.DistortionModeSoftClip},
		{"saturate", effects.DistortionModeSaturate},
		{"chebyshev", effects.DistortionModeChebyshev},
		{"unknown", effects.DistortionModeSoftClip},
		{"", effects.DistortionModeSoftClip},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()

			got := normalizeDistortionMode(tt.raw)
			if got != tt.want {
				t.Errorf("normalizeDistortionMode(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNormalizeDistortionApproxMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw  string
		want effects.DistortionApproxMode
	}{
		{"polynomial", effects.DistortionApproxPolynomial},
		{"exact", effects.DistortionApproxExact},
		{"unknown", effects.DistortionApproxExact},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()

			got := normalizeDistortionApproxMode(tt.raw)
			if got != tt.want {
				t.Errorf("normalizeDistortionApproxMode(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNormalizeChebyshevHarmonicMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw  string
		want effects.ChebyshevHarmonicMode
	}{
		{"odd", effects.ChebyshevHarmonicOdd},
		{"even", effects.ChebyshevHarmonicEven},
		{"all", effects.ChebyshevHarmonicAll},
		{"unknown", effects.ChebyshevHarmonicAll},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()

			got := normalizeChebyshevHarmonicMode(tt.raw)
			if got != tt.want {
				t.Errorf("normalizeChebyshevHarmonicMode(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNormalizeDynamicsTopology(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw  string
		want dynamics.DynamicsTopology
	}{
		{"feedback", dynamics.DynamicsTopologyFeedback},
		{"feedforward", dynamics.DynamicsTopologyFeedforward},
		{"unknown", dynamics.DynamicsTopologyFeedforward},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()

			got := normalizeDynamicsTopology(tt.raw)
			if got != tt.want {
				t.Errorf("normalizeDynamicsTopology(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNormalizeDynamicsDetectorMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw  string
		want dynamics.DetectorMode
	}{
		{"rms", dynamics.DetectorModeRMS},
		{"peak", dynamics.DetectorModePeak},
		{"unknown", dynamics.DetectorModePeak},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()

			got := normalizeDynamicsDetectorMode(tt.raw)
			if got != tt.want {
				t.Errorf("normalizeDynamicsDetectorMode(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNormalizeDeesserMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw  string
		want dynamics.DeEsserMode
	}{
		{"wideband", dynamics.DeEsserWideband},
		{"splitband", dynamics.DeEsserSplitBand},
		{"unknown", dynamics.DeEsserSplitBand},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()

			got := normalizeDeesserMode(tt.raw)
			if got != tt.want {
				t.Errorf("normalizeDeesserMode(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNormalizeDeesserDetector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw  string
		want dynamics.DeEsserDetector
	}{
		{"highpass", dynamics.DeEsserDetectHighpass},
		{"bandpass", dynamics.DeEsserDetectBandpass},
		{"unknown", dynamics.DeEsserDetectBandpass},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()

			got := normalizeDeesserDetector(tt.raw)
			if got != tt.want {
				t.Errorf("normalizeDeesserDetector(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}
