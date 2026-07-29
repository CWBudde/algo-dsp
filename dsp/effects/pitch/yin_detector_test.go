package pitch

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/cwbudde/algo-dsp/internal/testutil"
)

const yinTestSampleRate = 48000.0

// yinSine generates a sine with an explicit starting phase.
func yinSine(freqHz, sampleRate, amplitude, phase float64, n int) []float64 {
	out := make([]float64, n)

	step := 2 * math.Pi * freqHz / sampleRate
	for i := range out {
		out[i] = amplitude * math.Sin(step*float64(i)+phase)
	}

	return out
}

// yinHarmonics sums harmonics of freqHz with the given relative amplitudes.
// Harmonics at or above Nyquist are skipped, which keeps every test signal
// free of aliasing; a naive ramp or step waveform would alias badly enough that
// an octave test would be measuring the aliasing rather than the detector.
func yinHarmonics(freqHz, sampleRate float64, harmonics []int, amps []float64, n int) []float64 {
	out := make([]float64, n)
	nyquist := sampleRate / 2

	for k, h := range harmonics {
		hz := freqHz * float64(h)
		if hz >= nyquist {
			continue
		}

		step := 2 * math.Pi * hz / sampleRate
		for i := range out {
			out[i] += amps[k] * math.Sin(step*float64(i))
		}
	}

	return out
}

// yinSawtooth builds a band-limited sawtooth: harmonics 1..K with 1/k weights.
func yinSawtooth(freqHz, sampleRate, amplitude float64, n int) []float64 {
	var harmonics []int

	var amps []float64

	for h := 1; float64(h)*freqHz < sampleRate/2; h++ {
		harmonics = append(harmonics, h)
		amps = append(amps, amplitude/float64(h))
	}

	return yinHarmonics(freqHz, sampleRate, harmonics, amps, n)
}

// yinSquare builds a band-limited square wave: odd harmonics with 1/k weights.
func yinSquare(freqHz, sampleRate, amplitude float64, n int) []float64 {
	var harmonics []int

	var amps []float64

	for h := 1; float64(h)*freqHz < sampleRate/2; h += 2 {
		harmonics = append(harmonics, h)
		amps = append(amps, amplitude/float64(h))
	}

	return yinHarmonics(freqHz, sampleRate, harmonics, amps, n)
}

// yinMixAtSNR adds deterministic white noise to sig at the requested SNR.
func yinMixAtSNR(sig []float64, snrDB float64, seed int64) []float64 {
	noise := testutil.DeterministicNoise(seed, 1.0, len(sig))

	sigRMS := frameRMS(sig)
	noiseRMS := frameRMS(noise)

	if noiseRMS < yinTiny {
		return sig
	}

	scale := sigRMS / noiseRMS * math.Pow(10, -snrDB/20)

	out := make([]float64, len(sig))
	for i := range out {
		out[i] = sig[i] + scale*noise[i]
	}

	return out
}

// centsError returns the signed error of got relative to want, in cents.
func centsError(gotHz, wantHz float64) float64 {
	if gotHz <= 0 || wantHz <= 0 {
		return math.Inf(1)
	}

	return CentsBetween(wantHz, gotHz)
}

func newTestYIN(t *testing.T, opts ...YINDetectorOption) *YINDetector {
	t.Helper()

	d, err := NewYINDetector(yinTestSampleRate, opts...)
	if err != nil {
		t.Fatalf("NewYINDetector: %v", err)
	}

	return d
}

func detectOrFail(t *testing.T, d *YINDetector, frame []float64) PitchEstimate {
	t.Helper()

	est, err := d.Detect(frame)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	return est
}

func TestNewYINDetectorDefaults(t *testing.T) {
	d := newTestYIN(t)

	if got := d.Threshold(); got != defaultYINThreshold {
		t.Errorf("Threshold() = %g, want %g", got, defaultYINThreshold)
	}

	if got := d.MinFrequency(); got != defaultYINMinFrequency {
		t.Errorf("MinFrequency() = %g, want %g", got, defaultYINMinFrequency)
	}

	if got := d.MaxFrequency(); got != defaultYINMaxFrequency {
		t.Errorf("MaxFrequency() = %g, want %g", got, defaultYINMaxFrequency)
	}

	if got, want := d.MaxTau(), int(yinTestSampleRate/defaultYINMinFrequency); got != want {
		t.Errorf("MaxTau() = %d, want %d", got, want)
	}

	if got := d.MinTau(); got < minYINTau {
		t.Errorf("MinTau() = %d, want at least %d", got, minYINTau)
	}

	if got, want := d.FrameSize(), yinFrameSizePeriods*d.MaxTau(); got != want {
		t.Errorf("FrameSize() = %d, want %d", got, want)
	}

	if !d.ParabolicInterpolation() {
		t.Errorf("ParabolicInterpolation() = false, want true")
	}
}

