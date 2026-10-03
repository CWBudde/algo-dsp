package loudness

import (
	"errors"
	"fmt"
	"math"
	"testing"
)

func assertTargetFusionState(t *testing.T, fused, reference *TargetAnalyzer) {
	t.Helper()

	if fused.frames != reference.frames || fused.nextHop != reference.nextHop || fused.hopIndex != reference.hopIndex || fused.absCount != reference.absCount || len(fused.energies) != len(reference.energies) {
		t.Fatal("fused path changed frames/endpoints/window counts")
	}

	compare := func(label string, got, want float64) {
		if math.Float64bits(got) != math.Float64bits(want) {
			t.Fatalf("%s differs in bits: %016x != %016x", label, math.Float64bits(got), math.Float64bits(want))
		}
	}
	compare("hop sum", fused.hopSum, reference.hopSum)
	compare("source peak", fused.peak, reference.peak)
	compare("absolute mean", fused.absMean, reference.absMean)

	for index := range fused.hops {
		compare("hop", fused.hops[index], reference.hops[index])
	}

	for index := range fused.energies {
		compare("window", fused.energies[index], reference.energies[index])
	}

	for channel, state := range fused.filters {
		want := reference.filters[channel]
		compare("shelf state0", state.s0, want.s0)
		compare("shelf state1", state.s1, want.s1)
		compare("highpass state0", state.h0, want.h0)
		compare("highpass state1", state.h1, want.h1)
	}
}

func finishFusionMeasurement(t *testing.T, a *TargetAnalyzer) (IntegratedResult, error) {
	t.Helper()

	for step := 0; step < 100000; step++ {
		done, err := a.FinishMeasurementStep(1)
		if err != nil {
			return IntegratedResult{}, err
		}

		if done {
			return a.MeasurementResult()
		}
	}

	t.Fatal("measurement did not finish")

	return IntegratedResult{}, ErrState
}

func finishFusionTarget(t *testing.T, a *TargetAnalyzer) (TargetResult, error) {
	t.Helper()

	for step := 0; step < 100000; step++ {
		done, err := a.FinishStep(1)
		if err != nil {
			return TargetResult{}, err
		}

		if done {
			return a.Result()
		}
	}

	t.Fatal("target did not finish")

	return TargetResult{}, ErrState
}

func fusionTestSample(pattern string, channel, frame, frames int) float64 {
	switch pattern {
	case "extreme":
		switch frame % 211 {
		case 0:
			return math.MaxFloat32
		case 1:
			return -math.MaxFloat32
		case 2:
			return math.Copysign(0, -1)
		case 3:
			return math.SmallestNonzeroFloat32
		case 4:
			return -math.SmallestNonzeroFloat32
		default:
			return float64((frame*13+channel*19)%31-15) * 1e20
		}
	case "tiny":
		if frame < frames/3 {
			return math.Copysign(0, -1)
		}

		return float64((frame*17+channel*31)%257-128) * math.SmallestNonzeroFloat32
	default:
		gain := 1.0
		if frame > frames/3 && frame < 2*frames/3 {
			gain = 0.000001
		}

		return gain * (float64((frame*17+channel*31)%257-128)/512 + float64(channel)/32)
	}
}

