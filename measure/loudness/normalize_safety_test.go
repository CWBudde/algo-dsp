package loudness

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestNormalizationSafetyPlanValidation(t *testing.T) {
	measured := IntegratedResult{LUFS: -20, SamplePeak: 0.5, Frames: 48000}

	plan, err := PlanNormalization(measured, -26.020599913279624)
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(plan.GainDB+6.020599913279624) > 1e-12 || math.Abs(plan.Gain-0.5) > 1e-12 || math.Abs(plan.PredictedSamplePeak-0.25) > 1e-12 {
		t.Fatalf("unexpected exact half-gain plan: %+v", plan)
	}

	for _, tc := range []struct {
		name     string
		measured IntegratedResult
		target   float64
	}{
		{"nan target", measured, math.NaN()},
		{"positive infinite target", measured, math.Inf(1)},
		{"negative infinite target", measured, math.Inf(-1)},
		{"gain overflow", measured, 1e6},
		{"gain underflow", measured, -1e6},
		{"nan measurement", IntegratedResult{LUFS: math.NaN(), SamplePeak: 0.5, Frames: 48000}, -23},
		{"infinite measurement", IntegratedResult{LUFS: math.Inf(-1), SamplePeak: 0.5, Frames: 48000}, -23},
		{"nan peak", IntegratedResult{LUFS: -20, SamplePeak: math.NaN(), Frames: 48000}, -23},
		{"infinite peak", IntegratedResult{LUFS: -20, SamplePeak: math.Inf(1), Frames: 48000}, -23},
		{"negative peak", IntegratedResult{LUFS: -20, SamplePeak: -1, Frames: 48000}, -23},
		{"predicted peak overflow", IntegratedResult{LUFS: -20, SamplePeak: math.MaxFloat64, Frames: 48000}, -10},
		{"predicted peak underflow", IntegratedResult{LUFS: -20, SamplePeak: 0.01, Frames: 48000}, -6480},
		{"subtraction overflow", IntegratedResult{LUFS: -math.MaxFloat64, SamplePeak: 0.5, Frames: 48000}, math.MaxFloat64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := PlanNormalization(tc.measured, tc.target); err == nil {
				t.Fatal("invalid normalization plan accepted")
			}
		})
	}
}

func TestNormalizationSafetyFreshOwnedOutputAndTarget(t *testing.T) {
	input := safetySignal(48000, 1)
	shared := [][]float64{input[0], input[0]}
	before := append([]float64{}, input[0]...)
	config := IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: 48000}

	measurer := safetyAnalyzer(t, 48000, 2)
	if err := measurer.ProcessPlanar(shared); err != nil {
		t.Fatal(err)
	}

	measured := safetyFinish(t, measurer)

	plan, err := PlanNormalization(measured, -23)
	if err != nil {
		t.Fatal(err)
	}

	output, err := NormalizeLoudness(shared, -23, config)
	if err != nil {
		t.Fatal(err)
	}

	if len(output) != 2 || len(output[0]) != 48000 || len(output[1]) != 48000 {
		t.Fatalf("normalization changed planar shape: %d channels", len(output))
	}

	if &output[0][0] == &input[0][0] || &output[1][0] == &input[0][0] || &output[0][0] == &output[1][0] {
		t.Fatal("fresh normalized channels alias input or each other")
	}

	for channel := range output {
		for frame, value := range output[channel] {
			if expected := before[frame] * plan.Gain; math.Abs(value-expected) > 1e-14 {
				t.Fatalf("gain ratio changed at channel=%d frame=%d: %g != %g", channel, frame, value, expected)
			}
		}
	}

	if !reflect.DeepEqual(input[0], before) {
		t.Fatal("normalization mutated caller input")
	}

	remeasure := safetyAnalyzer(t, 48000, 2)
	if err := remeasure.ProcessPlanar(output); err != nil {
		t.Fatal(err)
	}

	result := safetyFinish(t, remeasure)
	if math.Abs(result.LUFS+23) > 1e-8 || math.Abs(result.SamplePeak-plan.PredictedSamplePeak) > 1e-12 {
		t.Fatalf("normalized output did not reach target: %+v plan=%+v", result, plan)
	}

	output[0][7] = 42
	if output[1][7] == 42 || input[0][7] != before[7] {
		t.Fatal("output mutation escaped independently owned channel")
	}
}

