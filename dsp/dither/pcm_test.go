package dither

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
)

func pcmTestNoise(rng *rand.Rand, kind DitherType) float64 {
	switch kind {
	case DitherRectangular:
		return rng.Float64()*2 - 1
	case DitherTriangular:
		return rng.Float64() - rng.Float64()
	case DitherGaussian:
		return rng.NormFloat64()
	case DitherFastGaussian:
		var sum float64
		for range 6 {
			sum += rng.Float64()
		}

		return sum - 3
	default:
		return 0
	}
}

func TestQuantizerPCMReferenceAndLegacyParity(t *testing.T) {
	for _, bits := range []int{1, 8, 16, 24, 32} {
		for kind := DitherNone; kind <= DitherFastGaussian; kind++ {
			for _, pcm := range []bool{false, true} {
				for _, coefficients := range [][]float64{nil, {1}, {1, -.5}, Preset9FC.Coefficients(), PresetSBM.Coefficients(), SharpPresetForSampleRate(48000)} {
					t.Run(fmt.Sprintf("%d/%s/pcm%t/order%d", bits, kind, pcm, len(coefficients)), func(t *testing.T) {
						options := []Option{WithBitDepth(bits), WithDitherType(kind), WithNoiseShaper(NewFIRShaper(coefficients)), WithRNG(rand.New(rand.NewPCG(42, 0)))}
						if pcm {
							options = append(options, WithPCMQuantization())
						}

						q, err := NewQuantizer(48000, options...)
						if err != nil {
							t.Fatal(err)
						}

						rng := rand.New(rand.NewPCG(42, 0))

						scale := math.Exp2(float64(bits - 1))
						if !pcm {
							scale -= .5
						}

						lo, hi := -math.Exp2(float64(bits-1)), math.Exp2(float64(bits-1))-1
						// Independent chronological history, without the production
						// shaper ring or quantizer helpers. Test full-scale then tails.
						history := make([]float64, len(coefficients))

						for frame := range 1025 {
							input := float64((frame*17)%101-50) / 64
							if frame < 8 {
								input = []float64{1, -1, 0, .5, -.5, 1.125, -1.125, 0}[frame]
							}

							scaledInput := input
							if pcm {
								scaledInput = max(-1, min(1, scaledInput))
							}

							shaped := scaledInput * scale
							for index, coefficient := range coefficients {
								shaped -= coefficient * history[index]
							}

							want := shaped + pcmTestNoise(rng, kind)
							if pcm {
								want = math.Round(want)
							} else {
								want = math.Floor(want)
							}

							quantizationError := want - shaped

							want = max(lo, min(hi, want))
							if !pcm {
								quantizationError = want - shaped
							}

							if got := q.ProcessInteger(input); got != int(want) {
								t.Fatalf("frame%d got%d want%.0f", frame, got, want)
							}

							if len(history) > 0 {
								copy(history[1:], history)
								history[0] = quantizationError
							}
						}
					})
				}
			}
		}
	}
}

func TestQuantizerPCMOverloadRecovery(t *testing.T) {
	for _, bits := range []int{8, 16, 24, 32} {
		for kind := DitherNone; kind <= DitherFastGaussian; kind++ {
			for preset := PresetNone; preset < presetCount; preset++ {
				newQuantizer := func() *Quantizer {
					q, err := NewQuantizer(48000, WithPCMQuantization(), WithBitDepth(bits), WithDitherType(kind), WithFIRPreset(preset), WithRNG(rand.New(rand.NewPCG(42, 0))))
					if err != nil {
						t.Fatal(err)
					}

					return q
				}
				overload, reference := newQuantizer(), newQuantizer()

				for frame := range 2049 {
					input := 0.
					if frame < 257 {
						input = []float64{math.MaxFloat32, -math.MaxFloat32, 1e20, -1e20, math.MaxFloat64, -math.MaxFloat64, 1.125, -1.125}[frame%8]
					}

					got := overload.ProcessInteger(input)

					want := reference.ProcessInteger(max(-1, min(1, input)))
					if got != want {
						t.Fatalf("overload recovery bits%d kind%s preset%s frame%d got%d want%d", bits, kind, preset, frame, got, want)
					}

					if frame > 300 && math.Abs(float64(got)) > 128 {
						t.Fatalf("persistent clipping distortion bits%d kind%s preset%s frame%d code%d", bits, kind, preset, frame, got)
					}
				}
			}
		}
	}
}

