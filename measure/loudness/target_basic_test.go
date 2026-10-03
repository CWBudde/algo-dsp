package loudness

import (
	"errors"
	"math"
	"testing"
)

func finishTargetBasic(t *testing.T, analyzer *TargetAnalyzer) TargetResult {
	t.Helper()

	for steps := 0; steps < 100000; steps++ {
		done, err := analyzer.FinishStep(1)
		if err != nil {
			t.Fatal(err)
		}

		if done {
			result, err := analyzer.Result()
			if err != nil {
				t.Fatal(err)
			}

			return result
		}
	}

	t.Fatal("bounded target finish did not terminate")

	return TargetResult{}
}

func TestTargetBasicSourceAndFreshRemeasurement(t *testing.T) {
	for _, amplitude := range []float64{0.2, 0.000001} {
		t.Run(fmtAmplitudeBasic(amplitude), func(t *testing.T) {
			input := make([]float64, 48000)
			for frame := range input {
				input[frame] = amplitude * math.Sin(2*math.Pi*1000*float64(frame)/48000)
			}

			config := IntegratedConfig{SampleRate: 48000, Channels: 1, MaxFrames: 48000}

			analyzer, err := NewTargetAnalyzer(config, -23)
			if err != nil {
				t.Fatal(err)
			}

			if err := analyzer.ProcessPlanar([][]float64{input}); err != nil {
				t.Fatal(err)
			}

			result := finishTargetBasic(t, analyzer)
			if result.HasMeasuredLUFS != (amplitude == 0.2) || !result.NeedsFloat32Verification {
				t.Fatalf("source metric/rounding qualification missing: %+v", result)
			}

			if !result.HasMeasuredLUFS && result.MeasuredLUFS != 0 {
				t.Fatal("invented source LUFS below gate")
			}

			if math.Abs(result.PredictedLUFS+23) > 0.01 || result.Frames != 48000 {
				t.Fatalf("incorrect target result: %+v", result)
			}

			for frame := range input {
				input[frame] *= result.Plan.Gain
			}

			reference, err := NewIntegratedAnalyzer(config)
			if err != nil {
				t.Fatal(err)
			}

			if err := reference.ProcessPlanar([][]float64{input}); err != nil {
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

			measured, err := reference.Result()
			if err != nil || math.Abs(measured.LUFS+23) > 0.01 {
				t.Fatalf("independent gained output missed target: %+v error=%v", measured, err)
			}
		})
	}
}

func fmtAmplitudeBasic(amplitude float64) string {
	if amplitude == 0.2 {
		return "measurable source"
	}

	return "positive below-gate source"
}

func TestTargetBasicQuietWindowsReenterAndSortingIsBounded(t *testing.T) {
	const count = 502

	a, err := NewTargetAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 1, MaxFrames: (count + 3) * 4800}, -20)
	if err != nil {
		t.Fatal(err)
	}
	// Capture a deliberately unsorted window set: quiet windows originally
	// below -70 LUFS must join both gates after normalization. The old
	// target-minus-source-LUFS coefficient would miss by more than 20 LU.
	for index := 0; index < count; index++ {
		energy := 1e-8
		if index == 1 || index == count-1 {
			energy = 1e-5
			a.absCount++
			a.absMean += (energy - a.absMean) / float64(a.absCount)
		}

		a.energies = append(a.energies, energy)
	}

	a.frames, a.peak = (count+3)*4800, 0.01
	if done, err := a.FinishStep(1); done || err != nil || a.phase != targetSort {
		t.Fatalf("one work unit hid a complete sort: done=%v error=%v phase=%v", done, err, a.phase)
	}

	result := finishTargetBasic(t, a)
	mean := (500*1e-8 + 2*1e-5) / 502

	expected := -20 - (-0.691 + 10*math.Log10(mean))
	if math.Abs(result.Plan.GainDB-expected) > 1e-10 || math.Abs(result.PredictedLUFS+20) > 0.01 {
		t.Fatalf("below-gate windows were discarded: %+v expected gain dB=%g", result, expected)
	}

	if math.Abs(result.MeasuredLUFS-(-0.691+10*math.Log10(1e-5))) > 1e-12 {
		t.Fatalf("source measurement was re-gated by target: %+v", result)
	}
}

func TestTargetBasicAtomicRejectionResetAndAllocation(t *testing.T) {
	a, err := NewTargetAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 1, MaxFrames: 24000}, -23)
	if err != nil {
		t.Fatal(err)
	}

	input := make([]float32, 24000)
	for frame := range input {
		input[frame] = float32(0.2 * math.Sin(2*math.Pi*1000*float64(frame)/48000))
	}

	input[23999] = float32(math.NaN())
	if err := a.ProcessPlanar32([][]float32{input}); !errors.Is(err, ErrNonFinite) || a.frames != 0 || a.peak != 0 {
		t.Fatalf("preflight changed state: frames=%d peak=%g error=%v", a.frames, a.peak, err)
	}

	input[23999] = 0.1
	block := [][]float32{input}

	var failure error

	allocations := testing.AllocsPerRun(2, func() {
		a.Reset()

		if failure = a.ProcessPlanar32(block); failure != nil {
			return
		}

		for {
			var done bool

			done, failure = a.FinishStep(256)
			if done || failure != nil {
				return
			}
		}
	})
	if failure != nil || allocations != 0 {
		t.Fatalf("target path allocated: allocations=%g error=%v", allocations, failure)
	}

	first, err := a.Result()
	if err != nil {
		t.Fatal(err)
	}

	a.Reset()

	if err := a.ProcessPlanar32(block); err != nil {
		t.Fatal(err)
	}

	if next := finishTargetBasic(t, a); next != first {
		t.Fatalf("Reset left planning/sorting state: %+v != %+v", next, first)
	}
}
