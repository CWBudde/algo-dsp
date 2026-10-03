package melody

import (
	"errors"
	"math"
	"testing"
)

// track builds a pitch track from (value, frames) runs; voicing is 0.5 on
// voiced frames.
func track(runs ...[2]float64) (pitch, voicing []float64) {
	for _, r := range runs {
		for range int(r[1]) {
			pitch = append(pitch, r[0])

			v := 0.0
			if r[0] != 0 {
				v = 0.5
			}

			voicing = append(voicing, v)
		}
	}

	return pitch, voicing
}

func TestSegmentNotes(t *testing.T) {
	t.Parallel()

	const fr = 100.0

	tests := []struct {
		name  string
		runs  [][2]float64
		opts  []Option
		notes []Note
	}{
		{
			name:  "single",
			runs:  [][2]float64{{0, 5}, {60.2, 20}, {0, 5}},
			notes: []Note{{Start: 0.05, End: 0.25, MIDI: 60, Strength: 0.5}},
		},
		{
			name:  "too-short",
			runs:  [][2]float64{{0, 5}, {60, 5}, {0, 5}},
			notes: []Note{},
		},
		{
			name: "short-gap-bridged",
			runs: [][2]float64{{62, 10}, {0, 2}, {62, 10}},
			// The bridged frames are not part of the note's values but the
			// note spans them.
			notes: []Note{{Start: 0, End: 0.22, MIDI: 62, Strength: 0.5}},
		},
		{
			name: "long-gap-splits",
			runs: [][2]float64{{62, 10}, {0, 3}, {62, 10}},
			notes: []Note{
				{Start: 0, End: 0.10, MIDI: 62, Strength: 0.5},
				{Start: 0.13, End: 0.23, MIDI: 62, Strength: 0.5},
			},
		},
		{
			name: "jump-splits",
			runs: [][2]float64{{60, 10}, {64, 10}},
			notes: []Note{
				{Start: 0, End: 0.10, MIDI: 60, Strength: 0.5},
				{Start: 0.10, End: 0.20, MIDI: 64, Strength: 0.5},
			},
		},
		{
			name: "brief-deviation-kept",
			runs: [][2]float64{{60, 10}, {61, 2}, {60, 10}},
			notes: []Note{
				{Start: 0, End: 0.22, MIDI: 60, Strength: 0.5},
			},
		},
		{
			name: "onset-reattack-and-snap",
			runs: [][2]float64{{0, 2}, {65, 30}},
			opts: []Option{WithOnsets([]float64{0.015, 0.17})},
			notes: []Note{
				{Start: 0.015, End: 0.17, MIDI: 65, Strength: 0.5},
				{Start: 0.17, End: 0.32, MIDI: 65, Strength: 0.5},
			},
		},
		{
			name: "early-onset-ignored",
			runs: [][2]float64{{65, 30}},
			opts: []Option{WithOnsets([]float64{0.03}), WithOnsetSnap(0)},
			notes: []Note{
				{Start: 0, End: 0.30, MIDI: 65, Strength: 0.5},
			},
		},
		{
			name: "custom-durations",
			runs: [][2]float64{{60, 4}, {0, 1}, {67, 4}},
			opts: []Option{WithNoteMinDuration(0.04), WithNoteGap(0.01), WithNoteJump(2, 0.01)},
			notes: []Note{
				{Start: 0, End: 0.04, MIDI: 60, Strength: 0.5},
				{Start: 0.05, End: 0.09, MIDI: 67, Strength: 0.5},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p, v := track(tc.runs...)

			notes, err := SegmentNotes(p, v, fr, tc.opts...)
			if err != nil {
				t.Fatal(err)
			}

			if len(notes) != len(tc.notes) {
				t.Fatalf("notes %+v, want %+v", notes, tc.notes)
			}

			for i, want := range tc.notes {
				got := notes[i]
				if got.MIDI != want.MIDI || math.Abs(got.Start-want.Start) > 1e-12 ||
					math.Abs(got.End-want.End) > 1e-12 || math.Abs(got.Strength-want.Strength) > 1e-12 {
					t.Fatalf("note %d: %+v, want %+v", i, got, want)
				}
			}
		})
	}
}

func TestSegmentNotesErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		pitch     []float64
		voicing   []float64
		frameRate float64
		want      error
	}{
		{"length", []float64{60}, nil, 100, ErrLengthMismatch},
		{"rate-zero", nil, nil, 0, ErrInvalidFrameRate},
		{"rate-nan", nil, nil, math.NaN(), ErrInvalidFrameRate},
	}

	for _, tc := range tests {
		_, err := SegmentNotes(tc.pitch, tc.voicing, tc.frameRate)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err %v, want %v", tc.name, err, tc.want)
		}
	}

	notes, err := SegmentNotes(nil, nil, 100)
	if err != nil || notes == nil || len(notes) != 0 {
		t.Fatalf("empty track: %v, %v", notes, err)
	}
}

func TestMedianVoiced(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		x      []float64
		radius int
		want   []float64
	}{
		{"outlier", []float64{60, 60, 72, 60, 60}, 2, []float64{60, 60, 60, 60, 60}},
		{"gaps-kept", []float64{0, 60, 0, 61, 0}, 1, []float64{0, 60, 0, 61, 0}},
		{"upper-median", []float64{60, 62, 0}, 1, []float64{62, 62, 0}},
		{"radius-zero", []float64{60, 72, 60}, 0, []float64{60, 72, 60}},
		{"radius-negative", []float64{60, 72}, -3, []float64{60, 72}},
		{"empty", nil, 2, []float64{}},
	}

	for _, tc := range tests {
		in := append([]float64(nil), tc.x...)
		got := MedianVoiced(tc.x, tc.radius)

		assertSameBits(t, tc.name, got, tc.want)
		assertSameBits(t, tc.name+" input", tc.x, in)
	}
}
