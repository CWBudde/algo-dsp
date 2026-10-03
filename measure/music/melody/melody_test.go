package melody

import (
	"errors"
	"math"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
)

const testRate = 24000.0

type synthNote struct {
	start, end float64
	midi       int
}

// addHarmonicNote writes a 6-harmonic tone with 5 ms fades, like a plain
// synth lead.
func addHarmonicNote(x []float64, sampleRate float64, n synthNote) {
	f0 := pitch.MIDIToFrequency(float64(n.midi), pitch.DefaultReferenceHz)
	fade := 0.005 * sampleRate
	start, end := int(n.start*sampleRate), int(n.end*sampleRate)

	for i := start; i < end && i < len(x); i++ {
		env := math.Min(1, math.Min(float64(i-start)/fade, float64(end-i)/fade))
		time := float64(i-start) / sampleRate

		for h := 1; h <= 6; h++ {
			x[i] += env * 0.3 / float64(h) * math.Sin(2*math.Pi*f0*float64(h)*time)
		}
	}
}

// addSine adds a constant-amplitude sine at a fractional MIDI pitch.
func addSine(x []float64, sampleRate, midi, amplitude float64) {
	f := pitch.MIDIToFrequency(midi, pitch.DefaultReferenceHz)
	for i := range x {
		x[i] += amplitude * math.Sin(2*math.Pi*f*float64(i)/sampleRate)
	}
}

var testSequence = []synthNote{
	{0.20, 0.45, 60},
	{0.55, 0.80, 64},
	{0.80, 1.05, 67},
	{1.20, 1.60, 72},
	{1.70, 1.95, 69},
}

func sequenceSignal(sampleRate float64) []float64 {
	x := make([]float64, int(3*sampleRate))
	for _, n := range testSequence {
		addHarmonicNote(x, sampleRate, n)
	}

	return x
}

func TestAnalyzeRecoversNoteSequence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		sampleRate float64
		opts       []Option
	}{
		{name: "24k-defaults", sampleRate: testRate},
		{name: "44k1-hop441", sampleRate: 44100, opts: []Option{WithHop(441)}},
		{name: "48k-hop480-fft8192", sampleRate: 48000, opts: []Option{WithHop(480), WithFFTSize(8192)}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res, err := Analyze(sequenceSignal(tc.sampleRate), tc.sampleRate, tc.opts...)
			if err != nil {
				t.Fatal(err)
			}

			if res.FrameRate != 100 {
				t.Fatalf("frame rate %g, want 100", res.FrameRate)
			}

			if len(res.Notes) != len(testSequence) {
				t.Fatalf("expected %d notes, got %+v", len(testSequence), res.Notes)
			}

			for i, want := range testSequence {
				got := res.Notes[i]
				if got.MIDI != want.midi {
					t.Errorf("note %d: midi %d, want %d", i, got.MIDI, want.midi)
				}

				if math.Abs(got.Start-want.start) > 1.0/60 {
					t.Errorf("note %d starts at %.3f, want %.3f", i, got.Start, want.start)
				}

				if math.Abs(got.End-want.end) > 0.03 {
					t.Errorf("note %d ends at %.3f, want %.3f", i, got.End, want.end)
				}

				if got.Strength <= 0 || got.Strength > 1 {
					t.Errorf("note %d strength %f", i, got.Strength)
				}
			}

			if res.Pitch[10] != 0 || res.Voicing[10] != 0 {
				t.Error("silence before the first note reported as voiced")
			}
		})
	}
}

func TestAnalyzeSnapsNoteStartsToOnsets(t *testing.T) {
	t.Parallel()

	x := make([]float64, int(testRate))
	addHarmonicNote(x, testRate, synthNote{0.30, 0.70, 67})

	res, err := Analyze(x, testRate, WithOnsets([]float64{0.322}))
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Notes) != 1 || res.Notes[0].Start != 0.322 {
		t.Fatalf("note start not snapped to the nearby onset: %+v", res.Notes)
	}
}

func TestAnalyzeChromaFindsTriad(t *testing.T) {
	t.Parallel()

	x := make([]float64, int(testRate))
	for _, midi := range []float64{60, 64, 67} {
		addSine(x, testRate, midi, 0.2)
	}

	res, err := Analyze(x, testRate)
	if err != nil {
		t.Fatal(err)
	}

	type pc struct {
		class int
		value float64
	}

	classes := []pc{}
	for c := range res.Chroma {
		classes = append(classes, pc{c, res.Chroma[c][50]})
	}

	sort.Slice(classes, func(i, j int) bool { return classes[i].value > classes[j].value })

	top := map[int]bool{classes[0].class: true, classes[1].class: true, classes[2].class: true}
	if !top[int(pitch.PitchClassC)] || !top[int(pitch.PitchClassE)] || !top[int(pitch.PitchClassG)] {
		t.Fatalf("C major triad chroma: %+v", classes)
	}

	if classes[3].value > 0.35 {
		t.Fatalf("chroma leaks into pitch class %d: %f", classes[3].class, classes[3].value)
	}
}

