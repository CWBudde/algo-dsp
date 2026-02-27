//revive:disable:var-naming
package signal

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/core"
)

func TestSineLength(t *testing.T) {
	generator := NewGenerator(core.WithSampleRate(48000))

	sine, err := generator.Sine(1000, 1, 64)
	if err != nil {
		t.Fatalf("Sine() error = %v", err)
	}

	if len(sine) != 64 {
		t.Fatalf("len = %d, want 64", len(sine))
	}
}

func TestWhiteNoiseDeterministic(t *testing.T) {
	generator1 := NewGeneratorWithOptions(nil, WithSeed(42))
	generator2 := NewGeneratorWithOptions(nil, WithSeed(42))

	noise1, err := generator1.WhiteNoise(1, 16)
	if err != nil {
		t.Fatalf("WhiteNoise() error = %v", err)
	}

	noise2, err := generator2.WhiteNoise(1, 16)
	if err != nil {
		t.Fatalf("WhiteNoise() error = %v", err)
	}

	for i := range noise1 {
		if noise1[i] != noise2[i] {
			t.Fatalf("noise mismatch at %d: %v != %v", i, noise1[i], noise2[i])
		}
	}
}

func TestSetSeed(t *testing.T) {
	generator := NewGenerator()
	generator.SetSeed(99)

	if generator.Seed() != 99 {
		t.Fatalf("Seed()=%d, want 99", generator.Seed())
	}

	whiteNoise1, err := generator.WhiteNoise(1, 8)
	if err != nil {
		t.Fatalf("WhiteNoise() error = %v", err)
	}

	generator.SetSeed(100)

	whiteNoise2, err := generator.WhiteNoise(1, 8)
	if err != nil {
		t.Fatalf("WhiteNoise() error = %v", err)
	}

	same := true

	for i := range whiteNoise1 {
		if whiteNoise1[i] != whiteNoise2[i] {
			same = false
			break
		}
	}

	if same {
		t.Fatal("expected different seeds to produce different noise")
	}
}

func TestPinkNoiseDeterministic(t *testing.T) {
	generator1 := NewGeneratorWithOptions(nil, WithSeed(42))
	generator2 := NewGeneratorWithOptions(nil, WithSeed(42))

	pinkNoise1, err := generator1.PinkNoise(1, 128)
	if err != nil {
		t.Fatalf("PinkNoise() error = %v", err)
	}

	pinkNoise2, err := generator2.PinkNoise(1, 128)
	if err != nil {
		t.Fatalf("PinkNoise() error = %v", err)
	}

	for i := range pinkNoise1 {
		if pinkNoise1[i] != pinkNoise2[i] {
			t.Fatalf("pink noise mismatch at %d: %v != %v", i, pinkNoise1[i], pinkNoise2[i])
		}
	}
}

func TestPinkNoiseLength(t *testing.T) {
	g := NewGeneratorWithOptions(nil, WithSeed(1))

	out, err := g.PinkNoise(1, 256)
	if err != nil {
		t.Fatalf("PinkNoise() error = %v", err)
	}

	if len(out) != 256 {
		t.Fatalf("len = %d, want 256", len(out))
	}
}

func TestPinkNoiseBounded(t *testing.T) {
	g := NewGeneratorWithOptions(nil, WithSeed(7))
	amp := 0.5

	out, err := g.PinkNoise(amp, 10000)
	if err != nil {
		t.Fatalf("PinkNoise() error = %v", err)
	}

	// Sum of all 5 band weights ≈ 1.0, so theoretical max is amplitude * 1.0.
	// Allow a small margin for floating-point accumulation.
	limit := amp * 1.1
	for i, v := range out {
		if v > limit || v < -limit {
			t.Fatalf("sample[%d] = %v exceeds limit ±%v", i, v, limit)
		}
	}
}

func TestPinkNoiseDifferentSeeds(t *testing.T) {
	generator1 := NewGeneratorWithOptions(nil, WithSeed(1))
	generator2 := NewGeneratorWithOptions(nil, WithSeed(2))

	pinkNoise1, err := generator1.PinkNoise(1, 64)
	if err != nil {
		t.Fatalf("PinkNoise() error = %v", err)
	}

	pinkNoise2, err := generator2.PinkNoise(1, 64)
	if err != nil {
		t.Fatalf("PinkNoise() error = %v", err)
	}

	same := true

	for i := range pinkNoise1 {
		if pinkNoise1[i] != pinkNoise2[i] {
			same = false
			break
		}
	}

	if same {
		t.Fatal("expected different seeds to produce different pink noise")
	}
}

