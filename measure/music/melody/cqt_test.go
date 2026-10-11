package melody

import (
	"errors"
	"math"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/cqt"
	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
)

func TestWithCQTValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opt  Option
		want error
	}{
		{"bpo-zero", WithCQT(0), ErrInvalidOption},
		{"bpo-negative", WithCQT(-12), ErrInvalidOption},
		{"nil-cqt-option", WithCQT(36, cqt.WithFilterScale(1), nil), ErrNilOption},
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

// TestWithCQTErrors pins the errors of impossible CQT configurations, which
// Analyze reports (the cqt options are only checked by cqt.New).
func TestWithCQTErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rate float64
		opts []Option
		want error
	}{
		// 250 = 2*125 allows only 2 octaves.
		{"hop-250", testRate, []Option{WithHop(250), WithCQT(36)}, cqt.ErrHop},
		// 44.1 kHz at 100 frames/s: hop 441 is odd, so only 1 octave fits.
		{"hop-441", 44100, []Option{WithHop(441), WithCQT(36)}, cqt.ErrHop},
		// C1 (32.7 Hz) to 5 kHz spans 7.3 octaves; hop 240 allows 5.
		{"range-c1", testRate, []Option{WithMIDIRange(24, 96), WithCQT(36)}, cqt.ErrHop},
		// At 12 bins per octave the extra bin below E3 is a semitone, and
		// 155.6 Hz to 5 kHz needs a 6th octave for its top bin.
		{"bpo-12-default-band", testRate, []Option{WithCQT(12)}, cqt.ErrHop},
		// Early downsampling by 4 leaves hop 60, not divisible by 16; this
		// is why WithCQT turns it off.
		{
			"early-downsampling", testRate,
			append(BassPreset(), WithCQT(36, cqt.WithEarlyDownsampling(true))), cqt.ErrHop,
		},
		{"cqt-option", testRate, []Option{WithCQT(36, cqt.WithFilterScale(-1))}, cqt.ErrInvalidOption},
		// Overriding fmin without the bin count puts the top bin above Nyquist.
		{"fmin-override", testRate, []Option{WithCQT(36, cqt.WithFMin(2000))}, cqt.ErrNyquist},
		// The lowest candidate (E7, 2.6 kHz) is above the band.
		{"empty-band", testRate, []Option{WithMIDIRange(100, 110), WithFrequencyRange(100, 2000), WithCQT(36)}, ErrInvalidOption},
	}

	x := make([]float64, 4800)
	for _, tc := range tests {
		_, err := Analyze(x, tc.rate, tc.opts...)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err %v, want %v", tc.name, err, tc.want)
		}
	}

	// 12 bins per octave work with a slightly lower band edge.
	_, err := Analyze(x, testRate, WithCQT(12), WithFrequencyRange(DefaultMinHz, 4900))
	if err != nil {
		t.Errorf("12 bins per octave up to 4.9 kHz: %v", err)
	}
}

func TestWithCQTSetsVoicingThreshold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts []Option
		want float64
	}{
		{"cqt", []Option{WithCQT(36)}, DefaultCQTVoicingThreshold},
		{"cqt-then-threshold", []Option{WithCQT(36), WithVoicingThreshold(0.4)}, 0.4},
		{"threshold-then-cqt", []Option{WithVoicingThreshold(0.4), WithCQT(36)}, DefaultCQTVoicingThreshold},
		{"bass-then-cqt", append(BassPreset(), WithCQT(36)), DefaultCQTVoicingThreshold},
		{"cqt-then-bass", append([]Option{WithCQT(36)}, BassPreset()...), DefaultVoicingThreshold},
	}

	for _, tc := range tests {
		cfg, err := newConfig(tc.opts)
		if err != nil {
			t.Fatal(err)
		}

		if cfg.voicing != tc.want || cfg.cqt == nil || cfg.cqt.binsPerOctave != 36 {
			t.Errorf("%s: voicing %g (want %g), cqt %+v", tc.name, cfg.voicing, tc.want, cfg.cqt)
		}
	}

	cfg, err := newConfig(nil)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.cqt != nil {
		t.Fatal("CQT front end enabled by default")
	}
}

