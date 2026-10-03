package loudness

import (
	"errors"
	"math"
	"testing"
)

func finishTargetMeasurementTest(t *testing.T, a *TargetAnalyzer) IntegratedResult {
	t.Helper()

	for steps := 0; steps < 10000; steps++ {
		done, err := a.FinishMeasurementStep(1)
		if err != nil {
			t.Fatal(err)
		}

		if done {
			result, err := a.MeasurementResult()
			if err != nil {
				t.Fatal(err)
			}

			return result
		}
	}

	t.Fatal("bounded measurement did not complete")

	return IntegratedResult{}
}

func TestTargetMeasurementReferenceAndModeGuards(t *testing.T) {
	config := IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: 48000}

	a, err := NewTargetAnalyzer(config, -23)
	if err != nil {
		t.Fatal(err)
	}

	input := integratedBenchmarkFixture(48000)

	if _, err := a.MeasurementResult(); !errors.Is(err, ErrState) {
		t.Fatalf("unfinished measurement result: %v", err)
	}

	if _, err := a.FinishMeasurementStep(0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid budget accepted: %v", err)
	}

	if err := a.ProcessPlanar(input); err != nil {
		t.Fatalf("invalid budget froze input: %v", err)
	}

	if done, err := a.FinishMeasurementStep(1); done || err != nil || a.index != 1 {
		t.Fatalf("one work unit was not bounded: done=%v error=%v index=%d", done, err, a.index)
	}

	if _, err := a.FinishStep(1); !errors.Is(err, ErrState) {
		t.Fatalf("target planner entered measurement mode: %v", err)
	}

	if err := a.ProcessPlanar([][]float64{{}, {}}); !errors.Is(err, ErrState) {
		t.Fatalf("input after finalization accepted: %v", err)
	}

	got := finishTargetMeasurementTest(t, a)
	if _, err := a.Result(); !errors.Is(err, ErrState) {
		t.Fatalf("measurement fabricated target result: %v", err)
	}

	if done, err := a.FinishMeasurementStep(1); !done || err != nil {
		t.Fatalf("completed measurement not idempotent: done=%v error=%v", done, err)
	}

	reference, err := NewIntegratedAnalyzer(config)
	if err != nil {
		t.Fatal(err)
	}

	if err := reference.ProcessPlanar(input); err != nil {
		t.Fatal(err)
	}

	for {
		done, err := reference.FinishStep(1)
		if err != nil {
			t.Fatal(err)
		}

		if done {
			break
		}
	}

	want, err := reference.Result()
	if err != nil || math.Abs(got.LUFS-want.LUFS) > 1e-10 || got.SamplePeak != want.SamplePeak || got.Frames != want.Frames {
		t.Fatalf("fast measurement differs from reference: got %+v want %+v error=%v", got, want, err)
	}

	a.Reset()

	if err := a.ProcessPlanar(input); err != nil {
		t.Fatal(err)
	}

	if _, err := a.FinishStep(1); err != nil {
		t.Fatal(err)
	}

	if _, err := a.FinishMeasurementStep(1); !errors.Is(err, ErrState) {
		t.Fatalf("measurement entered target-planning mode: %v", err)
	}

	_ = finishTargetBasic(t, a)
}

func TestTargetMeasurementSamplePeakAtomicFailureAndReset(t *testing.T) {
	a, err := NewTargetAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 2, ChannelWeights: []float64{1, 0}, MaxFrames: 24000}, -23)
	if err != nil {
		t.Fatal(err)
	}

	if a.SamplePeak() != 0 || (*TargetAnalyzer)(nil).SamplePeak() != 0 {
		t.Fatal("new/unconfigured sample peak was not zero")
	}

	if err := a.ProcessPlanar([][]float64{{0.2}, {-3}}); err != nil || a.SamplePeak() != 3 {
		t.Fatalf("zero-weight peak ignored: peak=%g error=%v", a.SamplePeak(), err)
	}

	if err := a.ProcessPlanar([][]float64{{4}, {math.NaN()}}); !errors.Is(err, ErrNonFinite) || a.SamplePeak() != 3 {
		t.Fatalf("late rejected block changed peak: peak=%g error=%v", a.SamplePeak(), err)
	}

	if err := a.ProcessPlanar([][]float64{{math.MaxFloat64}, {0}}); !errors.Is(err, ErrNumericalOverflow) {
		t.Fatalf("expected terminal arithmetic error: %v", err)
	}

	if !integratedFinite(a.SamplePeak()) || a.SamplePeak() < 3 {
		t.Fatalf("terminal failure destroyed finite diagnostic: %g", a.SamplePeak())
	}

	if _, err := a.MeasurementResult(); !errors.Is(err, ErrNumericalOverflow) {
		t.Fatalf("terminal peak diagnostic fabricated measurement: %v", err)
	}

	a.Reset()

	if a.SamplePeak() != 0 {
		t.Fatal("Reset retained peak")
	}

	if err := a.ProcessPlanar(integratedBenchmarkFixture(24000)); err != nil {
		t.Fatal(err)
	}

	_ = finishTargetMeasurementTest(t, a)
}

func TestTargetMeasurementUndefinedAndZeroAllocations(t *testing.T) {
	for _, tc := range []struct {
		name   string
		frames int
		value  float64
		want   error
	}{
		{"short", 19199, 0.2, ErrTooShort},
		{"silence", 24000, 0, ErrBelowGate},
		{"positive below gate", 24000, 1e-8, ErrBelowGate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := NewTargetAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 1, MaxFrames: 24000}, -23)
			if err != nil {
				t.Fatal(err)
			}

			input := make([]float64, tc.frames)
			for frame := range input {
				input[frame] = tc.value * math.Sin(2*math.Pi*1000*float64(frame)/48000)
			}

			if err := a.ProcessPlanar([][]float64{input}); err != nil {
				t.Fatal(err)
			}

			if _, err := a.FinishMeasurementStep(1); !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}

			if _, err := a.MeasurementResult(); !errors.Is(err, tc.want) {
				t.Fatalf("undefined result was invented: %v", err)
			}
		})
	}

	a, err := NewTargetAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: 48000}, -23)
	if err != nil {
		t.Fatal(err)
	}

	input := integratedBenchmarkFixture(48000)

	var failure error

	allocations := testing.AllocsPerRun(2, func() {
		a.Reset()

		if failure = a.ProcessPlanar(input); failure != nil {
			return
		}

		for {
			var done bool

			done, failure = a.FinishMeasurementStep(1)
			if done || failure != nil {
				return
			}
		}
	})
	if failure != nil || allocations != 0 {
		t.Fatalf("fast measurement allocated: allocations=%g error=%v", allocations, failure)
	}
}
