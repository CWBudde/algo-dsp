package features_test

import (
	"testing"

	"github.com/cwbudde/algo-dsp/measure/music/features"
)

func requireIdentical(t *testing.T, name string, got, want []float64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s: length %d, want %d", name, len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s[%d] = %v, want %v (not bit-identical)", name, i, got[i], want[i])
		}
	}
}

func TestExtractParity(t *testing.T) {
	t.Parallel()

	stereo := musicFixture(4)
	tests := []struct {
		name     string
		channels [][]float64
	}{
		{"stereo", stereo},
		{"mono", stereo[:1]},
		{"three", [][]float64{stereo[0], stereo[1], stereo[0]}},
		{"short", [][]float64{stereo[0][:1000], stereo[1][:1000]}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			want, err := refAnalyze(tc.channels)
			if err != nil {
				t.Fatal(err)
			}

			got, err := features.Extract(tc.channels, features.DefaultConfig(),
				features.WithLogSpectrogram(64, 25, 12000))
			if err != nil {
				t.Fatal(err)
			}

			requireIdentical(t, "RMS", got.RMS, want.RMS)
			requireIdentical(t, "Peak", got.Peak, want.Peak)
			requireIdentical(t, "Centroid", got.Centroid, want.Centroid)
			requireIdentical(t, "Width", got.Width, want.Width)
			requireIdentical(t, "Flux", got.Flux, want.Flux)

			for b := range want.Bands {
				requireIdentical(t, "Bands", got.Bands[b], want.Bands[b])
			}
		})
	}
}

func TestNormalizeParity(t *testing.T) {
	t.Parallel()

	f, err := features.Extract(musicFixture(4), features.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	frameRate := f.FrameRate()
	cases := []struct {
		name            string
		x               []float64
		attack, release float64
	}{
		{"energy", f.RMS, 0.01, 0.15},
		{"band0", f.Bands[0], 0.01, 0.12},
		{"band4", f.Bands[4], 0.01, 0.12},
		{"silent", make([]float64, 100), 0.01, 0.12},
	}

	for _, tc := range cases {
		got, err := features.Normalize(tc.x, features.DefaultGate, tc.attack, tc.release, frameRate)
		if err != nil {
			t.Fatal(err)
		}

		requireIdentical(t, tc.name, got, refNormalize(tc.x, tc.attack, tc.release))
	}
}

func TestPercentileParity(t *testing.T) {
	t.Parallel()

	x := musicFixture(1)[0]
	for _, p := range []float64{0, 0.1, 0.5, 0.95, 1} {
		if got, want := features.Percentile(x, p), percentile(x, p); got != want {
			t.Fatalf("Percentile(%v) = %v, want %v", p, got, want)
		}
	}
}

func TestSilenceParity(t *testing.T) {
	t.Parallel()

	channels := musicFixture(4)
	// Add a silent tail so an interval reaches the end of the signal.
	for _, ch := range channels {
		clear(ch[len(ch)-5000:])
	}

	got, err := features.Silence(channels, SampleRate, -45, 0.15)
	if err != nil {
		t.Fatal(err)
	}

	want := refFindSilence(channels)
	if len(got) != len(want) || len(want) != 2 {
		t.Fatalf("intervals %+v, want %+v", got, want)
	}

	for i := range want {
		if got[i].Start != want[i].Start || got[i].End != want[i].End {
			t.Fatalf("interval %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
