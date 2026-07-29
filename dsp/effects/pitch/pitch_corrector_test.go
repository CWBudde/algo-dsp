package pitch

import (
	"math"
	"strings"
	"testing"

	"github.com/cwbudde/algo-dsp/internal/testutil"
)

// correctorTestSeconds is long enough for the tracker to settle and for the
// retune glide to reach its target at the default speed.
const correctorTestSeconds = 2.0

func newTestCorrector(t *testing.T, opts ...PitchCorrectorOption) *PitchCorrector {
	t.Helper()

	c, err := NewPitchCorrector(yinTestSampleRate, opts...)
	if err != nil {
		t.Fatalf("NewPitchCorrector: %v", err)
	}

	return c
}

// correctorInput builds a harmonically rich test tone long enough to settle.
func correctorInput(freqHz float64) []float64 {
	return yinSawtooth(freqHz, yinTestSampleRate, 0.4, int(correctorTestSeconds*yinTestSampleRate))
}

// outputFrequency measures the pitch of the settled tail of an output buffer
// with an autocorrelation estimator that is deliberately independent of the
// YIN detector under test, so a detector bug cannot hide itself.
func outputFrequency(t *testing.T, out []float64, minHz, maxHz float64) float64 {
	t.Helper()

	const tail = 16384

	segment := out
	if len(segment) > tail {
		segment = segment[len(segment)-tail:]
	}

	return estimateFrequencyAutoCorrelation(segment, yinTestSampleRate, minHz, maxHz)
}

func TestNewPitchCorrectorDefaults(t *testing.T) {
	c := newTestCorrector(t)

	if got := c.Amount(); got != defaultCorrectionAmount {
		t.Errorf("Amount() = %g, want %g", got, defaultCorrectionAmount)
	}

	if got := c.MaxCorrectionSemitones(); got != defaultMaxCorrectionSemitones {
		t.Errorf("MaxCorrectionSemitones() = %g, want %g", got, defaultMaxCorrectionSemitones)
	}

	if got := c.ReferenceHz(); got != DefaultReferenceHz {
		t.Errorf("ReferenceHz() = %g, want %g", got, DefaultReferenceHz)
	}

	if got := c.BlockSize(); got != defaultCorrectionBlockSize {
		t.Errorf("BlockSize() = %d, want %d", got, defaultCorrectionBlockSize)
	}

	if got := c.Mode(); got != CorrectionModeScale {
		t.Errorf("Mode() = %v, want CorrectionModeScale", got)
	}

	if got := len(c.Scale().Degrees()); got != pitchClassCount {
		t.Errorf("default scale has %d degrees, want the chromatic %d", got, pitchClassCount)
	}

	if _, ok := c.Shifter().(*SpectralPitchShifter); !ok {
		t.Errorf("default shifter is %T, want *SpectralPitchShifter", c.Shifter())
	}

	if got, want := c.Latency(), c.Tracker().Latency()+c.BlockSize()+c.CrossfadeLen(); got != want {
		t.Errorf("Latency() = %d, want %d", got, want)
	}
}