func TestNewYINDetectorValidation(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate float64
		opts       []YINDetectorOption
	}{
		{"zero sample rate", 0, nil},
		{"negative sample rate", -48000, nil},
		{"NaN sample rate", math.NaN(), nil},
		{"Inf sample rate", math.Inf(1), nil},
		{"threshold zero", yinTestSampleRate, []YINDetectorOption{WithYINThreshold(0)}},
		{"threshold one", yinTestSampleRate, []YINDetectorOption{WithYINThreshold(1)}},
		{"threshold negative", yinTestSampleRate, []YINDetectorOption{WithYINThreshold(-0.1)}},
		{"threshold NaN", yinTestSampleRate, []YINDetectorOption{WithYINThreshold(math.NaN())}},
		{"min above max", yinTestSampleRate, []YINDetectorOption{WithYINFrequencyRange(800, 400)}},
		{"min equals max", yinTestSampleRate, []YINDetectorOption{WithYINFrequencyRange(400, 400)}},
		{"min zero", yinTestSampleRate, []YINDetectorOption{WithYINFrequencyRange(0, 400)}},
		{"min NaN", yinTestSampleRate, []YINDetectorOption{WithYINFrequencyRange(math.NaN(), 400)}},
		{"max at Nyquist", yinTestSampleRate, []YINDetectorOption{WithYINFrequencyRange(60, 24000)}},
		{"max above Nyquist", yinTestSampleRate, []YINDetectorOption{WithYINFrequencyRange(60, 30000)}},
		{"frame size zero", yinTestSampleRate, []YINDetectorOption{WithYINFrameSize(0)}},
		{"frame size negative", yinTestSampleRate, []YINDetectorOption{WithYINFrameSize(-1)}},
		{"frame size too short", yinTestSampleRate, []YINDetectorOption{WithYINFrameSize(64)}},
		{
			"silence threshold NaN", yinTestSampleRate,
			[]YINDetectorOption{WithYINSilenceThresholdDB(math.NaN())},
		},
		{"nil option", yinTestSampleRate, []YINDetectorOption{nil}},
		{
			"range too narrow for lags", yinTestSampleRate,
			[]YINDetectorOption{WithYINFrequencyRange(11000, 11500)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := NewYINDetector(tt.sampleRate, tt.opts...)
			if err == nil {
				t.Fatalf("NewYINDetector(%g) = %v, want an error", tt.sampleRate, d)
			}

			if !strings.Contains(err.Error(), "yin detector") {
				t.Errorf("error %q does not name the processor", err)
			}
		})
	}
}

func TestYINDetectorSineAccuracy(t *testing.T) {
	// A grid spanning the bass, vocal and upper-instrument ranges, including
	// frequencies whose period is deliberately non-integral at both rates.
	frequencies := []float64{
		55, 82.41, 110, 146.83, 220, 261.63, 330, 440, 523.25, 880, 1318.5,
	}
	rates := []float64{44100, 48000}

	for _, rate := range rates {
		for _, freq := range frequencies {
			t.Run(fmt.Sprintf("fs=%g/f0=%g", rate, freq), func(t *testing.T) {
				// The range is widened past the defaults so that the low end of
				// the grid, A1 at 55 Hz, is inside the search range rather than
				// being clamped to the 60 Hz default floor.
				d, err := NewYINDetector(rate, WithYINFrequencyRange(50, 2000))
				if err != nil {
					t.Fatalf("NewYINDetector: %v", err)
				}

				frame := yinSine(freq, rate, 0.5, 0, d.FrameSize())
				est := detectOrFail(t, d, frame)

				if !est.Voiced {
					t.Fatalf("unvoiced, aperiodicity %.4f", est.Aperiodicity)
				}

				// Measured worst case across this grid is 0.35 cents; the bound
				// is set to roughly twice that rather than to the observation.
				if cents := centsError(est.FrequencyHz, freq); math.Abs(cents) > 1 {
					t.Errorf("got %.4f Hz (%.3f cents off)", est.FrequencyHz, cents)
				}

				if est.Confidence < 0.9 {
					t.Errorf("confidence %.4f, want at least 0.9", est.Confidence)
				}
			})
		}
	}
}

