package loudness

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
)

func certifiedTestPeak(block [][]float32) float64 {
	var peak float64

	for _, channel := range block {
		for _, value := range channel {
			peak = max(peak, math.Abs(float64(value)))
		}
	}

	return peak
}

func TestTargetCertifiedPlanar32IndependentParity(t *testing.T) {
	for _, rate := range []float64{8000, 44100.5, 48000, 384000} {
		for _, channels := range []int{1, 2} {
			for _, pattern := range []string{"transitions", "tiny", "extreme"} {
				t.Run(fmt.Sprintf("%g/%d/%s", rate, channels, pattern), func(t *testing.T) {
					runTargetFusionParity(t, rate, channels, pattern, []int{1, 31, 4799, 3, 8191, 2, 65536}, func(a *TargetAnalyzer, block [][]float32) error {
						return a.ProcessCertifiedPlanar32(block, certifiedTestPeak(block))
					})
				})
			}
		}
	}
}

func TestTargetCertifiedCustomWeightsAndPartitionParity(t *testing.T) {
	const frames = 5000

	config := IntegratedConfig{SampleRate: 8000, Channels: 3, MaxFrames: frames, ChannelWeights: []float64{1, 0, 0.5}}

	ordinary, err := NewTargetAnalyzer(config, -23)
	if err != nil {
		t.Fatal(err)
	}

	certified, err := NewTargetAnalyzer(config, -23)
	if err != nil {
		t.Fatal(err)
	}

	input := make([][]float32, 3)
	for channel := range input {
		input[channel] = make([]float32, frames)
		for frame := range input[channel] {
			input[channel][frame] = float32(fusionTestSample("transitions", channel, frame, frames))
		}
	}
	// A channel excluded from loudness still contributes to diagnostic peak.
	input[1][2700] = -32
	if err := ordinary.ProcessPlanar32(input); err != nil {
		t.Fatal(err)
	}

	block := make([][]float32, 3)

	for start := 0; start < frames; {
		count := min(127, frames-start)
		for channel := range block {
			block[channel] = input[channel][start : start+count]
		}

		if err := certified.ProcessCertifiedPlanar32(block, certifiedTestPeak(block)); err != nil {
			t.Fatal(err)
		}

		start += count
	}

	assertTargetFusionState(t, certified, ordinary)

	if certified.peak != 32 {
		t.Fatalf("zero-weight channel peak lost: %g", certified.peak)
	}

	actual, err := finishFusionTarget(t, certified)

	want, wantErr := finishFusionTarget(t, ordinary)
	if err != nil || wantErr != nil || actual != want {
		t.Fatalf("partition/weight target parity: %+v/%v != %+v/%v", actual, err, want, wantErr)
	}
}

func TestTargetCertifiedCertificateValidationAtomic(t *testing.T) {
	config := IntegratedConfig{SampleRate: 8000, Channels: 2, MaxFrames: 8000}

	for _, peak := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -1, math.MaxFloat64, float64(math.MaxFloat32) * 2, 0.1, math.SmallestNonzeroFloat64} {
		t.Run(fmt.Sprintf("%g", peak), func(t *testing.T) {
			a, err := NewTargetAnalyzer(config, -23)
			if err != nil {
				t.Fatal(err)
			}

			control, err := NewTargetAnalyzer(config, -23)
			if err != nil {
				t.Fatal(err)
			}

			block := [][]float32{{0.25, -0.5}, {-0.25, 0.5}}
			for _, analyzer := range []*TargetAnalyzer{a, control} {
				if err := analyzer.ProcessPlanar32(block); err != nil {
					t.Fatal(err)
				}
			}

			if err := a.ProcessCertifiedPlanar32(block, peak); !errors.Is(err, ErrInvalid) {
				t.Fatalf("certificate error: %v", err)
			}

			if !reflect.DeepEqual(a, control) {
				t.Fatal("invalid certificate changed analyzer state")
			}
		})
	}
}

