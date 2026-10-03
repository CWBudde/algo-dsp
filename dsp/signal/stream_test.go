package signal_test

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/signal"
)

func streamConfig(kind signal.StreamKind) signal.StreamConfig {
	return signal.StreamConfig{Kind: kind, SampleRate: 8, Amplitude: .5, StartHz: 1, EndHz: 2, Frames: 109, Seed: 0}
}

func TestStreamPartitions(t *testing.T) {
	t.Parallel()

	for _, kind := range []signal.StreamKind{
		signal.StreamSilence, signal.StreamSine, signal.StreamWhiteNoise,
		signal.StreamPinkNoise, signal.StreamLinearSweep, signal.StreamLogSweep,
	} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()

			cfg := streamConfig(kind)

			whole, err := signal.NewStreamGenerator(cfg)
			if err != nil {
				t.Fatal(err)
			}

			split, err := signal.NewStreamGenerator(cfg)
			if err != nil {
				t.Fatal(err)
			}

			a, b := make([]float32, cfg.Frames), make([]float32, cfg.Frames)
			if err := whole.GenerateInto32(a); err != nil {
				t.Fatal(err)
			}

			if err := split.GenerateInto32(nil); err != nil || split.Position() != 0 {
				t.Fatalf("empty block: %v", err)
			}

			for start := 0; start < len(b); {
				end := min(len(b), start+1+start%17)
				if err := split.GenerateInto32(b[start:end]); err != nil {
					t.Fatal(err)
				}

				start = end
			}

			if !slices.Equal(a, b) || split.Position() != cfg.Frames {
				t.Fatal("partition mismatch")
			}

			for _, sample := range a {
				if math.Abs(float64(sample)) > cfg.Amplitude {
					t.Fatal("amplitude exceeded", sample)
				}
			}

			dst := []float32{99}
			if err := split.GenerateInto32(dst); err == nil || dst[0] != 99 || split.Position() != cfg.Frames {
				t.Fatal("overrun changed state/output")
			}
		})
	}
}

func TestStreamAnalyticGolden(t *testing.T) {
	t.Parallel()

	for _, kind := range []signal.StreamKind{signal.StreamSilence, signal.StreamSine, signal.StreamLinearSweep, signal.StreamLogSweep} {
		cfg := streamConfig(kind)
		cfg.Frames, cfg.SampleRate, cfg.StartHz, cfg.EndHz, cfg.Amplitude = 4, 4, 1, 2, 1

		g, err := signal.NewStreamGenerator(cfg)
		if err != nil {
			t.Fatal(err)
		}

		got := make([]float32, 4)
		if err := g.GenerateInto32(got); err != nil {
			t.Fatal(err)
		}

		for i, sample := range got {
			time := float64(i) / 4

			var want float64

			switch kind {
			case signal.StreamSine:
				want = []float64{0, 1, 0, -1}[i]
			case signal.StreamLinearSweep:
				want = math.Sin(2 * math.Pi * (time + time*time/2))
			case signal.StreamLogSweep:
				want = math.Sin(2 * math.Pi * (math.Pow(2, time) - 1) / math.Log(2))
			}

			if math.Abs(float64(sample)-want) > 5e-8 {
				t.Fatalf("%s index %d got=%v want=%v", kind, i, sample, want)
			}
		}
	}

	cfg := streamConfig(signal.StreamLogSweep)
	cfg.EndHz = cfg.StartHz

	g, err := signal.NewStreamGenerator(cfg)
	if err != nil {
		t.Fatal(err)
	}

	got := make([]float32, 9)
	if err := g.GenerateInto32(got); err != nil {
		t.Fatal(err)
	}

	for i, sample := range got {
		if sample != float32(.5*math.Sin(2*math.Pi*float64(i)/8)) {
			t.Fatal("equal-frequency sweep differs from sine")
		}
	}
}

