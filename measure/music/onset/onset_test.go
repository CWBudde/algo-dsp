package onset_test

import (
	"errors"
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/measure/music/features"
	"github.com/cwbudde/algo-dsp/measure/music/onset"
)

func extract(t *testing.T, channels [][]float64) *features.Frames {
	t.Helper()

	f, err := features.Extract(channels, features.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	return f
}

func toRef(f *features.Frames) *refTrack {
	r := &refTrack{RMS: f.RMS, Flux: f.Flux}
	copy(r.Bands[:], f.Bands)

	return r
}

// drumFixture is the kick/snare/hat signal of the AudioVisualizer
// TestClassifyDrums: a decaying 60 Hz kick at 0.25 s, a 190 Hz body plus
// 2–5 kHz noise snare at 0.75 s and a 7–11 kHz hat at 1.25 s.
func drumFixture() []float64 {
	x := make([]float64, 2*SampleRate)
	hit := func(time float64, f func(j int) float64) {
		start := int(time * SampleRate)
		for j := range SampleRate / 3 {
			x[start+j] += f(j)
		}
	}
	decay := func(j int, seconds float64) float64 { return math.Exp(-float64(j) / (SampleRate * seconds)) }
	sines := func(j int, lo, hi float64) float64 {
		v := 0.0

		for k := range 40 {
			f := lo + (hi-lo)*float64(k)/40
			v += 0.05 * math.Sin(2*math.Pi*f*float64(j)/SampleRate+float64(k*k))
		}

		return v
	}

	hit(0.25, func(j int) float64 { return 0.8 * math.Sin(2*math.Pi*60*float64(j)/SampleRate) * decay(j, 0.06) })
	hit(0.75, func(j int) float64 {
		return (0.4*math.Sin(2*math.Pi*190*float64(j)/SampleRate) + sines(j, 2000, 5000)) * decay(j, 0.03)
	})
	hit(1.25, func(j int) float64 { return sines(j, 7000, 11000) * decay(j, 0.015) })

	return x
}

func TestDetectAndClassifyParity(t *testing.T) {
	t.Parallel()

	stereo := musicFixture(6)
	tests := []struct {
		name     string
		channels [][]float64
	}{
		{"music stereo", stereo},
		{"music mono", stereo[:1]},
		{"drums", [][]float64{drumFixture()}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := extract(t, tc.channels)
			ref := toRef(f)
			ref.Events = refDetectOnsets(tc.channels, ref)

			got, err := onset.Detect(tc.channels, f)
			if err != nil {
				t.Fatal(err)
			}

			if len(got) != len(ref.Events) || len(got) < 3 {
				t.Fatalf("%d events, reference %d", len(got), len(ref.Events))
			}

			got, err = onset.ClassifyDrums(got, f)
			if err != nil {
				t.Fatal(err)
			}

			refClassifyDrums(ref)

			for i, want := range ref.Events {
				if got[i].Time != want.Time || got[i].Strength != want.Strength || got[i].Kind.String() != want.Kind {
					t.Fatalf("event %d = %+v (%v), want %+v", i, got[i], got[i].Kind, want)
				}
			}
		})
	}
}

func TestDetectSilence(t *testing.T) {
	t.Parallel()

	x := [][]float64{make([]float64, SampleRate)}

	events, err := onset.Detect(x, extract(t, x))
	if err != nil {
		t.Fatal(err)
	}

	if events != nil {
		t.Fatalf("silent audio produced onsets %+v", events)
	}
}

func TestDetectTransientTiming(t *testing.T) {
	t.Parallel()

	x := make([]float64, 2*SampleRate)
	for _, time := range []float64{0.25, 0.75, 1.25, 1.75} {
		start := int(time * SampleRate)
		for j := range SampleRate / 20 {
			x[start+j] = 0.8 * math.Cos(2*math.Pi*1000*float64(j)/SampleRate) * math.Exp(-float64(j)/(SampleRate*0.006))
		}
	}

	channels := [][]float64{x}
	f := extract(t, channels)

	tests := []struct {
		name string
		opts []onset.Option
		tol  float64
	}{
		{"refined", nil, 1.0 / 60},
		{"refined 2 ms", []onset.Option{onset.WithRefinement(0.05, 0.002)}, 1.0 / 60},
		{"frame centres", []onset.Option{onset.WithRefinement(0, 0.005)}, 0.05},
	}

	for _, tc := range tests {
		events, err := onset.Detect(channels, f, tc.opts...)
		if err != nil {
			t.Fatal(err)
		}

		if len(events) != 4 {
			t.Fatalf("%s: expected 4 isolated transients, got %+v", tc.name, events)
		}

		for i, e := range events {
			if math.Abs(e.Time-(0.25+float64(i)*0.5)) > tc.tol {
				t.Fatalf("%s: onset %d off: %+v", tc.name, i, e)
			}

			if e.Strength <= 0 || e.Strength > 1 || e.Kind != onset.KindUnknown {
				t.Fatalf("%s: event %+v", tc.name, e)
			}
		}
	}
}

