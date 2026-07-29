package pitch

import (
	"math"
	"strings"
	"testing"

	"github.com/cwbudde/algo-dsp/internal/testutil"
)

func newTestTracker(t *testing.T, opts ...PitchTrackerOption) *PitchTracker {
	t.Helper()

	tr, err := NewPitchTracker(yinTestSampleRate, opts...)
	if err != nil {
		t.Fatalf("NewPitchTracker: %v", err)
	}

	return tr
}

func TestNewPitchTrackerDefaults(t *testing.T) {
	tr := newTestTracker(t)

	if got, want := tr.Hop(), tr.Detector().FrameSize()/trackerHopDivisor; got != want {
		t.Errorf("Hop() = %d, want %d", got, want)
	}

	if got := tr.MedianTaps(); got != defaultTrackerMedianTaps {
		t.Errorf("MedianTaps() = %d, want %d", got, defaultTrackerMedianTaps)
	}

	if got := tr.HoldFrames(); got != defaultTrackerHoldFrames {
		t.Errorf("HoldFrames() = %d, want %d", got, defaultTrackerHoldFrames)
	}

	if got, want := tr.Latency(), tr.Detector().FrameSize(); got != want {
		t.Errorf("Latency() = %d, want %d", got, want)
	}

	if tr.Estimate().Voiced {
		t.Errorf("a fresh tracker reports voiced before any samples arrive")
	}
}

func TestNewPitchTrackerValidation(t *testing.T) {
	mismatched, err := NewYINDetector(44100)
	if err != nil {
		t.Fatalf("NewYINDetector: %v", err)
	}

	tests := []struct {
		name       string
		sampleRate float64
		opts       []PitchTrackerOption
	}{
		{"zero sample rate", 0, nil},
		{"NaN sample rate", math.NaN(), nil},
		{"zero hop", yinTestSampleRate, []PitchTrackerOption{WithTrackerHop(0)}},
		{"negative hop", yinTestSampleRate, []PitchTrackerOption{WithTrackerHop(-1)}},
		{"hop beyond frame", yinTestSampleRate, []PitchTrackerOption{WithTrackerHop(1 << 20)}},
		{"even median taps", yinTestSampleRate, []PitchTrackerOption{WithTrackerMedianFilter(4)}},
		{"median taps too many", yinTestSampleRate, []PitchTrackerOption{WithTrackerMedianFilter(7)}},
		{"negative hold", yinTestSampleRate, []PitchTrackerOption{WithTrackerHoldFrames(-1)}},
		{"nil detector", yinTestSampleRate, []PitchTrackerOption{WithTrackerDetector(nil)}},
		{
			"detector rate mismatch", yinTestSampleRate,
			[]PitchTrackerOption{WithTrackerDetector(mismatched)},
		},
		{
			"bad detector option", yinTestSampleRate,
			[]PitchTrackerOption{WithTrackerDetectorOptions(WithYINThreshold(2))},
		},
		{"nil option", yinTestSampleRate, []PitchTrackerOption{nil}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, err := NewPitchTracker(tt.sampleRate, tt.opts...)
			if err == nil {
				t.Fatalf("NewPitchTracker = %v, want an error", tr)
			}

			if !strings.Contains(err.Error(), "pitch tracker") &&
				!strings.Contains(err.Error(), "yin detector") {
				t.Errorf("error %q does not name a processor", err)
			}
		})
	}
}

func TestPitchTrackerConvergesOnSine(t *testing.T) {
	const freq = 220.0

	tr := newTestTracker(t)
	tr.Write(yinSine(freq, yinTestSampleRate, 0.5, 0, 4*tr.Detector().FrameSize()))

	est := tr.Estimate()
	if !est.Voiced {
		t.Fatalf("unvoiced after four frames of a steady tone")
	}

	if cents := math.Abs(centsError(est.FrequencyHz, freq)); cents > 1 {
		t.Errorf("got %.4f Hz (%.3f cents off), want %g Hz", est.FrequencyHz, cents, freq)
	}

	// Tau must stay consistent with the smoothed frequency.
	if got, want := est.Tau, yinTestSampleRate/est.FrequencyHz; math.Abs(got-want) > 1e-9 {
		t.Errorf("Tau = %.9f, want %.9f", got, want)
	}
}

func TestPitchTrackerNoEstimateBeforeFirstFrame(t *testing.T) {
	tr := newTestTracker(t)

	tr.Write(yinSine(220, yinTestSampleRate, 0.5, 0, tr.Detector().FrameSize()-1))

	if tr.Estimate().Voiced {
		t.Errorf("reported an estimate before a full frame had arrived")
	}

	tr.Write(yinSine(220, yinTestSampleRate, 0.5, 0, 1))

	if !tr.Estimate().Voiced {
		t.Errorf("no estimate once the first full frame completed")
	}
}

func TestPitchTrackerBlockSizeInvariance(t *testing.T) {
	const freq = 330.0

	total := 4 * 3200
	signal := yinSine(freq, yinTestSampleRate, 0.5, 0, total)

	var reference PitchEstimate

	for i, blockSize := range []int{64, 256, 1000, total} {
		tr := newTestTracker(t)

		for start := 0; start < total; start += blockSize {
			end := min(start+blockSize, total)
			tr.Write(signal[start:end])
		}

		est := tr.Estimate()
		if i == 0 {
			reference = est
			continue
		}

		if est != reference {
			t.Errorf("block size %d gave %+v, want %+v", blockSize, est, reference)
		}
	}
}

