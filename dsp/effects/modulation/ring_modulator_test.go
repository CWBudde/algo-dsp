package modulation

import (
	"math"
	"testing"
)

func TestRingModulatorProcessInPlaceMatchesProcess(t *testing.T) {
	r1, err := NewRingModulator(48000)
	if err != nil {
		t.Fatalf("NewRingModulator() error = %v", err)
	}

	r2, err := NewRingModulator(48000)
	if err != nil {
		t.Fatalf("NewRingModulator() error = %v", err)
	}

	input := make([]float64, 128)
	for i := range input {
		input[i] = math.Sin(2 * math.Pi * float64(i) / 31)
	}

	want := make([]float64, len(input))
	copy(want, input)

	for i := range want {
		want[i] = r1.Process(want[i])
	}

	got := make([]float64, len(input))
	copy(got, input)
	r2.ProcessInPlace(got)

	for i := range got {
		if diff := math.Abs(got[i] - want[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g diff=%g", i, got[i], want[i], diff)
		}
	}
}

func TestRingModulatorResetRestoresState(t *testing.T) {
	rm, err := NewRingModulator(48000,
		WithRingModCarrierHz(300),
	)
	if err != nil {
		t.Fatalf("NewRingModulator() error = %v", err)
	}

	in := make([]float64, 96)
	in[0] = 1

	out1 := make([]float64, len(in))
	for i := range in {
		out1[i] = rm.Process(in[i])
	}

	rm.Reset()

	out2 := make([]float64, len(in))
	for i := range in {
		out2[i] = rm.Process(in[i])
	}

	for i := range out1 {
		if diff := math.Abs(out1[i] - out2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: got=%g want=%g diff=%g", i, out2[i], out1[i], diff)
		}
	}
}

func TestRingModulatorMixZeroIsTransparent(t *testing.T) {
	rm, err := NewRingModulator(48000,
		WithRingModCarrierHz(1000),
		WithRingModMix(0),
	)
	if err != nil {
		t.Fatalf("NewRingModulator() error = %v", err)
	}

	for i := 0; i < 512; i++ {
		in := 0.5 * math.Sin(2*math.Pi*440*float64(i)/48000)

		out := rm.Process(in)
		if diff := math.Abs(out - in); diff > 1e-12 {
			t.Fatalf("sample %d: mix=0 should be transparent, got=%g want=%g", i, out, in)
		}
	}
}

func TestRingModulatorDCInputProducesSine(t *testing.T) {
	// Ring modulating a DC signal (constant 1.0) with a carrier should
	// produce a pure sine wave at the carrier frequency.
	const (
		sampleRate = 48000.0
		carrierHz  = 100.0
		nSamples   = 480 // one full carrier cycle at 100 Hz / 48000 Hz
	)

	rm, err := NewRingModulator(sampleRate,
		WithRingModCarrierHz(carrierHz),
		WithRingModMix(1),
	)
	if err != nil {
		t.Fatalf("NewRingModulator() error = %v", err)
	}

	for i := 0; i < nSamples; i++ {
		got := rm.Process(1.0)
		phase := 2 * math.Pi * carrierHz * float64(i) / sampleRate

		want := math.Sin(phase)
		if diff := math.Abs(got - want); diff > 1e-9 {
			t.Fatalf("sample %d: got=%g want=%g diff=%g", i, got, want, diff)
		}
	}
}

func TestRingModulatorSilenceInputProducesSilence(t *testing.T) {
	rm, err := NewRingModulator(48000,
		WithRingModCarrierHz(1000),
		WithRingModMix(1),
	)
	if err != nil {
		t.Fatalf("NewRingModulator() error = %v", err)
	}

	for i := 0; i < 256; i++ {
		out := rm.Process(0)
		if out != 0 {
			t.Fatalf("sample %d: silent input should produce 0, got=%g", i, out)
		}
	}
}

func TestRingModulatorSumDifferenceFrequencies(t *testing.T) {
	// A ring modulator of sin(A) * sin(B) = 0.5*[cos(A-B) - cos(A+B)].
	// With input at 400 Hz and carrier at 100 Hz, we expect energy at
	// 300 Hz (difference) and 500 Hz (sum), but not at 400 Hz or 100 Hz.
	const (
		sampleRate = 48000.0
		inputHz    = 400.0
		carrierHz  = 100.0
		nSamples   = 4800 // 100 ms
	)

	rm, err := NewRingModulator(sampleRate,
		WithRingModCarrierHz(carrierHz),
		WithRingModMix(1),
	)
	if err != nil {
		t.Fatalf("NewRingModulator() error = %v", err)
	}

	output := make([]float64, nSamples)
	for i := range output {
		in := math.Sin(2 * math.Pi * inputHz * float64(i) / sampleRate)
		output[i] = rm.Process(in)
	}

	// Goertzel-style magnitude at specific frequencies.
	mag := func(freq float64) float64 {
		var sinSum, cosSum float64

		for i, v := range output {
			angle := 2 * math.Pi * freq * float64(i) / sampleRate
			cosSum += v * math.Cos(angle)
			sinSum += v * math.Sin(angle)
		}

		return math.Sqrt(cosSum*cosSum+sinSum*sinSum) / float64(nSamples)
	}

	sumMag := mag(inputHz + carrierHz)  // 500 Hz
	diffMag := mag(inputHz - carrierHz) // 300 Hz
	inputMag := mag(inputHz)            // 400 Hz - should be suppressed
	carrierMag := mag(carrierHz)        // 100 Hz - should be absent

	if sumMag < 0.2 {
		t.Errorf("expected strong sum frequency (500 Hz), got magnitude=%g", sumMag)
	}

	if diffMag < 0.2 {
		t.Errorf("expected strong difference frequency (300 Hz), got magnitude=%g", diffMag)
	}

	if inputMag > 0.01 {
		t.Errorf("expected suppressed input frequency (400 Hz), got magnitude=%g", inputMag)
	}

	if carrierMag > 0.01 {
		t.Errorf("expected absent carrier frequency (100 Hz), got magnitude=%g", carrierMag)
	}
}

func TestRingModulatorValidation(t *testing.T) {
	tests := []struct {
		name string
		fn   func() error
	}{
		{"zero sample rate", func() error {
			_, err := NewRingModulator(0)
			return err
		}},
		{"negative sample rate", func() error {
			_, err := NewRingModulator(-1)
			return err
		}},
		{"NaN sample rate", func() error {
			_, err := NewRingModulator(math.NaN())
			return err
		}},
		{"Inf sample rate", func() error {
			_, err := NewRingModulator(math.Inf(1))
			return err
		}},
		{"zero carrier Hz", func() error {
			_, err := NewRingModulator(48000, WithRingModCarrierHz(0))
			return err
		}},
		{"negative carrier Hz", func() error {
			_, err := NewRingModulator(48000, WithRingModCarrierHz(-100))
			return err
		}},
		{"NaN carrier Hz", func() error {
			_, err := NewRingModulator(48000, WithRingModCarrierHz(math.NaN()))
			return err
		}},
		{"mix below range", func() error {
			_, err := NewRingModulator(48000, WithRingModMix(-0.1))
			return err
		}},
		{"mix above range", func() error {
			_, err := NewRingModulator(48000, WithRingModMix(1.1))
			return err
		}},
		{"NaN mix", func() error {
			_, err := NewRingModulator(48000, WithRingModMix(math.NaN()))
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

func TestRingModulatorSetterValidation(t *testing.T) {
	rm, err := NewRingModulator(48000)
	if err != nil {
		t.Fatalf("NewRingModulator() error = %v", err)
	}

	if err := rm.SetSampleRate(0); err == nil {
		t.Error("SetSampleRate(0) expected error")
	}

	if err := rm.SetSampleRate(math.NaN()); err == nil {
		t.Error("SetSampleRate(NaN) expected error")
	}

	if err := rm.SetCarrierHz(0); err == nil {
		t.Error("SetCarrierHz(0) expected error")
	}

	if err := rm.SetCarrierHz(-1); err == nil {
		t.Error("SetCarrierHz(-1) expected error")
	}

	if err := rm.SetMix(-0.1); err == nil {
		t.Error("SetMix(-0.1) expected error")
	}

	if err := rm.SetMix(1.1); err == nil {
		t.Error("SetMix(1.1) expected error")
	}
}

func TestRingModulatorGetters(t *testing.T) {
	rm, err := NewRingModulator(48000,
		WithRingModCarrierHz(300),
		WithRingModMix(0.7),
	)
	if err != nil {
		t.Fatalf("NewRingModulator() error = %v", err)
	}

	if rm.SampleRate() != 48000 {
		t.Errorf("SampleRate() = %g, want 48000", rm.SampleRate())
	}

	if rm.CarrierHz() != 300 {
		t.Errorf("CarrierHz() = %g, want 300", rm.CarrierHz())
	}

	if rm.Mix() != 0.7 {
		t.Errorf("Mix() = %g, want 0.7", rm.Mix())
	}
}

func TestRingModulatorSettersUpdateState(t *testing.T) {
	rm, err := NewRingModulator(48000)
	if err != nil {
		t.Fatalf("NewRingModulator() error = %v", err)
	}

	if err := rm.SetSampleRate(96000); err != nil {
		t.Fatalf("SetSampleRate() error = %v", err)
	}

	if rm.SampleRate() != 96000 {
		t.Errorf("SampleRate() = %g, want 96000", rm.SampleRate())
	}

	if err := rm.SetCarrierHz(1000); err != nil {
		t.Fatalf("SetCarrierHz() error = %v", err)
	}

	if rm.CarrierHz() != 1000 {
		t.Errorf("CarrierHz() = %g, want 1000", rm.CarrierHz())
	}

	if err := rm.SetMix(0.5); err != nil {
		t.Fatalf("SetMix() error = %v", err)
	}

	if rm.Mix() != 0.5 {
		t.Errorf("Mix() = %g, want 0.5", rm.Mix())
	}
}

func TestRingModulatorNilOption(t *testing.T) {
	rm, err := NewRingModulator(48000, nil)
	if err != nil {
		t.Fatalf("NewRingModulator() with nil option should not fail: %v", err)
	}

	if rm.CarrierHz() != defaultRingModCarrierHz {
		t.Errorf("CarrierHz() = %g, want default %g", rm.CarrierHz(), defaultRingModCarrierHz)
	}
}

func BenchmarkRingModulatorProcessSample(b *testing.B) {
	rm, err := NewRingModulator(48000,
		WithRingModCarrierHz(440),
		WithRingModMix(1),
	)
	if err != nil {
		b.Fatalf("NewRingModulator() error = %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		rm.Process(0.5)
	}
}

func BenchmarkRingModulatorProcessInPlace(b *testing.B) {
	rm, err := NewRingModulator(48000,
		WithRingModCarrierHz(440),
		WithRingModMix(1),
	)
	if err != nil {
		b.Fatalf("NewRingModulator() error = %v", err)
	}

	buf := make([]float64, 1024)
	for i := range buf {
		buf[i] = math.Sin(2 * math.Pi * float64(i) / 31)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		rm.ProcessInPlace(buf)
	}
}
