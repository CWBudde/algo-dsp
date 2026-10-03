package signal

import (
	"math"
	"testing"
)

func TestStreamOptimizedBitsMatchOriginal(t *testing.T) {
	t.Parallel()

	for _, kind := range []StreamKind{StreamSilence, StreamSine, StreamWhiteNoise, StreamPinkNoise, StreamLinearSweep, StreamLogSweep} {
		for _, position := range []int64{0, 1234567, (1 << 53) - 17, (1 << 53) + 17} {
			cfg := StreamConfig{
				Kind: kind, SampleRate: 48000, Amplitude: .7, StartHz: 1000,
				EndHz: 10000, Frames: math.MaxInt64, Seed: 775533,
			}

			got, err := NewStreamGenerator(cfg)
			if err != nil {
				t.Fatal(err)
			}

			reference, err := NewStreamGenerator(cfg)
			if err != nil {
				t.Fatal(err)
			}

			got.position, reference.position = position, position

			for _, size := range []int{0, 3, 17, 97, 4096} {
				out := make([]float32, size)
				if err := got.GenerateInto32(out); err != nil {
					t.Fatal(err)
				}

				for i, sample := range out {
					want := float32(cfg.Amplitude * reference.sample(reference.position+int64(i)))
					if math.Float32bits(sample) != math.Float32bits(want) {
						t.Fatalf("%s position%d size%d index%d got%v want%v", kind, position, size, i, sample, want)
					}
				}

				reference.position += int64(size)
				if got.position != reference.position || got.random != reference.random || got.pink != reference.pink {
					t.Fatal("optimized stream state mismatch")
				}
			}
		}
	}
}

func TestSplitMixUniformMatchesOriginalConversion(t *testing.T) {
	t.Parallel()

	for _, seed := range []uint64{0, 1, math.MaxUint64, 775533} {
		g := &StreamGenerator{random: seed}

		state := seed
		for index := range 10000 {
			state += 0x9e3779b97f4a7c15
			z := state
			z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
			z = (z ^ (z >> 27)) * 0x94d049bb133111eb
			z ^= z >> 31

			want := float64(z>>11) * (1.0 / (1 << 53))
			if got := g.uniform(); math.Float64bits(got) != math.Float64bits(want) || g.random != state {
				t.Fatalf("seed%d index%d got%v want%v", seed, index, got, want)
			}
		}
	}
}