func TestNewPitchCorrectorValidation(t *testing.T) {
	mismatched, err := NewSpectralPitchShifter(44100)
	if err != nil {
		t.Fatalf("NewSpectralPitchShifter: %v", err)
	}

	tests := []struct {
		name       string
		sampleRate float64
		opts       []PitchCorrectorOption
	}{
		{"zero sample rate", 0, nil},
		{"NaN sample rate", math.NaN(), nil},
		{"amount below zero", yinTestSampleRate, []PitchCorrectorOption{WithCorrectionAmount(-0.1)}},
		{"amount above one", yinTestSampleRate, []PitchCorrectorOption{WithCorrectionAmount(1.1)}},
		{"amount NaN", yinTestSampleRate, []PitchCorrectorOption{WithCorrectionAmount(math.NaN())}},
		{"max correction zero", yinTestSampleRate,
			[]PitchCorrectorOption{WithMaxCorrectionSemitones(0)}},
		{"max correction too large", yinTestSampleRate,
			[]PitchCorrectorOption{WithMaxCorrectionSemitones(25)}},
		{"negative speed", yinTestSampleRate, []PitchCorrectorOption{WithCorrectionSpeedMs(-1)}},
		{"block size zero", yinTestSampleRate, []PitchCorrectorOption{WithCorrectionBlockSize(0)}},
		{"block size too small", yinTestSampleRate, []PitchCorrectorOption{WithCorrectionBlockSize(8)}},
		{"negative crossfade", yinTestSampleRate,
			[]PitchCorrectorOption{WithCorrectionCrossfadeMs(-1)}},
		{"reference too low", yinTestSampleRate,
			[]PitchCorrectorOption{WithCorrectionReferenceHz(300)}},
		{"reference too high", yinTestSampleRate,
			[]PitchCorrectorOption{WithCorrectionReferenceHz(500)}},
		{"confidence above one", yinTestSampleRate,
			[]PitchCorrectorOption{WithCorrectionConfidence(1.5)}},
		{"target zero", yinTestSampleRate, []PitchCorrectorOption{WithCorrectionTargetHz(0)}},
		{"target negative", yinTestSampleRate, []PitchCorrectorOption{WithCorrectionTargetHz(-440)}},
		{"zero scale", yinTestSampleRate, []PitchCorrectorOption{WithCorrectionScale(Scale{})}},
		{"nil shifter", yinTestSampleRate, []PitchCorrectorOption{WithCorrectionShifter(nil)}},
		{"nil tracker", yinTestSampleRate, []PitchCorrectorOption{WithCorrectionTracker(nil)}},
		{"shifter rate mismatch", yinTestSampleRate,
			[]PitchCorrectorOption{WithCorrectionShifter(mismatched)}},
		{"nil option", yinTestSampleRate, []PitchCorrectorOption{nil}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewPitchCorrector(tt.sampleRate, tt.opts...)
			if err == nil {
				t.Fatalf("NewPitchCorrector = %v, want an error", c)
			}

			if !strings.Contains(err.Error(), "pitch corrector") {
				t.Errorf("error %q does not name the processor", err)
			}
		})
	}
}

func TestPitchCorrectorSnapsSharpNoteToTarget(t *testing.T) {
	// 452 Hz is about 47 cents sharp of A4.
	const inputHz = 452.0

	c := newTestCorrector(t, WithCorrectionSpeedMs(0))
	out := c.Process(correctorInput(inputHz))

	wantSemitones := RatioToSemitones(DefaultReferenceHz / inputHz)
	if got := c.AppliedSemitones(); math.Abs(got-wantSemitones) > 0.05 {
		t.Errorf("AppliedSemitones() = %.4f, want %.4f", got, wantSemitones)
	}

	if got := outputFrequency(t, out, 300, 700); math.Abs(centsError(got, 440)) > 20 {
		t.Errorf("output f0 = %.3f Hz (%.2f cents off A4)", got, centsError(got, 440))
	}
}

func TestPitchCorrectorSnapsFlatNoteToTarget(t *testing.T) {
	// 428 Hz is about 48 cents flat of A4.
	const inputHz = 428.0

	c := newTestCorrector(t, WithCorrectionSpeedMs(0))
	out := c.Process(correctorInput(inputHz))

	wantSemitones := RatioToSemitones(DefaultReferenceHz / inputHz)
	if got := c.AppliedSemitones(); math.Abs(got-wantSemitones) > 0.05 {
		t.Errorf("AppliedSemitones() = %.4f, want %.4f", got, wantSemitones)
	}

	if got := outputFrequency(t, out, 300, 700); math.Abs(centsError(got, 440)) > 20 {
		t.Errorf("output f0 = %.3f Hz (%.2f cents off A4)", got, centsError(got, 440))
	}
}

