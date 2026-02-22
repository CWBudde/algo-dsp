package dither

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestNewQuantizerValidation(t *testing.T) {
	tests := []struct {
		name string
		sr   float64
		opts []Option
	}{
		{"zero sr", 0, nil},
		{"negative sr", -44100, nil},
		{"NaN sr", math.NaN(), nil},
		{"Inf sr", math.Inf(1), nil},
		{"neg Inf sr", math.Inf(-1), nil},
		{"bad bit depth", 44100, []Option{WithBitDepth(0)}},
		{"bad dither type", 44100, []Option{WithDitherType(DitherType(99))}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewQuantizer(tt.sr, tt.opts...)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestNewQuantizerDefaults(t *testing.T) {
	quant, err := NewQuantizer(44100)
	if err != nil {
		t.Fatal(err)
	}

	if quant.BitDepth() != 16 {
		t.Errorf("BitDepth() = %d, want 16", quant.BitDepth())
	}

	if quant.DitherType() != DitherTriangular {
		t.Errorf("DitherType() = %v, want Triangular", quant.DitherType())
	}

	if quant.DitherAmplitude() != 1.0 {
		t.Errorf("DitherAmplitude() = %v, want 1.0", quant.DitherAmplitude())
	}

	if !quant.Limit() {
		t.Error("Limit() should be true by default")
	}

	if quant.SampleRate() != 44100 {
		t.Errorf("SampleRate() = %v, want 44100", quant.SampleRate())
	}
}

func TestQuantizerNilOption(t *testing.T) {
	quant, err := NewQuantizer(44100, nil, WithBitDepth(8), nil)
	if err != nil {
		t.Fatal(err)
	}

	if quant.BitDepth() != 8 {
		t.Errorf("BitDepth = %d, want 8", quant.BitDepth())
	}
}

func TestQuantizerSilencePreservation(t *testing.T) {
	// With no dither and no shaping, zero input produces a constant near-zero
	// output. The +0.5 normalization offset in ProcessSample means the output
	// is (0 + 0.5) * bitDiv, which is non-zero but extremely small.
	quant, err := NewQuantizer(44100,
		WithDitherType(DitherNone),
		WithFIRPreset(PresetNone),
	)
	if err != nil {
		t.Fatal(err)
	}

	for idx := range 100 {
		got := quant.ProcessInteger(0)
		if got != 0 {
			t.Fatalf("sample %d: ProcessInteger(0) = %d, want 0", idx, got)
		}
	}
}

func TestQuantizerProcessInteger(t *testing.T) {
	quant, err := NewQuantizer(44100,
		WithBitDepth(8),
		WithDitherType(DitherNone),
		WithFIRPreset(PresetNone),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Zero input -> zero output.
	got := quant.ProcessInteger(0)
	if got != 0 {
		t.Errorf("ProcessInteger(0) = %d, want 0", got)
	}
}

func TestQuantizerDeterministic(t *testing.T) {
	// Two quantizers with the same RNG seed produce identical output.
	makeQuant := func() *Quantizer {
		quant, err := NewQuantizer(44100,
			WithDitherType(DitherTriangular),
			WithRNG(rand.New(rand.NewPCG(42, 0))),
		)
		if err != nil {
			t.Fatal(err)
		}

		return quant
	}

	quant1 := makeQuant()
	quant2 := makeQuant()

	for idx := range 1000 {
		out1 := quant1.ProcessSample(0.3)
		out2 := quant2.ProcessSample(0.3)

		if out1 != out2 {
			t.Fatalf("sample %d: %v != %v", idx, out1, out2)
		}
	}
}

func TestQuantizerReset(t *testing.T) {
	quant, err := NewQuantizer(44100,
		WithDitherType(DitherNone),
		WithFIRPreset(PresetEFB),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Process some samples to build state.
	for range 100 {
		quant.ProcessSample(0.5)
	}

	quant.Reset()

	// After reset with no dither, zero input should produce zero integer.
	got := quant.ProcessInteger(0)
	if got != 0 {
		t.Errorf("after Reset: ProcessInteger(0) = %d, want 0", got)
	}
}

func TestQuantizerLimiting(t *testing.T) {
	quant, err := NewQuantizer(44100,
		WithBitDepth(8),
		WithDitherType(DitherNone),
		WithFIRPreset(PresetNone),
		WithLimit(true),
	)
	if err != nil {
		t.Fatal(err)
	}

	// 8-bit range: [-128, 127]
	got := quant.ProcessInteger(1.0)
	if got > 127 {
		t.Errorf("ProcessInteger(1.0) = %d, exceeds 127", got)
	}

	got = quant.ProcessInteger(-1.0)
	if got < -128 {
		t.Errorf("ProcessInteger(-1.0) = %d, below -128", got)
	}

	// Extreme overload.
	got = quant.ProcessInteger(100.0)
	if got != 127 {
		t.Errorf("ProcessInteger(100.0) = %d, want 127", got)
	}

	got = quant.ProcessInteger(-100.0)
	if got != -128 {
		t.Errorf("ProcessInteger(-100.0) = %d, want -128", got)
	}
}

func TestQuantizerLimitDisabled(t *testing.T) {
	quant, err := NewQuantizer(44100,
		WithBitDepth(8),
		WithDitherType(DitherNone),
		WithFIRPreset(PresetNone),
		WithLimit(false),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Without limiting, extreme input should exceed the bit range.
	got := quant.ProcessInteger(100.0)
	if got <= 127 {
		t.Errorf("ProcessInteger(100.0) with no limit = %d, expected > 127", got)
	}
}

func TestQuantizerStability(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))

	quant, err := NewQuantizer(44100,
		WithDitherType(DitherTriangular),
		WithFIRPreset(Preset9FC),
		WithLimit(true),
		WithRNG(rng),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Run 10000 hot samples; verify no NaN/Inf.
	inputRng := rand.New(rand.NewPCG(99, 0))

	for idx := range 10000 {
		input := (inputRng.Float64()*2 - 1) * 100 // deliberately clipping
		val := quant.ProcessSample(input)

		if math.IsNaN(val) || math.IsInf(val, 0) {
			t.Fatalf("sample %d: got %v for input %v", idx, val, input)
		}
	}
}

func TestQuantizerProcessInPlaceParity(t *testing.T) {
	// ProcessSample loop and ProcessInPlace must produce identical results.
	rng1 := rand.New(rand.NewPCG(99, 0))
	rng2 := rand.New(rand.NewPCG(99, 0))

	quant1, _ := NewQuantizer(44100,
		WithDitherType(DitherTriangular),
		WithFIRPreset(Preset9FC),
		WithRNG(rng1),
	)

	quant2, _ := NewQuantizer(44100,
		WithDitherType(DitherTriangular),
		WithFIRPreset(Preset9FC),
		WithRNG(rng2),
	)

	// Generate input signal.
	inputRng := rand.New(rand.NewPCG(7, 0))
	input := make([]float64, 512)

	for idx := range input {
		input[idx] = inputRng.Float64()*2 - 1
	}

	// ProcessSample loop.
	sampleResults := make([]float64, len(input))
	for idx, val := range input {
		sampleResults[idx] = quant1.ProcessSample(val)
	}

	// ProcessInPlace.
	buf := make([]float64, len(input))
	copy(buf, input)
	quant2.ProcessInPlace(buf)

	for idx := range buf {
		if buf[idx] != sampleResults[idx] {
			t.Fatalf("sample %d: ProcessInPlace=%v, ProcessSample=%v",
				idx, buf[idx], sampleResults[idx])
		}
	}
}

func TestQuantizerSharpPreset(t *testing.T) {
	rates := []float64{40000, 44100, 48000, 64000, 96000}
	for _, sampleRate := range rates {
		quant, err := NewQuantizer(sampleRate,
			WithSharpPreset(),
			WithDitherType(DitherNone),
		)
		if err != nil {
			t.Fatalf("sr=%g: %v", sampleRate, err)
		}

		// Verify silence preservation at integer level.
		got := quant.ProcessInteger(0)
		if got != 0 {
			t.Errorf("sr=%g: ProcessInteger(0) = %d", sampleRate, got)
		}
	}
}

func TestQuantizerIIRShelf(t *testing.T) {
	quant, err := NewQuantizer(44100,
		WithIIRShelf(10000),
		WithDitherType(DitherNone),
	)
	if err != nil {
		t.Fatal(err)
	}

	got := quant.ProcessInteger(0)
	if got != 0 {
		t.Errorf("ProcessInteger(0) = %d", got)
	}
}
