package rhythm

import (
	"fmt"
	"math"
)

// Default grid parameters.
const (
	// DefaultBeatsPerBar is the default number of beats per bar (4/4).
	DefaultBeatsPerBar = 4
	// DefaultSubdivisions is the default number of grid slots per beat
	// (sixteenth notes in 4/4).
	DefaultSubdivisions = 4
)

// maxGridEntries bounds the number of generated beats and bar starts, so a
// tiny period over a long duration cannot exhaust memory.
const maxGridEntries = 1 << 24

// Grid is a constant-tempo beat grid with three levels: slots (sixteenth
// notes with the default four subdivisions per beat), beats and bars.
//
// Slots and beats count from the beat origin (slot 0 and beat 0 are at
// [Grid.Origin]). Bars count from the bar that contains the origin: when the
// downbeat index is not a multiple of the beats per bar, bar 0 is a pickup
// bar that starts before the origin and the first full downbeat starts bar 1.
//
// A Grid is an immutable value built by [NewGrid]; the zero value is not a
// valid grid (see [Grid.Validate]). It is safe for concurrent use.
type Grid struct {
	bpm          float64
	origin       float64
	beatSeconds  float64
	slotSeconds  float64
	barSeconds   float64
	beatsPerBar  int
	subdivisions int
	downbeat     int
	duration     float64
	beats        []float64
	barStarts    []float64
	barOrigin    float64
}

// GridOption configures [NewGrid]. Options return an error wrapping
// [ErrInvalidArgument] for out-of-range values.
type GridOption func(*gridConfig) error

type gridConfig struct {
	beatsPerBar  int
	subdivisions int
	beats        []float64
}

// WithBeatsPerBar sets the number of beats per bar, >= 1 (default
// [DefaultBeatsPerBar]). It must match the beatsPerBar given to [Downbeat].
func WithBeatsPerBar(n int) GridOption {
	return func(cfg *gridConfig) error {
		if n < 1 {
			return fmt.Errorf("%w: beats per bar must be >= 1, got %d", ErrInvalidArgument, n)
		}

		cfg.beatsPerBar = n

		return nil
	}
}

// WithSubdivisions sets the number of grid slots per beat, >= 1 (default
// [DefaultSubdivisions]: sixteenth notes under a quarter-note beat).
func WithSubdivisions(n int) GridOption {
	return func(cfg *gridConfig) error {
		if n < 1 {
			return fmt.Errorf("%w: subdivisions must be >= 1, got %d", ErrInvalidArgument, n)
		}

		cfg.subdivisions = n

		return nil
	}
}

// WithBeats supplies measured beat times in seconds, typically the output of
// [BeatGrid]. They are reported by [Grid.Beats] unchanged; the slot, beat and
// bar arithmetic always uses the constant tempo. The slice is copied. Without
// it (or with an empty slice) [Grid.Beats] lists [Grid.BeatStart] of 0, 1, ...
// before the duration.
func WithBeats(seconds []float64) GridOption {
	beats := append([]float64(nil), seconds...)

	return func(cfg *gridConfig) error {
		for i, t := range beats {
			if math.IsNaN(t) || math.IsInf(t, 0) {
				return fmt.Errorf("%w: beat %d is %v", ErrInvalidArgument, i, t)
			}
		}

		cfg.beats = beats

		return nil
	}
}

// NewGrid returns the constant-tempo grid of bpm beats per minute whose beat
// 0 (and slot 0) is at origin seconds. downbeat is the index of the first
// beat that starts a bar, as returned by [Downbeat]; only downbeat modulo the
// beats per bar matters. duration (seconds) limits [Grid.Beats] and
// [Grid.BarStarts] to the times before it.
//
// The usual inputs come from this package:
//
//	phase := FitBeatPhase(lowBand, timing, bpm, duration)
//	beats, err := BeatGrid(phase, bpm, duration)
//	downbeat := Downbeat(beats, accents, 4, 0.06)
//	grid, err := NewGrid(bpm, phase, downbeat, duration, WithBeats(beats))
//
// bpm must be positive and finite, origin finite, downbeat >= 0 and duration
// finite and >= 0; otherwise NewGrid returns an error wrapping
// [ErrInvalidArgument]. With the default options the grid reproduces
// AudioVisualizer's story grid (16th slots, 4/4 bars) bit for bit.
func NewGrid(bpm, origin float64, downbeat int, duration float64, opts ...GridOption) (Grid, error) {
	cfg := gridConfig{beatsPerBar: DefaultBeatsPerBar, subdivisions: DefaultSubdivisions}

	for i, opt := range opts {
		if opt == nil {
			return Grid{}, fmt.Errorf("%w: grid option %d", ErrNilOption, i)
		}

		err := opt(&cfg)
		if err != nil {
			return Grid{}, err
		}
	}

	switch {
	case !positive(bpm):
		return Grid{}, fmt.Errorf("%w: tempo %v BPM", ErrInvalidArgument, bpm)
	case math.IsNaN(origin) || math.IsInf(origin, 0):
		return Grid{}, fmt.Errorf("%w: origin %v s", ErrInvalidArgument, origin)
	case downbeat < 0:
		return Grid{}, fmt.Errorf("%w: downbeat index %d", ErrInvalidArgument, downbeat)
	case math.IsNaN(duration) || math.IsInf(duration, 0) || duration < 0:
		return Grid{}, fmt.Errorf("%w: duration %v s", ErrInvalidArgument, duration)
	}

	beat := 60 / bpm
	g := Grid{
		bpm:          bpm,
		origin:       origin,
		beatSeconds:  beat,
		slotSeconds:  beat / float64(cfg.subdivisions),
		barSeconds:   float64(cfg.beatsPerBar) * beat,
		beatsPerBar:  cfg.beatsPerBar,
		subdivisions: cfg.subdivisions,
		downbeat:     downbeat,
		duration:     duration,
	}

	if (duration-math.Min(origin, 0))/beat > maxGridEntries {
		return Grid{}, fmt.Errorf("%w: %v s at %v BPM exceeds %d beats", ErrInvalidArgument, duration, bpm, maxGridEntries)
	}

	g.barOrigin = g.origin + float64(downbeat%cfg.beatsPerBar)*beat
	if downbeat%cfg.beatsPerBar != 0 {
		g.barOrigin -= g.barSeconds
	}

	g.beats = cfg.beats
	for i := 0; len(cfg.beats) == 0 && g.BeatStart(i) < duration; i++ {
		g.beats = append(g.beats, g.BeatStart(i))
	}

	for i := 0; g.BarStart(i) < duration; i++ {
		g.barStarts = append(g.barStarts, g.BarStart(i))
	}

	return g, nil
}

