//nolint:all // Verbatim reference copy; kept byte-for-byte close to the original.
package rhythm_test

// Verbatim copy of AudioVisualizer's internal/story/grid.go (Grid, NewGrid
// and its methods) and of Grid.Tick from internal/story/midi.go. Adapted to
// standalone test code: the audioanalysis.Rhythm input is replaced by its
// plain fields, smf.PPQ by storyPPQ, identifiers carry a story prefix, and
// the app-specific frame mapping (frames, r6, r3) is omitted. The arithmetic
// and its operation order are unchanged; the parity tests compare rhythm.Grid
// with this copy bit for bit.

import "math"

const storyPPQ = 480

type storyRhythm struct {
	BPM        float64
	BeatOrigin float64
	Beats      []float64
	Downbeat   int
}

type storyGrid struct {
	BPM              float64   `json:"bpm"`
	OriginSeconds    float64   `json:"originSeconds"`
	BeatSeconds      float64   `json:"beatSeconds"`
	SixteenthSeconds float64   `json:"sixteenthSeconds"`
	BarSeconds       float64   `json:"barSeconds"`
	BeatsPerBar      int       `json:"beatsPerBar"`
	Downbeat         int       `json:"downbeatBeatIndex"`
	Duration         float64   `json:"durationSeconds"`
	Beats            []float64 `json:"beatsSeconds"`
	BarStarts        []float64 `json:"barStartsSeconds"`
	barOrigin        float64
}

func newStoryGrid(r storyRhythm, duration float64) storyGrid {
	beat := 60 / r.BPM
	g := storyGrid{BPM: r.BPM, OriginSeconds: r.BeatOrigin, BeatSeconds: beat, SixteenthSeconds: beat / 4, BarSeconds: 4 * beat, BeatsPerBar: 4, Downbeat: r.Downbeat, Duration: duration}
	g.barOrigin = g.OriginSeconds + float64(r.Downbeat%4)*beat
	if r.Downbeat%4 != 0 {
		g.barOrigin -= g.BarSeconds
	}
	g.Beats = append([]float64(nil), r.Beats...)
	for i := 0; len(r.Beats) == 0 && g.BeatStart(i) < duration; i++ {
		g.Beats = append(g.Beats, g.BeatStart(i))
	}
	for i := 0; g.BarStart(i) < duration; i++ {
		g.BarStarts = append(g.BarStarts, g.BarStart(i))
	}
	return g
}

func (g storyGrid) Slot(t float64) int            { return int(math.Round((t - g.OriginSeconds) / g.SixteenthSeconds)) }
func (g storyGrid) SlotTime(slot int) float64     { return g.OriginSeconds + float64(slot)*g.SixteenthSeconds }
func (g storyGrid) Bar(t float64) int             { return max(0, int(math.Floor((t-g.barOrigin)/g.BarSeconds+1e-9))) }
func (g storyGrid) Bars() int                     { return len(g.BarStarts) }
func (g storyGrid) BarStart(n int) float64        { return g.barOrigin + float64(n)*g.BarSeconds }
func (g storyGrid) BeatStart(n int) float64       { return g.OriginSeconds + float64(n)*g.BeatSeconds }
func (g storyGrid) BarPosition(t float64) float64 { return (t - g.barOrigin) / g.BarSeconds }

// Tick maps seconds to MIDI ticks at the grid tempo. By default MIDI time 0
// is audio time 0, so the file lines up with the WAV in a DAW; gridAligned
// moves the first beat to tick 0 instead.
func (g storyGrid) Tick(seconds float64, gridAligned bool) int {
	if gridAligned {
		seconds -= g.OriginSeconds
	}
	return int(math.Round(seconds * g.BPM / 60 * storyPPQ))
}