// TestCQTFrontEndLayout pins the derived transform: one bin below the
// lowest candidate up to the band edge or Nyquist, without early
// downsampling, at the analysis hop.
func TestCQTFrontEndLayout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		rate          float64
		opts          []Option
		bins, octaves int
		lowestMIDI    float64
		top           float64
	}{
		{"defaults", testRate, []Option{WithCQT(36)}, 179, 5, DefaultMinMIDI, DefaultMaxHz},
		{"bass", testRate, append(BassPreset(), WithCQT(36)), 177, 5, BassMinMIDI, BassMaxHz},
		{"low", testRate, append(lowOptions(), WithCQT(36)), 179, 5, 36, 2000},
		{"nyquist-8k", 8000, []Option{WithHop(80), WithCQT(36)}, 167, 5, DefaultMinMIDI, 4000},
		{"bpo-24", testRate, []Option{WithCQT(24)}, 120, 5, DefaultMinMIDI, DefaultMaxHz},
	}

	for _, tc := range tests {
		cfg, err := newConfig(tc.opts)
		if err != nil {
			t.Fatal(err)
		}

		a, err := newAnalyzer(&cfg, tc.rate)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}

		tr := a.cq.tr
		freqs := tr.Frequencies()
		f1 := pitch.MIDIToFrequency(tc.lowestMIDI, pitch.DefaultReferenceHz)

		if tr.Bins() != tc.bins || tr.Octaves() != tc.octaves || tr.DownsampleFactor() != 1 || tr.Hop() != cfg.hop {
			t.Errorf("%s: %d bins, %d octaves, factor %d, hop %d", tc.name, tr.Bins(), tr.Octaves(), tr.DownsampleFactor(), tr.Hop())
		}

		if math.Abs(freqs[1]/f1-1) > 1e-12 || freqs[len(freqs)-1] > tc.top {
			t.Errorf("%s: bins %g, %g .. %g Hz, want bin 1 at %g and the top <= %g", tc.name, freqs[0], freqs[1],
				freqs[len(freqs)-1], f1, tc.top)
		}

		// The next bin would be above the edge.
		if next := freqs[len(freqs)-1] * math.Pow(2, 1/a.cq.binsPerOctave); next <= tc.top {
			t.Errorf("%s: bin at %g Hz <= %g Hz missing", tc.name, next, tc.top)
		}

		if a.lo != 0 || a.hi != len(freqs)-1 {
			t.Errorf("%s: band bins %d..%d", tc.name, a.lo, a.hi)
		}
	}
}

// TestCQTMagnitudeScale verifies the claim behind the wrap normalization:
// a sine of amplitude A at a bin centre reads about A at any frequency,
// while the librosa normalization weights bin b by sqrt(Lengths()[b])/2
// relative to it, which would tilt the harmonic sum towards the low bins.
func TestCQTMagnitudeScale(t *testing.T) {
	t.Parallel()

	newFront := func(opts ...cqt.Option) *cqtFront {
		cfg, err := newConfig([]Option{WithCQT(36, opts...)})
		if err != nil {
			t.Fatal(err)
		}

		a, err := newAnalyzer(&cfg, testRate)
		if err != nil {
			t.Fatal(err)
		}

		return a.cq
	}

	wrap := newFront()
	librosa := newFront(cqt.WithNormalization(cqt.NormalizationLibrosa))
	freqs, lengths := wrap.tr.Frequencies(), wrap.tr.Lengths()

	const amplitude = 0.5

	for _, b := range []int{1, 40, 90, 150, 178} {
		x := make([]float64, 2*int(testRate))
		for i := range x {
			x[i] = amplitude * math.Sin(2*math.Pi*freqs[b]*float64(i)/testRate)
		}

		for _, c := range []*cqtFront{wrap, librosa} {
			if err := c.process(x); err != nil {
				t.Fatal(err)
			}
		}

		got := wrap.row(100)[b]
		if math.Abs(got/amplitude-1) > 0.01 {
			t.Errorf("bin %d (%.1f Hz): magnitude %.5f, want %.2f within 1%%", b, freqs[b], got, amplitude)
		}

		ratio := librosa.row(100)[b] / got
		if want := math.Sqrt(lengths[b]) / 2; math.Abs(ratio/want-1) > 1e-12 {
			t.Errorf("bin %d: librosa/wrap %g, want sqrt(%g)/2 = %g", b, ratio, lengths[b], want)
		}
	}

	if tilt := math.Sqrt(lengths[1] / lengths[178]); tilt < 5 {
		t.Errorf("librosa tilt between bins 1 and 178 only %g", tilt)
	}
}

