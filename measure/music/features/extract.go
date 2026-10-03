package features

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/core"
	"github.com/cwbudde/algo-dsp/dsp/stft"
	"github.com/cwbudde/algo-dsp/stats/frequency"
)

// DBFloor is the amplitude floor of the dB conversions in this package:
// 20·log10(max(x, DBFloor)), so silence reads -120 dB.
const DBFloor = 1e-6

// Frames holds frame-wise features. All slices have one value per frame;
// frame i is centred at Timing.FrameTime(i).
type Frames struct {
	// Timing is the frame grid.
	Timing
	// RMS is the RMS amplitude of all channels pooled over ±Hop samples.
	RMS []float64
	// Peak is the absolute peak of all channels over ±Hop samples.
	Peak []float64
	// Centroid is the spectral centroid in Hz (zero below the gate).
	Centroid []float64
	// Width is the stereo width side/(mid+side) in [0, 1]; zero unless the
	// input has exactly two channels.
	Width []float64
	// Flux is the positive spectral flux on log1p magnitudes (zero below
	// the gate and in frame 0).
	Flux []float64
	// Bands holds one RMS amplitude envelope per band, band-major:
	// Bands[b][i] is band b in frame i. It is the square root of the
	// one-sided power of the FFT bins in the band, normalized by N·Σw²:
	// every bin counts twice, except DC and (for an even FFT size) the
	// Nyquist bin, which count once. A band covering all bins thus reads
	// the window-weighted RMS of the frame (Parseval).
	Bands [][]float64
	// Spectrogram is the log-frequency spectrogram, or nil unless
	// [WithLogSpectrogram] was given.
	Spectrogram *LogSpectrogram
}

// Len returns the number of frames.
func (f *Frames) Len() int { return len(f.RMS) }

// LogSpectrogram is a frame-major log-frequency spectrogram in dBFS.
type LogSpectrogram struct {
	// Scale is the frequency mapping.
	Scale *LogScale
	// DB holds Scale.Bins() values per frame, frame-major:
	// DB[i*Scale.Bins()+b] is log bin b of frame i, in dBFS
	// (20·log10(max(sqrt(L[b]), DBFloor))).
	DB []float64
}

// BinFrequencies returns the centre frequency of each log bin in Hz.
func (s *LogSpectrogram) BinFrequencies() []float64 { return s.Scale.BinFrequencies() }

// Frame returns the log bins of frame i (a sub-slice of DB, not a copy).
func (s *LogSpectrogram) Frame(i int) []float64 {
	n := s.Scale.Bins()

	return s.DB[i*n : (i+1)*n]
}

// Option configures [Extract].
type Option func(*options) error

type options struct {
	logBins          int
	logMin, logMax   float64
	logSpectrogramOn bool
}

// WithLogSpectrogram makes [Extract] also compute a log-frequency
// spectrogram of bins log bins between fmin and fmax Hz (the AudioVisualizer
// uses 64 bins from 25 Hz to 12 kHz). See [LogScale] for the mapping.
func WithLogSpectrogram(bins int, fmin, fmax float64) Option {
	return func(o *options) error {
		if bins < 1 || !(fmin > 0) || !(fmax > fmin) || math.IsInf(fmax, 0) {
			return fmt.Errorf("%w: log spectrogram %d bins, %v..%v Hz", ErrInvalidArgument, bins, fmin, fmax)
		}

		o.logBins, o.logMin, o.logMax = bins, fmin, fmax
		o.logSpectrogramOn = true

		return nil
	}
}

// Extract computes the frame-wise features of channels (one slice per
// channel, all of equal non-zero length) sampled at cfg.SampleRate. Any
// number of channels is accepted; Width is only computed for two.
func Extract(channels [][]float64, cfg Config, opts ...Option) (*Frames, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, err
	}

	var o options

	for i, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("%w: option %d", ErrNilOption, i)
		}

		err = opt(&o)
		if err != nil {
			return nil, err
		}
	}

	length, err := checkChannels(channels)
	if err != nil {
		return nil, err
	}

	tr, err := stft.New(cfg.FFTSize, cfg.Hop)
	if err != nil {
		return nil, fmt.Errorf("features: %w", err)
	}

	e := newExtractor(tr, cfg, len(channels))

	if o.logSpectrogramOn {
		e.scale, err = NewLogScale(o.logBins, o.logMin, o.logMax, cfg.SampleRate, cfg.FFTSize)
		if err != nil {
			return nil, err
		}
	}

	f := e.allocFrames(tr.FrameCount(length))

	for frame := range f.RMS {
		err = e.spectralFrame(f, channels, frame)
		if err != nil {
			return nil, err
		}

		e.temporalFrame(f, channels, frame, length)
	}

	return f, nil
}

// extractor holds the per-call state of Extract.
type extractor struct {
	cfg      Config
	tr       *stft.STFT
	scale    *LogScale
	norm     float64 // N·Σw², the one-sided power normalization
	channels float64
	bandOf   []int // band index of each FFT bin, or -1

	spec      []complex128
	power     []float64
	mag       []float64
	previous  []float64
	bandPower []float64
	logPower  []float64
}

