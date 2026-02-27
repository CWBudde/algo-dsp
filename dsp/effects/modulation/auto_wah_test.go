package modulation

import (
	"math"
	"testing"
)

func TestAutoWahProcessInPlaceMatchesProcess(t *testing.T) {
	a1, err := NewAutoWah(48000)
	if err != nil {
		t.Fatalf("NewAutoWah() error = %v", err)
	}

	a2, err := NewAutoWah(48000)
	if err != nil {
		t.Fatalf("NewAutoWah() error = %v", err)
	}

	input := make([]float64, 256)
	for i := range input {
		input[i] = 0.5 * math.Sin(2*math.Pi*float64(i)/37)
	}

	want := make([]float64, len(input))
	copy(want, input)

	for i := range want {
		want[i] = a1.Process(want[i])
	}

	got := make([]float64, len(input))
	copy(got, input)

	err = a2.ProcessInPlace(got)
	if err != nil {
		t.Fatalf("ProcessInPlace() error = %v", err)
	}

	for i := range got {
		if diff := math.Abs(got[i] - want[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g diff=%g", i, got[i], want[i], diff)
		}
	}
}

func TestAutoWahResetRestoresState(t *testing.T) {
	autoWah, err := NewAutoWah(48000,
		WithAutoWahSensitivity(4),
	)
	if err != nil {
		t.Fatalf("NewAutoWah() error = %v", err)
	}

	in := make([]float64, 128)
	for i := range in {
		in[i] = 0.25 * math.Sin(2*math.Pi*440*float64(i)/48000)
	}

	out1 := make([]float64, len(in))
	for i := range in {
		out1[i] = autoWah.Process(in[i])
	}

	autoWah.Reset()

	out2 := make([]float64, len(in))
	for i := range in {
		out2[i] = autoWah.Process(in[i])
	}

	for i := range out1 {
		if diff := math.Abs(out1[i] - out2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: got=%g want=%g diff=%g", i, out2[i], out1[i], diff)
		}
	}
}

func TestAutoWahMixZeroIsTransparent(t *testing.T) {
	autoWah, err := NewAutoWah(48000,
		WithAutoWahMix(0),
	)
	if err != nil {
		t.Fatalf("NewAutoWah() error = %v", err)
	}

	for i := range 512 {
		in := 0.4 * math.Sin(2*math.Pi*330*float64(i)/48000)

		out := autoWah.Process(in)
		if diff := math.Abs(out - in); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g", i, out, in)
		}
	}
}

func TestAutoWahCenterFrequencyTracksEnvelope(t *testing.T) {
	autoWah, err := NewAutoWah(48000,
		WithAutoWahFrequencyRangeHz(300, 2500),
		WithAutoWahSensitivity(6),
		WithAutoWahAttackMs(1),
		WithAutoWahReleaseMs(120),
	)
	if err != nil {
		t.Fatalf("NewAutoWah() error = %v", err)
	}

	for i := range 1024 {
		in := 0.02 * math.Sin(2*math.Pi*440*float64(i)/48000)
		autoWah.Process(in)
	}

	lowFreq := autoWah.CurrentCenterHz()

	autoWah.Reset()

	for i := range 1024 {
		in := 0.9 * math.Sin(2*math.Pi*440*float64(i)/48000)
		autoWah.Process(in)
	}

	highFreq := autoWah.CurrentCenterHz()

	if highFreq <= lowFreq+400 {
		t.Fatalf("expected higher center frequency for louder signal: low=%g high=%g", lowFreq, highFreq)
	}
}

func TestAutoWahFiniteOutput(t *testing.T) {
	autoWah, err := NewAutoWah(48000,
		WithAutoWahQ(8),
		WithAutoWahSensitivity(8),
		WithAutoWahMix(1),
	)
	if err != nil {
		t.Fatalf("NewAutoWah() error = %v", err)
	}

	for i := range 12000 {
		in := 0.8 * math.Sin(2*math.Pi*220*float64(i)/48000)

		out := autoWah.Process(in)
		if math.IsNaN(out) || math.IsInf(out, 0) {
			t.Fatalf("non-finite output at sample %d: %v", i, out)
		}
	}
}

