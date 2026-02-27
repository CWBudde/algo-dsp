package spatial

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

const (
	defaultWidenerWidth = 1.0

	minWidenerWidth = 0.0
	maxWidenerWidth = 4.0

	defaultWidenerBassMonoFreq = 0.0 // disabled by default

	minWidenerBassMonoFreq = 20.0
	maxWidenerBassMonoFreq = 500.0

	bassMonoFilterOrder = 2
)

// StereoWidenerOption mutates stereo widener construction parameters.
type StereoWidenerOption func(*stereoWidenerConfig) error

type stereoWidenerConfig struct {
	width        float64
	bassMonoFreq float64 // 0 means disabled
}

func defaultStereoWidenerConfig() stereoWidenerConfig {
	return stereoWidenerConfig{
		width:        defaultWidenerWidth,
		bassMonoFreq: defaultWidenerBassMonoFreq,
	}
}

// WithWidth sets the stereo width factor.
// 0 = mono, 1 = unchanged, >1 = widened (up to 4).
func WithWidth(width float64) StereoWidenerOption {
	return func(cfg *stereoWidenerConfig) error {
		if width < minWidenerWidth || width > maxWidenerWidth ||
			math.IsNaN(width) || math.IsInf(width, 0) {
			return fmt.Errorf("stereo widener width must be in [%g, %g]: %f",
				minWidenerWidth, maxWidenerWidth, width)
		}

		cfg.width = width

		return nil
	}
}

// WithBassMonoFreq enables bass mono mode: frequencies below the given
// crossover (in Hz) are collapsed to mono to preserve low-end coherence.
// Set to 0 to disable (default). Valid range when enabled: [20, 500] Hz.
func WithBassMonoFreq(freq float64) StereoWidenerOption {
	return func(cfg *stereoWidenerConfig) error {
		if freq == 0 {
			cfg.bassMonoFreq = 0
			return nil
		}

		if freq < minWidenerBassMonoFreq || freq > maxWidenerBassMonoFreq ||
			math.IsNaN(freq) || math.IsInf(freq, 0) {
			return fmt.Errorf("stereo widener bass mono freq must be 0 (disabled) or in [%g, %g]: %f",
				minWidenerBassMonoFreq, maxWidenerBassMonoFreq, freq)
		}

		cfg.bassMonoFreq = freq

		return nil
	}
}

// StereoWidener adjusts the width of a stereo image using mid/side processing.
//
// The processor encodes left/right channels into mid (sum) and side (difference)
// components, scales the side signal by a configurable width factor, and decodes
// back to left/right. A width of 1 leaves the signal unchanged, 0 collapses to
// mono, and values above 1 widen the stereo image (up to 4x).
//
// An optional bass mono crossover collapses low frequencies to mono, which
// preserves low-end coherence and phase alignment when the width is increased.
//
// This processor is stereo, real-time safe, and not thread-safe.
type StereoWidener struct {
	sampleRate   float64
	width        float64
	bassMonoFreq float64

	// Bass mono crossover filters (nil when disabled).
	// LP filters extract the bass for mono collapsing.
	// HP filters extract the portion that gets widened.
	bassLPL *biquad.Chain
	bassLPR *biquad.Chain
	bassHPL *biquad.Chain
	bassHPR *biquad.Chain
}

// NewStereoWidener creates a stereo widener with practical defaults and
// optional overrides.
func NewStereoWidener(sampleRate float64, opts ...StereoWidenerOption) (*StereoWidener, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("stereo widener sample rate must be > 0 and finite: %f", sampleRate)
	}

	cfg := defaultStereoWidenerConfig()

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	w := &StereoWidener{
		sampleRate:   sampleRate,
		width:        cfg.width,
		bassMonoFreq: cfg.bassMonoFreq,
	}

	if cfg.bassMonoFreq > 0 {
		err := w.rebuildBassMonoFilters()
		if err != nil {
			return nil, err
		}
	}

	return w, nil
}

// ProcessStereo processes a single stereo sample pair and returns the
// widened left and right outputs.
func (w *StereoWidener) ProcessStereo(left, right float64) (float64, float64) {
	if w.bassLPL != nil {
		return w.processStereoWithBassMono(left, right)
	}

	return w.processStereoSimple(left, right)
}

func (w *StereoWidener) processStereoSimple(left, right float64) (float64, float64) {
	mid := (left + right) * 0.5
	side := (left - right) * 0.5

	outL := mid + side*w.width
	outR := mid - side*w.width

	return outL, outR
}

func (w *StereoWidener) processStereoWithBassMono(left, right float64) (float64, float64) {
	// Split into bass and non-bass bands.
	bassL := w.bassLPL.ProcessSample(left)
	bassR := w.bassLPR.ProcessSample(right)
	highL := w.bassHPL.ProcessSample(left)
	highR := w.bassHPR.ProcessSample(right)

	// Bass band: collapse to mono.
	bassMono := (bassL + bassR) * 0.5

	// High band: apply M/S widening.
	midHigh := (highL + highR) * 0.5
	sideHigh := (highL - highR) * 0.5

	outL := bassMono + midHigh + sideHigh*w.width
	outR := bassMono + midHigh - sideHigh*w.width

	return outL, outR
}

