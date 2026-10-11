package chroma

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/cqt"
	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
)

// Sentinel errors returned by this package. Errors are wrapped with context,
// so test for them with errors.Is.
var (
	// ErrNilOption reports a nil [Option]. Errors wrapping it also wrap
	// [ErrInvalidOption].
	ErrNilOption = errors.New("chroma: nil option")
	// ErrInvalidOption reports an invalid option value or a configuration
	// the constant-Q transform rejects; such errors also wrap the dsp/cqt
	// sentinel (for example cqt.ErrHop or cqt.ErrNyquist).
	ErrInvalidOption = errors.New("chroma: invalid option")
	// ErrInvalidSampleRate reports a sample rate that is not a positive,
	// finite number.
	ErrInvalidSampleRate = errors.New("chroma: invalid sample rate")
	// ErrEmptyInput reports an empty signal.
	ErrEmptyInput = errors.New("chroma: empty input")
	// ErrInvalidArgument reports an invalid argument of [BeatSync]: an
	// invalid grid or frame rate, a negative beat count, or chroma rows of
	// unequal length.
	ErrInvalidArgument = errors.New("chroma: invalid argument")
)

// pitchClasses is the number of chroma bins.
const pitchClasses = 12

// silentPower is the in-band power below which a frame is silent, as in
// melody.
const silentPower = 1e-12

// fold maps one CQT bin to one pitch class, or splits it evenly between two
// neighbouring classes when it lies half-way between two semitones.
type fold struct {
	bin   int
	pc    int
	other int  // second class of a split bin
	split bool // half the power to pc, half to other
}

// Chromagram computes per-frame 12-bin chroma from a constant-Q transform.
// Create it with [New].
//
// A Chromagram holds the transform's scratch buffers and is not safe for
// concurrent use; see [Chromagram.Clone].
type Chromagram struct {
	tr    *cqt.Transform
	folds []fold
	freqs []float64 // centre frequencies of the folded bins
	gain  float64   // power gain undoing cqt's early downsampling factor
	norm  Normalization

	mag []float64 // scratch: frame-major CQT magnitudes
}

// New returns a Chromagram for signals sampled at sampleRate Hz. See the
// package documentation for the derived constant-Q transform and the folding
// rule.
//
// Besides option errors, New returns an error wrapping [ErrInvalidOption]
// (and the dsp/cqt sentinel) when the transform cannot be built, for example
// cqt.ErrNyquist when the band reaches above the Nyquist frequency or
// cqt.ErrHop when the hop is not divisible by 2^(octaves−1), when the
// transform's bins per octave are not a multiple of 12, or when no bin lies
// inside the band.
func New(sampleRate float64, opts ...Option) (*Chromagram, error) {
	if !finitePositive(sampleRate) {
		return nil, fmt.Errorf("%w: %g", ErrInvalidSampleRate, sampleRate)
	}

	cfg := defaultConfig()

	for i, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("%w: option %d: %w", ErrNilOption, i, ErrInvalidOption)
		}

		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	ref := cfg.referenceHz
	loMIDI := pitch.FrequencyToMIDI(cfg.minHz, ref)
	hiMIDI := pitch.FrequencyToMIDI(cfg.maxHz, ref)

	// The derived bins are j/k semitones on the tuned grid, k per semitone.
	k := float64(cfg.binsPerOctave / pitchClasses)
	jLo := math.Ceil((loMIDI - tolSemitones) * k)
	jHi := math.Floor((hiMIDI + tolSemitones) * k)

	if jHi < jLo {
		return nil, fmt.Errorf("%w: no bin of %d per octave between %g and %g Hz",
			ErrInvalidOption, cfg.binsPerOctave, cfg.minHz, cfg.maxHz)
	}

	cqtOpts := make([]cqt.Option, 0, len(cfg.cqtOpts)+7)
	cqtOpts = append(cqtOpts,
		cqt.WithBinsPerOctave(cfg.binsPerOctave),
		cqt.WithFMin(pitch.MIDIToFrequency(jLo/k, ref)),
		cqt.WithBins(int(jHi-jLo)+1),
		cqt.WithHopLength(DefaultHop),
		cqt.WithNormalization(cqt.NormalizationWrap),
		cqt.WithBasisNorm(cqt.NormL1),
		cqt.WithCenter(cqt.PadZero),
	)
	cqtOpts = append(cqtOpts, cfg.cqtOpts...)
	cqtOpts = append(cqtOpts, cqt.WithOutput(cqt.OutputMagnitude))

	tr, err := cqt.New(sampleRate, cqtOpts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidOption, err)
	}

	freqs := tr.Frequencies()

	err = checkBinsPerOctave(freqs)
	if err != nil {
		return nil, err
	}

	c := &Chromagram{tr: tr, norm: cfg.norm}

	for b, f := range freqs {
		m := pitch.FrequencyToMIDI(f, ref)
		if m < loMIDI-tolSemitones || m > hiMIDI+tolSemitones {
			continue
		}

		c.folds = append(c.folds, foldOf(b, m))
		c.freqs = append(c.freqs, f)
	}

	if len(c.folds) == 0 {
		return nil, fmt.Errorf("%w: no CQT bin between %g and %g Hz", ErrInvalidOption, cfg.minHz, cfg.maxHz)
	}

	g := 1 / float64(tr.DownsampleFactor())
	c.gain = g * g

	return c, nil
}

