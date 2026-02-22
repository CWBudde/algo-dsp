package modulation

import (
	"math"
	"testing"
)

func TestRingModulatorProcessInPlaceMatchesProcess(t *testing.T) {
	ringMod1, err := NewRingModulator(48000)
	if err != nil {
		t.Fatalf("NewRingModulator() error = %v", err)
	}

	ringMod2, err := NewRingModulator(48000)
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
		want[i] = ringMod1.Process(want[i])
	}

	got := make([]float64, len(input))
	copy(got, input)
	ringMod2.ProcessInPlace(got)

	for i := range got {
		if diff := math.Abs(got[i] - want[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g diff=%g", i, got[i], want[i], diff)
		}
	}
}

func TestRingModulatorResetRestoresState(t *testing.T) {
	ringModulator, err := NewRingModulator(48000,
		WithRingModCarrierHz(300),
	)
	if err != nil {
		t.Fatalf("NewRingModulator() error = %v", err)
	}

	inData := make([]float64, 96)
	inData[0] = 1

	outData1 := make([]float64, len(inData))
	for i := range inData {
		outData1[i] = ringModulator.Process(inData[i])
	}

	ringModulator.Reset()

	outData2 := make([]float64, len(inData))
	for i := range inData {
		outData2[i] = ringModulator.Process(inData[i])
	}

	for i := range outData1 {
		if diff := math.Abs(outData1[i] - outData2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: got=%g want=%g diff=%g", i, outData2[i], outData1[i], diff)
		}
	}
}

func TestRingModulatorMixZeroIsTransparent(t *testing.T) {
	ringModulator, err := NewRingModulator(48000,
		WithRingModCarrierHz(1000),
		WithRingModMix(0),
	)
	if err != nil {
		t.Fatalf("NewRingModulator() error = %v", err)
	}

	for i := 0; i < 512; i++ {
		in := 0.5 * math.Sin(2*math.Pi*440*float64(i)/48000)

		out := ringModulator.Process(in)
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

	ringModulator, err := NewRingModulator(sampleRate,
		WithRingModCarrierHz(carrierHz),
		WithRingModMix(1),
	)
	if err != nil {
		t.Fatalf("NewRingModulator() error = %v", err)
	}

	for i := 0; i < nSamples; i++ {
		got := ringModulator.Process(1.0)
		phase := 2 * math.Pi * carrierHz * float64(i) / sampleRate

		want := math.Sin(phase)
		if diff := math.Abs(got - want); diff > 1e-9 {
			t.Fatalf("sample %d: got=%g want=%g diff=%g", i, got, want, diff)
		}
	}
}

func TestRingModulatorSilenceInputProducesSilence(t *testing.T) {
	ringModulator, err := NewRingModulator(48000,
		WithRingModCarrierHz(1000),
		WithRingModMix(1),
	)
	if err != nil {
		t.Fatalf("NewRingModulator() error = %v", err)
	}

	for i := 0; i < 256; i++ {
		out := ringModulator.Process(0)
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

	ringModulator, err := NewRingModulator(sampleRate,
		WithRingModCarrierHz(carrierHz),
		WithRingModMix(1),
	)
	if err != nil {
		t.Fatalf("NewRingModulator() error = %v", err)
	}

	output := make([]float64, nSamples)
	for i := range output {
		in := math.Sin(2 * math.Pi * inputHz * float64(i) / sampleRate)
		output[i] = ringModulator.Process(in)
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
	ringModulator, err := NewRingModulator(48000)
	if err != nil {
		t.Fatalf("NewRingModulator() error = %v", err)
	}

	if err := ringModulator.SetSampleRate(0); err == nil {
		t.Error("SetSampleRate(0) expected error")
	}

	if err := ringModulator.SetSampleRate(math.NaN()); err == nil {
		t.Error("SetSampleRate(NaN) expected error")
	}

	if err := ringModulator.SetCarrierHz(0); err == nil {
		t.Error("SetCarrierHz(0) expected error")
	}

	if err := ringModulator.SetCarrierHz(-1); err == nil {
		t.Error("SetCarrierHz(-1) expected error")
	}

	if err := ringModulator.SetMix(-0.1); err == nil {
		t.Error("SetMix(-0.1) expected error")
	}

	if err := ringModulator.SetMix(1.1); err == nil {
		t.Error("SetMix(1.1) expected error")
	}
}

func TestRingModulatorGetters(t *testing.T) {
	ringModulator, err := NewRingModulator(48000,
		WithRingModCarrierHz(300),
		WithRingModMix(0.7),
	)
	if err != nil {
		t.Fatalf("NewRingModulator() error = %v", err)
	}

	if ringModulator.SampleRate() != 48000 {
		t.Errorf("SampleRate() = %g, want 48000", ringModulator.SampleRate())
	}

	if ringModulator.CarrierHz() != 300 {
		t.Errorf("CarrierHz() = %g, want 300", ringModulator.CarrierHz())
	}

	if ringModulator.Mix() != 0.7 {
		t.Errorf("Mix() = %g, want 0.7", ringModulator.Mix())
	}
}

func TestRingModulatorSettersUpdateState(t *testing.T) {
	ringModulator, err := NewRingModulator(48000)
	if err != nil {
		t.Fatalf("NewRingModulator() error = %v", err)
	}

	if err := ringModulator.SetSampleRate(96000); err != nil {
		t.Fatalf("SetSampleRate() error = %v", err)
	}

	if ringModulator.SampleRate() != 96000 {
		t.Errorf("SampleRate() = %g, want 96000", ringModulator.SampleRate())
	}

	if err := ringModulator.SetCarrierHz(1000); err != nil {
		t.Fatalf("SetCarrierHz() error = %v", err)
	}

	if ringModulator.CarrierHz() != 1000 {
		t.Errorf("CarrierHz() = %g, want 1000", ringModulator.CarrierHz())
	}

	if err := ringModulator.SetMix(0.5); err != nil {
		t.Fatalf("SetMix() error = %v", err)
	}

	if ringModulator.Mix() != 0.5 {
		t.Errorf("Mix() = %g, want 0.5", ringModulator.Mix())
	}
}

func TestRingModulatorNilOption(t *testing.T) {
	ringModulator, err := NewRingModulator(48000, nil)
	if err != nil {
		t.Fatalf("NewRingModulator() with nil option should not fail: %v", err)
	}

	if ringModulator.CarrierHz() != defaultRingModCarrierHz {
		t.Errorf("CarrierHz() = %g, want default %g", ringModulator.CarrierHz(), defaultRingModCarrierHz)
	}
}

func BenchmarkRingModulatorProcessSample(b *testing.B) {
	ringModulator, err := NewRingModulator(48000,
		WithRingModCarrierHz(440),
		WithRingModMix(1),
	)
	if err != nil {
		b.Fatalf("NewRingModulator() error = %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ringModulator.Process(0.5)
	}
}

func BenchmarkRingModulatorProcessInPlace(b *testing.B) {
	ringModulator, err := NewRingModulator(48000,
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
		ringModulator.ProcessInPlace(buf)
	}
}
