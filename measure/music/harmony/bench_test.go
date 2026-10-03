package harmony

import (
	"math/rand/v2"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
)

// benchWindows returns n windows of a noisy progression: a three-minute song
// at 105 BPM has about 160 half-bar windows.
func benchWindows(n int) []Window {
	rng := rand.New(rand.NewPCG(45, 5))

	return toWindows(progressionWindows(rng, int(pitch.PitchClassG), false, n))
}

func BenchmarkEstimateKey(b *testing.B) {
	c := cadence(7, Major)

	var evidence [12]float64

	evidence[7] = 1

	b.ReportAllocs()

	for b.Loop() {
		_, err := EstimateKey(c, WithTonicEvidence(evidence))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWindows(b *testing.B) {
	rng := rand.New(rand.NewPCG(45, 6))
	d := randomFrames(rng, 18000, parityFrameRate) // three minutes
	spans := halfBarSpans(d.duration)

	b.ReportAllocs()

	for b.Loop() {
		_, err := Windows(d.chroma, d.rms, parityFrameRate, spans, WithBassChroma(d.bassChroma, d.voicing), WithBassNotes(d.notes))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkChords(b *testing.B) {
	windows := benchWindows(160)
	key := Key{Tonic: pitch.PitchClassG}

	b.ReportAllocs()

	for b.Loop() {
		_, err := Chords(windows, key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkChorder is the allocation-free path: a reused Chorder and dst.
func BenchmarkChorder(b *testing.B) {
	windows := benchWindows(160)
	key := Key{Tonic: pitch.PitchClassG}

	c, err := NewChorder()
	if err != nil {
		b.Fatal(err)
	}

	dst, err := c.Chords(nil, windows, key)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	for b.Loop() {
		dst, err = c.Chords(dst, windows, key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkReferenceDetectChords measures the AudioVisualizer original for
// comparison.
func BenchmarkReferenceDetectChords(b *testing.B) {
	rng := rand.New(rand.NewPCG(45, 5))
	windows := progressionWindows(rng, int(pitch.PitchClassG), false, 160)
	key := refKeyEstimate{Tonic: 7, Mode: "major"}

	b.ReportAllocs()

	for b.Loop() {
		_, _ = refDetectChords(windows, key, refDefaultChordParams())
	}
}
