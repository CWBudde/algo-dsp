package harmony

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/core"
	"github.com/cwbudde/algo-dsp/measure/music/melody"
)

// DefaultLevelFloor is the default linear RMS floor of [Window.LevelDB]:
// windows without frames or with silent frames report 20·log10(1e-6) =
// −120 dBFS (see [WithLevelFloor]).
const DefaultLevelFloor = 1e-6

// Span is a time span [Start, End) in seconds, for example a beat, half bar
// or bar of a beat grid.
type Span struct {
	Start, End float64
}

// Window is the harmonic evidence pooled over one [Span]: the input of
// [Chords], and with [Normalize] also of [EstimateKey].
type Window struct {
	// Start and End are the span boundaries in seconds.
	Start, End float64
	// Chroma is the RMS-weighted sum of the frame chroma, indexed by
	// [pitch.PitchClass]. Its scale depends on the span length.
	Chroma [12]float64
	// Bass is the bass pitch-class weight in seconds (see [WithBassChroma]
	// and [WithBassNotes]); all zero without bass evidence.
	Bass [12]float64
	// LevelDB is the RMS level of the span's frames in dBFS, limited below
	// by the level floor.
	LevelDB float64
}

// WindowOption configures [Windows]. Options return an error wrapping
// [ErrInvalidOption], [ErrInvalidInput] or [ErrLengthMismatch] for invalid
// values.
type WindowOption func(*windowConfig) error

type windowConfig struct {
	bassChroma  [pitchClasses][]float64
	bassVoicing []float64
	hasBass     bool
	notes       []melody.Note
	floor       float64
}

// WithBassChroma adds frame-wise bass evidence: the chroma and voicing of a
// bass melody analysis (for example [melody.Result.Chroma] and
// [melody.Result.Voicing] of a bass stem) at the same frame rate as the main
// chroma. Each frame in a span adds voicing·chroma/frameRate to [Window.Bass],
// so the weight is in voiced seconds. All rows must have the voicing's length;
// the slices are not copied.
func WithBassChroma(chroma [12][]float64, voicing []float64) WindowOption {
	return func(cfg *windowConfig) error {
		for pc := range chroma {
			if len(chroma[pc]) != len(voicing) {
				return fmt.Errorf("%w: bass chroma row %d has %d frames, voicing %d",
					ErrLengthMismatch, pc, len(chroma[pc]), len(voicing))
			}
		}

		err := checkWeights("bass voicing", -1, voicing)
		if err != nil {
			return err
		}

		for pc := range chroma {
			err = checkWeights("bass chroma", pc, chroma[pc])
			if err != nil {
				return err
			}
		}

		cfg.bassChroma, cfg.bassVoicing, cfg.hasBass = chroma, voicing, true

		return nil
	}
}

// WithBassNotes adds note-wise bass evidence: each note adds its overlap with
// the span in seconds times its strength to [Window.Bass] at the pitch class
// of its MIDI note. Use the cleaned notes of a bass melody. The slice is not
// copied.
func WithBassNotes(notes []melody.Note) WindowOption {
	return func(cfg *windowConfig) error {
		for i, n := range notes {
			if !finite(n.Start) || !finite(n.End) || !finite(n.Strength) {
				return fmt.Errorf("%w: bass note %d (%g..%g s, strength %g)",
					ErrInvalidOption, i, n.Start, n.End, n.Strength)
			}
		}

		cfg.notes = notes

		return nil
	}
}

// WithLevelFloor sets the linear RMS floor of [Window.LevelDB] (default
// [DefaultLevelFloor]). It must be positive and finite.
func WithLevelFloor(linear float64) WindowOption {
	return func(cfg *windowConfig) error {
		if !finite(linear) || linear <= 0 {
			return fmt.Errorf("%w: level floor must be > 0, got %g", ErrInvalidOption, linear)
		}

		cfg.floor = linear

		return nil
	}
}