func newExtractor(tr *stft.STFT, cfg Config, channels int) *extractor {
	windowPower := 0.0
	for _, v := range tr.Window() {
		windowPower += v * v
	}

	bins := tr.Bins()
	e := &extractor{
		cfg:       cfg,
		tr:        tr,
		norm:      float64(cfg.FFTSize) * windowPower,
		channels:  float64(channels),
		bandOf:    make([]int, bins),
		spec:      make([]complex128, bins),
		power:     make([]float64, bins),
		mag:       make([]float64, bins),
		previous:  make([]float64, bins),
		bandPower: make([]float64, bins),
	}

	edges := cfg.BandEdges

	for k := range e.bandOf {
		e.bandOf[k] = -1
		hz := e.binHz(k)

		for b := range len(edges) - 1 {
			if hz >= edges[b] && hz < edges[b+1] {
				e.bandOf[k] = b
			}
		}
	}

	return e
}

// isEdgeBin reports whether FFT bin k is DC or, for an even FFT size, the
// Nyquist bin: the bins without a negative-frequency partner, which count
// once instead of twice in the one-sided band power.
func (e *extractor) isEdgeBin(k int) bool {
	return k == 0 || (e.cfg.FFTSize%2 == 0 && k == e.cfg.FFTSize/2)
}

func (e *extractor) binHz(k int) float64 {
	return float64(k) * e.cfg.SampleRate / float64(e.cfg.FFTSize)
}

func (e *extractor) allocFrames(count int) *Frames {
	f := &Frames{
		Timing:   e.cfg.Timing(),
		RMS:      make([]float64, count),
		Peak:     make([]float64, count),
		Centroid: make([]float64, count),
		Width:    make([]float64, count),
		Flux:     make([]float64, count),
		Bands:    make([][]float64, len(e.cfg.BandEdges)-1),
	}

	for b := range f.Bands {
		f.Bands[b] = make([]float64, count)
	}

	if e.scale != nil {
		e.logPower = make([]float64, e.scale.Bins())
		f.Spectrogram = &LogSpectrogram{Scale: e.scale, DB: make([]float64, count*e.scale.Bins())}
	}

	return f
}

// spectralFrame computes centroid, flux, bands and the log spectrogram of
// one frame. The operation order matches the AudioVisualizer reference.
func (e *extractor) spectralFrame(f *Frames, channels [][]float64, frame int) error {
	clear(e.power)

	for _, ch := range channels {
		err := e.tr.FrameInto(e.spec, ch, frame)
		if err != nil {
			return fmt.Errorf("features: %w", err)
		}

		for k, v := range e.spec {
			e.power[k] += (real(v)*real(v) + imag(v)*imag(v)) / e.channels
		}
	}

	for k, p := range e.power {
		e.mag[k] = math.Sqrt(p)

		logmag := math.Log1p(e.mag[k])
		if frame > 0 {
			f.Flux[frame] += math.Max(0, logmag-e.previous[k])
		}

		e.previous[k] = logmag

		q := 2 * p / e.norm
		e.bandPower[k] = q

		if b := e.bandOf[k]; b >= 0 {
			if e.isEdgeBin(k) {
				// DC and Nyquist have no negative-frequency partner.
				f.Bands[b][frame] += p / e.norm
			} else {
				f.Bands[b][frame] += q
			}
		}
	}

	for b := range f.Bands {
		f.Bands[b][frame] = math.Sqrt(f.Bands[b][frame])
	}

	if e.scale != nil {
		e.scale.apply(e.logPower, e.bandPower)

		row := f.Spectrogram.Frame(frame)
		for b, v := range e.logPower {
			row[b] = core.LinearToDBFloor(math.Sqrt(v), DBFloor)
		}
	}

	f.Centroid[frame] = frequency.Centroid(e.mag, e.cfg.SampleRate)

	return nil
}

// temporalFrame computes RMS, peak and width of one frame and applies the
// RMS gate to centroid and flux.
func (e *extractor) temporalFrame(f *Frames, channels [][]float64, frame, length int) {
	hop := e.cfg.Hop
	center := frame * hop
	n, energy, mid, side := 0.0, 0.0, 0.0, 0.0
	stereo := len(channels) == 2

	for j := max(0, center-hop); j < min(length, center+hop); j++ {
		for _, ch := range channels {
			v := ch[j]
			energy += v * v
			n++
			f.Peak[frame] = math.Max(f.Peak[frame], math.Abs(v))
		}

		if stereo {
			l, r := channels[0][j], channels[1][j]
			mid += (l + r) * (l + r) / 4
			side += (l - r) * (l - r) / 4
		}
	}

	f.RMS[frame] = math.Sqrt(energy / math.Max(1, n))

	if mid+side > 1e-12 {
		f.Width[frame] = side / (mid + side)
	}

	if f.RMS[frame] < e.cfg.Gate {
		f.Centroid[frame] = 0
		f.Flux[frame] = 0
	}
}
