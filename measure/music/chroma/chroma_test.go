package chroma

import (
	"errors"
	"math"
	"slices"
	"sort"
	"sync"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/cqt"
	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
	"github.com/cwbudde/algo-dsp/measure/music/harmony"
)

func TestDefaultConstants(t *testing.T) {
	t.Parallel()

	ref := pitch.DefaultReferenceHz

	if got, want := DefaultMinHz, pitch.MIDIToFrequency(23.5, ref); math.Abs(got-want) > 1e-12*want {
		t.Errorf("DefaultMinHz = %.17g, want C1 − 50 cents = %.17g", got, want)
	}

	if got, want := DefaultMaxHz, pitch.MIDIToFrequency(107.5, ref); math.Abs(got-want) > 1e-12*want {
		t.Errorf("DefaultMaxHz = %.17g, want B7 + 50 cents = %.17g", got, want)
	}
}

func TestDerivedTransform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sampleRate float64
		factor     int
	}{
		{22050, 1},
		{24000, 1},
		{44100, 2},
		{48000, 2},
	}

	for _, tt := range tests {
		c, err := New(tt.sampleRate)
		if err != nil {
			t.Fatalf("New(%g): %v", tt.sampleRate, err)
		}

		// Seven octaves of 36 bins from C1 − 1/3 to B7 + 1/3 semitone.
		freqs := c.Frequencies()
		if len(freqs) != 252 || c.tr.Bins() != 252 {
			t.Fatalf("%g Hz: %d folded of %d bins, want 252", tt.sampleRate, len(freqs), c.tr.Bins())
		}

		first := pitch.FrequencyToMIDI(freqs[0], DefaultReferenceHz)
		last := pitch.FrequencyToMIDI(freqs[251], DefaultReferenceHz)

		if math.Abs(first-(24-1.0/3)) > 1e-9 || math.Abs(last-(107+1.0/3)) > 1e-9 {
			t.Errorf("%g Hz: bins span MIDI %.9f..%.9f", tt.sampleRate, first, last)
		}

		if c.tr.DownsampleFactor() != tt.factor {
			t.Errorf("%g Hz: early downsampling factor %d, want %d", tt.sampleRate, c.tr.DownsampleFactor(), tt.factor)
		}

		if got, want := c.FrameRate(), tt.sampleRate/DefaultHop; got != want {
			t.Errorf("%g Hz: frame rate %g, want %g", tt.sampleRate, got, want)
		}

		if got := c.FrameCount(10000); got != 10000/DefaultHop+1 {
			t.Errorf("%g Hz: FrameCount(10000) = %d, want %d", tt.sampleRate, got, 10000/DefaultHop+1)
		}
	}
}

// TestBinAmplitude verifies the derived CQT scale: L1 kernels, wrap
// normalization and the undone early downsampling factor read a sinusoid
// of amplitude A at a bin centre as magnitude A at every frequency and
// sample rate.
func TestBinAmplitude(t *testing.T) {
	t.Parallel()

	const amp = 0.5

	for _, sr := range []float64{22050, 48000} {
		c, err := New(sr)
		if err != nil {
			t.Fatal(err)
		}

		freqs := c.tr.Frequencies()
		bins := c.tr.Bins()
		scale := 1 / float64(c.tr.DownsampleFactor())

		for _, m := range []float64{33, 45, 69, 93, 105} {
			f := pitch.MIDIToFrequency(m, DefaultReferenceHz)
			x := sine(int(4*sr), sr, f, amp)

			mag, err := c.tr.Process(x)
			if err != nil {
				t.Fatal(err)
			}

			b := sort.SearchFloat64s(freqs, f*(1-1e-9))
			frame := c.tr.FrameCount(len(x)) / 2
			got := mag[frame*bins+b] * scale

			if math.Abs(got-amp) > 0.002*amp {
				t.Errorf("%g Hz, MIDI %g: bin magnitude %.5f, want %g", sr, m, got, amp)
			}
		}
	}
}