func TestAnalyzeSilenceIsUnvoiced(t *testing.T) {
	t.Parallel()

	res, err := Analyze(make([]float64, int(testRate)), testRate)
	if err != nil {
		t.Fatal(err)
	}

	if res.Notes == nil || len(res.Notes) != 0 {
		t.Fatalf("silence produced notes %+v (nil=%v)", res.Notes, res.Notes == nil)
	}

	for c := range res.Chroma {
		for _, v := range res.Chroma[c] {
			if v != 0 {
				t.Fatal("silence produced chroma")
			}
		}
	}

	for i := range res.Pitch {
		if res.Pitch[i] != 0 || res.Voicing[i] != 0 {
			t.Fatalf("silent frame %d voiced", i)
		}
	}
}

func TestAnalyzeFrameCount(t *testing.T) {
	t.Parallel()

	for _, n := range []int{1, 239, 240, 241, 24000} {
		res, err := Analyze(make([]float64, n), testRate)
		if err != nil {
			t.Fatal(err)
		}

		want := (n + DefaultHop - 1) / DefaultHop
		if len(res.Pitch) != want || len(res.Voicing) != want || len(res.Chroma[11]) != want {
			t.Fatalf("n=%d: %d frames, want %d", n, len(res.Pitch), want)
		}
	}
}

func TestAnalyzeOptionsChangeResult(t *testing.T) {
	t.Parallel()

	x := sequenceSignal(testRate)

	// A gate above the signal level leaves every frame unvoiced.
	res, err := Analyze(x, testRate, WithGateDB(0))
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Notes) != 0 {
		t.Errorf("gate 0 dBFS kept %d notes", len(res.Notes))
	}

	// A MIDI range without the played notes cannot recover them.
	res, err = Analyze(x, testRate, WithMIDIRange(84, 96))
	if err != nil {
		t.Fatal(err)
	}

	for _, n := range res.Notes {
		if n.MIDI < 84 {
			t.Errorf("note %d below the candidate range", n.MIDI)
		}
	}

	// A voicing threshold of 1 rejects every frame of a multi-harmonic tone.
	res, err = Analyze(x, testRate, WithVoicingThreshold(1))
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Notes) != 0 {
		t.Errorf("voicing threshold 1 kept %d notes", len(res.Notes))
	}

	// A reference of 415 Hz shifts the A4 = 440 Hz grid up by one semitone.
	res, err = Analyze(x, testRate, WithReferenceHz(415.3046975799451), WithGateDB(math.Inf(-1)),
		WithSmoothing(0), WithHarmonics(4, 1), WithFrequencyRange(80, 20000))
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Notes) != len(testSequence) {
		t.Fatalf("expected %d notes, got %+v", len(testSequence), res.Notes)
	}

	for i, want := range testSequence {
		if res.Notes[i].MIDI != want.midi+1 {
			t.Errorf("note %d: midi %d, want %d", i, res.Notes[i].MIDI, want.midi+1)
		}
	}
}

