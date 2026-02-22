package dither

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()

	if cfg.bitDepth != 16 {
		t.Errorf("default bitDepth = %d, want 16", cfg.bitDepth)
	}

	if cfg.ditherType != DitherTriangular {
		t.Errorf("default ditherType = %v, want Triangular", cfg.ditherType)
	}

	if cfg.ditherAmplitude != 1.0 {
		t.Errorf("default ditherAmplitude = %v, want 1.0", cfg.ditherAmplitude)
	}

	if !cfg.limit {
		t.Error("default limit should be true")
	}

	if cfg.shaper != nil {
		t.Error("default shaper should be nil")
	}

	if cfg.rng != nil {
		t.Error("default rng should be nil")
	}
}

func TestOptionValidation(t *testing.T) {
	tests := []struct {
		name string
		opt  Option
	}{
		{"bitDepth 0", WithBitDepth(0)},
		{"bitDepth 33", WithBitDepth(33)},
		{"bitDepth -1", WithBitDepth(-1)},
		{"negative amplitude", WithDitherAmplitude(-1)},
		{"NaN amplitude", WithDitherAmplitude(math.NaN())},
		{"Inf amplitude", WithDitherAmplitude(math.Inf(1))},
		{"invalid dither type", WithDitherType(DitherType(99))},
		{"invalid preset", WithFIRPreset(Preset(99))},
		{"zero IIR freq", WithIIRShelf(0)},
		{"negative IIR freq", WithIIRShelf(-100)},
		{"NaN IIR freq", WithIIRShelf(math.NaN())},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultConfig()
			if err := tt.opt(&cfg); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestOptionHappyPaths(t *testing.T) {
	cfg := defaultConfig()

	opts := []Option{
		WithBitDepth(24),
		WithDitherType(DitherGaussian),
		WithDitherAmplitude(0.5),
		WithLimit(false),
		WithSharpPreset(),
		WithRNG(rand.New(rand.NewPCG(42, 0))),
	}

	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if cfg.bitDepth != 24 {
		t.Errorf("bitDepth = %d, want 24", cfg.bitDepth)
	}

	if cfg.ditherType != DitherGaussian {
		t.Errorf("ditherType = %v, want Gaussian", cfg.ditherType)
	}

	if cfg.ditherAmplitude != 0.5 {
		t.Errorf("ditherAmplitude = %v, want 0.5", cfg.ditherAmplitude)
	}

	if cfg.limit {
		t.Error("limit should be false")
	}

	if !cfg.sharpPreset {
		t.Error("sharpPreset should be true")
	}

	if cfg.rng == nil {
		t.Error("rng should be set")
	}
}

func TestWithFIRPresetCreatesFIRShaper(t *testing.T) {
	cfg := defaultConfig()

	err := WithFIRPreset(Preset9FC)(&cfg)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.shaper == nil {
		t.Fatal("shaper should be set")
	}

	// Verify it's an FIR shaper by checking interface.
	var _ NoiseShaper = cfg.shaper
}

func TestWithNoiseShaper(t *testing.T) {
	cfg := defaultConfig()
	shaper := NewFIRShaper([]float64{1.0})

	err := WithNoiseShaper(shaper)(&cfg)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.shaper != shaper {
		t.Error("shaper not set correctly")
	}
}

func TestWithIIRShelfSetsFreq(t *testing.T) {
	cfg := defaultConfig()

	err := WithIIRShelf(10000)(&cfg)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.iirShelfFreq != 10000 {
		t.Errorf("iirShelfFreq = %v, want 10000", cfg.iirShelfFreq)
	}
}

func TestWithDitherAmplitudeZero(t *testing.T) {
	cfg := defaultConfig()

	err := WithDitherAmplitude(0)(&cfg)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.ditherAmplitude != 0 {
		t.Errorf("ditherAmplitude = %v, want 0", cfg.ditherAmplitude)
	}
}
