package separate

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/stft"
)

// Default HPSS settings.
const (
	// DefaultNFFT is the default FFT size of the signal-level APIs.
	DefaultNFFT = 2048
	// DefaultHop is the default hop size of the signal-level APIs.
	DefaultHop = 512
	// DefaultHarmonicKernel is the default length, in frames, of the median
	// filter across time that enhances harmonic (steady) components.
	DefaultHarmonicKernel = 17
	// DefaultPercussiveKernel is the default length, in bins, of the median
	// filter across frequency that enhances percussive (broadband) components.
	DefaultPercussiveKernel = 17
	// DefaultPower is the default soft-mask power (Wiener-style masks).
	DefaultPower = 2.0
	// DefaultCentreExponent is the default exponent applied to the
	// inter-channel coherence by [CentreExtractor].
	DefaultCentreExponent = 2.0
)

// stftSettings describes the transform used by the signal-level APIs.
type stftSettings struct {
	nfft int
	hop  int
	opts []stft.Option
}

func defaultSTFTSettings() stftSettings {
	return stftSettings{nfft: DefaultNFFT, hop: DefaultHop}
}

func (s stftSettings) build() (*stft.STFT, error) {
	t, err := stft.New(s.nfft, s.hop, s.opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidSTFT, err)
	}

	return t, nil
}

// Option configures an [HPSS].
type Option func(*hpssConfig) error

type hpssConfig struct {
	harmKernel int
	percKernel int
	power      float64
	margin     bool
	marginH    float64
	marginP    float64
	stft       stftSettings
}

func defaultHPSSConfig() hpssConfig {
	return hpssConfig{
		harmKernel: DefaultHarmonicKernel,
		percKernel: DefaultPercussiveKernel,
		power:      DefaultPower,
		marginH:    1,
		marginP:    1,
		stft:       defaultSTFTSettings(),
	}
}

// WithKernels sets the median-filter lengths: harmonic is the length in
// frames of the filter across time, percussive the length in bins of the
// filter across frequency. Both must be odd and >= 1. Longer harmonic
// kernels demand steadier tones; longer percussive kernels demand broader
// transients. Defaults are [DefaultHarmonicKernel] and
// [DefaultPercussiveKernel].
func WithKernels(harmonic, percussive int) Option {
	return func(cfg *hpssConfig) error {
		for _, k := range []int{harmonic, percussive} {
			if k < 1 || k%2 == 0 {
				return fmt.Errorf("%w: %d must be odd and >= 1", ErrInvalidKernel, k)
			}
		}

		cfg.harmKernel = harmonic
		cfg.percKernel = percussive

		return nil
	}
}

// WithPower sets the soft-mask power p > 0 (default [DefaultPower]). p = 1
// gives magnitude-ratio masks, p = 2 Wiener-style masks, and p = +Inf binary
// masks.
func WithPower(p float64) Option {
	return func(cfg *hpssConfig) error {
		err := checkPower(p)
		if err != nil {
			return err
		}

		cfg.power = p

		return nil
	}
}

// WithMargin enables the margin variant of Driedger et al. (2014) with
// separation factors harmonic, percussive >= 1, in the form librosa uses:
//
//	M_h = H^p / (H^p + (βh·P)^p)
//	M_p = P^p / (P^p + (βp·H)^p)
//	M_r = 1 − M_h − M_p
//
// where H and P are the median-filtered magnitudes. A bin goes to the
// harmonic or percussive output only to the extent that it dominates the
// other one by the margin; the rest is the residual output. With
// WithPower(math.Inf(1)) the masks are binary (harmonic where H > βh·P,
// percussive where P > βp·H, residual elsewhere), as in the paper. Margins
// of 1 reproduce the plain masks.
func WithMargin(harmonic, percussive float64) Option {
	return func(cfg *hpssConfig) error {
		for _, b := range []float64{harmonic, percussive} {
			if !(b >= 1) || math.IsInf(b, 1) {
				return fmt.Errorf("%w: %g must be finite and >= 1", ErrInvalidMargin, b)
			}
		}

		cfg.margin = true
		cfg.marginH = harmonic
		cfg.marginP = percussive

		return nil
	}
}

// WithSTFT sets the transform used by [HPSS.SeparateSignal]: FFT size nfft,
// hop size hop and dsp/stft options (window, framing, scaling). The default
// is nfft [DefaultNFFT], hop [DefaultHop], periodic Hann window and centred
// zero-padded framing, which reconstructs every sample. The options slice is
// copied; the settings are validated by the constructor.
func WithSTFT(nfft, hop int, opts ...stft.Option) Option {
	s := stftSettings{nfft: nfft, hop: hop, opts: append([]stft.Option(nil), opts...)}

	return func(cfg *hpssConfig) error {
		cfg.stft = s

		return nil
	}
}

// CentreOption configures a [CentreExtractor].
type CentreOption func(*centreConfig) error

type centreConfig struct {
	exponent float64
	stft     stftSettings
}

func defaultCentreConfig() centreConfig {
	return centreConfig{exponent: DefaultCentreExponent, stft: defaultSTFTSettings()}
}

// WithCentreExponent sets the exponent e > 0 applied to the inter-channel
// coherence (default [DefaultCentreExponent]). Larger values pass only bins
// that are closer to identical in both channels.
func WithCentreExponent(e float64) CentreOption {
	return func(cfg *centreConfig) error {
		if !(e > 0) || math.IsInf(e, 1) {
			return fmt.Errorf("%w: %g must be finite and > 0", ErrInvalidExponent, e)
		}

		cfg.exponent = e

		return nil
	}
}

// WithCentreSTFT sets the transform used by [CentreExtractor.SeparateSignal];
// see [WithSTFT] for the meaning and defaults.
func WithCentreSTFT(nfft, hop int, opts ...stft.Option) CentreOption {
	s := stftSettings{nfft: nfft, hop: hop, opts: append([]stft.Option(nil), opts...)}

	return func(cfg *centreConfig) error {
		cfg.stft = s

		return nil
	}
}

// applyOptions applies opts to cfg, rejecting nil options.
func applyOptions[C any, O ~func(*C) error](cfg *C, opts []O) error {
	for i, opt := range opts {
		if opt == nil {
			return fmt.Errorf("%w: option %d", ErrNilOption, i)
		}

		err := opt(cfg)
		if err != nil {
			return err
		}
	}

	return nil
}