func TestTriads(t *testing.T) {
	t.Parallel()

	const sr = 22050

	tests := []struct {
		name  string
		notes []float64
	}{
		{"C major, octave 3", []float64{48, 52, 55}},
		{"F# minor, octave 4", []float64{66, 69, 73}},
		{"Bb major, octave 2", []float64{46, 50, 53}},
		{"E minor, spread", []float64{40, 67, 83}},
		{"D major, first inversion", []float64{54, 57, 62}},
	}

	for _, tt := range tests {
		x := synth(sr, 3, DefaultReferenceHz, event{0, 3, tt.notes})

		c, rate, err := Compute(x, sr)
		if err != nil {
			t.Fatal(err)
		}

		want := classSet(tt.notes...)
		lo, hi := int(math.Ceil(1*rate)), int(2*rate) // away from the edges
		worst := math.Inf(1)

		for i := lo; i < hi; i++ {
			v := frameOf(c, i)
			top := ranked(v)[:3]
			slices.Sort(top)

			if !slices.Equal(top, want) {
				t.Fatalf("%s: frame %d top classes %v, want %v (%v)", tt.name, i, top, want, v)
			}

			weakest, other := math.Inf(1), 0.0

			for pc, p := range v {
				if slices.Contains(want, pc) {
					weakest = math.Min(weakest, p)
				} else {
					other = math.Max(other, p)
				}
			}

			worst = math.Min(worst, weakest/other)
		}

		// Third harmonics of the third and fifth (amplitude 1/4, power
		// 1/16) are the strongest non-chord classes.
		if worst < 4 {
			t.Errorf("%s: weakest chord tone only %.2f× the strongest other class", tt.name, worst)
		}

		t.Logf("%s: weakest chord tone ≥ %.1f× strongest other class", tt.name, worst)
	}
}

func TestKeyEstimation(t *testing.T) {
	t.Parallel()

	const sr = 24000

	// Four chords of one second each, a triad plus its root an octave
	// below the voicing's lowest note.
	tests := []struct {
		name   string
		chords [][]float64
		tonic  pitch.PitchClass
		mode   harmony.Mode
	}{
		{"C major I-IV-V-I", [][]float64{
			{48, 60, 64, 67}, {41, 57, 60, 65}, {43, 55, 59, 62}, {48, 60, 64, 67},
		}, pitch.PitchClassC, harmony.Major},
		{"A minor i-iv-V-i", [][]float64{
			{45, 57, 60, 64}, {50, 62, 65, 69}, {40, 56, 59, 64}, {45, 57, 60, 64},
		}, pitch.PitchClassA, harmony.Minor},
		{"G major I-vi-IV-V-I", [][]float64{
			{43, 55, 59, 62}, {40, 55, 59, 64}, {48, 55, 60, 64}, {50, 57, 62, 66}, {43, 55, 59, 62},
		}, pitch.PitchClassG, harmony.Major},
	}

	for _, tt := range tests {
		events := make([]event, len(tt.chords))
		for i, ch := range tt.chords {
			events[i] = event{float64(i), float64(i + 1), ch}
		}

		x := synth(sr, float64(len(tt.chords)), DefaultReferenceHz, events...)

		c, _, err := Compute(x, sr)
		if err != nil {
			t.Fatal(err)
		}

		key, err := harmony.EstimateKey(Profile(c, 0, len(c[0])))
		if err != nil {
			t.Fatal(err)
		}

		if key.Tonic != tt.tonic || key.Mode != tt.mode {
			t.Errorf("%s: key %v, want %v %v", tt.name, key, tt.tonic, tt.mode)
		}

		t.Logf("%s: %v r=%.3f, runner-up %v r=%.3f, ambiguous %v", tt.name, key, key.Correlation,
			key.RunnerUp, key.RunnerUp.Correlation, key.Ambiguous)
	}
}

