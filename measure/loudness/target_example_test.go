package loudness_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/measure/loudness"
)

func ExampleTargetAnalyzer() {
	config := loudness.IntegratedConfig{SampleRate: 48000, Channels: 1, MaxFrames: 48000}

	analyzer, err := loudness.NewTargetAnalyzer(config, -23)
	if err != nil {
		panic(err)
	}

	input := make([]float32, 48000)
	for frame := range input {
		input[frame] = float32(0.2 * math.Sin(2*math.Pi*1000*float64(frame)/48000))
	}

	if err := analyzer.ProcessPlanar32([][]float32{input}); err != nil {
		panic(err)
	}

	for {
		done, err := analyzer.FinishStep(256)
		if err != nil {
			panic(err)
		}

		if done {
			break
		}
		// A worker can service cancellation between these bounded calls.
	}

	result, err := analyzer.Result()
	if err != nil {
		panic(err)
	}

	fmt.Printf("predicted %.2f LUFS; verify float32 output: %t\n", result.PredictedLUFS, result.NeedsFloat32Verification)
	// Output: predicted -23.00 LUFS; verify float32 output: true
}

func ExampleTargetAnalyzer_FinishMeasurementStep() {
	analyzer, err := loudness.NewTargetAnalyzer(loudness.IntegratedConfig{SampleRate: 48000, Channels: 1, MaxFrames: 48000}, -23)
	if err != nil {
		panic(err)
	}

	input := make([]float32, 48000)
	for frame := range input {
		input[frame] = float32(0.2 * math.Sin(2*math.Pi*1000*float64(frame)/48000))
	}

	if err := analyzer.ProcessPlanar32([][]float32{input}); err != nil {
		panic(err)
	}

	for {
		done, err := analyzer.FinishMeasurementStep(256)
		if err != nil {
			panic(err)
		}

		if done {
			break
		}
	}

	result, err := analyzer.MeasurementResult()
	if err != nil {
		panic(err)
	}

	fmt.Printf("measured %d actual frames, finite LUFS: %t\n", result.Frames, !math.IsInf(result.LUFS, 0) && !math.IsNaN(result.LUFS))
	// Output: measured 48000 actual frames, finite LUFS: true
}

func ExampleTargetAnalyzer_SamplePeak() {
	analyzer, err := loudness.NewTargetAnalyzer(loudness.IntegratedConfig{SampleRate: 48000, Channels: 1, MaxFrames: 48000}, -23)
	if err != nil {
		panic(err)
	}

	if err := analyzer.ProcessPlanar32([][]float32{{0.25, -0.5}}); err != nil {
		panic(err)
	}

	fmt.Printf("progressive sample peak %.2f\n", analyzer.SamplePeak())
	analyzer.Reset()
	fmt.Printf("after Reset %.2f\n", analyzer.SamplePeak())
	// Output:
	// progressive sample peak 0.50
	// after Reset 0.00
}