// Validate reports whether g was built by [NewGrid]. The zero Grid returns
// an error wrapping [ErrInvalidArgument].
func (g Grid) Validate() error {
	if !positive(g.beatSeconds) || g.beatsPerBar < 1 || g.subdivisions < 1 {
		return fmt.Errorf("%w: grid not built by NewGrid", ErrInvalidArgument)
	}

	return nil
}

// BPM returns the tempo in beats per minute.
func (g Grid) BPM() float64 { return g.bpm }

// Origin returns the time in seconds of beat 0 and slot 0.
func (g Grid) Origin() float64 { return g.origin }

// BeatSeconds returns the beat period 60/BPM in seconds.
func (g Grid) BeatSeconds() float64 { return g.beatSeconds }

// SlotSeconds returns the slot period in seconds: the beat period divided by
// the subdivisions (a sixteenth note by default).
func (g Grid) SlotSeconds() float64 { return g.slotSeconds }

// BarSeconds returns the bar period in seconds: beats per bar times the beat
// period.
func (g Grid) BarSeconds() float64 { return g.barSeconds }

// BeatsPerBar returns the number of beats per bar.
func (g Grid) BeatsPerBar() int { return g.beatsPerBar }

// Subdivisions returns the number of slots per beat.
func (g Grid) Subdivisions() int { return g.subdivisions }

// SlotsPerBar returns the number of slots per bar.
func (g Grid) SlotsPerBar() int { return g.beatsPerBar * g.subdivisions }

// Downbeat returns the downbeat index the grid was built with.
func (g Grid) Downbeat() int { return g.downbeat }

// Duration returns the duration in seconds the grid was built with.
func (g Grid) Duration() float64 { return g.duration }

// Beats returns a copy of the beat times in seconds: those given with
// [WithBeats], or else the constant-tempo beats before the duration.
func (g Grid) Beats() []float64 { return append([]float64(nil), g.beats...) }

// BarStarts returns a copy of the start times in seconds of the bars that
// start before the duration, [Grid.BarStart] of 0, 1, .... The first entry is
// the pickup bar's start when the downbeat is not on beat 0 (mod beats per
// bar), and may be negative.
func (g Grid) BarStarts() []float64 { return append([]float64(nil), g.barStarts...) }

// Slot returns the slot nearest to t seconds: round((t-origin)/slot period).
// Times before the origin give negative slots.
func (g Grid) Slot(t float64) int { return int(math.Round((t - g.origin) / g.slotSeconds)) }

// SlotTime returns the time in seconds of slot n.
func (g Grid) SlotTime(n int) float64 { return g.origin + float64(n)*g.slotSeconds }

// Bar returns the index of the bar containing t seconds, clamped at 0; a
// tolerance of 1e-9 bar puts times a hair before a bar line into that bar.
func (g Grid) Bar(t float64) int {
	return max(0, int(math.Floor((t-g.barOrigin)/g.barSeconds+1e-9)))
}

// Bars returns the number of bars that start before the duration, the
// length of [Grid.BarStarts].
func (g Grid) Bars() int { return len(g.barStarts) }

// BarStart returns the start time in seconds of bar n. Bar 0 is the pickup
// bar when the downbeat is not on beat 0 (mod beats per bar).
func (g Grid) BarStart(n int) float64 { return g.barOrigin + float64(n)*g.barSeconds }

// BeatStart returns the time in seconds of beat n of the constant-tempo grid.
func (g Grid) BeatStart(n int) float64 { return g.origin + float64(n)*g.beatSeconds }

// BarPosition returns the position of t seconds in bars, measured from the
// start of bar 0 (unclamped and fractional).
func (g Grid) BarPosition(t float64) float64 { return (t - g.barOrigin) / g.barSeconds }

// Tick maps t seconds to MIDI ticks at the grid tempo with ppq ticks per
// quarter note (one beat), rounded to the nearest tick. Tick time 0 is time
// 0; pass t-[Grid.Origin] to put the first beat on tick 0 instead.
func (g Grid) Tick(t float64, ppq int) int {
	return int(math.Round(t * g.bpm / 60 * float64(ppq)))
}