func TestPitchCorrectorAmountZeroIsIdentity(t *testing.T) {
	c := newTestCorrector(t, WithCorrectionAmount(0))

	in := correctorInput(452)
	out := c.Process(in)

	if c.AppliedSemitones() != 0 {
		t.Errorf("AppliedSemitones() = %g, want exactly 0", c.AppliedSemitones())
	}

	// With nothing to correct the shifter is bypassed and every seam crossfades
	// a segment against itself, so the result must be bit-exact.
	for i := range in {
		if out[i] != in[i] {
			t.Fatalf("sample %d = %v, want %v (bypass must be bit-exact)", i, out[i], in[i])
		}
	}
}

func TestPitchCorrectorPartialAmount(t *testing.T) {
	// A note 30 cents sharp of A4, corrected halfway, must end up 15 cents
	// sharp: the interpolation happens in the semitone domain.
	inputHz := MIDIToFrequency(69+0.30, DefaultReferenceHz)

	c := newTestCorrector(t, WithCorrectionAmount(0.5), WithCorrectionSpeedMs(0))
	c.Process(correctorInput(inputHz))

	if got := c.AppliedSemitones(); math.Abs(got-(-0.15)) > 0.03 {
		t.Errorf("AppliedSemitones() = %.4f, want %.4f", got, -0.15)
	}
}

func TestPitchCorrectorMaxCorrectionClamp(t *testing.T) {
	// Two semitones sharp with a half-semitone ceiling: the correction is
	// clamped, not abandoned.
	inputHz := MIDIToFrequency(69+2, DefaultReferenceHz)

	c := newTestCorrector(t,
		WithCorrectionTargetHz(DefaultReferenceHz),
		WithMaxCorrectionSemitones(0.5),
		WithCorrectionSpeedMs(0))
	c.Process(correctorInput(inputHz))

	if got := c.AppliedSemitones(); math.Abs(got-(-0.5)) > 1e-9 {
		t.Errorf("AppliedSemitones() = %.9f, want exactly -0.5", got)
	}
}

func TestPitchCorrectorHoldsOnUnvoiced(t *testing.T) {
	c := newTestCorrector(t, WithCorrectionSpeedMs(0))
	c.Process(correctorInput(452))

	held := c.AppliedSemitones()
	if held == 0 {
		t.Fatalf("no correction was applied to the voiced tone")
	}

	// A block of noise is unvoiced; the held target must survive it.
	c.Process(testutil.DeterministicNoise(11, 0.4, c.BlockSize()))

	if c.Voiced() {
		t.Errorf("noise block reported voiced")
	}

	if got := c.AppliedSemitones(); math.Abs(got-held) > 1e-9 {
		t.Errorf("AppliedSemitones() = %.9f after an unvoiced block, want the held %.9f", got, held)
	}

	if got := c.DetectedFrequency(); got != 0 {
		t.Errorf("DetectedFrequency() = %g on an unvoiced block, want 0", got)
	}
}

func TestPitchCorrectorConfidenceGate(t *testing.T) {
	// A confidence floor just below 1 rejects everything short of a perfectly
	// periodic frame, so a noisy tone is never corrected.
	c := newTestCorrector(t, WithCorrectionConfidence(0.999), WithCorrectionSpeedMs(0))

	noisy := yinMixAtSNR(correctorInput(452), 6, 5)
	c.Process(noisy)

	if c.Voiced() {
		t.Errorf("correction engaged despite the confidence gate")
	}

	if got := c.AppliedSemitones(); got != 0 {
		t.Errorf("AppliedSemitones() = %g, want 0 while gated", got)
	}
}

