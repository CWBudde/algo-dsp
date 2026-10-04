package restoration

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestInvalidControlsAndAtomicRejection(t *testing.T) {
	for _, size := range []int{0, 255, 257, 16384} {
		if _, err := NewNoiseProfile(size, 48000); err == nil {
			t.Fatal(size)
		}
	}

	for _, rate := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, err := NewNoiseProfile(1024, rate); err == nil {
			t.Fatal(rate)
		}
	}

	p, _ := NewNoiseProfile(256, 48000)
	bins := make([]complex128, 129)
	bins[2] = 2

	if p.AddSpectrum(nil) == nil {
		t.Fatal("short bins")
	}

	if _, err := NewNoiseReducer(p, 24, "wiener"); err == nil {
		t.Fatal("empty profile")
	}

	_ = p.AddSpectrum(bins)

	_ = p.AddSpectrum(bins)
	for _, cfg := range []struct {
		db   float64
		mode string
	}{{-1, "wiener"}, {61, "wiener"}, {math.NaN(), "wiener"}, {24, "bad"}} {
		if _, err := NewNoiseReducer(p, cfg.db, cfg.mode); err == nil {
			t.Fatal(cfg)
		}
	}

	r, _ := NewNoiseReducer(p, 24, "gate")
	if r.ProcessSpectrum(nil) == nil {
		t.Fatal("short")
	}

	bins[3] = complex(math.Inf(1), 0)
	if r.ProcessSpectrum(bins) == nil {
		t.Fatal("infinite")
	}

	bins[3] = 0

	if r.gain[2] != r.floor {
		t.Fatal("mutated")
	}

	var nilProfile *NoiseProfile
	if nilProfile.Frames() != 0 {
		t.Fatal("nil frames")
	}

	for _, mask := range []Mask{{}, {Start: -1, End: 10, HighHz: 100}, {End: 101, HighHz: 100}, {End: 10, HighHz: 24001}, {End: 10, LowHz: math.NaN(), HighHz: 100}, {End: 10, HighHz: 100, Points: []Point{{0, 0}}}, {End: 10, HighHz: 100, Points: []Point{{0, 0}, {1, 0}, {100, 100}}}} {
		if mask.Validate(100, 48000) == nil {
			t.Fatal(mask)
		}
	}

	if (Mask{End: 10, HighHz: 100}).Contains(11, 50) {
		t.Fatal("outside")
	}

	for _, input := range [][]float64{nil, make([]float64, 10), {0, 0, math.NaN(), 0, 0}} {
		if RepairGap(input, 2, 3) == nil && len(input) != 10 {
			t.Fatal("bad gap")
		}
	}

	x := make([]float64, 100)
	if RepairGap(x, 2, 4) != nil {
		t.Fatal("fallback")
	}

	if RepairGap(x, 0, 4) == nil || RepairGap(x, 2, 2) == nil {
		t.Fatal("invalid interval")
	}

	if _, ok := solve(make([]float64, 4), make([]float64, 2), 2); ok {
		t.Fatal("singular")
	}

	for _, v := range []float64{0, 31, math.NaN()} {
		if RepairClicks(x, v, 32) == nil {
			t.Fatal("bad clicks")
		}
	}

	if RepairClicks(x, 8, 0) == nil || Declip(x, 0, 32) == nil || Declip(x, 0.9, 257) == nil {
		t.Fatal("limits")
	}

	x[50] = math.NaN()
	if RepairGap(x, 40, 45) == nil || RepairClicks(x, 8, 32) == nil || Declip(x, 0.9, 32) == nil {
		t.Fatal("nonfinite")
	}

	for _, cfg := range []struct {
		rate, hz, q float64
		n           int
	}{{0, 50, 30, 8}, {48000, 55, 30, 8}, {48000, 50, 0, 8}, {48000, 50, 30, 0}, {48000, 50, 30, 17}} {
		if _, err := NewHumRemover(cfg.rate, cfg.hz, cfg.q, cfg.n); err == nil {
			t.Fatal(cfg)
		}
	}

	h, _ := NewHumRemover(48000, 60, 30, 8)
	if h.ProcessInPlace(x) == nil {
		t.Fatal("nan hum")
	}

	var nilHum *HumRemover
	if nilHum.ProcessInPlace(nil) == nil {
		t.Fatal("nil hum")
	}
}

func TestSpectralFailurePaths(t *testing.T) {
	base := SpectralConfig{Mode: "remove", FFTSize: 1024, SampleRate: 48000, Mask: Mask{End: 1000, HighHz: 24000}}
	read := func(dst []float64, _ int64) int { return len(dst) }

	for _, change := range []func(*SpectralConfig){func(c *SpectralConfig) { c.Mode = "bad" }, func(c *SpectralConfig) { c.FFTSize = 99 }, func(c *SpectralConfig) { c.SampleRate = 1 }, func(c *SpectralConfig) { c.GainDB = 1 }, func(c *SpectralConfig) { c.Mask.End = 0 }, func(c *SpectralConfig) { c.Mode = "noise" }, func(c *SpectralConfig) { c.Mode = "heal" }} {
		c := base
		change(&c)

		if _, err := NewSpectralProcessor(1000, read, c); err == nil {
			t.Fatal(c)
		}
	}

	if _, err := NewSpectralProcessor(0, read, base); err == nil {
		t.Fatal("empty")
	}

	if _, err := NewSpectralProcessor(1000, nil, base); err == nil {
		t.Fatal("nil read")
	}

	for _, reader := range []Reader{func([]float64, int64) int { return 0 }, func(dst []float64, _ int64) int { dst[0] = math.NaN(); return len(dst) }} {
		p, _ := NewSpectralProcessor(1000, reader, base)
		if _, err := p.Step(context.Background(), func(int64, []float64) error { return nil }); err == nil {
			t.Fatal("invalid source")
		}

		if _, err := p.Step(context.Background(), func(int64, []float64) error { return nil }); err == nil {
			t.Fatal("terminal")
		}
	}

	p, _ := NewSpectralProcessor(1000, read, base)
	//nolint:staticcheck // Deliberately exercise nil-context rejection.
	if _, err := p.Step(nil, func(int64, []float64) error { return nil }); err == nil {
		t.Fatal("nil context")
	}

	if _, err := p.Step(context.Background(), nil); err == nil {
		t.Fatal("nil callback")
	}

	failure := errors.New("sink failed")

	for {
		_, err := p.Step(context.Background(), func(int64, []float64) error { return failure })
		if err != nil {
			if !errors.Is(err, failure) {
				t.Fatal(err)
			}

			break
		}
	}

	c := base
	c.Mode = "heal"
	c.Mask = Mask{Start: 400, End: 404, HighHz: 24000}

	p, _ = NewSpectralProcessor(1000, func(dst []float64, start int64) int {
		if len(dst) == 516 {
			return 0
		}

		return len(dst)
	}, c)
	if _, err := p.Step(context.Background(), func(int64, []float64) error { return nil }); err == nil {
		t.Fatal("short repair context")
	}
}