func TestQuantizerClipsBeforeIntegerConversion(t *testing.T) {
	for _, bits := range []int{8, 16, 24, 32} {
		for _, pcm := range []bool{false, true} {
			for kind := DitherNone; kind <= DitherFastGaussian; kind++ {
				for _, input := range []float64{1e20, -1e20, math.MaxFloat64, -math.MaxFloat64} {
					options := []Option{WithBitDepth(bits), WithDitherType(kind), WithFIRPreset(PresetNone), WithRNG(rand.New(rand.NewPCG(42, 0)))}
					if pcm {
						options = append(options, WithPCMQuantization())
					}

					q, err := NewQuantizer(48000, options...)
					if err != nil {
						t.Fatal(err)
					}

					want := int((uint64(1) << uint(bits-1)) - 1)
					if input < 0 {
						want = -want - 1
					}

					if got := q.ProcessInteger(input); got != want {
						t.Fatalf("bits%d pcm%t kind%s input%g got%d want%d", bits, pcm, kind, input, got, want)
					}
				}
			}
		}
	}
}

func TestQuantizerPCMScaleAndSilenceStatistics(t *testing.T) {
	q, err := NewQuantizer(48000, WithPCMQuantization(), WithBitDepth(16), WithDitherType(DitherNone), WithFIRPreset(PresetNone))
	if err != nil {
		t.Fatal(err)
	}

	for _, input := range []float64{0, .25, -.25, math.Exp2(-16), -math.Exp2(-16), 1, -1} {
		want := max(-32768., min(32767., math.Round(input*32768)))
		if got := q.ProcessSample(input); got != want/32768 {
			t.Fatalf("input%g got%g want%g", input, got, want/32768)
		}
	}

	if err := q.SetBitDepth(8); err != nil {
		t.Fatal(err)
	}

	if got := q.ProcessInteger(.5); got != 64 {
		t.Fatalf("changed depth scale: %d", got)
	}

	q, err = NewQuantizer(48000, WithPCMQuantization(), WithDitherType(DitherTriangular), WithFIRPreset(PresetNone), WithRNG(rand.New(rand.NewPCG(42, 0))))
	if err != nil {
		t.Fatal(err)
	}

	var sum, square float64

	const samples = 200000
	for range samples {
		value := float64(q.ProcessInteger(0))
		sum += value
		square += value * value
	}

	if mean := sum / samples; math.Abs(mean) > .005 {
		t.Fatalf("silent TPDF PCM bias %.6f LSB", mean)
	}

	if energy := square / samples; math.Abs(energy-.25) > .005 {
		t.Fatalf("silent TPDF PCM noise %.6f LSB², want .25", energy)
	}
}

func TestQuantizerPCMPartitionParityAndAllocations(t *testing.T) {
	newQuantizer := func() *Quantizer {
		q, err := NewQuantizer(48000, WithPCMQuantization(), WithSharpPreset(), WithRNG(rand.New(rand.NewPCG(42, 0))))
		if err != nil {
			t.Fatal(err)
		}

		return q
	}

	input := make([]float64, 2051)
	for index := range input {
		input[index] = float64(index%31-15) / 32
	}

	want := append([]float64(nil), input...)
	newQuantizer().ProcessInPlace(want)
	q := newQuantizer()

	for start, iteration := 0, 0; start < len(input); iteration++ {
		end := min(len(input), start+[]int{1, 17, 1024, 3}[iteration%4])
		q.ProcessInPlace(input[start:end])
		start = end
	}

	for index := range input {
		if math.Float64bits(input[index]) != math.Float64bits(want[index]) {
			t.Fatalf("partition sample%d differs", index)
		}
	}

	if got := testing.AllocsPerRun(100, func() { q.ProcessInteger(.25) }); got != 0 {
		t.Fatalf("allocated%g", got)
	}
}
