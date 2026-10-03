package loudness

import (
	"errors"
	"math"
	"testing"
)

func TestTargetConfigurationAndStateSafety(t *testing.T) {
	valid := IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: 24000}
	for _, target := range []float64{-70, -71, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if a, err := NewTargetAnalyzer(valid, target); a != nil || !errors.Is(err, ErrInvalid) {
			t.Fatalf("accepted invalid target %g: %v", target, err)
		}
	}

	for _, config := range []IntegratedConfig{
		{SampleRate: 7999, Channels: 1, MaxFrames: 1},
		{SampleRate: 384001, Channels: 1, MaxFrames: 1},
		{SampleRate: 48000, Channels: 0, MaxFrames: 1},
		{SampleRate: 48000, Channels: 2, MaxFrames: 0},
		{SampleRate: 48000, Channels: 2, MaxFrames: 1, ChannelWeights: []float64{1}},
		{SampleRate: 48000, Channels: 2, MaxFrames: 1, ChannelWeights: []float64{0, 0}},
		{SampleRate: 48000, Channels: 1, MaxFrames: math.MaxInt64},
		{SampleRate: 48000, Channels: 1, MaxFrames: 4800 * 4500000},
	} {
		if a, err := NewTargetAnalyzer(config, -23); a != nil || err == nil {
			t.Fatalf("accepted unsafe config %+v", config)
		}
	}

	var nilAnalyzer *TargetAnalyzer
	for _, a := range []*TargetAnalyzer{nilAnalyzer, {}} {
		if err := a.ProcessPlanar(nil); !errors.Is(err, ErrState) {
			t.Fatal(err)
		}

		if _, err := a.FinishStep(1); !errors.Is(err, ErrState) {
			t.Fatal(err)
		}

		if _, err := a.Result(); !errors.Is(err, ErrState) {
			t.Fatal(err)
		}

		a.Reset()
	}

	a, err := NewTargetAnalyzer(valid, -69)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := a.Result(); !errors.Is(err, ErrState) {
		t.Fatal("early result", err)
	}

	if _, err := a.FinishStep(0); !errors.Is(err, ErrInvalid) || a.phase != targetInput {
		t.Fatal("invalid work budget froze input", err)
	}

	if _, err := a.FinishStep(1); !errors.Is(err, ErrTooShort) {
		t.Fatal(err)
	}

	if err := a.ProcessPlanar([][]float64{{}, {}}); !errors.Is(err, ErrTooShort) {
		t.Fatal("terminal finish did not fence input", err)
	}

	a.Reset()

	if err := a.ProcessPlanar([][]float64{{}, {}}); err != nil {
		t.Fatal("empty input", err)
	}
}

func TestTargetAtomicInvalidBlocksAndOverflowRecovery(t *testing.T) {
	a, err := NewTargetAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: 24000}, -23)
	if err != nil {
		t.Fatal(err)
	}

	for _, block := range [][][]float64{
		nil,
		{{1}},
		{{1, 2}, {1}},
		{{1}, {math.NaN()}},
		{{1}, {math.Inf(-1)}},
		{make([]float64, 65537), make([]float64, 65537)},
		{make([]float64, 24001), make([]float64, 24001)},
	} {
		if err := a.ProcessPlanar(block); err == nil || a.frames != 0 || a.peak != 0 || a.filters[0] != (targetFilterState{}) {
			t.Fatal("non-atomic preflight", err)
		}
	}

	if err := a.ProcessPlanar([][]float64{{math.MaxFloat64}, {1}}); !errors.Is(err, ErrNumericalOverflow) {
		t.Fatal("expected arithmetic overflow", err)
	}

	if _, err := a.FinishStep(1); !errors.Is(err, ErrNumericalOverflow) {
		t.Fatal("overflow not terminal", err)
	}

	a.Reset()

	if err := a.ProcessPlanar([][]float64{{0.1}, {0.2}}); err != nil {
		t.Fatal("Reset did not recover", err)
	}

	if a.frames != 1 || a.peak != 0.2 {
		t.Fatal("stale state after Reset")
	}
}

func TestTargetWeightOwnershipSilenceAndInputFreeze(t *testing.T) {
	weights := []float64{1, 0}

	a, err := NewTargetAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: 24000, ChannelWeights: weights}, -23)
	if err != nil {
		t.Fatal(err)
	}

	weights[0] = 0
	if a.weights[0] != 1 {
		t.Fatal("retained caller weights")
	}

	left, right := make([]float64, 24000), make([]float64, 24000)
	for frame := range left {
		left[frame] = 0.1 * math.Sin(2*math.Pi*1000*float64(frame)/48000)
		right[frame] = 4
	}

	if err := a.ProcessPlanar([][]float64{left, right}); err != nil {
		t.Fatal(err)
	}

	if done, err := a.FinishStep(1); done || err != nil {
		t.Fatal("unbounded first finish", err)
	}

	if err := a.ProcessPlanar([][]float64{{}, {}}); !errors.Is(err, ErrState) {
		t.Fatal("input not frozen", err)
	}

	result := finishTargetBasic(t, a)
	if result.SamplePeak != 4 || result.Plan.PredictedSamplePeak != 4*result.Plan.Gain {
		t.Fatal("zero-weight linked peak lost", result)
	}

	a.Reset()
	clear(left)
	clear(right)

	if err := a.ProcessPlanar([][]float64{left, right}); err != nil {
		t.Fatal(err)
	}

	if _, err := a.FinishStep(math.MaxInt); !errors.Is(err, ErrBelowGate) {
		t.Fatal("silence fabricated loudness", err)
	}
}
