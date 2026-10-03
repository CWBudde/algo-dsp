package loudness_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/measure/loudness"
)

// These checks reuse only the test-only direct-form-I / prefix-sum oracle,
// never the production filters, source analyzer or normalization planner.
func TestTargetIndependentRatesWeightsAndStoredFloat32(t *testing.T) {
	for _, rate := range []float64{8000, 8005, 11025, 44100, 48000, 48000.5, 96000, 384000} {
		t.Run(fmt.Sprint(rate), func(t *testing.T) {
			frames := int64(math.Round(rate * 1.3))
			config := loudness.IntegratedConfig{SampleRate: rate, Channels: 3, ChannelWeights: []float64{1, 1.41, 0}, MaxFrames: frames}

			source := func(channel int, frame int64) float64 {
				if channel == 2 {
					return 0.75
				} // Excluded from loudness, included in linked peak.

				amplitude := 0.1 + 0.07*float64(frame)/rate
				if frame > int64(rate*0.4) && frame < int64(rate*0.8) {
					amplitude *= 0.0001
				}

				return amplitude * math.Sin(2*math.Pi*(997+113*float64(channel))*float64(frame)/rate)
			}
			for _, as32 := range []bool{false, true} {
				input := source
				if as32 {
					input = func(ch int, f int64) float64 { return float64(float32(source(ch, f))) }
				}

				whole := targetConformanceAnalyze(t, config, -23, []int{65536}, input, as32)

				irregular := targetConformanceAnalyze(t, config, -23, []int{1, 7, 4799, 31, 65536, 3}, input, as32)
				if whole != irregular {
					t.Fatalf("partition changed plan: %+v != %+v", whole, irregular)
				}

				want := conformanceReference(config, frames, input)
				conformanceNear(t, "source DF1", whole.MeasuredLUFS, want.LUFS, 1e-8)
				conformanceNear(t, "source peak", whole.SamplePeak, 0.75, 0)

				if !whole.HasMeasuredLUFS || !whole.NeedsFloat32Verification || whole.Frames != frames {
					t.Fatalf("unqualified plan: %+v", whole)
				}

				gained := func(ch int, f int64) float64 {
					value := input(ch, f) * whole.Plan.Gain
					if as32 {
						return float64(float32(value))
					}

					return value
				}
				actual := conformanceReference(config, frames, gained)
				conformanceNear(t, "fresh gained DF1", actual.LUFS, -23, 0.00001)
				measured := targetConformanceMeasurement(t, config, gained)
				conformanceNear(t, "fast candidate measurement DF1", measured.LUFS, actual.LUFS, 1e-8)
				conformanceNear(t, "fast candidate peak", measured.SamplePeak, actual.SamplePeak, 0)
				conformanceNear(t, "predicted target", whole.PredictedLUFS, -23, 1e-9)
			}
		})
	}
}

func targetConformanceMeasurement(t *testing.T, config loudness.IntegratedConfig, source func(int, int64) float64) loudness.IntegratedResult {
	t.Helper()

	a, err := loudness.NewTargetAnalyzer(config, -23)
	if err != nil {
		t.Fatal(err)
	}

	block := make([][]float64, config.Channels)
	for channel := range block {
		block[channel] = make([]float64, 8191)
	}

	for start := int64(0); start < config.MaxFrames; {
		count := int(min(config.MaxFrames-start, 8191))
		for channel := range block {
			block[channel] = block[channel][:count]
			for frame := range count {
				block[channel][frame] = source(channel, start+int64(frame))
			}
		}

		if err := a.ProcessPlanar(block); err != nil {
			t.Fatal(err)
		}

		start += int64(count)
	}

	for steps := 0; steps < 100000; steps++ {
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

	t.Fatal("bounded measurement did not finish")

	return loudness.IntegratedResult{}
}

func TestTargetIndependentStaticGoldenAndBelowOriginalGate(t *testing.T) {
	const frames int64 = 57600

	source := func(channel int, frame int64) float64 {
		amplitude := 1.0
		if frame >= 38400 {
			amplitude = 2
		} else if frame >= 19200 {
			amplitude = 0.0001
		}

		if channel == 0 {
			return float64(frame%31-15) / 128 * amplitude
		}

		return float64(frame%17-8) / 64 * amplitude
	}
	config := loudness.IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: frames}
	got := targetConformanceAnalyze(t, config, -23, []int{13, 4096, 257, 65536}, source, false)
	// Offline DF1 golden from integrated_conformance_test.go; the gate set is
	// unchanged for this attenuation, so linked target gain has an exact oracle.
	conformanceNear(t, "static source", got.MeasuredLUFS, -14.970897903199129, 1e-9)
	conformanceNear(t, "static linked gain", got.Plan.GainDB, -8.029102096800871, 1e-9)

	quiet := func(ch int, f int64) float64 { return source(ch, f) * 1e-7 }

	below := targetConformanceAnalyze(t, config, -23, []int{1, 37, 65536}, quiet, false)
	if below.HasMeasuredLUFS || below.MeasuredLUFS != 0 {
		t.Fatalf("fabricated below-gate source LUFS: %+v", below)
	}

	actual := conformanceReference(config, frames, func(ch int, f int64) float64 { return quiet(ch, f) * below.Plan.Gain })
	conformanceNear(t, "below-gate boost DF1", actual.LUFS, -23, 1e-8)
}

func targetConformanceAnalyze(t *testing.T, config loudness.IntegratedConfig, target float64, chunks []int, source func(int, int64) float64, as32 bool) loudness.TargetResult {
	t.Helper()

	a, err := loudness.NewTargetAnalyzer(config, target)
	if err != nil {
		t.Fatal(err)
	}

	block := make([][]float64, config.Channels)

	block32 := make([][]float32, config.Channels)
	for ch := range block {
		block[ch] = make([]float64, 65536)
		block32[ch] = make([]float32, 65536)
	}

	for start, iteration := int64(0), 0; start < config.MaxFrames; iteration++ {
		count := int(min(config.MaxFrames-start, int64(chunks[iteration%len(chunks)])))
		for ch := range block {
			block[ch], block32[ch] = block[ch][:count], block32[ch][:count]
			for frame := range count {
				value := source(ch, start+int64(frame))
				block[ch][frame], block32[ch][frame] = value, float32(value)
			}
		}

		if as32 {
			err = a.ProcessPlanar32(block32)
		} else {
			err = a.ProcessPlanar(block)
		}

		if err != nil {
			t.Fatal(err)
		}

		for ch := range block {
			for frame := range count {
				value := source(ch, start+int64(frame))
				if as32 {
					if math.Float32bits(block32[ch][frame]) != math.Float32bits(float32(value)) {
						t.Fatal("mutated f32 input")
					}
				} else if math.Float64bits(block[ch][frame]) != math.Float64bits(value) {
					t.Fatal("mutated f64 input")
				}
			}
		}

		start += int64(count)
	}

	for calls := 0; calls < 1000000; calls++ {
		done, err := a.FinishStep(1)
		if err != nil {
			t.Fatal(err)
		}

		if done {
			result, err := a.Result()
			if err != nil {
				t.Fatal(err)
			}

			if done, err := a.FinishStep(1); !done || err != nil {
				t.Fatal("completed plan is not idempotent", err)
			}

			return result
		}
	}

	t.Fatal("bounded planner did not terminate")

	return loudness.TargetResult{}
}
