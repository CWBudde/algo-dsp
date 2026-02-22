package effects

import (
	"math"
	"testing"
)

func TestBitCrusherProcessInPlaceMatchesSample(t *testing.T) {
	bc1, err := NewBitCrusher(48000)
	if err != nil {
		t.Fatalf("NewBitCrusher() error = %v", err)
	}

	bc2, err := NewBitCrusher(48000)
	if err != nil {
		t.Fatalf("NewBitCrusher() error = %v", err)
	}

	input := make([]float64, 128)
	for i := range input {
		input[i] = math.Sin(2 * math.Pi * float64(i) / 31)
	}

	want := make([]float64, len(input))
	copy(want, input)

	for i := range want {
		want[i] = bc1.ProcessSample(want[i])
	}

	got := make([]float64, len(input))
	copy(got, input)
	bc2.ProcessInPlace(got)

	for i := range got {
		if diff := math.Abs(got[i] - want[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g diff=%g", i, got[i], want[i], diff)
		}
	}
}

func TestBitCrusherResetRestoresState(t *testing.T) {
	bc, err := NewBitCrusher(48000,
		WithBitCrusherBitDepth(4),
		WithBitCrusherDownsample(4),
	)
	if err != nil {
		t.Fatalf("NewBitCrusher() error = %v", err)
	}

	in := make([]float64, 96)
	for i := range in {
		in[i] = math.Sin(2 * math.Pi * float64(i) / 17)
	}

	out1 := make([]float64, len(in))
	for i := range in {
		out1[i] = bc.ProcessSample(in[i])
	}

	bc.Reset()

	out2 := make([]float64, len(in))
	for i := range in {
		out2[i] = bc.ProcessSample(in[i])
	}

	for i := range out1 {
		if diff := math.Abs(out1[i] - out2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: got=%g want=%g diff=%g", i, out2[i], out1[i], diff)
		}
	}
}

func TestBitCrusherMixZeroIsTransparent(t *testing.T) {
	bc, err := NewBitCrusher(48000,
		WithBitCrusherBitDepth(2),
		WithBitCrusherDownsample(8),
		WithBitCrusherMix(0),
	)
	if err != nil {
		t.Fatalf("NewBitCrusher() error = %v", err)
	}

	for i := 0; i < 512; i++ {
		in := 0.5 * math.Sin(2*math.Pi*440*float64(i)/48000)

		out := bc.ProcessSample(in)
		if diff := math.Abs(out - in); diff > 1e-12 {
			t.Fatalf("sample %d: mix=0 should be transparent, got=%g want=%g", i, out, in)
		}
	}
}

func TestBitCrusherHighBitDepthIsTransparent(t *testing.T) {
	// At 32-bit depth with no downsampling, quantization error should be
	// negligible for signals in [-1, 1].
	bc, err := NewBitCrusher(48000,
		WithBitCrusherBitDepth(32),
		WithBitCrusherDownsample(1),
		WithBitCrusherMix(1),
	)
	if err != nil {
		t.Fatalf("NewBitCrusher() error = %v", err)
	}

	for i := 0; i < 512; i++ {
		in := 0.75 * math.Sin(2*math.Pi*440*float64(i)/48000)
		out := bc.ProcessSample(in)
		// 2^31 levels → quantization step ≈ 4.66e-10
		if diff := math.Abs(out - in); diff > 1e-9 {
			t.Fatalf("sample %d: 32-bit should be near-transparent, got=%g want=%g diff=%g", i, out, in, diff)
		}
	}
}

func TestBitCrusherQuantization(t *testing.T) {
	// 1-bit depth → quantLevels = 2^0 = 1 → output is round(x*1)/1 = round(x)
	// so output should be -1, 0, or 1 for any input in [-1.5, 1.5].
	bc, err := NewBitCrusher(48000,
		WithBitCrusherBitDepth(1),
		WithBitCrusherDownsample(1),
		WithBitCrusherMix(1),
	)
	if err != nil {
		t.Fatalf("NewBitCrusher() error = %v", err)
	}

	tests := []struct {
		input float64
		want  float64
	}{
		{0.0, 0.0},
		{0.3, 0.0},
		{0.5, 1.0}, // math.Round rounds half away from zero: Round(0.5) = 1
		{0.6, 1.0},
		{1.0, 1.0},
		{-0.3, 0.0},
		{-0.6, -1.0},
		{-1.0, -1.0},
	}

	for _, tt := range tests {
		bc.Reset()

		got := bc.ProcessSample(tt.input)
		if got != tt.want {
			t.Errorf("1-bit quantize(%g) = %g, want %g", tt.input, got, tt.want)
		}
	}
}

func TestBitCrusherDownsampleHold(t *testing.T) {
	// With downsample=4, every group of 4 samples should output the same
	// quantized value (the first sample of the group after reset).
	bc, err := NewBitCrusher(48000,
		WithBitCrusherBitDepth(32),
		WithBitCrusherDownsample(4),
		WithBitCrusherMix(1),
	)
	if err != nil {
		t.Fatalf("NewBitCrusher() error = %v", err)
	}

	input := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8}

	output := make([]float64, len(input))
	for i, v := range input {
		output[i] = bc.ProcessSample(v)
	}

	// After reset, holdCounter=0. First ProcessSample increments to 1,
	// which is < 4, so it doesn't update yet — holdValue stays 0.
	// Sample 0: counter 0→1 (<4), hold=0 → out=0
	// Sample 1: counter 1→2 (<4), hold=0 → out=0
	// Sample 2: counter 2→3 (<4), hold=0 → out=0
	// Sample 3: counter 3→4 (>=4), hold=quant(0.4), counter→0 → out=quant(0.4)
	// Sample 4: counter 0→1 (<4), hold=quant(0.4) → out=quant(0.4)
	// Sample 5: counter 1→2 (<4), hold=quant(0.4) → out=quant(0.4)
	// Sample 6: counter 2→3 (<4), hold=quant(0.4) → out=quant(0.4)
	// Sample 7: counter 3→4 (>=4), hold=quant(0.8), counter→0 → out=quant(0.8)

	// First 3 samples should be 0 (initial hold value).
	for i := 0; i < 3; i++ {
		if output[i] != 0 {
			t.Errorf("sample %d: expected 0 (initial hold), got %g", i, output[i])
		}
	}

	// Samples 3-6 should all be the same (quantized value of input[3]).
	held := output[3]
	for i := 3; i < 7; i++ {
		if output[i] != held {
			t.Errorf("sample %d: expected held value %g, got %g", i, held, output[i])
		}
	}

	// Sample 7 picks up a new value.
	if output[7] == held {
		t.Errorf("sample 7: expected new held value, still got %g", held)
	}
}