func TestCQTRecoversNoteSequence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rate float64
		opts []Option
	}{
		{"24k", testRate, []Option{WithCQT(DefaultCQTBinsPerOctave)}},
		{"48k-hop480", 48000, []Option{WithHop(480), WithCQT(DefaultCQTBinsPerOctave)}},
	}

	for _, tc := range tests {
		res, err := Analyze(sequenceSignal(tc.rate), tc.rate, tc.opts...)
		if err != nil {
			t.Fatal(err)
		}

		if len(res.Notes) != len(testSequence) {
			t.Fatalf("%s: expected %d notes, got %+v", tc.name, len(testSequence), res.Notes)
		}

		for i, want := range testSequence {
			got := res.Notes[i]
			if got.MIDI != want.midi || math.Abs(got.Start-want.start) > 1.0/60 || math.Abs(got.End-want.end) > 0.03 {
				t.Errorf("%s: note %d %+v, want %+v", tc.name, i, got, want)
			}
		}
	}
}

// TestCQTNoiseVoicing pins why WithCQT raises the voicing threshold: the
// STFT voices no frame of white noise at its default threshold, the CQT
// voices most frames at that threshold (182 of 200, 11 notes) but at most a
// stray frame at DefaultCQTVoicingThreshold (1 of 200, no notes).
func TestCQTNoiseVoicing(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(5, 6))
	x := make([]float64, 2*int(testRate))

	for i := range x {
		x[i] = 0.2 * (2*rng.Float64() - 1)
	}

	voiced := func(opts ...Option) (int, int) {
		res, err := Analyze(x, testRate, opts...)
		if err != nil {
			t.Fatal(err)
		}

		n := 0

		for _, p := range res.Pitch {
			if p != 0 {
				n++
			}
		}

		return n, len(res.Notes)
	}

	stftFrames, stftNotes := voiced()
	cqtFrames, cqtNotes := voiced(WithCQT(36))
	lowFrames, lowNotes := voiced(WithCQT(36), WithVoicingThreshold(DefaultVoicingThreshold))

	t.Logf("voiced frames of 200 (notes): STFT %d (%d), CQT %d (%d), CQT at 0.3 %d (%d)",
		stftFrames, stftNotes, cqtFrames, cqtNotes, lowFrames, lowNotes)

	if stftFrames != 0 || cqtFrames > 2 || cqtNotes != 0 || lowFrames < 150 {
		t.Fatalf("voiced frames: STFT %d, CQT %d (%d notes), CQT at 0.3 %d", stftFrames, cqtFrames, cqtNotes, lowFrames)
	}
}

func TestCQTChromaFindsTriad(t *testing.T) {
	t.Parallel()

	x := make([]float64, int(testRate))
	for _, midi := range []float64{60, 64, 67} {
		addSine(x, testRate, midi, 0.2)
	}

	res, err := Analyze(x, testRate, WithCQT(DefaultCQTBinsPerOctave))
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
	t.Logf("chroma %+v", classes)

	top := map[int]bool{classes[0].class: true, classes[1].class: true, classes[2].class: true}
	if !top[int(pitch.PitchClassC)] || !top[int(pitch.PitchClassE)] || !top[int(pitch.PitchClassG)] {
		t.Fatalf("C major triad chroma: %+v", classes)
	}

	if classes[2].value < 0.8 || classes[3].value > 0.1 {
		t.Fatalf("triad chroma not separated: %+v", classes)
	}
}

// TestCQTChromaCoverage pins that the CQT chroma does not see below the
// lowest bin (160 Hz with the defaults), while the STFT chroma reaches down
// to the band edge (100 Hz): a lone C3 (131 Hz) is the strongest class of
// the STFT chroma but only a leak into the lowest CQT bins (E3 and F3, 4
// semitones higher).
func TestCQTChromaCoverage(t *testing.T) {
	t.Parallel()

	x := make([]float64, int(testRate))
	addSine(x, testRate, 48, 0.3)

	stftRes, err := Analyze(x, testRate)
	if err != nil {
		t.Fatal(err)
	}

	cqtRes, err := Analyze(x, testRate, WithCQT(DefaultCQTBinsPerOctave))
	if err != nil {
		t.Fatal(err)
	}

	if v := stftRes.Chroma[pitch.PitchClassC][50]; v != 1 {
		t.Errorf("STFT chroma C %g, want 1", v)
	}

	if v := cqtRes.Chroma[pitch.PitchClassC][50]; v > 0.1 {
		t.Errorf("CQT chroma C %g, want the C3 unseen", v)
	}

	if cqtRes.Chroma[pitch.PitchClassE][50] != 1 && cqtRes.Chroma[pitch.PitchClassF][50] != 1 &&
		cqtRes.Chroma[pitch.PitchClassDSharp][50] != 1 {
		var row [12]float64
		for c := range row {
			row[c] = cqtRes.Chroma[c][50]
		}

		t.Errorf("CQT chroma %v, want the leak strongest around the lowest bin", row)
	}
}