func TestYINDetectorParabolicInterpolationSubSample(t *testing.T) {
	// 437.3 Hz at 48 kHz has a period of 109.7645... samples, so the whole-sample
	// estimate is necessarily off by a fraction of a sample and interpolation has
	// real work to do.
	const freq = 437.3

	on := newTestYIN(t)
	off := newTestYIN(t, WithYINParabolicInterpolation(false))

	frame := yinSine(freq, yinTestSampleRate, 0.5, 0, on.FrameSize())

	estOn := detectOrFail(t, on, frame)
	estOff := detectOrFail(t, off, frame)

	errOn := math.Abs(centsError(estOn.FrequencyHz, freq))
	errOff := math.Abs(centsError(estOff.FrequencyHz, freq))

	if errOn > 3 {
		t.Errorf("with interpolation: %.4f Hz (%.3f cents off), want within 3 cents",
			estOn.FrequencyHz, errOn)
	}

	// The point of the refinement: it must actually improve on the whole-sample
	// estimate, which also pins the sign of the parabolic shift.
	if errOn >= errOff {
		t.Errorf("interpolation did not help: on = %.3f cents, off = %.3f cents", errOn, errOff)
	}
}

func TestYINDetectorAmplitudeInvariance(t *testing.T) {
	const freq = 220.0

	d := newTestYIN(t)

	var reference float64

	for i, amp := range []float64{0.005, 0.05, 0.5, 1.0} {
		frame := yinSine(freq, yinTestSampleRate, amp, 0, d.FrameSize())

		est := detectOrFail(t, d, frame)
		if !est.Voiced {
			t.Fatalf("amplitude %g: unvoiced", amp)
		}

		if i == 0 {
			reference = est.FrequencyHz
			continue
		}

		if err := math.Abs(centsError(est.FrequencyHz, reference)); err > 0.5 {
			t.Errorf("amplitude %g: %.6f Hz differs from reference %.6f Hz by %.4f cents",
				amp, est.FrequencyHz, reference, err)
		}
	}
}

func TestYINDetectorPhaseInvariance(t *testing.T) {
	const freq = 330.0

	d := newTestYIN(t)

	var reference float64

	for i, phase := range []float64{0, math.Pi / 4, math.Pi / 2, math.Pi} {
		frame := yinSine(freq, yinTestSampleRate, 0.5, phase, d.FrameSize())

		est := detectOrFail(t, d, frame)
		if !est.Voiced {
			t.Fatalf("phase %g: unvoiced", phase)
		}

		if i == 0 {
			reference = est.FrequencyHz
			continue
		}

		if err := math.Abs(centsError(est.FrequencyHz, reference)); err > 0.5 {
			t.Errorf("phase %g: %.6f Hz differs from reference %.6f Hz by %.4f cents",
				phase, est.FrequencyHz, reference, err)
		}
	}
}

// assertNoOctaveError checks both that the estimate is close to the true
// fundamental and, explicitly, that it did not land on the octave above or
// below. The second half matters: a "within 5%" assertion alone would pass a
// detector that consistently reports 2*f0 for a different test frequency.
func assertNoOctaveError(t *testing.T, est PitchEstimate, wantHz float64) {
	t.Helper()

	if !est.Voiced {
		t.Fatalf("unvoiced, want %g Hz", wantHz)
	}

	if relativeError(est.FrequencyHz, wantHz) > 0.01 {
		t.Errorf("got %.4f Hz, want %g Hz (within 1%%)", est.FrequencyHz, wantHz)
	}

	if relativeError(est.FrequencyHz, wantHz/2) < 0.05 {
		t.Errorf("got %.4f Hz, an octave-down error from %g Hz", est.FrequencyHz, wantHz)
	}

	if relativeError(est.FrequencyHz, wantHz*2) < 0.05 {
		t.Errorf("got %.4f Hz, an octave-up error from %g Hz", est.FrequencyHz, wantHz)
	}
}

