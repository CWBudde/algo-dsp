package features_test

import (
	"errors"
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/internal/testutil"
	"github.com/cwbudde/algo-dsp/measure/music/features"
)

// stemLevels returns deterministic frame-level tracks at 100 frames/s that
// resemble separated stems: a loud pulsing track, a steady track with a
// break, a quiet track and a track that is mostly below the gate.
func stemLevels(frames int) map[string][]float64 {
	levels := map[string][]float64{
		"drums":  make([]float64, frames),
		"bass":   make([]float64, frames),
		"other":  make([]float64, frames),
		"vocals": make([]float64, frames),
	}
	noise := testutil.DeterministicNoise(11, 1, 4*frames)

	for i := range frames {
		pulse := math.Exp(-float64(i%46) / 8)
		levels["drums"][i] = 0.3*pulse + 0.02*math.Abs(noise[i])

		if i < frames/3 || i > frames/2 {
			levels["bass"][i] = 0.1 + 0.03*noise[frames+i]
		}

		levels["other"][i] = 0.01 * (1 + 0.5*math.Sin(float64(i)/40)) * math.Abs(noise[2*frames+i])

		if i%300 < 60 {
			levels["vocals"][i] = 0.002 * math.Abs(noise[3*frames+i])
		} else {
			levels["vocals"][i] = 5e-5 * math.Abs(noise[3*frames+i])
		}
	}

	return levels
}

// barSpans returns count bars of a 105 BPM 4/4 grid whose bar origin is
// before time 0 (a pickup bar), computed as the app's Grid.BarStart.
func barSpans(count int) [][2]float64 {
	const origin, bar = -0.37, 4 * 60.0 / 105

	start := func(n int) float64 { return origin + float64(n)*bar }

	spans := make([][2]float64, count)
	for b := range spans {
		spans[b] = [2]float64{start(b), start(b + 1)}
	}

	return spans
}

func toIntervals(spans [][2]float64) []features.Interval {
	out := make([]features.Interval, len(spans))
	for i, s := range spans {
		out[i] = features.Interval{Start: s[0], End: s[1]}
	}

	return out
}

func TestActivityParity(t *testing.T) {
	t.Parallel()

	const frames = 3000 // 30 s

	levels := stemLevels(frames)
	spans := barSpans(15) // the last bars run past the end of the tracks

	for _, threshold := range []float64{-12, -6, -20} {
		got, err := features.Activity(levels, refFrameRate, toIntervals(spans), features.WithActivityThreshold(threshold))
		if err != nil {
			t.Fatal(err)
		}

		if len(got) != len(levels) {
			t.Fatalf("%d tracks, want %d", len(got), len(levels))
		}

		for name, rms := range levels {
			act := got[name]

			if want := refP95DB(rms); act.ReferenceDB != want {
				t.Fatalf("%s: reference %v dB, want %v", name, act.ReferenceDB, want)
			}

			want := refActive(rms, spans, threshold)
			for b := range spans {
				if wantDB := refBarLevelDB(rms, spans[b]); act.SpanDB[b] != wantDB {
					t.Fatalf("%s bar %d: level %v dB, want %v", name, b, act.SpanDB[b], wantDB)
				}

				if act.Active[b] != want[b] {
					t.Fatalf("threshold %v, %s bar %d: active %v, want %v", threshold, name, b, act.Active[b], want[b])
				}
			}
		}
	}
}

func TestActivitySilentSpanInactive(t *testing.T) {
	t.Parallel()

	// 0.2 for 2 s, silence for 1 s, 0.2 for 2 s.
	level := make([]float64, 500)
	for i := range level {
		if i < 200 || i >= 300 {
			level[i] = 0.2
		}
	}

	spans := []features.Interval{{Start: 0, End: 2}, {Start: 2, End: 3}, {Start: 3, End: 5}, {Start: 6, End: 7}}

	got, err := features.Activity(map[string][]float64{"pad": level}, 100, spans)
	if err != nil {
		t.Fatal(err)
	}

	act := got["pad"]
	if want := []bool{true, false, true, false}; !equalBools(act.Active, want) {
		t.Fatalf("active %v, want %v", act.Active, want)
	}

	if act.SpanDB[1] != -120 || act.SpanDB[3] != -120 {
		t.Fatalf("silent span levels %v", act.SpanDB)
	}
}

func TestActivityQuietTrackActive(t *testing.T) {
	t.Parallel()

	// A loud track at -6 dBFS and a quiet one at -46 dBFS that pauses in
	// the middle second. Each is measured against its own level.
	loud := make([]float64, 300)
	quiet := make([]float64, 300)

	for i := range loud {
		loud[i] = 0.5
		if i < 100 || i >= 200 {
			quiet[i] = 0.005
		}
	}

	got, err := features.Activity(map[string][]float64{"loud": loud, "quiet": quiet}, 100,
		[]features.Interval{{Start: 0, End: 1}, {Start: 1, End: 2}, {Start: 2, End: 3}})
	if err != nil {
		t.Fatal(err)
	}

	if want := []bool{true, false, true}; !equalBools(got["quiet"].Active, want) {
		t.Fatalf("quiet active %v, want %v", got["quiet"].Active, want)
	}

	if want := []bool{true, true, true}; !equalBools(got["loud"].Active, want) {
		t.Fatalf("loud active %v, want %v", got["loud"].Active, want)
	}

	if math.Abs(got["quiet"].ReferenceDB+46.02) > 0.01 {
		t.Fatalf("quiet reference %v dB", got["quiet"].ReferenceDB)
	}
}

