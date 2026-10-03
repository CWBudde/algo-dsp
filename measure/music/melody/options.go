package melody

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
)

// Default analysis parameters. They reproduce the AudioVisualizer melody
// analysis (24 kHz analysis rate, 240-sample hop) exactly.
const (
	// DefaultHop is the default hop size in samples (10 ms at 24 kHz).
	DefaultHop = 240
	// DefaultFFTSize is the default FFT size in samples (171 ms at 24 kHz,
	// about 0.4 semitone bin spacing at the lowest default candidate).
	DefaultFFTSize = 4096
	// DefaultMinMIDI is the lowest default pitch candidate (E3, 165 Hz).
	DefaultMinMIDI = 52.0
	// DefaultMaxMIDI is the highest default pitch candidate (C7, 2.1 kHz).
	DefaultMaxMIDI = 96.0
	// DefaultHarmonics is the default number of harmonics summed per
	// candidate.
	DefaultHarmonics = 8
	// DefaultHarmonicDecay is the default weight ratio between consecutive
	// harmonics: harmonic h is weighted DefaultHarmonicDecay^(h-1).
	DefaultHarmonicDecay = 0.8
	// DefaultMinHz is the lower edge of the default analysis band.
	DefaultMinHz = 100.0
	// DefaultMaxHz is the upper edge of the default analysis band.
	DefaultMaxHz = 5000.0
	// DefaultVoicingThreshold is the default share of band power the chosen
	// harmonic series must explain for a frame to count as voiced.
	DefaultVoicingThreshold = 0.3
	// DefaultGateDB is the default RMS gate in dBFS. Frames whose RMS over
	// centre ± hop is below it are unvoiced and have zero chroma.
	DefaultGateDB = -50.0
	// DefaultSmoothing is the default radius in seconds of the median filter
	// applied to the voiced pitch track (2 frames at 100 frames/s).
	DefaultSmoothing = 0.02
	// DefaultNoteMinDuration is the default shortest note in seconds
	// (6 frames at 100 frames/s).
	DefaultNoteMinDuration = 0.06
	// DefaultNoteGap is the default run of unvoiced time in seconds that
	// ends a note (3 frames at 100 frames/s).
	DefaultNoteGap = 0.03
	// DefaultNoteJump is the default pitch deviation in semitones from the
	// running note median that starts a new note.
	DefaultNoteJump = 0.6
	// DefaultNoteJumpDuration is the default time in seconds the deviation
	// must persist before a new note starts (3 frames at 100 frames/s).
	DefaultNoteJumpDuration = 0.03
	// DefaultOnsetSnap is the default largest distance in seconds over which
	// a note start is moved onto a nearby onset.
	DefaultOnsetSnap = 0.04

	// pitchStep is the spacing of the candidate grid in semitones.
	pitchStep = 0.1
)

// Option configures [Analyze] and [SegmentNotes]. Options return an error
// wrapping [ErrInvalidOption] for out-of-range values.
type Option func(*config) error

type config struct {
	hop           int
	fftSize       int
	minMIDI       float64
	maxMIDI       float64
	harmonics     int
	harmonicDecay float64
	minHz         float64
	maxHz         float64
	voicing       float64
	gateDB        float64
	referenceHz   float64
	smoothing     float64
	onsets        []float64
	notes         noteConfig
}

// noteConfig holds the note segmentation parameters in seconds.
type noteConfig struct {
	minDuration  float64
	gap          float64
	jump         float64
	jumpDuration float64
	snap         float64
}

func defaultConfig() config {
	return config{
		hop:           DefaultHop,
		fftSize:       DefaultFFTSize,
		minMIDI:       DefaultMinMIDI,
		maxMIDI:       DefaultMaxMIDI,
		harmonics:     DefaultHarmonics,
		harmonicDecay: DefaultHarmonicDecay,
		minHz:         DefaultMinHz,
		maxHz:         DefaultMaxHz,
		voicing:       DefaultVoicingThreshold,
		gateDB:        DefaultGateDB,
		referenceHz:   pitch.DefaultReferenceHz,
		smoothing:     DefaultSmoothing,
		notes: noteConfig{
			minDuration:  DefaultNoteMinDuration,
			gap:          DefaultNoteGap,
			jump:         DefaultNoteJump,
			jumpDuration: DefaultNoteJumpDuration,
			snap:         DefaultOnsetSnap,
		},
	}
}

