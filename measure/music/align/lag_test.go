package align_test

import (
	"errors"
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/internal/testutil"
	"github.com/cwbudde/algo-dsp/measure/music/align"
)

// partials returns a deterministic band-limited test signal of n samples: a
// sum of sines between 40 and 800 Hz, delayed by delay samples (which may be
// fractional) and scaled by gain. Delaying the analytic sines gives an exact
// fractional delay.
func partials(n int, delay, gain float64) []float64 {
	const count = 30

	freq := testutil.DeterministicNoise(7, 1, count)
	phase := testutil.DeterministicNoise(8, math.Pi, count)
	x := make([]float64, n)

	for k := range count {
		f := 40 + 380*(freq[k]+1)
		amp := 0.5 / math.Sqrt(float64(count)) / (1 + float64(k)/count)

		for i := range x {
			t := (float64(i) - delay) / SampleRate
			x[i] += gain * amp * math.Sin(2*math.Pi*f*t+phase[k])
		}
	}

	return x
}

// rendered returns x delayed by lag whole samples (positive: later), scaled
// by gain, plus noise of the given amplitude, with length n.
func rendered(x []float64, n, lag int, gain, noise float64, seed int64) []float64 {
	out := testutil.DeterministicNoise(seed, noise, n)

	for i := range out {
		if j := i - lag; j >= 0 && j < len(x) {
			out[i] += gain * x[j]
		}
	}

	return out
}

func TestLagParity(t *testing.T) {
	t.Parallel()

	const n = 6 * SampleRate

	source := [][]float64{partials(n, 0, 1), partials(n, 3, 0.8)}
	for c := range source {
		for i, v := range testutil.DeterministicNoise(int64(20+c), 0.05, n) {
			source[c][i] += v
		}
	}

	windows := [][2]float64{{0, 1}, {0.5, 2.5}, {2, 4}, {3.3, 6.12}, {5.99, 7}}

	for _, tc := range []struct {
		name   string
		lag    int
		gain   float64
		length int
	}{
		{"aligned", 0, 1, n},
		{"late", 7, 0.97, n},
		{"early", -23, 1.02, n - 100},
		{"off grid", 333, 0.5, n + 50},
		{"near limit", -470, 1.1, n},
		{"beyond limit", 700, 1, n},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			x := [][]float64{
				rendered(source[0], tc.length, tc.lag, tc.gain, 1e-3, 31),
				rendered(source[1], tc.length, tc.lag, tc.gain, 1e-3, 32),
			}

			for _, w := range windows {
				want := refCompare(source, x, w[0], w[1])

				got, err := align.LagChannels(source, x, SampleRate, align.WithLagWindow(w[0], w[1]))
				if err != nil {
					t.Fatal(err)
				}

				lagMS := float64(got.Samples) * 1000 / SampleRate
				if lagMS != want.LagMS || got.Correlation != want.Correlation || got.GainDB != want.GainDB {
					t.Fatalf("window %v: got %+v, want %+v", w, got, want)
				}
			}
		})
	}
}

func TestLagRecoversIntegerLagAndGain(t *testing.T) {
	t.Parallel()

	const n = 4 * SampleRate

	ref := partials(n, 0, 1)

	for _, tc := range []struct {
		lag    int
		gainDB float64
	}{{0, 0}, {37, -3}, {-5, 1.5}, {250, -0.2}, {-401, 6}} {
		x := rendered(ref, n, tc.lag, math.Pow(10, tc.gainDB/20), 0, 0)

		res, err := align.Lag(ref, x, SampleRate)
		if err != nil {
			t.Fatal(err)
		}

		if res.Samples != tc.lag || res.Seconds != float64(tc.lag)/SampleRate {
			t.Errorf("lag %d: got %d samples, %v s", tc.lag, res.Samples, res.Seconds)
		}

		if math.Abs(res.GainDB-tc.gainDB) > 1e-9 || res.Correlation < 1-1e-12 {
			t.Errorf("lag %d: gain %v dB (want %v), correlation %v", tc.lag, res.GainDB, tc.gainDB, res.Correlation)
		}

		if math.Abs(res.RefinedSamples-float64(tc.lag)) > 0.01 {
			t.Errorf("lag %d: refined %v", tc.lag, res.RefinedSamples)
		}
	}
}