func relativeError(got, want float64) float64 {
	return math.Abs(got-want) / want
}

func TestYINDetectorSawtoothNoOctaveError(t *testing.T) {
	for _, freq := range []float64{100, 220, 440} {
		d := newTestYIN(t)
		frame := yinSawtooth(freq, yinTestSampleRate, 0.5, d.FrameSize())

		assertNoOctaveError(t, detectOrFail(t, d, frame), freq)
	}
}

func TestYINDetectorSquareNoOctaveError(t *testing.T) {
	for _, freq := range []float64{100, 220, 440} {
		d := newTestYIN(t)
		frame := yinSquare(freq, yinTestSampleRate, 0.5, d.FrameSize())

		assertNoOctaveError(t, detectOrFail(t, d, frame), freq)
	}
}

func TestYINDetectorMissingFundamental(t *testing.T) {
	// Harmonics 2 through 6 of 150 Hz with no energy at 150 Hz itself. YIN
	// estimates the period rather than picking a spectral peak, so it recovers
	// the fundamental that is not physically present.
	const freq = 150.0

	d := newTestYIN(t)
	frame := yinHarmonics(freq, yinTestSampleRate,
		[]int{2, 3, 4, 5, 6},
		[]float64{0.4, 0.3, 0.2, 0.15, 0.1},
		d.FrameSize())

	est := detectOrFail(t, d, frame)
	if !est.Voiced {
		t.Fatalf("unvoiced on a missing-fundamental signal")
	}

	if relativeError(est.FrequencyHz, freq) > 0.02 {
		t.Errorf("got %.4f Hz, want %g Hz within 2%%", est.FrequencyHz, freq)
	}
}

func TestYINDetectorOctaveErrorVsThreshold(t *testing.T) {
	// A signal whose second harmonic is stronger than its fundamental produces
	// a shallow dip at half the period, ahead of the true one. The default
	// threshold steps over that dip; a permissive threshold accepts it and
	// reports the octave above. This pins the default as a tested trade-off
	// rather than a magic number.
	const freq = 200.0

	strict := newTestYIN(t)
	frame := yinHarmonics(freq, yinTestSampleRate,
		[]int{1, 2, 3, 4},
		[]float64{0.2, 0.6, 0.3, 0.15},
		strict.FrameSize())

	est := detectOrFail(t, strict, frame)
	if !est.Voiced || relativeError(est.FrequencyHz, freq) > 0.02 {
		t.Errorf("default threshold %g: got %.4f Hz (voiced=%v), want %g Hz",
			defaultYINThreshold, est.FrequencyHz, est.Voiced, freq)
	}

	permissive := newTestYIN(t, WithYINThreshold(0.7))

	octave := detectOrFail(t, permissive, frame)
	if relativeError(octave.FrequencyHz, 2*freq) > 0.02 {
		t.Errorf("threshold 0.7: got %.4f Hz, expected the octave-up error at %g Hz "+
			"that the default avoids", octave.FrequencyHz, 2*freq)
	}
}

func TestYINDetectorNoiseRobustness(t *testing.T) {
	const freq = 220.0

	tests := []struct {
		name          string
		snrDB         float64
		maxRelError   float64
		allowUnvoiced bool
	}{
		// Tolerances are set to roughly twice the error measured on this grid.
		// A single 33 ms frame at 10 dB SNR is already marginal for YIN: the
		// normalized difference at the true period is pushed close to the
		// threshold, so noise can win a nearby lag. The tracker's median filter
		// is what recovers accuracy on real streaming material.
		{"20 dB", 20, 0.01, false},
		{"10 dB", 10, 0.09, false},
		{"3 dB", 3, 0.20, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, seed := range []int64{1, 2, 3} {
				d := newTestYIN(t)
				clean := yinSine(freq, yinTestSampleRate, 0.5, 0, d.FrameSize())
				frame := yinMixAtSNR(clean, tt.snrDB, seed)

				est := detectOrFail(t, d, frame)
				if !est.Voiced {
					if tt.allowUnvoiced {
						continue
					}

					t.Errorf("seed %d: unvoiced at %g dB SNR", seed, tt.snrDB)

					continue
				}

				if got := relativeError(est.FrequencyHz, freq); got > tt.maxRelError {
					t.Errorf("seed %d: got %.4f Hz, relative error %.4f exceeds %.4f",
						seed, est.FrequencyHz, got, tt.maxRelError)
				}
			}
		})
	}
}