func newConfig(opts []Option) (config, error) {
	cfg := defaultConfig()

	for i, opt := range opts {
		if opt == nil {
			return cfg, fmt.Errorf("%w: option %d", ErrNilOption, i)
		}

		err := opt(&cfg)
		if err != nil {
			return cfg, err
		}
	}

	return cfg, nil
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

// WithHop sets the hop size in samples (default [DefaultHop]). Frame i is
// centred on sample i*hop, the frame rate is sampleRate/hop, and the RMS gate
// is measured over centre ± hop.
func WithHop(hop int) Option {
	return func(cfg *config) error {
		if hop < 1 {
			return fmt.Errorf("%w: hop must be >= 1, got %d", ErrInvalidOption, hop)
		}

		cfg.hop = hop

		return nil
	}
}

// WithFFTSize sets the FFT size in samples (default [DefaultFFTSize]). The
// frames are windowed with a periodic Hann window of this length.
func WithFFTSize(n int) Option {
	return func(cfg *config) error {
		if n < 2 {
			return fmt.Errorf("%w: FFT size must be >= 2, got %d", ErrInvalidOption, n)
		}

		cfg.fftSize = n

		return nil
	}
}

// WithMIDIRange sets the range of fractional MIDI pitch candidates (default
// [DefaultMinMIDI]..[DefaultMaxMIDI]). Candidates are spaced 0.1 semitone
// apart, starting at minMIDI. minMIDI must be positive and below maxMIDI.
func WithMIDIRange(minMIDI, maxMIDI float64) Option {
	return func(cfg *config) error {
		if !finite(minMIDI) || !finite(maxMIDI) || minMIDI <= 0 || minMIDI >= maxMIDI {
			return fmt.Errorf("%w: MIDI range %g..%g", ErrInvalidOption, minMIDI, maxMIDI)
		}

		cfg.minMIDI, cfg.maxMIDI = minMIDI, maxMIDI

		return nil
	}
}

// WithHarmonics sets the number of harmonics summed per candidate (>= 1) and
// the weight ratio between consecutive harmonics, in (0, 1] (defaults
// [DefaultHarmonics], [DefaultHarmonicDecay]). Harmonics at or above the
// upper band edge are not summed.
func WithHarmonics(count int, decay float64) Option {
	return func(cfg *config) error {
		if count < 1 {
			return fmt.Errorf("%w: harmonic count must be >= 1, got %d", ErrInvalidOption, count)
		}

		if !(decay > 0 && decay <= 1) {
			return fmt.Errorf("%w: harmonic decay must be in (0, 1], got %g", ErrInvalidOption, decay)
		}

		cfg.harmonics, cfg.harmonicDecay = count, decay

		return nil
	}
}

// WithFrequencyRange sets the analysis band in Hz (default
// [DefaultMinHz]..[DefaultMaxHz]). Chroma and the voicing denominator use
// only the FFT bins inside it, and harmonics at or above maxHz are not
// summed. A maxHz above Nyquist is limited to the last FFT bin.
func WithFrequencyRange(minHz, maxHz float64) Option {
	return func(cfg *config) error {
		if !finite(minHz) || !finite(maxHz) || minHz <= 0 || minHz >= maxHz {
			return fmt.Errorf("%w: frequency range %g..%g Hz", ErrInvalidOption, minHz, maxHz)
		}

		cfg.minHz, cfg.maxHz = minHz, maxHz

		return nil
	}
}

// WithVoicingThreshold sets the share of band power, in [0, 1], that the
// chosen harmonic series must explain for a frame to be voiced (default
// [DefaultVoicingThreshold]).
func WithVoicingThreshold(threshold float64) Option {
	return func(cfg *config) error {
		if !(threshold >= 0 && threshold <= 1) {
			return fmt.Errorf("%w: voicing threshold must be in [0, 1], got %g", ErrInvalidOption, threshold)
		}

		cfg.voicing = threshold

		return nil
	}
}

// WithGateDB sets the RMS gate in dBFS (default [DefaultGateDB]). Frames
// whose RMS over centre ± hop is below the gate are skipped: they are
// unvoiced and have zero chroma. The long FFT window smears note edges; the
// short RMS gate keeps them sharp. math.Inf(-1) disables the gate.
func WithGateDB(db float64) Option {
	return func(cfg *config) error {
		if math.IsNaN(db) || math.IsInf(db, 1) {
			return fmt.Errorf("%w: gate must be finite or -Inf, got %g", ErrInvalidOption, db)
		}

		cfg.gateDB = db

		return nil
	}
}

// WithReferenceHz sets the frequency of A4 (MIDI 69) used for the pitch
// candidates and the chroma pitch classes (default
// [pitch.DefaultReferenceHz]).
func WithReferenceHz(hz float64) Option {
	return func(cfg *config) error {
		if !finite(hz) || hz <= 0 {
			return fmt.Errorf("%w: reference must be a positive frequency, got %g", ErrInvalidOption, hz)
		}

		cfg.referenceHz = hz

		return nil
	}
}

// WithSmoothing sets the radius in seconds of the median filter applied to
// the voiced frames of the pitch track (default [DefaultSmoothing]). The
// radius in frames is round(seconds*frameRate); 0 disables smoothing.
func WithSmoothing(seconds float64) Option {
	return func(cfg *config) error {
		if !finite(seconds) || seconds < 0 {
			return fmt.Errorf("%w: smoothing must be >= 0 s, got %g", ErrInvalidOption, seconds)
		}

		cfg.smoothing = seconds

		return nil
	}
}

// WithOnsets supplies onset times in seconds, for example from an onset
// detector. Onsets split re-articulated notes and pull note starts onto
// measured attack times (see [WithOnsetSnap]). The slice is copied and its
// order is kept: when two onsets are equally close to a note start, the later
// one in the slice wins.
func WithOnsets(seconds []float64) Option {
	onsets := append([]float64(nil), seconds...)

	return func(cfg *config) error {
		for i, t := range onsets {
			if !finite(t) {
				return fmt.Errorf("%w: onset %d is %g", ErrInvalidOption, i, t)
			}
		}

		cfg.onsets = onsets

		return nil
	}
}

// WithNoteMinDuration sets the shortest note in seconds (default
// [DefaultNoteMinDuration]). It also is the shortest time after a note start
// at which an onset re-articulates the note.
func WithNoteMinDuration(seconds float64) Option {
	return func(cfg *config) error {
		if !finite(seconds) || seconds <= 0 {
			return fmt.Errorf("%w: note minimum duration must be > 0 s, got %g", ErrInvalidOption, seconds)
		}

		cfg.notes.minDuration = seconds

		return nil
	}
}

// WithNoteGap sets the run of unvoiced time in seconds that ends a note
// (default [DefaultNoteGap]). Shorter dropouts are bridged.
func WithNoteGap(seconds float64) Option {
	return func(cfg *config) error {
		if !finite(seconds) || seconds <= 0 {
			return fmt.Errorf("%w: note gap must be > 0 s, got %g", ErrInvalidOption, seconds)
		}

		cfg.notes.gap = seconds

		return nil
	}
}

// WithNoteJump sets the pitch change that starts a new note: the pitch must
// deviate by more than semitones from the running median of the current note
// for duration seconds (defaults [DefaultNoteJump], [DefaultNoteJumpDuration]).
func WithNoteJump(semitones, duration float64) Option {
	return func(cfg *config) error {
		if !finite(semitones) || semitones <= 0 {
			return fmt.Errorf("%w: note jump must be > 0 semitones, got %g", ErrInvalidOption, semitones)
		}

		if !finite(duration) || duration <= 0 {
			return fmt.Errorf("%w: note jump duration must be > 0 s, got %g", ErrInvalidOption, duration)
		}

		cfg.notes.jump, cfg.notes.jumpDuration = semitones, duration

		return nil
	}
}

// WithOnsetSnap sets the largest distance in seconds over which a note start
// is moved onto an onset given with [WithOnsets] (default
// [DefaultOnsetSnap]). 0 snaps only onsets that coincide with a start.
func WithOnsetSnap(seconds float64) Option {
	return func(cfg *config) error {
		if !finite(seconds) || seconds < 0 {
			return fmt.Errorf("%w: onset snap must be >= 0 s, got %g", ErrInvalidOption, seconds)
		}

		cfg.notes.snap = seconds

		return nil
	}
}