func TestActivityBelowGateNeverActive(t *testing.T) {
	t.Parallel()

	got, err := features.Activity(map[string][]float64{"leak": {5e-5, 5e-5, 5e-5}, "empty": nil}, 100,
		[]features.Interval{{Start: 0, End: 0.03}})
	if err != nil {
		t.Fatal(err)
	}

	for name, act := range got {
		if act.Active[0] || act.ReferenceDB != -120 {
			t.Errorf("%s: %+v", name, act)
		}
	}

	// A lower gate makes the leak count.
	got, err = features.Activity(map[string][]float64{"leak": {5e-5, 5e-5, 5e-5}}, 100,
		[]features.Interval{{Start: 0, End: 0.03}}, features.WithActivityGate(1e-5))
	if err != nil {
		t.Fatal(err)
	}

	if !got["leak"].Active[0] {
		t.Errorf("leak with lower gate: %+v", got["leak"])
	}
}

func TestActivityPercentileAndThreshold(t *testing.T) {
	t.Parallel()

	// Levels 0.01 ... 1.00; a span over the first 50 frames has an RMS of
	// about 0.29 (-10.6 dB).
	level := make([]float64, 100)
	for i := range level {
		level[i] = float64(i+1) / 100
	}

	spans := []features.Interval{{Start: 0, End: 0.5}}

	got, err := features.Activity(map[string][]float64{"x": level}, 100, spans)
	if err != nil {
		t.Fatal(err)
	}

	// floor(0.95·99) = 94 → 0.95.
	if want := 20 * math.Log10(0.95); math.Abs(got["x"].ReferenceDB-want) > 1e-12 || !got["x"].Active[0] {
		t.Fatalf("default: %+v, want reference %v", got["x"], want)
	}

	got, err = features.Activity(map[string][]float64{"x": level}, 100, spans,
		features.WithActivityPercentile(0), features.WithActivityThreshold(-6))
	if err != nil {
		t.Fatal(err)
	}

	if got["x"].ReferenceDB != -40 || !got["x"].Active[0] {
		t.Fatalf("percentile 0: %+v", got["x"])
	}

	got, err = features.Activity(map[string][]float64{"x": level}, 100, spans, features.WithActivityThreshold(-3))
	if err != nil {
		t.Fatal(err)
	}

	if got["x"].Active[0] {
		t.Fatalf("threshold -3 dB: %+v", got["x"])
	}
}

func TestActivityErrors(t *testing.T) {
	t.Parallel()

	ok := map[string][]float64{"a": {0.1, 0.2}}
	spans := []features.Interval{{Start: 0, End: 1}}

	tests := []struct {
		name     string
		levels   map[string][]float64
		rate     float64
		spans    []features.Interval
		opts     []features.ActivityOption
		sentinel error
	}{
		{"nil option", ok, 100, spans, []features.ActivityOption{nil}, features.ErrNilOption},
		{"threshold", ok, 100, spans, []features.ActivityOption{features.WithActivityThreshold(math.NaN())}, features.ErrInvalidArgument},
		{"threshold inf", ok, 100, spans, []features.ActivityOption{features.WithActivityThreshold(math.Inf(-1))}, features.ErrInvalidArgument},
		{"gate", ok, 100, spans, []features.ActivityOption{features.WithActivityGate(-1)}, features.ErrInvalidArgument},
		{"percentile", ok, 100, spans, []features.ActivityOption{features.WithActivityPercentile(1.5)}, features.ErrInvalidArgument},
		{"frame rate", ok, 0, spans, nil, features.ErrInvalidArgument},
		{"frame rate inf", ok, math.Inf(1), spans, nil, features.ErrInvalidArgument},
		{"span order", ok, 100, []features.Interval{{Start: 2, End: 1}}, nil, features.ErrInvalidArgument},
		{"span nan", ok, 100, []features.Interval{{Start: math.NaN(), End: 1}}, nil, features.ErrInvalidArgument},
		{"negative level", map[string][]float64{"a": {-0.1}}, 100, spans, nil, features.ErrInvalidArgument},
		{"nan level", map[string][]float64{"a": {math.NaN()}}, 100, spans, nil, features.ErrInvalidArgument},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := features.Activity(tc.levels, tc.rate, tc.spans, tc.opts...)
			if !errors.Is(err, tc.sentinel) {
				t.Fatalf("got %v, want %v", err, tc.sentinel)
			}
		})
	}
}

func TestActivityEmpty(t *testing.T) {
	t.Parallel()

	got, err := features.Activity(nil, 100, nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func equalBools(a, b []bool) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func BenchmarkActivity(b *testing.B) {
	levels := stemLevels(18000) // 3 minutes at 100 frames/s
	spans := toIntervals(barSpans(80))

	b.ReportAllocs()

	for b.Loop() {
		_, err := features.Activity(levels, 100, spans)
		if err != nil {
			b.Fatal(err)
		}
	}
}