func TestPinkNoiseSpectralSlope(t *testing.T) {
	// Generate a long pink noise signal and verify approximate -3 dB/octave slope.
	generator := NewGeneratorWithOptions(
		[]core.ProcessorOption{core.WithSampleRate(48000)},
		WithSeed(42),
	)
	n := 1 << 16 // 65536 samples

	out, err := generator.PinkNoise(1, n)
	if err != nil {
		t.Fatalf("PinkNoise() error = %v", err)
	}

	// Compute average power in octave bands using sampled DFT bins.
	// We sample a fixed number of bins per band to keep the test fast.
	sampleRate := 48000.0
	bands := []float64{500, 1000, 2000, 4000}
	powers := make([]float64, len(bands))

	const binsPerBand = 8

	for bi, fc := range bands {
		loK := int(fc / math.Sqrt2 * float64(n) / sampleRate)
		hiK := int(fc * math.Sqrt2 * float64(n) / sampleRate)

		if loK < 1 {
			loK = 1
		}

		if hiK >= n/2 {
			hiK = n/2 - 1
		}

		step := max((hiK-loK)/binsPerBand, 1)

		power := 0.0
		count := 0

		for k := loK; k <= hiK; k += step {
			re, im := 0.0, 0.0

			freq := 2 * math.Pi * float64(k) / float64(n)
			for i, v := range out {
				re += v * math.Cos(freq*float64(i))
				im -= v * math.Sin(freq*float64(i))
			}

			power += re*re + im*im
			count++
		}

		if count > 0 {
			powers[bi] = power / float64(count)
		}
	}

	// Check slope between adjacent octave bands.
	// Pink noise: -3 dB/octave → power ratio ≈ 0.5 per octave.
	// Allow wide tolerance since this is stochastic.
	for i := range len(powers) - 1 {
		if powers[i] == 0 || powers[i+1] == 0 {
			continue
		}

		ratioDb := 10 * math.Log10(powers[i+1]/powers[i])
		// Expect roughly -3 dB, allow ±5 dB tolerance for stochastic signal.
		if ratioDb > 2 || ratioDb < -8 {
			t.Errorf("octave slope from %.0f to %.0f Hz: %.1f dB (want ≈ -3 dB)",
				bands[i], bands[i+1], ratioDb)
		}
	}
}

func TestNormalize(t *testing.T) {
	out, err := Normalize([]float64{-0.5, 1.0, -0.25}, 0.5)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if out[1] != 0.5 {
		t.Fatalf("peak = %v, want 0.5", out[1])
	}
}

func TestMultisineLength(t *testing.T) {
	g := NewGenerator(core.WithSampleRate(48000))

	out, err := g.Multisine([]float64{1000, 2000}, 1, 64)
	if err != nil {
		t.Fatalf("Multisine() error = %v", err)
	}

	if len(out) != 64 {
		t.Fatalf("len = %d, want 64", len(out))
	}
}

func TestImpulse(t *testing.T) {
	g := NewGenerator()

	out, err := g.Impulse(0.75, 8, 3)
	if err != nil {
		t.Fatalf("Impulse() error = %v", err)
	}

	for i, v := range out {
		want := 0.0
		if i == 3 {
			want = 0.75
		}

		if v != want {
			t.Fatalf("out[%d]=%v, want %v", i, v, want)
		}
	}
}

func TestLinearSweepLength(t *testing.T) {
	g := NewGenerator(core.WithSampleRate(48000))

	out, err := g.LinearSweep(20, 20000, 1, 128)
	if err != nil {
		t.Fatalf("LinearSweep() error = %v", err)
	}

	if len(out) != 128 {
		t.Fatalf("len = %d, want 128", len(out))
	}
}

func TestLogSweepLength(t *testing.T) {
	g := NewGenerator(core.WithSampleRate(48000))

	out, err := g.LogSweep(20, 20000, 1, 128)
	if err != nil {
		t.Fatalf("LogSweep() error = %v", err)
	}

	if len(out) != 128 {
		t.Fatalf("len = %d, want 128", len(out))
	}
}

func TestClip(t *testing.T) {
	out, err := Clip([]float64{-2, -0.5, 0.25, 2}, -1, 1)
	if err != nil {
		t.Fatalf("Clip() error = %v", err)
	}

	want := []float64{-1, -0.5, 0.25, 1}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("out[%d]=%v, want %v", i, out[i], want[i])
		}
	}
}

func TestRemoveDC(t *testing.T) {
	out, err := RemoveDC([]float64{1, 2, 3, 4})
	if err != nil {
		t.Fatalf("RemoveDC() error = %v", err)
	}

	sum := 0.0
	for _, v := range out {
		sum += v
	}

	if math.Abs(sum) > 1e-12 {
		t.Fatalf("sum=%v, want near 0", sum)
	}
}

func TestEnvelopeFollower(t *testing.T) {
	in := []float64{0, 1, 0, 1, 0}

	out, err := EnvelopeFollower(in, 1.0, 0.5)
	if err != nil {
		t.Fatalf("EnvelopeFollower() error = %v", err)
	}

	if out[0] != 0 || out[1] != 1 {
		t.Fatalf("unexpected attack behavior: %+v", out)
	}

	if !(out[2] < out[1] && out[2] > 0) {
		t.Fatalf("unexpected release behavior: %+v", out)
	}
}
