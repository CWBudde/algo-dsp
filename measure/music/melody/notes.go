package melody

import (
	"fmt"
	"math"
	"slices"
)

// MedianVoiced smooths the voiced (non-zero) frames of a pitch track with a
// centred median over ±radius frames, using only the voiced frames inside
// the window, and leaves unvoiced frames at 0, so gaps are neither filled nor
// widened. For an even number of voiced neighbours the upper median is used.
// x is not modified; a radius <= 0 returns a copy.
func MedianVoiced(x []float64, radius int) []float64 {
	y := make([]float64, len(x))
	radius = max(radius, 0)
	window := make([]float64, 0, 2*radius+1)

	for i, v := range x {
		if v == 0 {
			continue
		}

		window = window[:0]

		for j := max(0, i-radius); j <= min(len(x)-1, i+radius); j++ {
			if x[j] != 0 {
				window = append(window, x[j])
			}
		}

		y[i] = medianInPlace(window)
	}

	return y
}

// medianInPlace sorts x and returns its upper median x[len(x)/2].
func medianInPlace(x []float64) float64 {
	slices.Sort(x)

	return x[len(x)/2]
}

// segmentParams holds the note segmentation parameters in frames.
type segmentParams struct {
	minFrames  int
	gapFrames  int
	jumpFrames int
	jump       float64
	snap       float64
}

// framesOf converts seconds to a frame count of at least one frame.
func framesOf(seconds, frameRate float64) int {
	return max(1, int(math.Round(seconds*frameRate)))
}

func newSegmentParams(nc *noteConfig, frameRate float64) segmentParams {
	return segmentParams{
		minFrames:  framesOf(nc.minDuration, frameRate),
		gapFrames:  framesOf(nc.gap, frameRate),
		jumpFrames: framesOf(nc.jumpDuration, frameRate),
		jump:       nc.jump,
		snap:       nc.snap,
	}
}

// SegmentNotes splits a frame-wise pitch track into notes. pitch holds
// fractional MIDI note numbers with 0 for unvoiced frames, voicing the
// per-frame confidence used as note strength, and frame i is at time
// i/frameRate. The track can come from [Analyze] or from any other pitch
// tracker (for example pitch.YINDetector, converted with
// pitch.FrequencyToMIDI), which should be median-smoothed first (see
// [MedianVoiced]).
//
// A note starts at a voiced frame and ends when the track is unvoiced for
// [WithNoteGap], when the pitch deviates from the running note median by
// more than the [WithNoteJump] interval for its whole duration, or at an
// onset from [WithOnsets] at least [WithNoteMinDuration] after the note
// start. Notes shorter than [WithNoteMinDuration] are dropped. Finally each
// note start is moved onto the nearest onset within [WithOnsetSnap] that lies
// before the note end. Durations are converted to frames with
// round(seconds*frameRate), at least one frame.
//
// Only the onset and note options apply; the spectral options are validated
// and ignored.
func SegmentNotes(pitch, voicing []float64, frameRate float64, opts ...Option) ([]Note, error) {
	cfg, err := newConfig(opts)
	if err != nil {
		return nil, err
	}

	if len(pitch) != len(voicing) {
		return nil, fmt.Errorf("%w: %d pitch frames, %d voicing frames", ErrLengthMismatch, len(pitch), len(voicing))
	}

	if !finite(frameRate) || frameRate <= 0 {
		return nil, fmt.Errorf("%w: %g", ErrInvalidFrameRate, frameRate)
	}

	return segmentNotes(pitch, voicing, 1/frameRate, cfg.onsets, newSegmentParams(&cfg.notes, frameRate)), nil
}

// noteBuilder accumulates the voiced frames of the note being segmented.
type noteBuilder struct {
	p         segmentParams
	step      float64
	notes     []Note
	start     int
	values    []float64
	strengths []float64
	scratch   []float64
}

// median returns the upper median of the current note's pitch values.
func (b *noteBuilder) median() float64 {
	b.scratch = append(b.scratch[:0], b.values...)

	return medianInPlace(b.scratch)
}

// flush emits the current note, ending at frame end, if it is long enough,
// and resets the builder.
func (b *noteBuilder) flush(end int) {
	if b.start >= 0 && end-b.start >= b.p.minFrames {
		s := 0.0
		for _, v := range b.strengths {
			s += v
		}

		b.notes = append(b.notes, Note{
			Start:    float64(b.start) * b.step,
			End:      float64(end) * b.step,
			MIDI:     int(math.Round(b.median())),
			Strength: s / float64(len(b.strengths)),
		})
	}

	b.start, b.values, b.strengths = -1, b.values[:0], b.strengths[:0]
}

func segmentNotes(pitch, voicing []float64, step float64, onsets []float64, p segmentParams) []Note {
	onsetFrame := make([]bool, len(pitch))

	for _, t := range onsets {
		f := math.Round(t / step)
		if f >= 0 && f < float64(len(pitch)) {
			onsetFrame[int(f)] = true
		}
	}

	b := &noteBuilder{p: p, step: step, notes: []Note{}, start: -1}
	deviates := func(i int, reference float64) bool {
		return i < len(pitch) && pitch[i] != 0 && math.Abs(pitch[i]-reference) > p.jump
	}
	unvoiced := 0

	for i := range pitch {
		if pitch[i] == 0 {
			if b.start >= 0 {
				unvoiced++
				if unvoiced >= p.gapFrames {
					b.flush(i - unvoiced + 1)
				}
			}

			continue
		}

		if b.start >= 0 {
			current := b.median()
			changed := true

			for k := range p.jumpFrames {
				if !deviates(i+k, current) {
					changed = false

					break
				}
			}

			reattack := onsetFrame[i] && i-b.start >= p.minFrames
			if changed || reattack {
				b.flush(i - unvoiced)
			}
		}

		unvoiced = 0

		if b.start < 0 {
			b.start = i
		}

		b.values = append(b.values, pitch[i])
		b.strengths = append(b.strengths, voicing[i])
	}

	b.flush(len(pitch) - unvoiced)
	snapToOnsets(b.notes, onsets, p.snap)

	return b.notes
}

// snapToOnsets moves each note start onto the closest onset within snap
// seconds that lies before the note end. Onsets are scanned in order and
// each accepted onset becomes the reference for the next, so a later onset
// at an equal distance wins.
func snapToOnsets(notes []Note, onsets []float64, snap float64) {
	for i := range notes {
		best := snap

		for _, t := range onsets {
			if d := math.Abs(t - notes[i].Start); d <= best && t < notes[i].End {
				best = d
				notes[i].Start = t
			}
		}
	}
}