func TestTuningOffset(t *testing.T) {
	t.Parallel()

	const sr = 22050

	flat := DefaultReferenceHz * math.Pow(2, -0.6/12) // 60 cents flat
	aMajor := []float64{57, 61, 64}                   // A3 C#4 E4

	tests := []struct {
		name  string
		tuned float64 // A4 of the synthesized signal
		opts  []Option
		want  []int
	}{
		{"60 cents flat, default reference", flat, nil, classSet(56, 60, 63)}, // G# C D#
		{"60 cents flat, matching reference", flat, []Option{WithReferenceHz(flat)}, classSet(aMajor...)},
		{"20 cents sharp, default reference", 445, nil, classSet(aMajor...)},
		{"in tune, 12 bins per octave", DefaultReferenceHz, []Option{WithBinsPerOctave(12)}, classSet(aMajor...)},
	}

	for _, tt := range tests {
		x := synth(sr, 3, tt.tuned, event{0, 3, aMajor})

		c, rate, err := Compute(x, sr, tt.opts...)
		if err != nil {
			t.Fatal(err)
		}

		p := Profile(c, int(rate), int(2*rate))
		top := ranked(p)[:3]
		slices.Sort(top)

		if !slices.Equal(top, tt.want) {
			t.Errorf("%s: top classes %v, want %v (%.3f)", tt.name, top, tt.want, p)
		}

		t.Logf("%s: top %v, profile %.3f", tt.name, top, p)
	}
}

func TestHalfwayBinsSplit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		m         float64
		pc, other int
		split     bool
	}{
		{69, 9, 9, false},
		{69.5, 9, 10, true},
		{69.5 + 1e-7, 9, 10, true},
		{69.5 - 1e-7, 9, 10, true},
		{69.5 + 1e-5, 10, 10, false},
		{69.5 - 1e-5, 9, 9, false},
		{23.5, 11, 0, true},   // B0 / C1
		{-0.5, 11, 0, true},   // below MIDI 0
		{-1.2, 11, 11, false}, // MIDI −1 is B
	}

	for _, tt := range tests {
		f := foldOf(7, tt.m)
		if f.bin != 7 || f.pc != tt.pc || f.other != tt.other || f.split != tt.split {
			t.Errorf("foldOf(%g) = %+v, want pc %d other %d split %v", tt.m, f, tt.pc, tt.other, tt.split)
		}
	}

	// With 24 bins per octave every other bin lies half-way between two
	// semitones. A sine there loads A and A# almost equally; rounding the
	// bin to one side would give that side twice the power.
	const sr = 22050

	x := sine(3*sr, sr, pitch.MIDIToFrequency(69.5, DefaultReferenceHz), 0.5)

	c, _, err := Compute(x, sr, WithBinsPerOctave(24), WithNormalization(NormNone))
	if err != nil {
		t.Fatal(err)
	}

	p := Profile(c, 40, 80)
	ratio := p[9] / p[10]

	if math.Abs(ratio-1) > 0.05 {
		t.Errorf("A/A# = %.4f for a sine half-way between them, want ≈ 1 (%.4f)", ratio, p)
	}

	t.Logf("24 bins/oct, sine at MIDI 69.5: A %.5f, A# %.5f, ratio %.4f", p[9], p[10], ratio)
}

func TestNormalizations(t *testing.T) {
	t.Parallel()

	const sr = 22050

	x := synth(sr, 2, DefaultReferenceHz, event{0, 2, []float64{50, 54, 57, 60}})

	for _, n := range []Normalization{NormMax, NormL1, NormL2, NormNone} {
		c, _, err := Compute(x, sr, WithNormalization(n))
		if err != nil {
			t.Fatal(err)
		}

		for i := range c[0] {
			v := frameOf(c, i)
			maxV, sum, sq := 0.0, 0.0, 0.0

			for _, p := range v {
				if p < 0 {
					t.Fatalf("%v: negative value %g", n, p)
				}

				maxV, sum, sq = math.Max(maxV, p), sum+p, sq+p*p
			}

			var got float64

			switch n {
			case NormMax:
				got = maxV
			case NormL1:
				got = sum
			case NormL2:
				got = math.Sqrt(sq)
			case NormNone:
				continue
			}

			if math.Abs(got-1) > 1e-12 {
				t.Fatalf("%v: frame %d has norm %g, want 1", n, i, got)
			}
		}
	}
}