func TestAutoWahValidation(t *testing.T) {
	tests := []struct {
		name string
		fn   func() error
	}{
		{"zero sample rate", func() error {
			_, err := NewAutoWah(0)
			return err
		}},
		{"invalid range", func() error {
			_, err := NewAutoWah(48000, WithAutoWahFrequencyRangeHz(1000, 800))
			return err
		}},
		{"invalid Q", func() error {
			_, err := NewAutoWah(48000, WithAutoWahQ(0))
			return err
		}},
		{"invalid sensitivity", func() error {
			_, err := NewAutoWah(48000, WithAutoWahSensitivity(0))
			return err
		}},
		{"invalid mix", func() error {
			_, err := NewAutoWah(48000, WithAutoWahMix(1.2))
			return err
		}},
		{"above nyquist safety", func() error {
			_, err := NewAutoWah(48000, WithAutoWahFrequencyRangeHz(500, 30000))
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
		})
	}
}

func TestAutoWahSetterValidation(t *testing.T) {
	autoWah, err := NewAutoWah(48000)
	if err != nil {
		t.Fatalf("NewAutoWah() error = %v", err)
	}

	err = autoWah.SetSampleRate(0)
	if err == nil {
		t.Fatal("SetSampleRate(0) expected error")
	}

	err = autoWah.SetFrequencyRangeHz(1200, 700)
	if err == nil {
		t.Fatal("SetFrequencyRangeHz() expected error")
	}

	err = autoWah.SetQ(0)
	if err == nil {
		t.Fatal("SetQ(0) expected error")
	}

	if err := autoWah.SetSensitivity(-1); err == nil {
		t.Fatal("SetSensitivity(-1) expected error")
	}

	if err := autoWah.SetAttackMs(-1); err == nil {
		t.Fatal("SetAttackMs(-1) expected error")
	}

	if err := autoWah.SetReleaseMs(-1); err == nil {
		t.Fatal("SetReleaseMs(-1) expected error")
	}

	if err := autoWah.SetMix(1.1); err == nil {
		t.Fatal("SetMix(1.1) expected error")
	}
}

func TestAutoWahGettersAndSetters(t *testing.T) {
	autoWah, err := NewAutoWah(48000)
	if err != nil {
		t.Fatalf("NewAutoWah() error = %v", err)
	}

	if err := autoWah.SetFrequencyRangeHz(350, 1800); err != nil {
		t.Fatalf("SetFrequencyRangeHz() error = %v", err)
	}

	if err := autoWah.SetQ(1.2); err != nil {
		t.Fatalf("SetQ() error = %v", err)
	}

	if err := autoWah.SetSensitivity(3.5); err != nil {
		t.Fatalf("SetSensitivity() error = %v", err)
	}

	if err := autoWah.SetAttackMs(3); err != nil {
		t.Fatalf("SetAttackMs() error = %v", err)
	}

	if err := autoWah.SetReleaseMs(90); err != nil {
		t.Fatalf("SetReleaseMs() error = %v", err)
	}

	if err := autoWah.SetMix(0.7); err != nil {
		t.Fatalf("SetMix() error = %v", err)
	}

	if autoWah.SampleRate() != 48000 {
		t.Fatalf("SampleRate()=%g want=48000", autoWah.SampleRate())
	}

	if autoWah.MinFreqHz() != 350 {
		t.Fatalf("MinFreqHz()=%g want=350", autoWah.MinFreqHz())
	}

	if autoWah.MaxFreqHz() != 1800 {
		t.Fatalf("MaxFreqHz()=%g want=1800", autoWah.MaxFreqHz())
	}

	if autoWah.Q() != 1.2 {
		t.Fatalf("Q()=%g want=1.2", autoWah.Q())
	}

	if autoWah.Sensitivity() != 3.5 {
		t.Fatalf("Sensitivity()=%g want=3.5", autoWah.Sensitivity())
	}

	if autoWah.AttackMs() != 3 {
		t.Fatalf("AttackMs()=%g want=3", autoWah.AttackMs())
	}

	if autoWah.ReleaseMs() != 90 {
		t.Fatalf("ReleaseMs()=%g want=90", autoWah.ReleaseMs())
	}

	if autoWah.Mix() != 0.7 {
		t.Fatalf("Mix()=%g want=0.7", autoWah.Mix())
	}
}