func TestNormalizationSafetyOneShotFailuresDoNotMutate(t *testing.T) {
	for _, tc := range []struct {
		name      string
		input     [][]float64
		target    float64
		maxFrames int64
		want      error
	}{
		{"empty", [][]float64{{}, {}}, -23, 48000, ErrTooShort},
		{"short", safetySignal(19199, 2), -23, 48000, ErrTooShort},
		{"silence", [][]float64{make([]float64, 48000), make([]float64, 48000)}, -23, 48000, ErrBelowGate},
		{"missing channel", safetySignal(24000, 1), -23, 48000, nil},
		{"unequal", [][]float64{make([]float64, 24000), make([]float64, 23999)}, -23, 48000, nil},
		{"over frame limit", safetySignal(24000, 2), -23, 23999, ErrLimit},
		{"nonfinite late input", safetySignal(24000, 2), -23, 48000, ErrNonFinite},
		{"infinite late input", safetySignal(24000, 2), -23, 48000, ErrNonFinite},
		{"finite arithmetic overflow", [][]float64{{math.MaxFloat64}, {1}}, -23, 48000, ErrNumericalOverflow},
		{"nonfinite target", safetySignal(24000, 2), math.NaN(), 48000, nil},
		{"unrepresentable gain", safetySignal(24000, 2), 1e6, 48000, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "nonfinite late input" {
				tc.input[1][len(tc.input[1])-1] = math.NaN()
			}

			if tc.name == "infinite late input" {
				tc.input[1][len(tc.input[1])-1] = math.Inf(-1)
			}

			before := make([][]uint64, len(tc.input))
			for channel := range tc.input {
				before[channel] = make([]uint64, len(tc.input[channel]))
				for frame, value := range tc.input[channel] {
					before[channel][frame] = math.Float64bits(value)
				}
			}

			output, err := NormalizeLoudness(tc.input, tc.target, IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: tc.maxFrames})
			if err == nil || output != nil || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatalf("failure must not publish output: output=%v error=%v want=%v", output, err, tc.want)
			}

			for channel := range tc.input {
				for frame, value := range tc.input[channel] {
					if math.Float64bits(value) != before[channel][frame] {
						t.Fatalf("failure mutated input channel=%d frame=%d", channel, frame)
					}
				}
			}
		})
	}
}

func TestNormalizationSafetyOneShotChunksAndWeightZero(t *testing.T) {
	// More than the streaming call limit must be chunked by the one-shot helper,
	// not rejected, and weight-zero channels remain included in peak/gain.
	input := safetySignal(65537, 2)
	for frame := range input[1] {
		input[1][frame] = input[0][frame] * 2
	}

	config := IntegratedConfig{SampleRate: 48000, Channels: 2, ChannelWeights: []float64{1, 0}, MaxFrames: 65537}

	output, err := NormalizeLoudness(input, -23, config)
	if err != nil {
		t.Fatal(err)
	}

	for frame := range output[0] {
		if output[1][frame] != output[0][frame]*2 {
			t.Fatalf("weight-zero channel was not normalized at frame %d", frame)
		}
	}

	analyzer, err := NewIntegratedAnalyzer(config)
	if err != nil {
		t.Fatal(err)
	}

	for start := 0; start < len(output[0]); start += 65536 {
		end := min(start+65536, len(output[0]))
		if err := analyzer.ProcessPlanar([][]float64{output[0][start:end], output[1][start:end]}); err != nil {
			t.Fatal(err)
		}
	}

	if measured := safetyFinish(t, analyzer); math.Abs(measured.LUFS+23) > 1e-8 {
		t.Fatalf("chunked one-shot missed target: %+v", measured)
	}
}