func TestPitchCorrectorScaleSnapping(t *testing.T) {
	tests := []struct {
		name     string
		scale    Scale
		inputHz  float64
		wantMIDI float64
	}{
		// Frequencies are chosen off the exact midpoints between degrees, so a
		// hair of detection error cannot flip the expected note. Tie-breaking
		// itself is pinned by the Scale tests.
		//
		// 460 Hz sits between A4 and A#4, and A#4 is not in C major, so it is
		// pulled back down to A4.
		{"just above A4 under C major", ScaleMajor(PitchClassC), 460, 69},
		// 311.13 Hz is exactly D#4, a degree of C minor pentatonic, so it stays.
		{"D#4 under C minor pentatonic", ScaleMinorPentatonic(PitchClassC), 311.13, 63},
		// 320 Hz lies between D#4 and E4; E4 is not in the scale, so it snaps
		// back to D#4 rather than up to the next degree, F4.
		{"between D#4 and E4 under C minor pentatonic", ScaleMinorPentatonic(PitchClassC), 320, 63},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestCorrector(t, WithCorrectionScale(tt.scale), WithCorrectionSpeedMs(0))
			c.Process(correctorInput(tt.inputHz))

			want := MIDIToFrequency(tt.wantMIDI, DefaultReferenceHz)
			if got := c.TargetFrequency(); math.Abs(centsError(got, want)) > 1 {
				t.Errorf("TargetFrequency() = %.4f Hz, want %.4f Hz", got, want)
			}
		})
	}
}

func TestPitchCorrectorFixedTargetMode(t *testing.T) {
	const inputHz = 300.0

	c := newTestCorrector(t,
		WithCorrectionTargetHz(220),
		WithMaxCorrectionSemitones(12),
		WithCorrectionSpeedMs(0))
	c.Process(correctorInput(inputHz))

	if got := c.Mode(); got != CorrectionModeFixed {
		t.Errorf("Mode() = %v, want CorrectionModeFixed", got)
	}

	want := RatioToSemitones(220.0 / inputHz)
	if got := c.AppliedSemitones(); math.Abs(got-want) > 0.05 {
		t.Errorf("AppliedSemitones() = %.4f, want %.4f", got, want)
	}

	if got := c.TargetFrequency(); math.Abs(centsError(got, 220)) > 1 {
		t.Errorf("TargetFrequency() = %.4f Hz, want 220 Hz", got)
	}
}

func TestPitchCorrectorSpeedGlide(t *testing.T) {
	const inputHz = 452.0

	settled := newTestCorrector(t, WithCorrectionSpeedMs(0))
	settled.Process(correctorInput(inputHz))

	target := settled.AppliedSemitones()
	if target == 0 {
		t.Fatalf("reference corrector applied no shift")
	}

	slow := newTestCorrector(t, WithCorrectionSpeedMs(200))
	input := correctorInput(inputHz)

	// One block past the point where the tracker first reports a pitch, the
	// glide must have started but not finished.
	primed := slow.Tracker().Latency() + 2*slow.BlockSize()
	slow.Process(input[:primed])

	partial := slow.AppliedSemitones()
	if partial >= 0 || partial <= target {
		t.Errorf("after priming, applied = %.4f, want strictly between 0 and %.4f", partial, target)
	}

	// Given the full two seconds it converges on the same shift.
	slow.Reset()
	slow.Process(input)

	if got := slow.AppliedSemitones(); math.Abs(got-target) > 0.05*math.Abs(target) {
		t.Errorf("after 2 s the glide reached %.4f, want within 5%% of %.4f", got, target)
	}
}

func TestPitchCorrectorOutputLengthAndFinite(t *testing.T) {
	for _, n := range []int{1, 100, 2048, 5000} {
		c := newTestCorrector(t)
		in := yinSawtooth(452, yinTestSampleRate, 0.4, n)

		out := c.Process(in)
		if len(out) != n {
			t.Fatalf("Process on %d samples returned %d", n, len(out))
		}

		testutil.RequireFinite(t, out)
	}
}

func TestPitchCorrectorProcessInPlaceMatchesProcess(t *testing.T) {
	in := correctorInput(452)

	a := newTestCorrector(t)
	want := a.Process(in)

	b := newTestCorrector(t)

	got := make([]float64, len(in))
	copy(got, in)
	b.ProcessInPlace(got)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sample %d: in-place %v, one-shot %v", i, got[i], want[i])
		}
	}
}

