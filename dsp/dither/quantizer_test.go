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

func TestQuantizerNoiseShapingSpectralEffect(t *testing.T) {
	// Compare quantization noise spectrum with and without noise shaping.
	// With shaping, low-frequency energy should be lower.
	const (
		sampleRate = 44100.0
		numSamples = 8192
		seed       = 42
	)

	// Generate a low-level test signal (sine at 1 kHz, -20 dBFS).
	amplitude := math.Pow(10, -20.0/20.0)
	input := make([]float64, numSamples)

	for idx := range input {
		input[idx] = amplitude * math.Sin(2*math.Pi*1000*float64(idx)/sampleRate)
	}

	quantizeWith := func(shaper NoiseShaper) []float64 {
		quant, err := NewQuantizer(sampleRate,
			WithBitDepth(8), // aggressive quantization for visible noise
			WithDitherType(DitherTriangular),
			WithNoiseShaper(shaper),
			WithRNG(rand.New(rand.NewPCG(seed, 0))),
		)
		if err != nil {
			t.Fatal(err)
		}

		out := make([]float64, len(input))
		copy(out, input)
		quant.ProcessInPlace(out)

		return out
	}

	// Compute quantization noise (output - input).
	outputUnshaped := quantizeWith(NewFIRShaper(nil))
	outputShaped := quantizeWith(NewFIRShaper(Preset9FC.Coefficients()))

	noiseUnshaped := make([]float64, numSamples)
	noiseShaped := make([]float64, numSamples)

	for idx := range numSamples {
		noiseUnshaped[idx] = outputUnshaped[idx] - input[idx]
		noiseShaped[idx] = outputShaped[idx] - input[idx]
	}

	// Compute RMS of low-frequency noise (bins 1..numSamples/8, roughly 0-2.75 kHz).
	lowBins := numSamples / 8

	rmsLow := func(noise []float64) float64 {
		var energy float64

		for bin := 1; bin < lowBins; bin++ {
			var re, im float64

			omega := 2 * math.Pi * float64(bin) / float64(numSamples)

			for sampleIdx, val := range noise {
				re += val * math.Cos(omega*float64(sampleIdx))
				im -= val * math.Sin(omega*float64(sampleIdx))
			}

			energy += re*re + im*im
		}

		return math.Sqrt(energy / float64(lowBins))
	}

	unshaped := rmsLow(noiseUnshaped)
	shaped := rmsLow(noiseShaped)

	t.Logf("low-freq noise RMS: unshaped=%g, shaped=%g, ratio=%g",
		unshaped, shaped, shaped/unshaped)

	// Noise shaping should reduce low-frequency noise by at least 6 dB (ratio < 0.5).
	if shaped >= unshaped*0.5 {
		t.Errorf("noise shaping did not reduce low-freq noise enough: ratio=%g (want < 0.5)",
			shaped/unshaped)
	}
}

func TestQuantizerSetters(t *testing.T) {
	quant, _ := NewQuantizer(44100)

	err := quant.SetBitDepth(24)
	if err != nil {
		t.Fatal(err)
	}

	if quant.BitDepth() != 24 {
		t.Errorf("BitDepth = %d after Set", quant.BitDepth())
	}

	err = quant.SetDitherType(DitherGaussian)
	if err != nil {
		t.Fatal(err)
	}

	if quant.DitherType() != DitherGaussian {
		t.Errorf("DitherType = %v after Set", quant.DitherType())
	}

	err = quant.SetDitherAmplitude(0.5)
	if err != nil {
		t.Fatal(err)
	}

	if quant.DitherAmplitude() != 0.5 {
		t.Errorf("DitherAmplitude = %v after Set", quant.DitherAmplitude())
	}

	quant.SetLimit(false)

	if quant.Limit() {
		t.Error("Limit should be false after Set")
	}
}

func TestQuantizerSetterValidation(t *testing.T) {
	quant, _ := NewQuantizer(44100)

	err := quant.SetBitDepth(0)
	if err == nil {
		t.Error("expected error for SetBitDepth(0)")
	}

	err = quant.SetBitDepth(33)
	if err == nil {
		t.Error("expected error for SetBitDepth(33)")
	}

	err = quant.SetDitherType(DitherType(99))
	if err == nil {
		t.Error("expected error for invalid DitherType")
	}

	err = quant.SetDitherAmplitude(-1)
	if err == nil {
		t.Error("expected error for negative amplitude")
	}

	err = quant.SetDitherAmplitude(math.NaN())
	if err == nil {
		t.Error("expected error for NaN amplitude")
	}
}

func TestQuantizerAllDitherTypes(t *testing.T) {
	types := []DitherType{
		DitherNone, DitherRectangular, DitherTriangular,
		DitherGaussian, DitherFastGaussian,
	}
	for _, ditherType := range types {
		t.Run(ditherType.String(), func(t *testing.T) {
			quant, err := NewQuantizer(44100,
				WithDitherType(ditherType),
				WithRNG(rand.New(rand.NewPCG(42, 0))),
			)
			if err != nil {
				t.Fatal(err)
			}

			// Run 1000 samples, verify no NaN/Inf.
			for idx := range 1000 {
				val := quant.ProcessSample(0.3)
				if math.IsNaN(val) || math.IsInf(val, 0) {
					t.Fatalf("sample %d: %v", idx, val)
				}
			}
		})
	}
}

func TestQuantizerBitDepthRange(t *testing.T) {
	// Verify all valid bit depths work.
	for bits := 1; bits <= 32; bits++ {
		quant, err := NewQuantizer(44100,
			WithBitDepth(bits),
			WithDitherType(DitherNone),
			WithFIRPreset(PresetNone),
		)
		if err != nil {
			t.Fatalf("bitDepth=%d: %v", bits, err)
		}

		got := quant.ProcessInteger(0)
		if got != 0 {
			t.Errorf("bitDepth=%d: ProcessInteger(0) = %d", bits, got)
		}
	}
}