func TestLagRecoversFractionalLag(t *testing.T) {
	t.Parallel()

	const n = 4 * SampleRate

	ref := partials(n, 0, 1)

	for _, tc := range []struct {
		delay  float64
		gainDB float64
	}{{12.4, -2}, {-7.75, 0.5}, {100.5, 0}, {0.3, 3}} {
		x := partials(n, tc.delay, math.Pow(10, tc.gainDB/20))

		res, err := align.Lag(ref, x, SampleRate)
		if err != nil {
			t.Fatal(err)
		}

		if math.Abs(float64(res.Samples)-tc.delay) > 0.5+1e-9 {
			t.Errorf("delay %v: integer lag %d", tc.delay, res.Samples)
		}

		if math.Abs(res.RefinedSamples-tc.delay) > 0.05 {
			t.Errorf("delay %v: refined lag %v", tc.delay, res.RefinedSamples)
		}

		if math.Abs(res.GainDB-tc.gainDB) > 0.05 || res.Correlation < 0.99 {
			t.Errorf("delay %v: gain %v dB (want %v), correlation %v", tc.delay, res.GainDB, tc.gainDB, res.Correlation)
		}
	}
}

func TestLagMonoMatchesChannels(t *testing.T) {
	t.Parallel()

	const n = 2 * SampleRate

	ref := partials(n, 0, 1)
	x := rendered(ref, n, 19, 0.7, 1e-2, 3)

	mono, err := align.Lag(ref, x, SampleRate)
	if err != nil {
		t.Fatal(err)
	}

	multi, err := align.LagChannels([][]float64{ref}, [][]float64{x}, SampleRate)
	if err != nil {
		t.Fatal(err)
	}

	if mono != multi {
		t.Fatalf("Lag %+v != LagChannels %+v", mono, multi)
	}
}

func TestLagOptions(t *testing.T) {
	t.Parallel()

	const n = 2 * SampleRate

	ref := partials(n, 0, 1)
	x := rendered(ref, n, 37, 1, 0, 0)

	// No search at all: lag 0.
	res, err := align.Lag(ref, x, SampleRate, align.WithLagMaxLag(0), align.WithLagFineRadius(0))
	if err != nil {
		t.Fatal(err)
	}

	if res.Samples != 0 {
		t.Errorf("no search: lag %d", res.Samples)
	}

	// Exhaustive search with every sample.
	res, err = align.Lag(ref, x, SampleRate, align.WithLagCoarseStep(1), align.WithLagStride(1), align.WithLagFineRadius(0))
	if err != nil {
		t.Fatal(err)
	}

	if res.Samples != 37 {
		t.Errorf("exhaustive: lag %d", res.Samples)
	}

	// A range of ±1 ms (24 samples) plus a 4-sample fine search cannot reach
	// 37 samples.
	res, err = align.Lag(ref, x, SampleRate, align.WithLagMaxLag(0.001), align.WithLagFineRadius(4))
	if err != nil {
		t.Fatal(err)
	}

	if res.Samples > 28 {
		t.Errorf("limited range: lag %d", res.Samples)
	}
}