func TestPitchCorrectorBlockSizeInvariance(t *testing.T) {
	// Feeding the same signal in different call sizes changes where the seams
	// fall, so the samples differ; the corrected pitch must not.
	in := correctorInput(452)

	whole := newTestCorrector(t, WithCorrectionSpeedMs(0))
	wantHz := outputFrequency(t, whole.Process(in), 300, 700)

	chunked := newTestCorrector(t, WithCorrectionSpeedMs(0))

	out := make([]float64, 0, len(in))
	for start := 0; start < len(in); start += 4096 {
		end := min(start+4096, len(in))
		out = append(out, chunked.Process(in[start:end])...)
	}

	gotHz := outputFrequency(t, out, 300, 700)
	if cents := math.Abs(centsError(gotHz, wantHz)); cents > 15 {
		t.Errorf("chunked output f0 = %.3f Hz vs %.3f Hz whole (%.2f cents apart)",
			gotHz, wantHz, cents)
	}
}

func TestPitchCorrectorResetDeterministic(t *testing.T) {
	c := newTestCorrector(t)
	in := correctorInput(452)

	first := c.Process(in)

	c.Process(testutil.DeterministicNoise(3, 0.5, 8192))
	c.Reset()

	if c.AppliedSemitones() != 0 || c.Voiced() {
		t.Errorf("Reset left state behind: applied %g, voiced %v", c.AppliedSemitones(), c.Voiced())
	}

	second := c.Process(in)
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("sample %d differs after Reset: %v vs %v", i, second[i], first[i])
		}
	}
}

func TestPitchCorrectorEmptyInput(t *testing.T) {
	c := newTestCorrector(t)

	if got := c.Process(nil); got != nil {
		t.Errorf("Process(nil) = %v, want nil", got)
	}

	if got := c.Process([]float64{}); got != nil {
		t.Errorf("Process(empty) = %v, want nil", got)
	}

	c.ProcessInPlace(nil)
}

func TestPitchCorrectorWithWSOLAShifter(t *testing.T) {
	// The time-domain shifter's music preset needs a much longer block than the
	// default; its sequence is shortened and the block raised to suit. The
	// tolerance is looser than the spectral path, which documents the gap.
	shifter, err := NewPitchShifter(yinTestSampleRate)
	if err != nil {
		t.Fatalf("NewPitchShifter: %v", err)
	}

	if err := shifter.SetSequence(30); err != nil {
		t.Fatalf("SetSequence: %v", err)
	}

	c := newTestCorrector(t,
		WithCorrectionShifter(shifter),
		WithCorrectionBlockSize(8192),
		WithCorrectionSpeedMs(0))

	out := c.Process(correctorInput(452))

	testutil.RequireFinite(t, out)

	if got := outputFrequency(t, out, 300, 700); math.Abs(centsError(got, 440)) > 30 {
		t.Errorf("output f0 = %.3f Hz (%.2f cents off A4)", got, centsError(got, 440))
	}
}

func TestPitchCorrectorIsNotAPitchProcessor(t *testing.T) {
	// The ratio is derived from detection, so exposing the PitchProcessor
	// setters would let a caller believe they had changed something.
	var c any = newTestCorrector(t)

	if _, ok := c.(PitchProcessor); ok {
		t.Errorf("PitchCorrector satisfies PitchProcessor, which it must not")
	}
}

func TestPitchCorrectorSetSampleRatePropagates(t *testing.T) {
	c := newTestCorrector(t)

	if err := c.SetSampleRate(44100); err != nil {
		t.Fatalf("SetSampleRate: %v", err)
	}

	if got := c.SampleRate(); got != 44100 {
		t.Errorf("SampleRate() = %g, want 44100", got)
	}

	if got := c.Tracker().SampleRate(); got != 44100 {
		t.Errorf("tracker SampleRate() = %g, want 44100", got)
	}

	if got := c.Tracker().Detector().SampleRate(); got != 44100 {
		t.Errorf("detector SampleRate() = %g, want 44100", got)
	}

	if got := c.Shifter().SampleRate(); got != 44100 {
		t.Errorf("shifter SampleRate() = %g, want 44100", got)
	}

	// The corrector must still work at the new rate.
	out := c.Process(yinSawtooth(452, 44100, 0.4, 44100))
	testutil.RequireFinite(t, out)
}