func TestYINDetectorUnvoicedOnNoise(t *testing.T) {
	d := newTestYIN(t)

	const frames = 20

	voiced := 0

	for i := range frames {
		frame := testutil.DeterministicNoise(int64(i+1), 0.5, d.FrameSize())

		est := detectOrFail(t, d, frame)
		if est.Voiced {
			voiced++
		}
	}

	if voiced > frames/5 {
		t.Errorf("white noise reported voiced in %d of %d frames, want at most %d",
			voiced, frames, frames/5)
	}
}

func TestYINDetectorUnvoicedOnSilence(t *testing.T) {
	d := newTestYIN(t)

	est := detectOrFail(t, d, make([]float64, d.FrameSize()))

	if est.Voiced {
		t.Errorf("silence reported voiced")
	}

	if est.FrequencyHz != 0 || est.Tau != 0 {
		t.Errorf("silence gave frequency %g and tau %g, want 0 and 0", est.FrequencyHz, est.Tau)
	}

	for name, v := range map[string]float64{
		"FrequencyHz": est.FrequencyHz, "Tau": est.Tau,
		"Aperiodicity": est.Aperiodicity, "Confidence": est.Confidence, "RMS": est.RMS,
	} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Errorf("%s = %g, want finite", name, v)
		}
	}
}

func TestYINDetectorSilenceGate(t *testing.T) {
	d := newTestYIN(t)

	// A perfectly periodic tone well below the gate must still be rejected.
	frame := yinSine(440, yinTestSampleRate, 1e-5, 0, d.FrameSize())

	est := detectOrFail(t, d, frame)
	if est.Voiced {
		t.Errorf("tone at %g RMS passed the %g dBFS silence gate", est.RMS, d.SilenceThresholdDB())
	}

	// Lowering the gate lets the same frame through.
	if err := d.SetSilenceThresholdDB(-140); err != nil {
		t.Fatalf("SetSilenceThresholdDB: %v", err)
	}

	if est := detectOrFail(t, d, frame); !est.Voiced {
		t.Errorf("tone still gated after lowering the threshold to -140 dBFS")
	}
}

func TestYINDetectorRangeBounds(t *testing.T) {
	t.Run("below the lower bound", func(t *testing.T) {
		d := newTestYIN(t, WithYINFrequencyRange(80, 1600))

		frame := yinSine(45, yinTestSampleRate, 0.5, 0, d.FrameSize())

		est := detectOrFail(t, d, frame)
		if est.Voiced && est.FrequencyHz < 80 {
			t.Errorf("reported %.4f Hz, below the configured 80 Hz lower bound", est.FrequencyHz)
		}
	})

	t.Run("above the upper bound", func(t *testing.T) {
		d := newTestYIN(t, WithYINFrequencyRange(60, 1600))

		frame := yinSine(3000, yinTestSampleRate, 0.5, 0, d.FrameSize())

		est := detectOrFail(t, d, frame)
		if est.Voiced && est.FrequencyHz > 1600 {
			t.Errorf("reported %.4f Hz, above the configured 1600 Hz upper bound", est.FrequencyHz)
		}
	})
}

func TestYINDetectorRejectsShortFrame(t *testing.T) {
	d := newTestYIN(t)

	_, err := d.Detect(make([]float64, d.FrameSize()-1))
	if err == nil {
		t.Fatalf("Detect on a short frame returned no error")
	}

	if !strings.Contains(err.Error(), "yin detector") {
		t.Errorf("error %q does not name the processor", err)
	}
}

func TestYINDetectorAcceptsLongFrame(t *testing.T) {
	d := newTestYIN(t)

	long := yinSine(440, yinTestSampleRate, 0.5, 0, d.FrameSize()*2)
	short := long[:d.FrameSize()]

	estLong := detectOrFail(t, d, long)
	estShort := detectOrFail(t, d, short)

	if estLong != estShort {
		t.Errorf("extra samples changed the estimate: %+v vs %+v", estLong, estShort)
	}
}