func TestDownmix(t *testing.T) {
	t.Parallel()

	mono, err := Downmix([][]float64{{1, 2, -3}, {3, -2, 1}})
	if err != nil {
		t.Fatal(err)
	}

	want := []float64{2, 0, -1}
	for i := range want {
		if mono[i] != want[i] {
			t.Fatalf("mono %v, want %v", mono, want)
		}
	}

	in := []float64{0.5, -0.25}

	mono, err = Downmix([][]float64{in})
	if err != nil {
		t.Fatal(err)
	}

	mono[0] = 9

	if in[0] != 0.5 {
		t.Fatal("Downmix of one channel aliases its input")
	}

	errTests := []struct {
		name     string
		channels [][]float64
		want     error
	}{
		{name: "none", channels: nil, want: ErrEmptyInput},
		{name: "empty", channels: [][]float64{{}}, want: ErrEmptyInput},
		{name: "mismatch", channels: [][]float64{{1, 2}, {1}}, want: ErrLengthMismatch},
	}

	for _, tc := range errTests {
		_, err := Downmix(tc.channels)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestOptionValidation(t *testing.T) {
	t.Parallel()

	nan, inf := math.NaN(), math.Inf(1)
	tests := []struct {
		name string
		opt  Option
		want error
	}{
		{"nil", nil, ErrNilOption},
		{"hop", WithHop(0), ErrInvalidOption},
		{"fft", WithFFTSize(1), ErrInvalidOption},
		{"midi-order", WithMIDIRange(60, 60), ErrInvalidOption},
		{"midi-zero", WithMIDIRange(0, 60), ErrInvalidOption},
		{"midi-nan", WithMIDIRange(nan, 60), ErrInvalidOption},
		{"harmonics-count", WithHarmonics(0, 0.8), ErrInvalidOption},
		{"harmonics-decay", WithHarmonics(8, 0), ErrInvalidOption},
		{"harmonics-decay-high", WithHarmonics(8, 1.5), ErrInvalidOption},
		{"range-order", WithFrequencyRange(500, 100), ErrInvalidOption},
		{"range-zero", WithFrequencyRange(0, 100), ErrInvalidOption},
		{"range-inf", WithFrequencyRange(100, inf), ErrInvalidOption},
		{"voicing-high", WithVoicingThreshold(1.1), ErrInvalidOption},
		{"voicing-nan", WithVoicingThreshold(nan), ErrInvalidOption},
		{"gate-nan", WithGateDB(nan), ErrInvalidOption},
		{"gate-inf", WithGateDB(inf), ErrInvalidOption},
		{"reference", WithReferenceHz(0), ErrInvalidOption},
		{"smoothing", WithSmoothing(-0.01), ErrInvalidOption},
		{"onsets", WithOnsets([]float64{0.1, nan}), ErrInvalidOption},
		{"min-duration", WithNoteMinDuration(0), ErrInvalidOption},
		{"gap", WithNoteGap(-1), ErrInvalidOption},
		{"jump", WithNoteJump(0, 0.03), ErrInvalidOption},
		{"jump-duration", WithNoteJump(0.6, 0), ErrInvalidOption},
		{"snap", WithOnsetSnap(-0.01), ErrInvalidOption},
	}

	x := []float64{0, 0.1, 0}
	for _, tc := range tests {
		_, err := Analyze(x, testRate, tc.opt)
		if !errors.Is(err, tc.want) {
			t.Errorf("Analyze %s: err %v, want %v", tc.name, err, tc.want)
		}

		_, err = SegmentNotes(x, x, 100, tc.opt)
		if !errors.Is(err, tc.want) {
			t.Errorf("SegmentNotes %s: err %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestAnalyzeInputErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		x          []float64
		sampleRate float64
		want       error
	}{
		{"empty", nil, testRate, ErrEmptyInput},
		{"rate-zero", []float64{1}, 0, ErrInvalidSampleRate},
		{"rate-nan", []float64{1}, math.NaN(), ErrInvalidSampleRate},
		{"rate-inf", []float64{1}, math.Inf(1), ErrInvalidSampleRate},
	}

	for _, tc := range tests {
		_, err := Analyze(tc.x, tc.sampleRate)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestAnalyzeNyquistClamp(t *testing.T) {
	t.Parallel()

	// At 8 kHz the default 5 kHz band edge is above Nyquist and is limited
	// to the last bin; a 440 Hz tone is still found.
	x := make([]float64, 8000)
	addSine(x, 8000, 69, 0.3)
	addSine(x, 8000, 81, 0.1)

	res, err := Analyze(x, 8000, WithHop(80), WithFFTSize(2048))
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Notes) != 1 || res.Notes[0].MIDI != 69 {
		t.Fatalf("notes %+v, want one A4", res.Notes)
	}
}

func TestAnalyzeDeterministic(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(1, 2))
	x := sequenceSignal(testRate)

	for i := range x {
		x[i] += 0.01 * (rng.Float64() - 0.5)
	}

	a, err := Analyze(x, testRate)
	if err != nil {
		t.Fatal(err)
	}

	b, err := Analyze(x, testRate)
	if err != nil {
		t.Fatal(err)
	}

	assertSameBits(t, "pitch", a.Pitch, b.Pitch)
	assertSameBits(t, "voicing", a.Voicing, b.Voicing)
}

func assertSameBits(t *testing.T, name string, got, want []float64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s: length %d, want %d", name, len(got), len(want))
	}

	for i := range got {
		if math.Float64bits(got[i]) != math.Float64bits(want[i]) {
			t.Fatalf("%s[%d] = %v (%#x), want %v (%#x)", name, i,
				got[i], math.Float64bits(got[i]), want[i], math.Float64bits(want[i]))
		}
	}
}
