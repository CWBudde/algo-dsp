package loudness_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/measure/loudness"
)

func ExampleIntegratedAnalyzer() {
	// Stereo is explicitly two unit-weight channels, not inferred surround.
	analyzer, err := loudness.NewIntegratedAnalyzer(loudness.IntegratedConfig{
		SampleRate: 48000, Channels: 2, ChannelWeights: []float64{1, 1}, MaxFrames: 48000,
	})
	if err != nil {
		panic(err)
	}

	block := make([]float32, 4800)
	for chunk := range 10 {
		for i := range block {
			block[i] = float32(0.1 * math.Sin(2*math.Pi*1000*float64(chunk*len(block)+i)/48000))
		}

		if err := analyzer.ProcessPlanar32([][]float32{block, block}); err != nil {
			panic(err)
		}
	}

	for {
		done, err := analyzer.FinishStep(2)
		if err != nil {
			panic(err)
		}

		if done {
			break
		}
	}

	result, err := analyzer.Result()
	if err != nil {
		panic(err)
	}

	plan, err := loudness.PlanNormalization(result, -23)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%d frames, finite loudness: %v, attenuation: %v\n", result.Frames, !math.IsInf(result.LUFS, 0), plan.Gain < 1)
	// Output: 48000 frames, finite loudness: true, attenuation: true
}

func ExampleNormalizeLoudness() {
	input := make([]float64, 19200)
	for i := range input {
		input[i] = 0.5 * math.Sin(2*math.Pi*1000*float64(i)/48000)
	}

	output, err := loudness.NormalizeLoudness([][]float64{input}, -23, loudness.IntegratedConfig{
		SampleRate: 48000, Channels: 1, MaxFrames: int64(len(input)),
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("%d channel, %d frames, fresh output: %v\n", len(output), len(output[0]), &output[0][0] != &input[0])
	// Output: 1 channel, 19200 frames, fresh output: true
}
