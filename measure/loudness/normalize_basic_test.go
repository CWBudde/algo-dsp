package loudness

import (
	"errors"
	"math"
	"testing"
)

func TestNormalizeBasicLinkedGainAndFreshOutput(t *testing.T) {
	input := basicIntegratedTone(48000)
	before := append([]float64(nil), input...)
	planar := [][]float64{input, input}
	config := IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: int64(len(input))}

	output, err := NormalizeLoudness(planar, -23, config)
	if err != nil {
		t.Fatal(err)
	}

	if len(output) != 2 || len(output[0]) != len(input) || &output[0][0] == &input[0] || &output[0][0] == &output[1][0] {
		t.Fatal("output shape/storage is not fresh per channel")
	}

	for i := range input {
		if input[i] != before[i] || output[0][i] != output[1][i] {
			t.Fatalf("input mutation or independent channel gain at %d", i)
		}
	}

	a, err := NewIntegratedAnalyzer(config)
	if err != nil {
		t.Fatal(err)
	}

	if err := a.ProcessPlanar(output); err != nil {
		t.Fatal(err)
	}

	if got := basicIntegratedFinish(t, a).LUFS; math.Abs(got+23) > 1e-9 {
		t.Fatalf("normalized tone=%g LUFS, want -23", got)
	}
}

func TestNormalizeBasicPlanHasNoUIGainClamp(t *testing.T) {
	measured := IntegratedResult{LUFS: -69, SamplePeak: 1e-3, Frames: 19200}

	plan, err := PlanNormalization(measured, 0)
	if err != nil {
		t.Fatal(err)
	}

	if plan.GainDB != 69 || plan.Gain <= 1000 || plan.PredictedSamplePeak <= 1 {
		t.Fatalf("unexpected clipped plan: %+v", plan)
	}

	if _, err := PlanNormalization(measured, math.MaxFloat64); !errors.Is(err, ErrNumericalOverflow) {
		t.Fatalf("overflow plan=%v", err)
	}

	if _, err := PlanNormalization(measured, -math.MaxFloat64); !errors.Is(err, ErrNumericalOverflow) {
		t.Fatalf("underflow plan=%v", err)
	}
}

func TestNormalizeBasicPlanAcceptsRoundedGateBoundary(t *testing.T) {
	measured := IntegratedResult{LUFS: -70, SamplePeak: 1e-3, Frames: 19200}

	plan, err := PlanNormalization(measured, -23)
	if err != nil || plan.GainDB != 47 || plan.Gain <= 0 {
		t.Fatalf("rounded valid gate boundary rejected: plan=%+v error=%v", plan, err)
	}
}