func runTargetFusionParity[T ~float32 | ~float64](t *testing.T, rate float64, channels int, pattern string, chunks []int, process func(*TargetAnalyzer, [][]T) error) {
	t.Helper()

	frames := int(math.Ceil(rate * 0.61))
	config := IntegratedConfig{SampleRate: rate, Channels: channels, MaxFrames: int64(frames)}

	fused, err := NewTargetAnalyzer(config, -23)
	if err != nil {
		t.Fatal(err)
	}

	reference, err := NewTargetAnalyzer(config, -23)
	if err != nil {
		t.Fatal(err)
	}

	input := make([][]T, channels)

	block := make([][]T, channels)
	for channel := range input {
		input[channel] = make([]T, frames)
		for frame := range input[channel] {
			input[channel][frame] = T(fusionTestSample(pattern, channel, frame, frames))
		}
	}

	for start, iteration := 0, 0; start < frames; iteration++ {
		count := min(chunks[iteration%len(chunks)], frames-start)
		for channel := range block {
			block[channel] = input[channel][start : start+count]
		}

		if err := process(fused, block); err != nil {
			t.Fatal(err)
		}
		// Independent retained implementation: atomic scalar preflight, then
		// channel-major filter scratch and separate segmented aggregation. Do
		// not dispatch the reference through the new fused code.
		peak, err := preflightTargetPlanar(reference, block)
		if err != nil {
			t.Fatal(err)
		}

		if err := processPreparedTargetPlanarGeneric(reference, block, peak); err != nil {
			t.Fatal(err)
		}

		assertTargetFusionState(t, fused, reference)

		start += count
	}
	// Measurement reads immutable energies, so shallow value copies suffice
	// before the original analyzers independently sort their owned arrays.
	measuredFused, measuredReference := *fused, *reference
	actual, actualErr := finishFusionMeasurement(t, &measuredFused)

	wantActual, wantActualErr := finishFusionMeasurement(t, &measuredReference)
	if (actualErr == nil) != (wantActualErr == nil) || actual != wantActual || (actualErr != nil && (!errors.Is(actualErr, ErrBelowGate) || !errors.Is(wantActualErr, ErrBelowGate))) {
		t.Fatalf("measurement parity failed: %+v/%v != %+v/%v", actual, actualErr, wantActual, wantActualErr)
	}

	got, err := finishFusionTarget(t, fused)

	want, wantErr := finishFusionTarget(t, reference)
	if err != nil || wantErr != nil || got != want {
		t.Fatalf("target parity failed: %+v/%v != %+v/%v", got, err, want, wantErr)
	}

	for _, pair := range [][2]float64{
		{got.Plan.GainDB, want.Plan.GainDB},
		{got.Plan.Gain, want.Plan.Gain},
		{got.Plan.PredictedSamplePeak, want.Plan.PredictedSamplePeak},
		{got.SamplePeak, want.SamplePeak},
		{got.MeasuredLUFS, want.MeasuredLUFS},
		{got.PredictedLUFS, want.PredictedLUFS},
		{actual.LUFS, wantActual.LUFS},
	} {
		if math.Float64bits(pair[0]) != math.Float64bits(pair[1]) {
			t.Fatalf("final result changed float bits: %016x != %016x", math.Float64bits(pair[0]), math.Float64bits(pair[1]))
		}
	}
}

func TestTargetFusionSuccessfulBitParity(t *testing.T) {
	for _, rate := range []float64{8000, 8005, 11025, 44100.5, 48000, 96000, 384000} {
		for _, channels := range []int{1, 2} {
			for _, pattern := range []string{"transitions", "extreme", "tiny"} {
				for mode, chunks := range [][]int{{65536}, {1, 31, 4799, 3, 8191, 2, 65536}} {
					name := fmt.Sprintf("%g/%d/%s/chunks%d", rate, channels, pattern, mode)
					t.Run(name+"/float32", func(t *testing.T) {
						runTargetFusionParity(t, rate, channels, pattern, chunks, (*TargetAnalyzer).ProcessPlanar32)
					})
					t.Run(name+"/float64", func(t *testing.T) {
						runTargetFusionParity(t, rate, channels, pattern, chunks, (*TargetAnalyzer).ProcessPlanar)
					})
				}
			}
		}
	}
}

func TestTargetFusionTerminalOverflowResetAndAllocations(t *testing.T) {
	for _, channels := range []int{1, 2} {
		config := IntegratedConfig{SampleRate: 48000, Channels: channels, MaxFrames: 24000}

		a, err := NewTargetAnalyzer(config, -23)
		if err != nil {
			t.Fatal(err)
		}

		for _, value := range []float64{1e154, math.MaxFloat64} {
			a.Reset()

			block := make([][]float64, channels)
			for channel := range block {
				block[channel] = []float64{0, value}
			}

			if err := a.ProcessPlanar(block); !errors.Is(err, ErrNumericalOverflow) {
				t.Fatalf("fused overflow not terminal: %v", err)
			}

			if _, err := a.FinishMeasurementStep(1); !errors.Is(err, ErrNumericalOverflow) {
				t.Fatalf("poisoned state measured as finite: %v", err)
			}

			if !integratedFinite(a.SamplePeak()) {
				t.Fatal("terminal diagnostic peak became nonfinite")
			}
		}

		input := make([][]float32, channels)
		for channel := range input {
			input[channel] = make([]float32, 24000)
			for frame := range input[channel] {
				input[channel][frame] = float32(fusionTestSample("transitions", channel, frame, 24000))
			}
		}

		var failure error

		for _, measurementOnly := range []bool{false, true} {
			allocations := testing.AllocsPerRun(2, func() {
				a.Reset()

				if failure = a.ProcessPlanar32(input); failure != nil {
					return
				}

				for {
					var done bool
					if measurementOnly {
						done, failure = a.FinishMeasurementStep(256)
					} else {
						done, failure = a.FinishStep(256)
					}

					if done || failure != nil {
						return
					}
				}
			})
			if failure != nil || allocations != 0 {
				t.Fatalf("fused path/reset/finish allocated: channels=%d measurement=%v allocs=%g error=%v", channels, measurementOnly, allocations, failure)
			}
		}
	}
}