func TestPitchTrackerHoldsOnUnvoiced(t *testing.T) {
	const freq = 220.0

	tr := newTestTracker(t, WithTrackerHoldFrames(2))
	tr.Write(yinSine(freq, yinTestSampleRate, 0.5, 0, 4*tr.Detector().FrameSize()))

	held := tr.Estimate()
	if !held.Voiced {
		t.Fatalf("unvoiced after a steady tone")
	}

	// One hop of silence triggers one unvoiced analysis, which the hold covers.
	tr.Write(make([]float64, tr.Hop()))

	after := tr.Estimate()
	if !after.Voiced {
		t.Fatalf("hold did not keep the estimate voiced for one unvoiced frame")
	}

	if after.FrequencyHz != held.FrequencyHz {
		t.Errorf("held frequency = %.6f, want %.6f", after.FrequencyHz, held.FrequencyHz)
	}

	// Enough silence to exhaust the hold must drop back to unvoiced.
	tr.Write(make([]float64, 6*tr.Detector().FrameSize()))

	if tr.Estimate().Voiced {
		t.Errorf("still voiced after sustained silence")
	}
}

func TestPitchTrackerMedianRejectsOutlier(t *testing.T) {
	const freq = 220.0

	// A steady tone followed by a brief burst of octave-doubled material, which
	// is the shape a sporadic octave error takes. The unfiltered tracker jumps
	// straight to the octave; the median filter is what stops it.
	build := func(taps int) float64 {
		tr := newTestTracker(t, WithTrackerMedianFilter(taps))
		frameSize := tr.Detector().FrameSize()

		tr.Write(yinSine(freq, yinTestSampleRate, 0.5, 0, 4*frameSize))
		tr.Write(yinSine(2*freq, yinTestSampleRate, 0.5, 0, frameSize))

		return tr.Estimate().FrequencyHz
	}

	withoutFilter := build(1)
	if relativeError(withoutFilter, 2*freq) > 0.02 {
		t.Fatalf("unfiltered tracker gave %.3f Hz, expected it to follow the octave at %g Hz",
			withoutFilter, 2*freq)
	}

	withFilter := build(defaultTrackerMedianTaps)
	if relativeError(withFilter, freq) > 0.02 {
		t.Errorf("median-filtered tracker gave %.3f Hz, want it to hold %g Hz", withFilter, freq)
	}

	if relativeError(withFilter, 2*freq) < 0.02 {
		t.Errorf("median filter still followed the octave: %.3f Hz", withFilter)
	}
}

func TestPitchTrackerTracksGlide(t *testing.T) {
	// A linear glide from 200 Hz to 400 Hz over one second.
	const (
		startHz = 200.0
		endHz   = 400.0
		seconds = 1.0
	)

	n := int(seconds * yinTestSampleRate)
	signal := make([]float64, n)

	phase := 0.0

	for i := range signal {
		hz := startHz + (endHz-startHz)*float64(i)/float64(n-1)
		signal[i] = 0.5 * math.Sin(phase)
		phase += 2 * math.Pi * hz / yinTestSampleRate
	}

	tr := newTestTracker(t)
	tr.Write(signal)

	est := tr.Estimate()
	if !est.Voiced {
		t.Fatalf("unvoiced at the end of the glide")
	}

	// The estimate lags by roughly the frame and the median filter, so compare
	// against the instantaneous pitch a little before the end.
	if got := relativeError(est.FrequencyHz, endHz); got > 0.05 {
		t.Errorf("end of glide: got %.4f Hz, want near %g Hz (relative error %.4f)",
			est.FrequencyHz, endHz, got)
	}

	if est.FrequencyHz <= startHz {
		t.Errorf("tracker never followed the glide upward: %.4f Hz", est.FrequencyHz)
	}
}

func TestPitchTrackerResetDeterministic(t *testing.T) {
	tr := newTestTracker(t)
	signal := yinSawtooth(196, yinTestSampleRate, 0.4, 4*tr.Detector().FrameSize())

	tr.Write(signal)
	first := tr.Estimate()

	tr.Write(testutil.DeterministicNoise(7, 0.5, 2*tr.Detector().FrameSize()))
	tr.Reset()

	if tr.Estimate().Voiced {
		t.Errorf("Reset left a voiced estimate behind")
	}

	tr.Write(signal)

	if second := tr.Estimate(); second != first {
		t.Errorf("after Reset: %+v, want %+v", second, first)
	}
}

func TestPitchTrackerWriteZeroAlloc(t *testing.T) {
	tr := newTestTracker(t)
	block := yinSawtooth(220, yinTestSampleRate, 0.5, 256)

	// Prime the ring so the measured runs include analyses, not just buffering.
	tr.Write(yinSawtooth(220, yinTestSampleRate, 0.5, tr.Detector().FrameSize()))

	allocs := testing.AllocsPerRun(20, func() {
		tr.Write(block)
	})

	if allocs != 0 {
		t.Errorf("Write allocs = %g, want 0", allocs)
	}
}

func TestPitchTrackerAcceptsInjectedDetector(t *testing.T) {
	d, err := NewYINDetector(yinTestSampleRate, WithYINFrequencyRange(150, 900))
	if err != nil {
		t.Fatalf("NewYINDetector: %v", err)
	}

	tr := newTestTracker(t, WithTrackerDetector(d))

	if tr.Detector() != d {
		t.Errorf("Detector() did not return the injected detector")
	}

	tr.Write(yinSine(440, yinTestSampleRate, 0.5, 0, 4*d.FrameSize()))

	if est := tr.Estimate(); !est.Voiced || math.Abs(centsError(est.FrequencyHz, 440)) > 1 {
		t.Errorf("injected detector gave %+v, want ~440 Hz", est)
	}
}

func TestPitchTrackerEmptyWrite(t *testing.T) {
	tr := newTestTracker(t)

	tr.Write(nil)
	tr.Write([]float64{})

	if tr.Estimate().Voiced {
		t.Errorf("empty writes produced an estimate")
	}
}