func TestPitchCorrectorSetters(t *testing.T) {
	c := newTestCorrector(t)

	if err := c.SetCorrectionAmount(0.25); err != nil {
		t.Fatalf("SetCorrectionAmount: %v", err)
	}

	if got := c.Amount(); got != 0.25 {
		t.Errorf("Amount() = %g, want 0.25", got)
	}

	if err := c.SetTargetHz(330); err != nil {
		t.Fatalf("SetTargetHz: %v", err)
	}

	if c.Mode() != CorrectionModeFixed || c.TargetHz() != 330 {
		t.Errorf("SetTargetHz left mode %v target %g", c.Mode(), c.TargetHz())
	}

	if err := c.SetScale(ScaleMajor(PitchClassD)); err != nil {
		t.Fatalf("SetScale: %v", err)
	}

	if c.Mode() != CorrectionModeScale || c.Scale().Root() != PitchClassD {
		t.Errorf("SetScale left mode %v root %v", c.Mode(), c.Scale().Root())
	}

	if err := c.SetMaxCorrectionSemitones(3); err != nil {
		t.Fatalf("SetMaxCorrectionSemitones: %v", err)
	}

	if got := c.MaxCorrectionSemitones(); got != 3 {
		t.Errorf("MaxCorrectionSemitones() = %g, want 3", got)
	}

	if err := c.SetCorrectionSpeedMs(50); err != nil {
		t.Fatalf("SetCorrectionSpeedMs: %v", err)
	}

	if got := c.CorrectionSpeedMs(); got != 50 {
		t.Errorf("CorrectionSpeedMs() = %g, want 50", got)
	}

	if err := c.SetReferenceHz(432); err != nil {
		t.Fatalf("SetReferenceHz: %v", err)
	}

	if got := c.ReferenceHz(); got != 432 {
		t.Errorf("ReferenceHz() = %g, want 432", got)
	}

	if err := c.SetMinConfidence(0.8); err != nil {
		t.Fatalf("SetMinConfidence: %v", err)
	}

	if got := c.MinConfidence(); got != 0.8 {
		t.Errorf("MinConfidence() = %g, want 0.8", got)
	}
}

func TestPitchCorrectorSetterValidation(t *testing.T) {
	c := newTestCorrector(t)

	tests := []struct {
		name string
		call func() error
	}{
		{"amount", func() error { return c.SetCorrectionAmount(2) }},
		{"scale", func() error { return c.SetScale(Scale{}) }},
		{"target", func() error { return c.SetTargetHz(0) }},
		{"reference", func() error { return c.SetReferenceHz(200) }},
		{"max correction", func() error { return c.SetMaxCorrectionSemitones(100) }},
		{"speed", func() error { return c.SetCorrectionSpeedMs(-1) }},
		{"confidence", func() error { return c.SetMinConfidence(-0.1) }},
		{"sample rate", func() error { return c.SetSampleRate(0) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err == nil {
				t.Fatalf("expected an error")
			}
		})
	}
}

func TestPitchCorrectorReferenceHzShiftsTheGrid(t *testing.T) {
	// At A = 432 Hz the note grid moves down, so a 432 Hz input is already in
	// tune and needs no correction, while at A = 440 it would be pulled up.
	const inputHz = 432.0

	detuned := newTestCorrector(t, WithCorrectionReferenceHz(432), WithCorrectionSpeedMs(0))
	detuned.Process(correctorInput(inputHz))

	if got := math.Abs(detuned.AppliedSemitones()); got > 0.05 {
		t.Errorf("at A=432 a %g Hz note was shifted by %.4f semitones, want none", inputHz, got)
	}

	standard := newTestCorrector(t, WithCorrectionSpeedMs(0))
	standard.Process(correctorInput(inputHz))

	if got := standard.AppliedSemitones(); got <= 0.1 {
		t.Errorf("at A=440 a %g Hz note was shifted by %.4f semitones, want an upward pull",
			inputHz, got)
	}
}