// TestCQTShortSignal pins the behaviour for signals shorter than the CQT's
// decimation chain: they are zero padded at the end, so the result equals
// that of the explicitly padded signal, and the frame count stays
// ceil(n/hop).
func TestCQTShortSignal(t *testing.T) {
	t.Parallel()

	cfg, err := newConfig([]Option{WithCQT(36)})
	if err != nil {
		t.Fatal(err)
	}

	a, err := newAnalyzer(&cfg, testRate)
	if err != nil {
		t.Fatal(err)
	}

	// Four 2:1 decimations need 16 samples.
	if a.cq.tr.FrameCount(15) != 0 || a.cq.tr.FrameCount(16) != 1 {
		t.Fatalf("CQT frame counts %d, %d for 15, 16 samples", a.cq.tr.FrameCount(15), a.cq.tr.FrameCount(16))
	}

	for _, n := range []int{1, 10, 15, 16, 100, 239, 240, 241} {
		x := make([]float64, n)
		addSine(x, testRate, 69, 0.5)

		res, err := Analyze(x, testRate, WithCQT(36), WithGateDB(math.Inf(-1)))
		if err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}

		if want := (n + DefaultHop - 1) / DefaultHop; len(res.Pitch) != want || len(res.Chroma[0]) != want {
			t.Fatalf("n=%d: %d frames, want %d", n, len(res.Pitch), want)
		}

		if n >= 16 {
			continue
		}

		padded, err := Analyze(append(x, make([]float64, 16-n)...), testRate, WithCQT(36), WithGateDB(math.Inf(-1)))
		if err != nil {
			t.Fatal(err)
		}

		assertSameBits(t, "pitch", res.Pitch, padded.Pitch)
		assertSameBits(t, "voicing", res.Voicing, padded.Voicing)

		for c := range res.Chroma {
			assertSameBits(t, "chroma", res.Chroma[c], padded.Chroma[c])
		}
	}
}

func TestCQTDeterministic(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(1, 2))
	x := sequenceSignal(testRate)

	for i := range x {
		x[i] += 0.01 * (rng.Float64() - 0.5)
	}

	a, err := Analyze(x, testRate, WithCQT(36))
	if err != nil {
		t.Fatal(err)
	}

	b, err := Analyze(x, testRate, WithCQT(36))
	if err != nil {
		t.Fatal(err)
	}

	assertSameBits(t, "pitch", a.Pitch, b.Pitch)
	assertSameBits(t, "voicing", a.Voicing, b.Voicing)

	for c := range a.Chroma {
		assertSameBits(t, "chroma", a.Chroma[c], b.Chroma[c])
	}
}

// TestCQTOptionPrecedence checks which options reach the transform: the FFT
// size and a cqt hop or output are ignored, a cqt filter scale is not.
func TestCQTOptionPrecedence(t *testing.T) {
	t.Parallel()

	x := sequenceSignal(testRate)[:int(1.2*testRate)]

	analyze := func(opts ...Option) *Result {
		res, err := Analyze(x, testRate, opts...)
		if err != nil {
			t.Fatal(err)
		}

		return res
	}

	base := analyze(WithCQT(36))

	for _, other := range []*Result{
		analyze(WithCQT(36), WithFFTSize(8192)),
		analyze(WithCQT(36, cqt.WithHopLength(512), cqt.WithOutput(cqt.OutputComplex), cqt.WithBinsPerOctave(12))),
	} {
		assertSameBits(t, "pitch", other.Pitch, base.Pitch)
		assertSameBits(t, "voicing", other.Voicing, base.Voicing)
	}

	scaled := analyze(WithCQT(36, cqt.WithFilterScale(0.5)))
	same := true

	for i := range scaled.Voicing {
		if scaled.Voicing[i] != base.Voicing[i] {
			same = false
		}
	}

	if same {
		t.Fatal("cqt.WithFilterScale had no effect")
	}
}
