package melody

import (
	"math"
	"slices"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
)

// TestBassPresetValues checks the preset against AudioVisualizer's
// BassMelodyOptions: FFT 8192, MIDI 28–60, 30–1200 Hz, 6 harmonics with
// decay 0.8, voicing 0.3, smoothing 0.04 s, notes >= 0.10 s, jump 0.8
// semitone for 0.05 s, onset snap 0.06 s.
func TestBassPresetValues(t *testing.T) {
	t.Parallel()

	cfg, err := newConfig(BassPreset())
	if err != nil {
		t.Fatal(err)
	}

	want := defaultConfig()
	want.fftSize = 8192
	want.minMIDI, want.maxMIDI = 28, 60
	want.minHz, want.maxHz = 30, 1200
	want.harmonics, want.harmonicDecay = 6, 0.8
	want.voicing = 0.3
	want.smoothing = 0.04
	want.notes.minDuration = 0.10
	want.notes.jump, want.notes.jumpDuration = 0.8, 0.05
	want.notes.snap = 0.06

	if cfg.hop != want.hop || cfg.fftSize != want.fftSize || cfg.minMIDI != want.minMIDI || cfg.maxMIDI != want.maxMIDI ||
		cfg.harmonics != want.harmonics || cfg.harmonicDecay != want.harmonicDecay || cfg.minHz != want.minHz ||
		cfg.maxHz != want.maxHz || cfg.voicing != want.voicing || cfg.gateDB != want.gateDB ||
		cfg.referenceHz != want.referenceHz || cfg.smoothing != want.smoothing || cfg.onsets != nil || cfg.notes != want.notes {
		t.Fatalf("bass preset config\n got %+v\nwant %+v", cfg, want)
	}

	clean := defaultCleanConfig()
	for _, opt := range BassCleanOptions() {
		if err := opt(&clean); err != nil {
			t.Fatal(err)
		}
	}

	wantClean := defaultCleanConfig()
	wantClean.floorMIDI = 28

	if clean != wantClean {
		t.Fatalf("bass clean config %+v", clean)
	}
}

// addSawNote adds a band-limited sawtooth (10 harmonics, amplitude 1/h) of a
// MIDI note between start and end seconds, with 10 ms fades.
func addSawNote(x []float64, sampleRate float64, n synthNote) {
	f0 := pitch.MIDIToFrequency(float64(n.midi), pitch.DefaultReferenceHz)
	fade := 0.01 * sampleRate
	start, end := int(n.start*sampleRate), int(n.end*sampleRate)

	for i := start; i < end && i < len(x); i++ {
		env := math.Min(1, math.Min(float64(i-start)/fade, float64(end-i)/fade))
		time := float64(i-start) / sampleRate

		for h := 1; h <= 10; h++ {
			x[i] += env * 0.25 / float64(h) * math.Sin(2*math.Pi*f0*float64(h)*time)
		}
	}
}

// TestBassPresetTracksLowLine synthesises a sawtooth bass line between
// 41 Hz (E1) and 55 Hz (A1) at 24 kHz and checks that the preset tracks
// every note within 10 cents and segments it into the right MIDI notes.
func TestBassPresetTracksLowLine(t *testing.T) {
	t.Parallel()

	line := []synthNote{
		{0.10, 0.60, 28}, // E1 41.2 Hz
		{0.70, 1.20, 31}, // G1 49.0 Hz
		{1.30, 1.80, 33}, // A1 55.0 Hz
		{1.90, 2.40, 30}, // F#1 46.2 Hz
		{2.50, 3.00, 29}, // F1 43.7 Hz
		{3.10, 3.60, 32}, // G#1 51.9 Hz
	}

	x := make([]float64, int(3.8*testRate))
	for _, n := range line {
		addSawNote(x, testRate, n)
	}

	res, err := Analyze(x, testRate, BassPreset()...)
	if err != nil {
		t.Fatal(err)
	}

	for _, n := range line {
		var voiced []float64

		// Skip the edges, where the 341 ms window straddles the gaps.
		for i := int(math.Ceil((n.start + 0.17) * res.FrameRate)); float64(i)/res.FrameRate <= n.end-0.17; i++ {
			if res.Pitch[i] == 0 {
				t.Fatalf("note %d: frame %d unvoiced", n.midi, i)
			}

			voiced = append(voiced, res.Pitch[i])
		}

		if len(voiced) < 10 {
			t.Fatalf("note %d: %d frames", n.midi, len(voiced))
		}

		lo, hi := slices.Min(voiced), slices.Max(voiced)
		t.Logf("note %d: pitch %.4f..%.4f", n.midi, lo, hi)

		// 10 cents is also the candidate spacing; allow for its rounding.
		if math.Abs(lo-float64(n.midi)) > 0.1+1e-9 || math.Abs(hi-float64(n.midi)) > 0.1+1e-9 {
			t.Fatalf("note %d: pitch %.3f..%.3f, want within 10 cents", n.midi, lo, hi)
		}
	}

	got := make([]int, 0, len(res.Notes))
	for _, n := range res.Notes {
		got = append(got, n.MIDI)
	}

	if want := []int{28, 31, 33, 30, 29, 32}; !slices.Equal(got, want) {
		t.Fatalf("notes %v (%+v), want %v", got, res.Notes, want)
	}
}