// ProcessStereoInPlace applies stereo widening to paired left/right buffers
// in place. Both buffers must have the same length.
func (w *StereoWidener) ProcessStereoInPlace(left, right []float64) error {
	if len(left) != len(right) {
		return fmt.Errorf("stereo widener: left and right buffers must have equal length: %d != %d",
			len(left), len(right))
	}

	for i := range left {
		left[i], right[i] = w.ProcessStereo(left[i], right[i])
	}

	return nil
}

// ProcessInterleavedInPlace applies stereo widening to an interleaved stereo
// buffer (L, R, L, R, ...) in place. The buffer length must be even.
func (w *StereoWidener) ProcessInterleavedInPlace(buf []float64) error {
	if len(buf)%2 != 0 {
		return fmt.Errorf("stereo widener: interleaved buffer length must be even: %d", len(buf))
	}

	for i := 0; i < len(buf); i += 2 {
		buf[i], buf[i+1] = w.ProcessStereo(buf[i], buf[i+1])
	}

	return nil
}

// Reset clears internal filter state.
func (w *StereoWidener) Reset() {
	if w.bassLPL != nil {
		w.bassLPL.Reset()
	}

	if w.bassLPR != nil {
		w.bassLPR.Reset()
	}

	if w.bassHPL != nil {
		w.bassHPL.Reset()
	}

	if w.bassHPR != nil {
		w.bassHPR.Reset()
	}
}

// SampleRate returns the sample rate in Hz.
func (w *StereoWidener) SampleRate() float64 { return w.sampleRate }

// Width returns the current stereo width factor.
func (w *StereoWidener) Width() float64 { return w.width }

// BassMonoFreq returns the bass mono crossover frequency in Hz, or 0 if disabled.
func (w *StereoWidener) BassMonoFreq() float64 { return w.bassMonoFreq }

// SetSampleRate updates the sample rate and recomputes internal filters.
func (w *StereoWidener) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("stereo widener sample rate must be > 0 and finite: %f", sampleRate)
	}

	w.sampleRate = sampleRate
	if w.bassMonoFreq > 0 {
		return w.rebuildBassMonoFilters()
	}

	return nil
}

// SetWidth sets the stereo width factor.
// 0 = mono, 1 = unchanged, >1 = widened (up to 4).
func (w *StereoWidener) SetWidth(width float64) error {
	if width < minWidenerWidth || width > maxWidenerWidth ||
		math.IsNaN(width) || math.IsInf(width, 0) {
		return fmt.Errorf("stereo widener width must be in [%g, %g]: %f",
			minWidenerWidth, maxWidenerWidth, width)
	}

	w.width = width

	return nil
}

// SetBassMonoFreq sets the bass mono crossover frequency. Set to 0 to disable.
func (w *StereoWidener) SetBassMonoFreq(freq float64) error {
	if freq == 0 {
		w.bassMonoFreq = 0
		w.bassLPL = nil
		w.bassLPR = nil
		w.bassHPL = nil
		w.bassHPR = nil

		return nil
	}

	if freq < minWidenerBassMonoFreq || freq > maxWidenerBassMonoFreq ||
		math.IsNaN(freq) || math.IsInf(freq, 0) {
		return fmt.Errorf("stereo widener bass mono freq must be 0 (disabled) or in [%g, %g]: %f",
			minWidenerBassMonoFreq, maxWidenerBassMonoFreq, freq)
	}

	w.bassMonoFreq = freq

	return w.rebuildBassMonoFilters()
}

func (w *StereoWidener) rebuildBassMonoFilters() error {
	freq := w.bassMonoFreq
	if freq <= 0 {
		return nil
	}

	if freq >= w.sampleRate*0.5 {
		return fmt.Errorf("stereo widener bass mono freq must be below Nyquist (%g): %f",
			w.sampleRate*0.5, freq)
	}

	lpCoeffs := design.ButterworthLP(freq, bassMonoFilterOrder, w.sampleRate)

	hpCoeffs := design.ButterworthHP(freq, bassMonoFilterOrder, w.sampleRate)
	if len(lpCoeffs) == 0 || len(hpCoeffs) == 0 {
		return fmt.Errorf("stereo widener bass mono filter design failed for freq=%g sr=%g",
			freq, w.sampleRate)
	}

	w.bassLPL = biquad.NewChain(lpCoeffs)
	w.bassLPR = biquad.NewChain(lpCoeffs)
	w.bassHPL = biquad.NewChain(hpCoeffs)
	w.bassHPR = biquad.NewChain(hpCoeffs)

	return nil
}