func TestStreamNoiseGolden(t *testing.T) {
	t.Parallel()

	// SplitMix64 reference outputs for state zero from the published algorithm.
	// This fixed vector is independent of the generator implementation.
	words := []uint64{0xe220a8397b1dcdaf, 0x6e789e6aa1b965f4, 0x06c45d188009454f, 0xf88bb8a8724c81ec}
	cfg := streamConfig(signal.StreamWhiteNoise)
	cfg.Amplitude = 1

	g, err := signal.NewStreamGenerator(cfg)
	if err != nil {
		t.Fatal(err)
	}

	got := make([]float32, 4)
	if err := g.GenerateInto32(got); err != nil {
		t.Fatal(err)
	}

	for i, word := range words {
		want := float32(float64(word>>11)*(1.0/(1<<52)) - 1)
		if got[i] != want {
			t.Fatalf("white golden index %d got=%v want=%v", i, got[i], want)
		}
	}

	cfg.Kind = signal.StreamPinkNoise

	g, err = signal.NewStreamGenerator(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if err := g.GenerateInto32(got[:2]); err != nil {
		t.Fatal(err)
	}

	// First selector is .88331 (<.91578), updating band 4; second is .02643
	// (<.06378), updating band 2 while retaining band 4.
	band4 := (float64(words[1]>>11)*(1.0/(1<<52)) - 1) * .214463

	band2 := (float64(words[3]>>11)*(1.0/(1<<52)) - 1) * .16380

	weightSum := .23980 + .18727 + .16380 + .194685 + .214463
	if got[0] != float32(band4/weightSum) || got[1] != float32((band4+band2)/weightSum) {
		t.Fatal("pink golden", got[:2])
	}

	for _, kind := range []signal.StreamKind{signal.StreamWhiteNoise, signal.StreamPinkNoise} {
		cfg.Kind, cfg.Seed = kind, 17

		a, err := signal.NewStreamGenerator(cfg)
		if err != nil {
			t.Fatal(err)
		}

		cfg.Seed = 18

		b, err := signal.NewStreamGenerator(cfg)
		if err != nil {
			t.Fatal(err)
		}

		x, y := make([]float32, 64), make([]float32, 64)
		if err := a.GenerateInto32(x); err != nil {
			t.Fatal(err)
		}

		if err := b.GenerateInto32(y); err != nil {
			t.Fatal(err)
		}

		if slices.Equal(x, y) {
			t.Fatal("different seeds produce identical streams", kind)
		}
	}
}

func TestStreamValidation(t *testing.T) {
	t.Parallel()

	for _, mutate := range []func(*signal.StreamConfig){
		func(c *signal.StreamConfig) { c.Kind = "unknown" },
		func(c *signal.StreamConfig) { c.SampleRate = 0 },
		func(c *signal.StreamConfig) { c.SampleRate = math.Inf(1) },
		func(c *signal.StreamConfig) { c.Amplitude = -1 },
		func(c *signal.StreamConfig) { c.Amplitude = math.NaN() },
		func(c *signal.StreamConfig) { c.Amplitude = math.MaxFloat64 },
		func(c *signal.StreamConfig) { c.Frames = 0 },
		func(c *signal.StreamConfig) { c.StartHz = 0 },
		func(c *signal.StreamConfig) { c.StartHz = 5 },
		func(c *signal.StreamConfig) { c.StartHz = math.NaN() },
		func(c *signal.StreamConfig) { c.Kind = signal.StreamLogSweep; c.EndHz = 0 },
		func(c *signal.StreamConfig) { c.Kind = signal.StreamLinearSweep; c.EndHz = math.Inf(1) },
	} {
		cfg := streamConfig(signal.StreamSine)
		mutate(&cfg)

		if _, err := signal.NewStreamGenerator(cfg); err == nil {
			t.Fatalf("accepted invalid configuration %+v", cfg)
		}
	}
}

func TestStreamExtremeLogSweep(t *testing.T) {
	t.Parallel()

	for _, frequencies := range [][2]float64{{math.SmallestNonzeroFloat64, 2}, {2, math.SmallestNonzeroFloat64}} {
		g, err := signal.NewStreamGenerator(signal.StreamConfig{
			Kind: signal.StreamLogSweep, SampleRate: 4, Amplitude: 1,
			StartHz: frequencies[0], EndHz: frequencies[1], Frames: 1000,
		})
		if err != nil {
			t.Fatal(err)
		}

		got := make([]float32, 1000)
		if err := g.GenerateInto32(got); err != nil {
			t.Fatal(err)
		}

		for i, sample := range got {
			if math.IsNaN(float64(sample)) || math.IsInf(float64(sample), 0) || math.Abs(float64(sample)) > 1 {
				t.Fatalf("sample %d is not finite/bounded: %v", i, sample)
			}
		}
	}
}

func TestStreamAllocations(t *testing.T) {
	for _, kind := range []signal.StreamKind{
		signal.StreamSilence, signal.StreamSine, signal.StreamWhiteNoise,
		signal.StreamPinkNoise, signal.StreamLinearSweep, signal.StreamLogSweep,
	} {
		cfg := streamConfig(kind)
		cfg.Frames = 1 << 30

		g, err := signal.NewStreamGenerator(cfg)
		if err != nil {
			t.Fatal(err)
		}

		dst := make([]float32, 4096)

		allocations := testing.AllocsPerRun(10, func() {
			if err := g.GenerateInto32(dst); err != nil {
				t.Fatal(err)
			}
		})
		if allocations != 0 {
			t.Fatalf("%s allocates %v", kind, allocations)
		}
	}
}

func BenchmarkStreamInto32(b *testing.B) {
	for _, kind := range []signal.StreamKind{
		signal.StreamSilence, signal.StreamSine, signal.StreamWhiteNoise,
		signal.StreamPinkNoise, signal.StreamLinearSweep, signal.StreamLogSweep,
	} {
		b.Run(string(kind), func(b *testing.B) {
			cfg := streamConfig(kind)
			cfg.Frames = math.MaxInt64

			g, err := signal.NewStreamGenerator(cfg)
			if err != nil {
				b.Fatal(err)
			}

			dst := make([]float32, 4096)

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				if err := g.GenerateInto32(dst); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func ExampleStreamGenerator() {
	g, err := signal.NewStreamGenerator(signal.StreamConfig{
		Kind: signal.StreamSine, SampleRate: 4, Amplitude: 1, StartHz: 1, Frames: 4,
	})
	if err != nil {
		panic(err)
	}

	first, second := make([]float32, 2), make([]float32, 2)
	if err := g.GenerateInto32(first); err != nil {
		panic(err)
	}

	if err := g.GenerateInto32(second); err != nil {
		panic(err)
	}

	fmt.Printf("%.1f %.1f; position=%d\n", first, second, g.Position())
	// Output: [0.0 1.0] [0.0 -1.0]; position=4
}
