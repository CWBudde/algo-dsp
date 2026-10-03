package melody

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

// appAnalyze is the adapter AudioVisualizer uses in place of its own
// AnalyzeMelody: downmix the 24 kHz channels, analyse with a 240-sample hop
// and pass the onset times through.
func appAnalyze(channels [][]float64, onsets []float64) (*Result, error) {
	mono, err := Downmix(channels)
	if err != nil {
		return nil, err
	}

	return Analyze(mono, refSampleRate, WithHop(refHop), WithOnsets(onsets))
}

type parityCase struct {
	name     string
	channels [][]float64
	onsets   []float64
}

func parityCases() []parityCase {
	const n = 3 * refSampleRate

	rng := rand.New(rand.NewPCG(44, 1))
	noise := func(x []float64, amplitude float64, from, to float64) {
		for i := int(from * refSampleRate); i < int(to*refSampleRate) && i < len(x); i++ {
			x[i] += amplitude * (2*rng.Float64() - 1)
		}
	}

	sequence := sequenceSignal(refSampleRate)

	// Stereo: chord bed on the left, lead on both, decorrelated noise.
	left, right := make([]float64, n), make([]float64, n)
	for _, midi := range []float64{48, 55, 64} {
		addSine(left, refSampleRate, midi, 0.05)
	}

	for _, nt := range testSequence {
		addHarmonicNote(left, refSampleRate, nt)
		addHarmonicNote(right, refSampleRate, nt)
	}

	noise(left, 0.01, 0, 3)
	noise(right, 0.01, 0, 3)

	// Noise bursts between notes and silent gaps.
	bursts := make([]float64, n)
	addHarmonicNote(bursts, refSampleRate, synthNote{0.1, 0.5, 62})
	noise(bursts, 0.2, 0.6, 0.9)
	addHarmonicNote(bursts, refSampleRate, synthNote{1.2, 1.5, 74})
	noise(bursts, 0.05, 1.3, 1.4)
	noise(bursts, 0.3, 2.0, 2.2)

	// Vibrato and a glide exercise the jump logic and the median filter.
	glide := make([]float64, n)
	phase := 0.0

	for i := range glide {
		t := float64(i) / refSampleRate

		var midi float64

		switch {
		case t < 0.2 || (t >= 1.2 && t < 1.4) || t >= 2.6:
			continue
		case t < 1.2:
			midi = 67 + 0.8*math.Sin(2*math.Pi*5*t)
		default:
			midi = 60 + 12*(t-1.4)/1.2
		}

		phase += 2 * math.Pi * refMIDIHz(midi) / refSampleRate
		for h := 1; h <= 5; h++ {
			glide[i] += 0.25 / float64(h) * math.Sin(float64(h)*phase)
		}
	}

	// Repeated notes with short gaps, re-articulated by onsets.
	repeated := make([]float64, n)

	for k := range 6 {
		start := 0.2 + 0.25*float64(k)
		addHarmonicNote(repeated, refSampleRate, synthNote{start, start + 0.24, 65})
	}

	short := make([]float64, 100)
	addSine(short, refSampleRate, 69, 0.5)

	return []parityCase{
		{name: "sequence", channels: [][]float64{sequence}},
		{
			name:     "sequence-onsets",
			channels: [][]float64{sequence},
			onsets:   []float64{0.205, 0.553, 0.8, 0.95, 1.21, 1.4, 1.43, 1.69, 2.5},
		},
		{name: "stereo-chord-noise", channels: [][]float64{left, right}, onsets: []float64{0.2, 0.56, 1.19}},
		{name: "noise-bursts", channels: [][]float64{bursts}, onsets: []float64{0.11, 0.6, 1.21, 2.0}},
		{name: "vibrato-glide", channels: [][]float64{glide}, onsets: []float64{0.21, 1.41}},
		{
			name:     "repeated-onsets",
			channels: [][]float64{repeated},
			onsets:   []float64{0.2, 0.45, 0.7, 0.95, 1.2, 1.45, -0.5, 99},
		},
		{name: "silence", channels: [][]float64{make([]float64, refSampleRate)}},
		{name: "short", channels: [][]float64{short}},
	}
}

// TestParityWithAudioVisualizer compares the package against the verbatim
// copy of AudioVisualizer's AnalyzeMelody bit for bit.
func TestParityWithAudioVisualizer(t *testing.T) {
	t.Parallel()

	for _, tc := range parityCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			want, err := refAnalyzeMelody(tc.channels, tc.onsets)
			if err != nil {
				t.Fatal(err)
			}

			got, err := appAnalyze(tc.channels, tc.onsets)
			if err != nil {
				t.Fatal(err)
			}

			assertSameBits(t, "pitch", got.Pitch, want.Pitch)
			assertSameBits(t, "voicing", got.Voicing, want.Voicing)

			for c := range want.Chroma {
				assertSameBits(t, "chroma", got.Chroma[c], want.Chroma[c])
			}

			if len(got.Notes) != len(want.Notes) {
				t.Fatalf("%d notes, want %d:\n got %+v\nwant %+v", len(got.Notes), len(want.Notes), got.Notes, want.Notes)
			}

			for i, w := range want.Notes {
				g := got.Notes[i]
				if math.Float64bits(g.Start) != math.Float64bits(w.Start) ||
					math.Float64bits(g.End) != math.Float64bits(w.End) ||
					g.MIDI != w.MIDI ||
					math.Float64bits(g.Strength) != math.Float64bits(w.Strength) {
					t.Fatalf("note %d: got %+v, want %+v", i, g, w)
				}
			}

			// SegmentNotes on the returned track reproduces the notes too.
			notes, err := SegmentNotes(got.Pitch, got.Voicing, got.FrameRate, WithOnsets(tc.onsets))
			if err != nil {
				t.Fatal(err)
			}

			if len(notes) != len(got.Notes) {
				t.Fatalf("SegmentNotes: %d notes, Analyze %d", len(notes), len(got.Notes))
			}

			for i := range notes {
				if notes[i] != got.Notes[i] {
					t.Fatalf("SegmentNotes note %d: %+v, Analyze %+v", i, notes[i], got.Notes[i])
				}
			}
		})
	}
}

// TestParityExercisesBranches guards the parity fixtures against becoming
// trivial: together they must produce voiced frames, gated frames, notes and
// snapped starts.
func TestParityExercisesBranches(t *testing.T) {
	t.Parallel()

	voiced, notes, snapped := 0, 0, 0

	for _, tc := range parityCases() {
		res, err := appAnalyze(tc.channels, tc.onsets)
		if err != nil {
			t.Fatal(err)
		}

		for _, p := range res.Pitch {
			if p != 0 {
				voiced++
			}
		}

		notes += len(res.Notes)

		for _, n := range res.Notes {
			if slices.Contains(tc.onsets, n.Start) {
				snapped++
			}
		}
	}

	t.Logf("%d voiced frames, %d notes, %d snapped starts", voiced, notes, snapped)

	if voiced < 500 || notes < 15 || snapped < 3 {
		t.Fatalf("fixtures too weak: %d voiced frames, %d notes, %d snapped starts", voiced, notes, snapped)
	}
}