// checkBinsPerOctave requires the transform's bin spacing to be 1/(12k)
// octave for an integer k. A single bin has no spacing and passes.
func checkBinsPerOctave(freqs []float64) error {
	if len(freqs) < 2 {
		return nil
	}

	bpo := 1 / math.Log2(freqs[1]/freqs[0])
	r := math.Round(bpo)

	if math.Abs(bpo-r) > 1e-6 || int(r)%pitchClasses != 0 || r < pitchClasses {
		return fmt.Errorf("%w: transform has %.6g bins per octave, not a multiple of 12", ErrInvalidOption, bpo)
	}

	return nil
}

// foldOf returns the fold of bin b at fractional MIDI note m.
func foldOf(b int, m float64) fold {
	lower := math.Floor(m)
	if math.Abs(m-lower-0.5) < tolSemitones {
		return fold{bin: b, pc: pitchClassOf(int(lower)), other: pitchClassOf(int(lower) + 1), split: true}
	}

	pc := pitchClassOf(int(math.Round(m)))

	return fold{bin: b, pc: pc, other: pc}
}

func pitchClassOf(midi int) int { return ((midi % pitchClasses) + pitchClasses) % pitchClasses }

// Clone returns an independent Chromagram with the same configuration. The
// transform's kernels and the fold tables are shared (they are immutable);
// scratch buffers are not, so the clone can be used concurrently with the
// original. Clone must not run while c is processing on another goroutine.
func (c *Chromagram) Clone() *Chromagram {
	d := *c
	d.tr = c.tr.Clone()
	d.mag = nil

	return &d
}

// FrameRate returns the number of frames per second: the sample rate divided
// by the hop size given to the transform (before early downsampling).
// Frame i is centred on time i/FrameRate().
func (c *Chromagram) FrameRate() float64 { return c.tr.SampleRate() / float64(c.tr.Hop()) }

// FrameCount returns the number of frames [Chromagram.Process] returns for n
// input samples: n/hop+1 (integer division) without early downsampling, see
// cqt.Transform.FrameCount; 0 if n samples are too short.
func (c *Chromagram) FrameCount(n int) int { return c.tr.FrameCount(n) }

// Frequencies returns a copy of the centre frequencies in Hz of the CQT bins
// that are folded into the chroma, lowest first. Bins of the transform
// outside the band (only present when [WithCQT] widens the transform) are
// not listed.
func (c *Chromagram) Frequencies() []float64 { return append([]float64(nil), c.freqs...) }

// Process returns the chroma of x: one row per pitch class, indexed by
// pitch.PitchClass (C, C#, …, B), of FrameCount(len(x)) frames each. Frame i
// is centred on sample i*hop. Each frame holds the power of the folded bins
// per class, normalized as selected by [WithNormalization]; silent frames are
// all zero.
//
// Process allocates only the result once its scratch buffer has grown to the
// input length. It returns an error wrapping [ErrEmptyInput] for an empty
// signal and one wrapping cqt.ErrSignalTooShort for a signal too short to be
// transformed. NaN or Inf samples propagate into the frames that cover them.
func (c *Chromagram) Process(x []float64) ([12][]float64, error) {
	var out [12][]float64

	if len(x) == 0 {
		return out, fmt.Errorf("%w: no samples", ErrEmptyInput)
	}

	frames := c.tr.FrameCount(len(x))
	need := c.tr.OutputLen(len(x))

	if cap(c.mag) < need {
		c.mag = make([]float64, need)
	}

	mag := c.mag[:need]

	err := c.tr.ProcessInto(mag, x)
	if err != nil {
		return out, fmt.Errorf("chroma: %w", err)
	}

	data := make([]float64, pitchClasses*frames)
	for pc := range out {
		out[pc] = data[pc*frames : (pc+1)*frames : (pc+1)*frames]
	}

	bins := c.tr.Bins()

	for i := range frames {
		v := c.frame(mag[i*bins : (i+1)*bins])
		for pc := range out {
			out[pc][i] = v[pc]
		}
	}

	return out, nil
}

// frame folds and normalizes the magnitudes of one frame.
func (c *Chromagram) frame(mag []float64) [12]float64 {
	var v [12]float64

	total := 0.0

	for _, f := range c.folds {
		m := mag[f.bin]
		p := m * m * c.gain
		total += p

		if f.split {
			v[f.pc] += p / 2
			v[f.other] += p / 2
		} else {
			v[f.pc] += p
		}
	}

	if total < silentPower {
		return [12]float64{}
	}

	scale := 1.0

	switch c.norm {
	case NormMax:
		scale = 0
		for _, p := range v {
			scale = math.Max(scale, p)
		}
	case NormL1:
		scale = total
	case NormL2:
		scale = 0
		for _, p := range v {
			scale += p * p
		}

		scale = math.Sqrt(scale)
	case NormNone:
	}

	for pc := range v {
		v[pc] /= scale
	}

	return v
}

// Compute is the one-shot form of [New] and [Chromagram.Process]: it returns
// the chroma of x and its frame rate, which harmony.Windows takes directly.
func Compute(x []float64, sampleRate float64, opts ...Option) ([12][]float64, float64, error) {
	c, err := New(sampleRate, opts...)
	if err != nil {
		return [12][]float64{}, 0, err
	}

	out, err := c.Process(x)
	if err != nil {
		return [12][]float64{}, 0, err
	}

	return out, c.FrameRate(), nil
}