func TestDetectOptions(t *testing.T) {
	t.Parallel()

	channels := musicFixture(3)
	f := extract(t, channels)

	base, err := onset.Detect(channels, f)
	if err != nil {
		t.Fatal(err)
	}

	// A huge minimum spacing keeps a single event.
	one, err := onset.Detect(channels, f, onset.WithMinSpacing(100))
	if err != nil {
		t.Fatal(err)
	}

	if len(one) != 1 {
		t.Fatalf("min spacing: %d events", len(one))
	}

	// An unreachable RMS gate, threshold or local factor removes everything.
	for _, opt := range []onset.Option{
		onset.WithRMSGate(10), onset.WithThreshold(10), onset.WithLocalMean(25, 1000),
	} {
		none, err := onset.Detect(channels, f, opt)
		if err != nil {
			t.Fatal(err)
		}

		if none == nil || len(none) != 0 {
			t.Fatalf("expected an empty, non-nil result, got %+v", none)
		}
	}

	// Explicit defaults change nothing.
	same, err := onset.Detect(channels, f,
		onset.WithRMSGate(onset.DefaultRMSGate), onset.WithThreshold(onset.DefaultThreshold),
		onset.WithLocalMean(onset.DefaultLocalRadius, onset.DefaultLocalFactor),
		onset.WithMinSpacing(onset.DefaultMinSpacing),
		onset.WithRefinement(onset.DefaultRefineWindow, onset.DefaultAttackStep))
	if err != nil {
		t.Fatal(err)
	}

	if len(same) != len(base) {
		t.Fatalf("explicit defaults: %d events, want %d", len(same), len(base))
	}

	for i := range base {
		if same[i] != base[i] {
			t.Fatalf("explicit defaults: event %d differs", i)
		}
	}
}

func TestDetectErrors(t *testing.T) {
	t.Parallel()

	channels := [][]float64{make([]float64, 1000)}
	f := extract(t, channels)
	badTiming := *f
	badTiming.Hop = 0
	badLen := *f
	badLen.RMS = badLen.RMS[1:]

	tests := []struct {
		name     string
		channels [][]float64
		f        *features.Frames
		opts     []onset.Option
		want     error
	}{
		{"nil option", channels, f, []onset.Option{nil}, onset.ErrNilOption},
		{"gate", channels, f, []onset.Option{onset.WithRMSGate(-1)}, onset.ErrInvalidArgument},
		{"threshold", channels, f, []onset.Option{onset.WithThreshold(math.NaN())}, onset.ErrInvalidArgument},
		{"radius", channels, f, []onset.Option{onset.WithLocalMean(-1, 1)}, onset.ErrInvalidArgument},
		{"factor", channels, f, []onset.Option{onset.WithLocalMean(1, -1)}, onset.ErrInvalidArgument},
		{"spacing", channels, f, []onset.Option{onset.WithMinSpacing(math.Inf(1))}, onset.ErrInvalidArgument},
		{"window", channels, f, []onset.Option{onset.WithRefinement(-1, 0.005)}, onset.ErrInvalidArgument},
		{"step", channels, f, []onset.Option{onset.WithRefinement(0.05, 0)}, onset.ErrInvalidArgument},
		{"nil frames", channels, nil, nil, onset.ErrInvalidArgument},
		{"timing", channels, &badTiming, nil, features.ErrInvalidConfig},
		{"lengths", channels, &badLen, nil, onset.ErrInvalidArgument},
		{"no audio", nil, f, nil, onset.ErrInvalidArgument},
		{"unequal", [][]float64{make([]float64, 10), make([]float64, 9)}, f, nil, onset.ErrInvalidArgument},
	}

	for _, tc := range tests {
		_, err := onset.Detect(tc.channels, tc.f, tc.opts...)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestKindString(t *testing.T) {
	t.Parallel()

	want := map[onset.Kind]string{
		onset.KindUnknown: "unknown", onset.KindKick: "kick", onset.KindSnare: "snare",
		onset.KindHat: "hat", onset.Kind(9): "Kind(9)",
	}

	for k, s := range want {
		if k.String() != s {
			t.Errorf("%d.String() = %q, want %q", int(k), k.String(), s)
		}
	}
}