// TestRawPower pins the scale of NormNone: a sine of amplitude A at a bin
// centre reads magnitude A in its bin and A/2 in the two neighbours (the
// Hann window's response one bin off centre), which at 36 bins per octave
// fold into the same class. The class power is therefore ≈ 1.5·A², the
// Hann window's equivalent noise bandwidth in bins, at any sample rate.
func TestRawPower(t *testing.T) {
	t.Parallel()

	const amp = 0.5

	for _, sr := range []float64{22050, 44100} {
		for _, m := range []float64{45, 69, 93} {
			x := sine(int(3*sr), sr, pitch.MIDIToFrequency(m, DefaultReferenceHz), amp)

			c, rate, err := Compute(x, sr, WithNormalization(NormNone))
			if err != nil {
				t.Fatal(err)
			}

			p := Profile(c, int(rate), int(2*rate))
			pc := pitchClassOf(int(m))
			total := 0.0

			for _, v := range p {
				total += v
			}

			if got := p[pc] / (amp * amp); math.Abs(got-1.51) > 0.02 {
				t.Errorf("%g Hz, MIDI %g: class power %.4f·A², want ≈ 1.51·A²", sr, m, got)
			}

			if share := p[pc] / total; share < 0.999 {
				t.Errorf("%g Hz, MIDI %g: only %.4f of the power in its class", sr, m, share)
			}
		}
	}
}

func TestSilence(t *testing.T) {
	t.Parallel()

	const sr = 22050

	for _, x := range [][]float64{
		make([]float64, sr),
		sine(sr, sr, 440, 1e-7), // power ≈ 1.5e-14 < 1e-12
	} {
		for _, n := range []Normalization{NormMax, NormNone} {
			c, _, err := Compute(x, sr, WithNormalization(n))
			if err != nil {
				t.Fatal(err)
			}

			for pc := range c {
				for i, v := range c[pc] {
					if v != 0 {
						t.Fatalf("%v: class %d frame %d is %g, want 0", n, pc, i, v)
					}
				}
			}
		}
	}
}

// TestFrameTiming checks that frame i is centred on sample i·hop: a tone
// switching from A to C at 1.5 s gives A before and C after, up to the
// lowest kernel's half length.
func TestFrameTiming(t *testing.T) {
	t.Parallel()

	const sr = 22050

	x := synth(sr, 3, DefaultReferenceHz, event{0, 1.5, []float64{69}}, event{1.5, 3, []float64{72}})

	c, err := New(sr, WithFrequencyRange(200, 2000))
	if err != nil {
		t.Fatal(err)
	}

	out, err := c.Process(x)
	if err != nil {
		t.Fatal(err)
	}

	rate := c.FrameRate()

	for i := range out[0] {
		at := float64(i) / rate
		top := ranked(frameOf(out, i))[0]

		switch {
		case at < 1.4 && top != 9:
			t.Errorf("frame %d (%.3f s): strongest class %d, want A", i, at, top)
		case at > 1.6 && top != 0:
			t.Errorf("frame %d (%.3f s): strongest class %d, want C", i, at, top)
		}
	}
}

func TestOptionErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		sampleRate float64
		opts       []Option
		want       []error
	}{
		{"zero sample rate", 0, nil, []error{ErrInvalidSampleRate}},
		{"NaN sample rate", math.NaN(), nil, []error{ErrInvalidSampleRate}},
		{"infinite sample rate", math.Inf(1), nil, []error{ErrInvalidSampleRate}},
		{"nil option", 22050, []Option{nil}, []error{ErrNilOption, ErrInvalidOption}},
		{"bins per octave 0", 22050, []Option{WithBinsPerOctave(0)}, []error{ErrInvalidOption}},
		{"bins per octave 18", 22050, []Option{WithBinsPerOctave(18)}, []error{ErrInvalidOption}},
		{"bins per octave -12", 22050, []Option{WithBinsPerOctave(-12)}, []error{ErrInvalidOption}},
		{"range min 0", 22050, []Option{WithFrequencyRange(0, 100)}, []error{ErrInvalidOption}},
		{"range empty", 22050, []Option{WithFrequencyRange(100, 100)}, []error{ErrInvalidOption}},
		{"range reversed", 22050, []Option{WithFrequencyRange(200, 100)}, []error{ErrInvalidOption}},
		{"range NaN", 22050, []Option{WithFrequencyRange(math.NaN(), 100)}, []error{ErrInvalidOption}},
		{"range infinite", 22050, []Option{WithFrequencyRange(100, math.Inf(1))}, []error{ErrInvalidOption}},
		{"range between grid bins", 22050, []Option{
			WithBinsPerOctave(12), WithFrequencyRange(440*math.Pow(2, 0.1/12), 440*math.Pow(2, 0.4/12)),
		}, []error{ErrInvalidOption}},
		{"reference 0", 22050, []Option{WithReferenceHz(0)}, []error{ErrInvalidOption}},
		{"reference negative", 22050, []Option{WithReferenceHz(-440)}, []error{ErrInvalidOption}},
		{"reference NaN", 22050, []Option{WithReferenceHz(math.NaN())}, []error{ErrInvalidOption}},
		{"normalization 0", 22050, []Option{WithNormalization(0)}, []error{ErrInvalidOption}},
		{"normalization 5", 22050, []Option{WithNormalization(5)}, []error{ErrInvalidOption}},
		{"cqt bins per octave 10", 22050, []Option{
			WithCQT(cqt.WithBinsPerOctave(10), cqt.WithBins(20)),
		}, []error{ErrInvalidOption}},
		{"cqt bins per octave 10, seven octaves", 22050, []Option{
			WithCQT(cqt.WithBinsPerOctave(10)),
		}, []error{ErrInvalidOption, cqt.ErrNyquist}},
		{"cqt nil option", 22050, []Option{WithCQT(nil)}, []error{ErrInvalidOption, cqt.ErrNilOption}},
		{"cqt hop 240", 24000, []Option{WithCQT(cqt.WithHopLength(240))}, []error{ErrInvalidOption, cqt.ErrHop}},
		{"Nyquist", 8000, nil, []error{ErrInvalidOption, cqt.ErrNyquist}},
		{"no transform bin in band", 22050, []Option{
			WithFrequencyRange(100, 200), WithCQT(cqt.WithFMin(1000), cqt.WithBins(12)),
		}, []error{ErrInvalidOption}},
	}

	for _, tt := range tests {
		_, err := New(tt.sampleRate, tt.opts...)
		if err == nil {
			t.Errorf("%s: no error", tt.name)

			continue
		}

		for _, w := range tt.want {
			if !errors.Is(err, w) {
				t.Errorf("%s: error %q does not wrap %q", tt.name, err, w)
			}
		}

		_, _, err2 := Compute([]float64{1, 2, 3}, tt.sampleRate, tt.opts...)
		if err2 == nil || err2.Error() != err.Error() {
			t.Errorf("%s: Compute error %v, New error %v", tt.name, err2, err)
		}
	}
}

func TestValidOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		sampleRate float64
		opts       []Option
		bins       int
	}{
		// Hop 240 (melody's default at 24 kHz) allows five octaves, but only
		// without early downsampling (see TestHopEarlyDownsampling).
		{"hop 240, five octaves", 24000, []Option{
			WithFrequencyRange(pitch.MIDIToFrequency(35.5, 440), pitch.MIDIToFrequency(95.5, 440)),
			WithCQT(cqt.WithHopLength(240), cqt.WithEarlyDownsampling(false)),
		}, 180},
		{"24 bins per octave", 22050, []Option{WithBinsPerOctave(24)}, 169},
		{"cqt override to 24 bins per octave", 22050, []Option{
			WithCQT(cqt.WithBinsPerOctave(24), cqt.WithBins(120)),
		}, 120},
		{"wider transform", 22050, []Option{WithCQT(cqt.WithBins(260))}, 252},
		{"single bin", 22050, []Option{WithFrequencyRange(439, 441), WithBinsPerOctave(12)}, 1},
	}

	for _, tt := range tests {
		c, err := New(tt.sampleRate, tt.opts...)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)

			continue
		}

		if got := len(c.Frequencies()); got != tt.bins {
			t.Errorf("%s: %d folded bins, want %d", tt.name, got, tt.bins)
		}
	}
}

// TestHopEarlyDownsampling pins a dsp/cqt behaviour taken from nnAudio:
// early downsampling limits its factor by nextpow2(hop) rather than by the
// powers of two the hop contains. For hop 240 = 16·15 at 24 kHz it decimates
// by 2, leaving an effective hop of 120, which five octaves (needing a
// multiple of 16) reject.
func TestHopEarlyDownsampling(t *testing.T) {
	t.Parallel()

	band := WithFrequencyRange(pitch.MIDIToFrequency(35.5, 440), pitch.MIDIToFrequency(95.5, 440))

	_, err := New(24000, band, WithCQT(cqt.WithHopLength(240)))
	if !errors.Is(err, cqt.ErrHop) {
		t.Errorf("hop 240 with early downsampling: %v, want cqt.ErrHop", err)
	}

	c, err := New(24000, band, WithCQT(cqt.WithHopLength(240), cqt.WithEarlyDownsampling(false)))
	if err != nil {
		t.Fatal(err)
	}

	if c.FrameRate() != 100 {
		t.Errorf("frame rate %g, want 100", c.FrameRate())
	}
}