// Windows pools frame-wise chroma over time spans into chord evidence.
//
// chroma holds one row per pitch class (as [melody.Result.Chroma]) and rms
// the per-frame RMS of the same signal; frame i lies at i/frameRate seconds.
// Because frame chroma is normalised per frame, every frame is weighted by its
// RMS: Window.Chroma[pc] = Σ rms[i]·chroma[pc][i]. A span covers the frames
// ceil(Start·frameRate) ≤ i < ceil(End·frameRate), clipped to the signal, and
// [Window.LevelDB] is the RMS of those frames' rms values in dBFS. Options add
// bass evidence.
//
// All rows must have the length of rms, and all values must be finite and
// non-negative. Spans must have finite bounds with Start ≤ End; they may lie
// partly or wholly outside the signal and need not be ordered.
func Windows(chroma [12][]float64, rms []float64, frameRate float64, spans []Span, opts ...WindowOption) ([]Window, error) {
	cfg := windowConfig{floor: DefaultLevelFloor}

	for i, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("%w: window option %d", ErrNilOption, i)
		}

		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	if !finite(frameRate) || frameRate <= 0 {
		return nil, fmt.Errorf("%w: frame rate must be positive and finite, got %g", ErrInvalidInput, frameRate)
	}

	for pc := range chroma {
		if len(chroma[pc]) != len(rms) {
			return nil, fmt.Errorf("%w: chroma row %d has %d frames, rms %d", ErrLengthMismatch, pc, len(chroma[pc]), len(rms))
		}
	}

	err := checkWeights("rms", -1, rms)
	if err != nil {
		return nil, err
	}

	for pc := range chroma {
		err = checkWeights("chroma", pc, chroma[pc])
		if err != nil {
			return nil, err
		}
	}

	for i, s := range spans {
		if !finite(s.Start) || !finite(s.End) || s.Start > s.End {
			return nil, fmt.Errorf("%w: span %d is %g..%g s", ErrInvalidInput, i, s.Start, s.End)
		}
	}

	out := make([]Window, len(spans))
	for i, s := range spans {
		out[i] = pool(&cfg, chroma, rms, frameRate, s)
	}

	return out, nil
}

// pool computes one window. The arithmetic and its order match
// AudioVisualizer's harmony, bassPC and meanDB helpers.
func pool(cfg *windowConfig, chroma [pitchClasses][]float64, rms []float64, frameRate float64, s Span) Window {
	w := Window{Start: s.Start, End: s.End}

	lo, hi := frameRange(s.Start, s.End, frameRate, len(rms))
	for i := lo; i < hi; i++ {
		for pc := range pitchClasses {
			w.Chroma[pc] += rms[i] * chroma[pc][i]
		}
	}

	sum := 0.0
	for i := lo; i < hi; i++ {
		sum += rms[i] * rms[i]
	}

	if hi <= lo {
		w.LevelDB = core.LinearToDBFloor(0, cfg.floor)
	} else {
		w.LevelDB = core.LinearToDBFloor(math.Sqrt(sum/float64(hi-lo)), cfg.floor)
	}

	if cfg.hasBass {
		lo, hi = frameRange(s.Start, s.End, frameRate, len(cfg.bassVoicing))
		for i := lo; i < hi; i++ {
			for pc := range pitchClasses {
				w.Bass[pc] += cfg.bassVoicing[i] * cfg.bassChroma[pc][i] / frameRate
			}
		}
	}

	for _, n := range cfg.notes {
		if o := math.Min(n.End, s.End) - math.Max(n.Start, s.Start); o > 0 {
			w.Bass[pitchClassOf(n.MIDI)] += o * n.Strength
		}
	}

	return w
}

// frameRange returns the frames [lo, hi) of a span: ceil(start·rate) to
// ceil(end·rate), clipped to [0, n].
func frameRange(start, end, rate float64, n int) (int, int) {
	lo := ceilClamp(start*rate, n)

	return lo, max(lo, ceilClamp(end*rate, n))
}

// ceilClamp returns ceil(x) limited to [0, n], clamping before the integer
// conversion so that huge values cannot overflow.
func ceilClamp(x float64, n int) int {
	c := math.Ceil(x)

	switch {
	case c <= 0:
		return 0
	case c >= float64(n):
		return n
	default:
		return int(c)
	}
}

func pitchClassOf(midi int) int { return ((midi % pitchClasses) + pitchClasses) % pitchClasses }

// checkWeights requires finite, non-negative values; row is the pitch class
// of a chroma row, or -1.
func checkWeights(name string, row int, x []float64) error {
	for i, v := range x {
		if finite(v) && v >= 0 {
			continue
		}

		if row >= 0 {
			return fmt.Errorf("%w: %s row %d frame %d is %g", ErrInvalidInput, name, row, i, v)
		}

		return fmt.Errorf("%w: %s frame %d is %g", ErrInvalidInput, name, i, v)
	}

	return nil
}

// Normalize scales a chroma profile to unit sum. A profile summing to zero or
// less is returned unchanged. Use it to combine evidence of different scales,
// for example a key profile from pooled harmony chroma plus weighted bass
// notes.
func Normalize(c [12]float64) [12]float64 {
	sum := 0.0
	for _, v := range c {
		sum += v
	}

	if sum > 0 {
		for i := range c {
			c[i] /= sum
		}
	}

	return c
}