func TestTargetCertifiedStateAndShapeValidation(t *testing.T) {
	var nilAnalyzer *TargetAnalyzer
	for _, a := range []*TargetAnalyzer{nilAnalyzer, {}} {
		if err := a.ProcessCertifiedPlanar32(nil, 0); !errors.Is(err, ErrState) {
			t.Fatalf("unconfigured error: %v", err)
		}
	}

	config := IntegratedConfig{SampleRate: 8000, Channels: 2, MaxFrames: 8000}

	for _, test := range []struct {
		name  string
		block [][]float32
		peak  float64
		want  error
	}{
		{"no channels", nil, 0, ErrInvalid},
		{"channel count", [][]float32{{1}}, 1, ErrInvalid},
		{"unequal", [][]float32{{1}, {}}, 1, ErrInvalid},
		{"max frames", [][]float32{make([]float32, 8001), make([]float32, 8001)}, 0, ErrLimit},
		{"block limit", [][]float32{make([]float32, MaxIntegratedBlockFrames+1), make([]float32, MaxIntegratedBlockFrames+1)}, 0, ErrLimit},
		{"empty nonzero peak", [][]float32{{}, {}}, 1, ErrInvalid},
	} {
		t.Run(test.name, func(t *testing.T) {
			a, err := NewTargetAnalyzer(config, -23)
			if err != nil {
				t.Fatal(err)
			}

			control, err := NewTargetAnalyzer(config, -23)
			if err != nil {
				t.Fatal(err)
			}

			if err := a.ProcessCertifiedPlanar32(test.block, test.peak); !errors.Is(err, test.want) {
				t.Fatalf("validation error: %v", err)
			}

			if !reflect.DeepEqual(a, control) {
				t.Fatal("invalid shape changed state")
			}
		})
	}

	a, err := NewTargetAnalyzer(config, -23)
	if err != nil {
		t.Fatal(err)
	}

	if err := a.ProcessCertifiedPlanar32([][]float32{{}, {}}, math.Copysign(0, -1)); err != nil {
		t.Fatal(err)
	}

	if math.Signbit(a.peak) {
		t.Fatal("negative zero peak was not canonicalized")
	}

	if _, err := a.FinishStep(1); !errors.Is(err, ErrTooShort) {
		t.Fatalf("finish error: %v", err)
	}

	if err := a.ProcessCertifiedPlanar32([][]float32{{1}, {1}}, 1); !errors.Is(err, ErrTooShort) {
		t.Fatalf("terminal failure lost: %v", err)
	}

	a.Reset()

	finalBlock := [][]float32{make([]float32, 4000), make([]float32, 4000)}
	for channel := range finalBlock {
		for frame := range finalBlock[channel] {
			finalBlock[channel][frame] = float32(fusionTestSample("transitions", channel, frame, 4000))
		}
	}

	if err := a.ProcessCertifiedPlanar32(finalBlock, certifiedTestPeak(finalBlock)); err != nil {
		t.Fatal(err)
	}

	_, _ = finishFusionMeasurement(t, a)
	if err := a.ProcessCertifiedPlanar32([][]float32{{1}, {1}}, 1); !errors.Is(err, ErrState) {
		t.Fatalf("finalized error: %v", err)
	}
}

func TestTargetCertifiedProcessAllocations(t *testing.T) {
	a, err := NewTargetAnalyzer(IntegratedConfig{SampleRate: 8000, Channels: 2, MaxFrames: 8000}, -23)
	if err != nil {
		t.Fatal(err)
	}

	block := [][]float32{{0.25, -0.5}, {-0.125, 0.25}}

	if got := testing.AllocsPerRun(100, func() {
		a.Reset()

		if err := a.ProcessCertifiedPlanar32(block, 0.5); err != nil {
			t.Fatal(err)
		}
	}); got != 0 {
		t.Fatalf("processing allocated %g objects", got)
	}
}

func TestTargetCertifiedMixedPrecisionAndInputOwnership(t *testing.T) {
	config := IntegratedConfig{SampleRate: 8000, Channels: 1, MaxFrames: 8000}

	a, err := NewTargetAnalyzer(config, -23)
	if err != nil {
		t.Fatal(err)
	}

	control, err := NewTargetAnalyzer(config, -23)
	if err != nil {
		t.Fatal(err)
	}

	for _, analyzer := range []*TargetAnalyzer{a, control} {
		if err := analyzer.ProcessPlanar([][]float64{{1e40}}); err != nil {
			t.Fatal(err)
		}
	}

	block := [][]float32{{math.Float32frombits(0x80000000), -math.MaxFloat32, math.SmallestNonzeroFloat32}}

	before := append([]float32(nil), block[0]...)
	if err := a.ProcessCertifiedPlanar32(block, math.MaxFloat32); err != nil {
		t.Fatal(err)
	}

	if err := control.ProcessPlanar32(block); err != nil {
		t.Fatal(err)
	}

	for i := range block[0] {
		if math.Float32bits(block[0][i]) != math.Float32bits(before[i]) {
			t.Fatal("caller samples changed")
		}

		block[0][i] = 0
	}

	assertTargetFusionState(t, a, control)

	if a.peak != 1e40 {
		t.Fatalf("prior float64 peak changed: %g", a.peak)
	}

	if !reflect.DeepEqual(a, control) {
		t.Fatal("analyzer retained caller samples")
	}
}

func ExampleTargetAnalyzer_ProcessCertifiedPlanar32() {
	a, err := NewTargetAnalyzer(IntegratedConfig{SampleRate: 8000, Channels: 1, MaxFrames: 4000}, -23)
	if err != nil {
		panic(err)
	}
	// This immutable fixture has already been proved finite with exact peak .25.
	block := [][]float32{make([]float32, 4000)}
	for i := range block[0] {
		block[0][i] = 0.25
	}

	if err := a.ProcessCertifiedPlanar32(block, 0.25); err != nil {
		panic(err)
	}

	for {
		done, err := a.FinishMeasurementStep(32)
		if err != nil {
			panic(err)
		}

		if done {
			break
		}
	}

	result, err := a.MeasurementResult()
	if err != nil {
		panic(err)
	}

	fmt.Println(result.Frames, result.SamplePeak)
	// Output: 4000 0.25
}