func TestComputeMatchesProcess(t *testing.T) {
	t.Parallel()

	const sr = 22050

	x := synth(sr, 2, DefaultReferenceHz, event{0, 1, []float64{60, 64, 67}}, event{1, 2, []float64{62, 65, 69}})

	got, rate, err := Compute(x, sr, WithNormalization(NormL2))
	if err != nil {
		t.Fatal(err)
	}

	c, err := New(sr, WithNormalization(NormL2))
	if err != nil {
		t.Fatal(err)
	}

	// Process twice: the second call reuses the scratch buffer.
	for range 2 {
		want, err := c.Process(x)
		if err != nil {
			t.Fatal(err)
		}

		if rate != c.FrameRate() {
			t.Fatalf("frame rate %g, want %g", rate, c.FrameRate())
		}

		for pc := range want {
			if !slices.Equal(got[pc], want[pc]) {
				t.Fatalf("class %d differs", pc)
			}

			if len(got[pc]) != c.FrameCount(len(x)) {
				t.Fatalf("class %d has %d frames, want %d", pc, len(got[pc]), c.FrameCount(len(x)))
			}
		}
	}

	// Complex output requested through WithCQT is overridden.
	alt, _, err := Compute(x, sr, WithNormalization(NormL2), WithCQT(cqt.WithOutput(cqt.OutputComplex)))
	if err != nil {
		t.Fatal(err)
	}

	for pc := range alt {
		if !slices.Equal(alt[pc], got[pc]) {
			t.Fatalf("complex output override: class %d differs", pc)
		}
	}
}

func TestProcessErrors(t *testing.T) {
	t.Parallel()

	c, err := New(22050)
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.Process(nil)
	if !errors.Is(err, ErrEmptyInput) {
		t.Errorf("empty input: %v", err)
	}

	_, err = c.Process([]float64{1})
	if !errors.Is(err, cqt.ErrSignalTooShort) {
		t.Errorf("one sample: %v", err)
	}

	_, _, err = Compute(nil, 22050)
	if !errors.Is(err, ErrEmptyInput) {
		t.Errorf("Compute empty input: %v", err)
	}
}

func TestClone(t *testing.T) {
	t.Parallel()

	const sr = 22050

	c, err := New(sr)
	if err != nil {
		t.Fatal(err)
	}

	xs := [][]float64{
		synth(sr, 1, DefaultReferenceHz, event{0, 1, []float64{60, 64, 67}}),
		synth(sr, 1.5, DefaultReferenceHz, event{0, 1.5, []float64{57, 61, 64}}),
	}

	want := make([][12][]float64, len(xs))
	for i, x := range xs {
		want[i], err = c.Process(x)
		if err != nil {
			t.Fatal(err)
		}
	}

	d := c.Clone()

	var wg sync.WaitGroup

	got := make([][12][]float64, len(xs))
	errs := make([]error, len(xs))

	for i, p := range []*Chromagram{c, d} {
		wg.Add(1)

		go func() {
			defer wg.Done()

			got[i], errs[i] = p.Process(xs[i])
		}()
	}

	wg.Wait()

	for i := range xs {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}

		for pc := range got[i] {
			if !slices.Equal(got[i][pc], want[i][pc]) {
				t.Fatalf("signal %d class %d differs after Clone", i, pc)
			}
		}
	}
}

func TestNormalizationString(t *testing.T) {
	t.Parallel()

	for n, want := range map[Normalization]string{
		NormMax: "max", NormL1: "l1", NormL2: "l2", NormNone: "none", 0: "Normalization(0)", 7: "Normalization(7)",
	} {
		if got := n.String(); got != want {
			t.Errorf("Normalization(%d).String() = %q, want %q", int(n), got, want)
		}
	}
}