func TestLagErrors(t *testing.T) {
	t.Parallel()

	ref := partials(SampleRate, 0, 1)
	short := ref[:100]
	silent := make([]float64, SampleRate)

	tests := []struct {
		name     string
		ref, x   [][]float64
		rate     float64
		opts     []align.LagOption
		sentinel error
	}{
		{"nil option", [][]float64{ref}, [][]float64{ref}, SampleRate, []align.LagOption{nil}, align.ErrNilOption},
		{"max lag", [][]float64{ref}, [][]float64{ref}, SampleRate, []align.LagOption{align.WithLagMaxLag(-1)}, align.ErrInvalidArgument},
		{"max lag inf", [][]float64{ref}, [][]float64{ref}, SampleRate, []align.LagOption{align.WithLagMaxLag(math.Inf(1))}, align.ErrInvalidArgument},
		{"coarse step", [][]float64{ref}, [][]float64{ref}, SampleRate, []align.LagOption{align.WithLagCoarseStep(0)}, align.ErrInvalidArgument},
		{"fine radius", [][]float64{ref}, [][]float64{ref}, SampleRate, []align.LagOption{align.WithLagFineRadius(-1)}, align.ErrInvalidArgument},
		{"stride", [][]float64{ref}, [][]float64{ref}, SampleRate, []align.LagOption{align.WithLagStride(0)}, align.ErrInvalidArgument},
		{"window order", [][]float64{ref}, [][]float64{ref}, SampleRate, []align.LagOption{align.WithLagWindow(2, 1)}, align.ErrInvalidArgument},
		{"window start", [][]float64{ref}, [][]float64{ref}, SampleRate, []align.LagOption{align.WithLagWindow(math.NaN(), 1)}, align.ErrInvalidArgument},
		{"window past end", [][]float64{ref}, [][]float64{ref}, SampleRate, []align.LagOption{align.WithLagWindow(5, 6)}, align.ErrInvalidArgument},
		{"sample rate", [][]float64{ref}, [][]float64{ref}, 0, nil, align.ErrInvalidArgument},
		{"empty reference", [][]float64{}, [][]float64{}, SampleRate, nil, align.ErrInvalidArgument},
		{"empty channel", [][]float64{{}}, [][]float64{ref}, SampleRate, nil, align.ErrInvalidArgument},
		{"empty x", [][]float64{ref}, [][]float64{{}}, SampleRate, nil, align.ErrInvalidArgument},
		{"channel count", [][]float64{ref, ref}, [][]float64{ref}, SampleRate, nil, align.ErrInvalidArgument},
		{"reference lengths", [][]float64{ref, short}, [][]float64{ref, ref}, SampleRate, nil, align.ErrInvalidArgument},
		{"x lengths", [][]float64{ref, ref}, [][]float64{ref, short}, SampleRate, nil, align.ErrInvalidArgument},
		{"silent reference", [][]float64{silent}, [][]float64{ref}, SampleRate, nil, align.ErrInvalidArgument},
		{"silent x", [][]float64{ref}, [][]float64{silent}, SampleRate, nil, align.ErrInvalidArgument},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := align.LagChannels(tc.ref, tc.x, tc.rate, tc.opts...)
			if !errors.Is(err, tc.sentinel) {
				t.Fatalf("got %v, want %v", err, tc.sentinel)
			}
		})
	}
}

func TestLagAllocs(t *testing.T) {
	ref := partials(SampleRate, 0, 1)
	x := rendered(ref, SampleRate, 10, 1, 0, 0)
	opts := []align.LagOption{align.WithLagWindow(0.1, 0.9)}

	allocs := testing.AllocsPerRun(10, func() {
		_, err := align.Lag(ref, x, SampleRate)
		if err != nil {
			t.Fatal(err)
		}
	})

	if allocs != 0 {
		t.Fatalf("Lag allocates %v times per call", allocs)
	}

	allocs = testing.AllocsPerRun(10, func() {
		_, err := align.Lag(ref, x, SampleRate, opts...)
		if err != nil {
			t.Fatal(err)
		}
	})

	if allocs > 1 {
		t.Fatalf("Lag with options allocates %v times per call", allocs)
	}
}

func BenchmarkLag(b *testing.B) {
	const n = 10 * SampleRate

	ref := partials(n, 0, 1)
	x := rendered(ref, n, 123, 0.9, 1e-3, 5)

	b.ReportAllocs()

	for b.Loop() {
		_, err := align.Lag(ref, x, SampleRate)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLagChannels(b *testing.B) {
	const n = 10 * SampleRate

	ref := [][]float64{partials(n, 0, 1), partials(n, 2, 1)}
	x := [][]float64{rendered(ref[0], n, 123, 0.9, 1e-3, 5), rendered(ref[1], n, 123, 0.9, 1e-3, 6)}

	b.ReportAllocs()

	for b.Loop() {
		_, err := align.LagChannels(ref, x, SampleRate)
		if err != nil {
			b.Fatal(err)
		}
	}
}
