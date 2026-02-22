package spatial

import (
	"math"
	"testing"
)

func TestCrosstalkCancellerValidation(t *testing.T) {
	if _, err := NewCrosstalkCanceller(0); err == nil {
		t.Fatal("expected error for invalid sample rate")
	}

	if _, err := NewCrosstalkCanceller(48000, WithCancellerStages(0)); err == nil {
		t.Fatal("expected error for invalid stages")
	}

	if _, err := NewCrosstalkCanceller(48000,
		WithCancellerSpeakerDistance(0.2),
		WithCancellerHeadRadius(0.12)); err == nil {
		t.Fatal("expected geometry error when speaker distance <= 2*head radius")
	}
}

func TestCrosstalkCancellerDelayCalculation(t *testing.T) {
	c, err := NewCrosstalkCanceller(48000,
		WithCancellerListenerDistance(1),
		WithCancellerSpeakerDistance(2),
		WithCancellerHeadRadius(0.0875),
		WithCancellerStages(2),
	)
	if err != nil {
		t.Fatalf("NewCrosstalkCanceller() error = %v", err)
	}

	expected := int(math.Round((c.pathDeltaMeters() / defaultCancellerSpeedOfSound) * c.SampleRate()))
	if expected < 1 {
		expected = 1
	}

	if got := c.BaseDelaySamples(); got != expected {
		t.Fatalf("base delay mismatch: got=%d want=%d", got, expected)
	}

	if c.StageDelaySamples() < 1 {
		t.Fatalf("stage delay must be >= 1, got %d", c.StageDelaySamples())
	}
}

func TestCrosstalkCancellerInPlaceMatchesSampleBySample(t *testing.T) {
	c1, err := NewCrosstalkCanceller(48000,
		WithCancellerAttenuation(0.7),
		WithCancellerStages(3),
	)
	if err != nil {
		t.Fatalf("NewCrosstalkCanceller() error = %v", err)
	}

	c2, err := NewCrosstalkCanceller(48000,
		WithCancellerAttenuation(0.7),
		WithCancellerStages(3),
	)
	if err != nil {
		t.Fatalf("NewCrosstalkCanceller() error = %v", err)
	}

	n := 128
	inL := make([]float64, n)
	inR := make([]float64, n)
	for i := 0; i < n; i++ {
		inL[i] = math.Sin(2 * math.Pi * float64(i) / 21)
		inR[i] = math.Sin(2*math.Pi*float64(i)/19 + 0.2)
	}

	wantL := make([]float64, n)
	wantR := make([]float64, n)
	for i := range inL {
		wantL[i], wantR[i] = c1.ProcessStereo(inL[i], inR[i])
	}

	gotL := append([]float64(nil), inL...)
	gotR := append([]float64(nil), inR...)
	if err := c2.ProcessInPlace(gotL, gotR); err != nil {
		t.Fatalf("ProcessInPlace() error = %v", err)
	}

	for i := range gotL {
		if d := math.Abs(gotL[i] - wantL[i]); d > 1e-12 {
			t.Fatalf("left[%d] mismatch: got=%g want=%g", i, gotL[i], wantL[i])
		}
		if d := math.Abs(gotR[i] - wantR[i]); d > 1e-12 {
			t.Fatalf("right[%d] mismatch: got=%g want=%g", i, gotR[i], wantR[i])
		}
	}
}

func TestCrosstalkCancellerResetDeterministic(t *testing.T) {
	c, err := NewCrosstalkCanceller(48000, WithCancellerStages(2))
	if err != nil {
		t.Fatalf("NewCrosstalkCanceller() error = %v", err)
	}

	n := 96
	inL := make([]float64, n)
	inR := make([]float64, n)
	for i := 0; i < n; i++ {
		inL[i] = math.Sin(2 * math.Pi * float64(i) / 37)
		inR[i] = math.Cos(2 * math.Pi * float64(i) / 29)
	}

	outL1 := make([]float64, n)
	outR1 := make([]float64, n)
	for i := 0; i < n; i++ {
		outL1[i], outR1[i] = c.ProcessStereo(inL[i], inR[i])
	}

	c.Reset()

	for i := 0; i < n; i++ {
		outL2, outR2 := c.ProcessStereo(inL[i], inR[i])
		if math.Abs(outL1[i]-outL2) > 1e-12 {
			t.Fatalf("left mismatch at %d after reset", i)
		}
		if math.Abs(outR1[i]-outR2) > 1e-12 {
			t.Fatalf("right mismatch at %d after reset", i)
		}
	}
}
