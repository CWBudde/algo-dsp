package spatial

import (
	"math"
	"testing"
)

func TestCrosstalkSimulatorValidation(t *testing.T) {
	if _, err := NewCrosstalkSimulator(0); err == nil {
		t.Fatal("expected error for zero sample rate")
	}

	if _, err := NewCrosstalkSimulator(48000, WithSimulatorDiameter(0.01)); err == nil {
		t.Fatal("expected error for small diameter")
	}

	if _, err := NewCrosstalkSimulator(48000, WithSimulatorCrossfeedMix(2)); err == nil {
		t.Fatal("expected error for invalid mix")
	}

	if _, err := NewCrosstalkSimulator(48000, WithSimulatorPreset(CrosstalkPreset(99))); err == nil {
		t.Fatal("expected error for invalid preset")
	}

	if _, err := NewCrosstalkSimulator(48000, WithSimulatorSpeedOfSound(200)); err == nil {
		t.Fatal("expected error for invalid speed of sound")
	}
}

func TestCrosstalkSimulatorDelayCalculation(t *testing.T) {
	s, err := NewCrosstalkSimulator(48000, WithSimulatorDiameter(0.18))
	if err != nil {
		t.Fatalf("NewCrosstalkSimulator() error = %v", err)
	}

	expected := int(math.Round((0.18 / defaultSimulatorSpeed) * 48000))
	if expected < 1 {
		expected = 1
	}

	if s.DelaySamples() != expected {
		t.Fatalf("delay samples mismatch: got=%d want=%d", s.DelaySamples(), expected)
	}

	before := s.DelaySamples()
	if err := s.SetSpeedOfSound(320); err != nil {
		t.Fatalf("SetSpeedOfSound() error = %v", err)
	}

	after := s.DelaySamples()
	if after <= before {
		t.Fatalf("expected larger delay with lower speed of sound: before=%d after=%d", before, after)
	}
}

func TestCrosstalkSimulatorPresetsDifferentResponse(t *testing.T) {
	hand, err := NewCrosstalkSimulator(48000,
		WithSimulatorPreset(CrosstalkPresetHandcrafted),
		WithSimulatorCrossfeedMix(1),
	)
	if err != nil {
		t.Fatalf("NewCrosstalkSimulator(handcrafted) error = %v", err)
	}

	ircam, err := NewCrosstalkSimulator(48000,
		WithSimulatorPreset(CrosstalkPresetIRCAM),
		WithSimulatorCrossfeedMix(1),
	)
	if err != nil {
		t.Fatalf("NewCrosstalkSimulator(ircam) error = %v", err)
	}

	var handEnergy, ircamEnergy float64

	for i := 0; i < 512; i++ {
		inL := 0.0

		inR := 0.0
		if i == 0 {
			inR = 1
		}

		ohL, _ := hand.ProcessStereo(inL, inR)
		oiL, _ := ircam.ProcessStereo(inL, inR)
		handEnergy += ohL * ohL
		ircamEnergy += oiL * oiL
	}

	if math.Abs(handEnergy-ircamEnergy) < 1e-6 {
		t.Fatalf("expected different preset response, energies too close: hand=%g ircam=%g", handEnergy, ircamEnergy)
	}
}

func TestCrosstalkSimulatorPolarityInvert(t *testing.T) {
	s1, err := NewCrosstalkSimulator(48000,
		WithSimulatorCrossfeedMix(1),
		WithSimulatorPolarityInvert(false),
	)
	if err != nil {
		t.Fatalf("NewCrosstalkSimulator() error = %v", err)
	}

	s2, err := NewCrosstalkSimulator(48000,
		WithSimulatorCrossfeedMix(1),
		WithSimulatorPolarityInvert(true),
	)
	if err != nil {
		t.Fatalf("NewCrosstalkSimulator() error = %v", err)
	}

	// Prime delay.
	for i := 0; i < s1.DelaySamples(); i++ {
		s1.ProcessStereo(0, 0)
		s2.ProcessStereo(0, 0)
	}

	o1L, _ := s1.ProcessStereo(0, 1)
	o2L, _ := s2.ProcessStereo(0, 1)

	if math.Abs(o1L+o2L) > 1e-6 {
		t.Fatalf("polarity invert mismatch: normal=%g inverted=%g", o1L, o2L)
	}
}

func TestCrosstalkSimulatorInPlaceMatchesSampleBySample(t *testing.T) {
	s1, err := NewCrosstalkSimulator(48000, WithSimulatorCrossfeedMix(0.35))
	if err != nil {
		t.Fatalf("NewCrosstalkSimulator() error = %v", err)
	}

	s2, err := NewCrosstalkSimulator(48000, WithSimulatorCrossfeedMix(0.35))
	if err != nil {
		t.Fatalf("NewCrosstalkSimulator() error = %v", err)
	}

	n := 120
	inL := make([]float64, n)
	inR := make([]float64, n)

	for i := range inL {
		inL[i] = math.Sin(2 * math.Pi * float64(i) / 31)
		inR[i] = math.Sin(2*math.Pi*float64(i)/27 + 0.3)
	}

	wantL := make([]float64, n)

	wantR := make([]float64, n)
	for i := range inL {
		wantL[i], wantR[i] = s1.ProcessStereo(inL[i], inR[i])
	}

	gotL := append([]float64(nil), inL...)

	gotR := append([]float64(nil), inR...)
	if err := s2.ProcessInPlace(gotL, gotR); err != nil {
		t.Fatalf("ProcessInPlace() error = %v", err)
	}

	for i := range gotL {
		if math.Abs(gotL[i]-wantL[i]) > 1e-12 {
			t.Fatalf("left[%d] mismatch", i)
		}

		if math.Abs(gotR[i]-wantR[i]) > 1e-12 {
			t.Fatalf("right[%d] mismatch", i)
		}
	}
}