func TestBitCrusherSilenceProducesSilence(t *testing.T) {
	bc, err := NewBitCrusher(48000,
		WithBitCrusherBitDepth(4),
		WithBitCrusherDownsample(2),
		WithBitCrusherMix(1),
	)
	if err != nil {
		t.Fatalf("NewBitCrusher() error = %v", err)
	}

	for i := 0; i < 256; i++ {
		out := bc.ProcessSample(0)
		if out != 0 {
			t.Fatalf("sample %d: silent input should produce 0, got=%g", i, out)
		}
	}
}

func TestBitCrusherValidation(t *testing.T) {
	tests := []struct {
		name string
		fn   func() error
	}{
		{"zero sample rate", func() error {
			_, err := NewBitCrusher(0)
			return err
		}},
		{"negative sample rate", func() error {
			_, err := NewBitCrusher(-1)
			return err
		}},
		{"NaN sample rate", func() error {
			_, err := NewBitCrusher(math.NaN())
			return err
		}},
		{"Inf sample rate", func() error {
			_, err := NewBitCrusher(math.Inf(1))
			return err
		}},
		{"bit depth below range", func() error {
			_, err := NewBitCrusher(48000, WithBitCrusherBitDepth(0.5))
			return err
		}},
		{"bit depth above range", func() error {
			_, err := NewBitCrusher(48000, WithBitCrusherBitDepth(33))
			return err
		}},
		{"NaN bit depth", func() error {
			_, err := NewBitCrusher(48000, WithBitCrusherBitDepth(math.NaN()))
			return err
		}},
		{"downsample zero", func() error {
			_, err := NewBitCrusher(48000, WithBitCrusherDownsample(0))
			return err
		}},
		{"downsample negative", func() error {
			_, err := NewBitCrusher(48000, WithBitCrusherDownsample(-1))
			return err
		}},
		{"downsample too large", func() error {
			_, err := NewBitCrusher(48000, WithBitCrusherDownsample(257))
			return err
		}},
		{"mix below range", func() error {
			_, err := NewBitCrusher(48000, WithBitCrusherMix(-0.1))
			return err
		}},
		{"mix above range", func() error {
			_, err := NewBitCrusher(48000, WithBitCrusherMix(1.1))
			return err
		}},
		{"NaN mix", func() error {
			_, err := NewBitCrusher(48000, WithBitCrusherMix(math.NaN()))
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}

func TestBitCrusherSetterValidation(t *testing.T) {
	bc, err := NewBitCrusher(48000)
	if err != nil {
		t.Fatalf("NewBitCrusher() error = %v", err)
	}

	if err := bc.SetSampleRate(0); err == nil {
		t.Error("SetSampleRate(0) expected error")
	}

	if err := bc.SetSampleRate(math.NaN()); err == nil {
		t.Error("SetSampleRate(NaN) expected error")
	}

	if err := bc.SetBitDepth(0.5); err == nil {
		t.Error("SetBitDepth(0.5) expected error")
	}

	if err := bc.SetBitDepth(33); err == nil {
		t.Error("SetBitDepth(33) expected error")
	}

	if err := bc.SetDownsample(0); err == nil {
		t.Error("SetDownsample(0) expected error")
	}

	if err := bc.SetDownsample(257); err == nil {
		t.Error("SetDownsample(257) expected error")
	}

	if err := bc.SetMix(-0.1); err == nil {
		t.Error("SetMix(-0.1) expected error")
	}

	if err := bc.SetMix(1.1); err == nil {
		t.Error("SetMix(1.1) expected error")
	}
}

func TestBitCrusherGetters(t *testing.T) {
	bc, err := NewBitCrusher(48000,
		WithBitCrusherBitDepth(12),
		WithBitCrusherDownsample(3),
		WithBitCrusherMix(0.8),
	)
	if err != nil {
		t.Fatalf("NewBitCrusher() error = %v", err)
	}

	if bc.SampleRate() != 48000 {
		t.Errorf("SampleRate() = %g, want 48000", bc.SampleRate())
	}

	if bc.BitDepth() != 12 {
		t.Errorf("BitDepth() = %g, want 12", bc.BitDepth())
	}

	if bc.Downsample() != 3 {
		t.Errorf("Downsample() = %d, want 3", bc.Downsample())
	}

	if bc.Mix() != 0.8 {
		t.Errorf("Mix() = %g, want 0.8", bc.Mix())
	}
}

func TestBitCrusherSettersUpdateState(t *testing.T) {
	bc, err := NewBitCrusher(48000)
	if err != nil {
		t.Fatalf("NewBitCrusher() error = %v", err)
	}

	if err := bc.SetSampleRate(96000); err != nil {
		t.Fatalf("SetSampleRate() error = %v", err)
	}

	if bc.SampleRate() != 96000 {
		t.Errorf("SampleRate() = %g, want 96000", bc.SampleRate())
	}

	if err := bc.SetBitDepth(16); err != nil {
		t.Fatalf("SetBitDepth() error = %v", err)
	}

	if bc.BitDepth() != 16 {
		t.Errorf("BitDepth() = %g, want 16", bc.BitDepth())
	}

	if err := bc.SetDownsample(8); err != nil {
		t.Fatalf("SetDownsample() error = %v", err)
	}

	if bc.Downsample() != 8 {
		t.Errorf("Downsample() = %d, want 8", bc.Downsample())
	}

	if err := bc.SetMix(0.5); err != nil {
		t.Fatalf("SetMix() error = %v", err)
	}

	if bc.Mix() != 0.5 {
		t.Errorf("Mix() = %g, want 0.5", bc.Mix())
	}
}

func TestBitCrusherNilOption(t *testing.T) {
	bc, err := NewBitCrusher(48000, nil)
	if err != nil {
		t.Fatalf("NewBitCrusher() with nil option should not fail: %v", err)
	}

	if bc.BitDepth() != defaultBitCrusherBitDepth {
		t.Errorf("BitDepth() = %g, want default %g", bc.BitDepth(), defaultBitCrusherBitDepth)
	}
}

func TestBitCrusherQuantizationError(t *testing.T) {
	// With N-bit depth, quantization step = 1/2^(N-1).
	// Maximum error should be at most half the step size.
	for _, bits := range []float64{2, 4, 8, 16} {
		bc, err := NewBitCrusher(48000,
			WithBitCrusherBitDepth(bits),
			WithBitCrusherDownsample(1),
			WithBitCrusherMix(1),
		)
		if err != nil {
			t.Fatalf("NewBitCrusher(bits=%g) error = %v", bits, err)
		}

		step := 1.0 / math.Exp2(bits-1)
		maxErr := step / 2

		for i := 0; i < 1000; i++ {
			in := 2.0*float64(i)/999.0 - 1.0 // sweep [-1, 1]

			bc.Reset()

			out := bc.ProcessSample(in)
			if diff := math.Abs(out - in); diff > maxErr+1e-12 {
				t.Errorf("bits=%g sample %d: error %g exceeds max %g (in=%g out=%g)",
					bits, i, diff, maxErr, in, out)
			}
		}
	}
}

func BenchmarkBitCrusherProcessSample(b *testing.B) {
	bc, err := NewBitCrusher(48000,
		WithBitCrusherBitDepth(8),
		WithBitCrusherDownsample(4),
		WithBitCrusherMix(1),
	)
	if err != nil {
		b.Fatalf("NewBitCrusher() error = %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		bc.ProcessSample(0.5)
	}
}

func BenchmarkBitCrusherProcessInPlace(b *testing.B) {
	bc, err := NewBitCrusher(48000,
		WithBitCrusherBitDepth(8),
		WithBitCrusherDownsample(4),
		WithBitCrusherMix(1),
	)
	if err != nil {
		b.Fatalf("NewBitCrusher() error = %v", err)
	}

	buf := make([]float64, 1024)
	for i := range buf {
		buf[i] = math.Sin(2 * math.Pi * float64(i) / 31)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		bc.ProcessInPlace(buf)
	}
}