func TestYINDetectorDeterministic(t *testing.T) {
	a := newTestYIN(t)
	b := newTestYIN(t)

	frame := yinSawtooth(196, yinTestSampleRate, 0.4, a.FrameSize())

	if got, want := detectOrFail(t, a, frame), detectOrFail(t, b, frame); got != want {
		t.Errorf("identical detectors disagreed: %+v vs %+v", got, want)
	}
}

func TestYINDetectorResetRestoresState(t *testing.T) {
	d := newTestYIN(t)

	frame := yinSine(440, yinTestSampleRate, 0.5, 0, d.FrameSize())
	before := detectOrFail(t, d, frame)

	// Run an unrelated frame through, then reset and repeat the first frame.
	_ = detectOrFail(t, d, yinSawtooth(97, yinTestSampleRate, 0.6, d.FrameSize()))
	d.Reset()

	if after := detectOrFail(t, d, frame); after != before {
		t.Errorf("after Reset: %+v, want %+v", after, before)
	}
}

func TestYINDetectorSetters(t *testing.T) {
	d := newTestYIN(t)

	if err := d.SetThreshold(0.2); err != nil {
		t.Fatalf("SetThreshold: %v", err)
	}

	if got := d.Threshold(); got != 0.2 {
		t.Errorf("Threshold() = %g, want 0.2", got)
	}

	if err := d.SetFrequencyRange(100, 1000); err != nil {
		t.Fatalf("SetFrequencyRange: %v", err)
	}

	if got, want := d.MaxTau(), int(yinTestSampleRate/100); got != want {
		t.Errorf("MaxTau() = %d, want %d", got, want)
	}

	// The frame size follows the range automatically until it is pinned.
	if got, want := d.FrameSize(), yinFrameSizePeriods*d.MaxTau(); got != want {
		t.Errorf("FrameSize() = %d, want %d", got, want)
	}

	if err := d.SetFrameSize(4096); err != nil {
		t.Fatalf("SetFrameSize: %v", err)
	}

	if got := d.FrameSize(); got != 4096 {
		t.Errorf("FrameSize() = %d, want 4096", got)
	}

	if err := d.SetSampleRate(44100); err != nil {
		t.Fatalf("SetSampleRate: %v", err)
	}

	if got := d.SampleRate(); got != 44100 {
		t.Errorf("SampleRate() = %g, want 44100", got)
	}

	// Returning to the automatic frame size must track the range again.
	if err := d.SetFrameSize(0); err != nil {
		t.Fatalf("SetFrameSize(0): %v", err)
	}

	if got, want := d.FrameSize(), yinFrameSizePeriods*d.MaxTau(); got != want {
		t.Errorf("FrameSize() = %d, want %d after returning to automatic", got, want)
	}
}

func TestYINDetectorSetterErrorsLeaveDetectorUsable(t *testing.T) {
	d := newTestYIN(t)

	before := detectOrFail(t, d, yinSine(440, yinTestSampleRate, 0.5, 0, d.FrameSize()))

	tests := []struct {
		name string
		call func() error
	}{
		{"threshold", func() error { return d.SetThreshold(1.5) }},
		{"sample rate", func() error { return d.SetSampleRate(-1) }},
		{"frequency range", func() error { return d.SetFrequencyRange(2000, 1000) }},
		{"max above Nyquist", func() error { return d.SetFrequencyRange(60, 40000) }},
		{"frame size negative", func() error { return d.SetFrameSize(-5) }},
		{"frame size too short", func() error { return d.SetFrameSize(16) }},
		{"silence threshold", func() error { return d.SetSilenceThresholdDB(math.Inf(-1)) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err == nil {
				t.Fatalf("expected an error")
			}

			after := detectOrFail(t, d, yinSine(440, yinTestSampleRate, 0.5, 0, d.FrameSize()))
			if after != before {
				t.Errorf("a rejected setter changed behaviour: %+v, want %+v", after, before)
			}
		})
	}
}

func TestYINDetectorZeroAlloc(t *testing.T) {
	d := newTestYIN(t)
	frame := yinSawtooth(220, yinTestSampleRate, 0.5, d.FrameSize())

	allocs := testing.AllocsPerRun(20, func() {
		_, _ = d.Detect(frame)
	})

	if allocs != 0 {
		t.Errorf("Detect allocs = %g, want 0", allocs)
	}
}
