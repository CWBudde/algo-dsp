package loudness

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestTargetOptimizedFloat32PreflightAtomicAndBitPatterns(t *testing.T) {
	a, err := NewTargetAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: 24000, ChannelWeights: []float64{1, 0}}, -23)
	if err != nil {
		t.Fatal(err)
	}

	if err := a.ProcessPlanar32([][]float32{{0.2, -0.1}, {0.3, -0.5}}); err != nil {
		t.Fatal(err)
	}

	snapshot := *a
	snapshot.filters = append([]targetFilterState{}, a.filters...)
	snapshot.energies = append([]float64{}, a.energies...)
	snapshot.workspace = append([]float64{}, a.workspace...)

	snapshot.blockEnergy = append([]float64{}, a.blockEnergy...)
	for _, input := range [][][]float32{
		{{1, 2}, {3}},
		{{1, 2}, {3, float32(math.Inf(1))}},
		{{1, 2}, {3, float32(math.Inf(-1))}},
		{{1, 2}, {3, math.Float32frombits(0x7f800001)}},
		{{1, 2}, {3, math.Float32frombits(0xffc00042)}},
		{make([]float32, 65537), make([]float32, 65537)},
		{make([]float32, 24000), make([]float32, 24000)},
	} {
		if err := a.ProcessPlanar32(input); err == nil {
			t.Fatal("invalid float32 block accepted")
		}

		if !reflect.DeepEqual(*a, snapshot) {
			t.Fatal("float32 shape/limit/nonfinite preflight mutated streaming state")
		}
	}

	a.Reset()

	bits := []uint32{0, 0x80000000, 1, 0x80000001, 0x7f7fffff, 0xff7fffff}

	input := make([]float32, len(bits))
	for index, value := range bits {
		input[index] = math.Float32frombits(value)
	}

	if err := a.ProcessPlanar32([][]float32{make([]float32, len(input)), input}); err != nil {
		t.Fatal(err)
	}

	if a.SamplePeak() != float64(math.MaxFloat32) {
		t.Fatalf("unsigned peak classification failed: %g", a.SamplePeak())
	}

	for index, value := range input {
		if math.Float32bits(value) != bits[index] {
			t.Fatalf("preflight changed caller bit pattern at %d", index)
		}
	}

	a.Reset()

	if a.SamplePeak() != 0 {
		t.Fatal("Reset retained bitwise peak")
	}

	if err := a.ProcessPlanar([][]float64{{1e100}, {0}}); err != nil {
		t.Fatal(err)
	}

	if err := a.ProcessPlanar32([][]float32{{0.1}, {-1}}); err != nil || a.SamplePeak() != 1e100 {
		t.Fatalf("mixed float32 call rounded/lowered prior float64 peak: %g error=%v", a.SamplePeak(), err)
	}
}

func TestTargetOptimizedHopOrderAndRoundedEndpoints(t *testing.T) {
	for _, rate := range []float64{8000, 11025, 44100.5, 48000, 384000} {
		frames := int(math.Ceil(rate * 0.61))
		config := IntegratedConfig{SampleRate: rate, Channels: 2, MaxFrames: int64(frames)}

		got, err := NewTargetAnalyzer(config, -23)
		if err != nil {
			t.Fatal(err)
		}

		want, err := NewTargetAnalyzer(config, -23)
		if err != nil {
			t.Fatal(err)
		}

		input := make([][]float32, 2)
		for channel := range input {
			input[channel] = make([]float32, frames)
			for frame := range input[channel] {
				input[channel][frame] = float32((0.2 + float64(channel)*0.05) * math.Sin(2*math.Pi*1000*float64(frame)/rate))
			}
		}

		sizes := []int{1, 127, 4093, 2, 65536}
		for start, chunk := 0, 0; start < frames; chunk++ {
			count := min(sizes[chunk%len(sizes)], frames-start)
			block := [][]float32{input[0][start : start+count], input[1][start : start+count]}

			peak, err := preflightTargetPlanar32(got, block)
			if err != nil {
				t.Fatal(err)
			}
			// Exercise the retained generic segment aggregation specifically;
			// mono/stereo fusion has independent prepared-path parity tests.
			if err := processPreparedTargetPlanarGeneric(got, block, peak); err != nil {
				t.Fatal(err)
			}
			// Original scalar aggregation is a reference for exact addition
			// ordering. Both paths consume identical prepared filter energies.
			for _, energy := range got.blockEnergy[:count] {
				want.hopSum += energy
				if !integratedFinite(want.hopSum) {
					t.Fatal("reference overflow")
				}

				want.frames++
				if want.frames == want.nextHop {
					if err := want.recordHop(); err != nil {
						t.Fatal(err)
					}
				}
			}

			if !reflect.DeepEqual(got.energies, want.energies) || got.hopSum != want.hopSum || got.hops != want.hops || got.frames != want.frames || got.nextHop != want.nextHop || got.absMean != want.absMean {
				t.Fatalf("segment aggregation changed scalar order or endpoint at rate=%g frame=%d", rate, start+count)
			}

			start += count
		}

		if len(got.energies) != 3 {
			t.Fatalf("complete-window count drifted at rate=%g: %d", rate, len(got.energies))
		}

		want.peak = got.peak
		if actual, expected := finishTargetBasic(t, got), finishTargetBasic(t, want); actual != expected {
			t.Fatalf("normalization result changed at rate=%g: %+v != %+v", rate, actual, expected)
		}
	}
}

func TestTargetOptimizedOverflowInsidePartialHopAndAtEndpoint(t *testing.T) {
	for _, frames := range []int{1, 4800} {
		a, err := NewTargetAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 1, MaxFrames: 24000}, -23)
		if err != nil {
			t.Fatal(err)
		}

		input := make([]float64, frames)

		input[frames-1] = 1e154 // finite filter states, but infinite squared energy
		if err := a.ProcessPlanar([][]float64{input}); !errors.Is(err, ErrNumericalOverflow) {
			t.Fatalf("overflow delayed beyond partial/complete hop (frames=%d): %v", frames, err)
		}

		if err := a.ProcessPlanar32([][]float32{{0.1}}); !errors.Is(err, ErrNumericalOverflow) {
			t.Fatalf("overflow did not poison both precision paths: %v", err)
		}

		if _, err := a.FinishMeasurementStep(1); !errors.Is(err, ErrNumericalOverflow) {
			t.Fatalf("terminal overflow finalized as finite: %v", err)
		}

		a.Reset()

		if err := a.ProcessPlanar(integratedBenchmarkFixture(24000)[:1]); err != nil {
			t.Fatal(err)
		}

		if result := finishTargetMeasurementTest(t, a); result.Frames != 24000 {
			t.Fatalf("Reset retained partial failed hop: %+v", result)
		}
	}
}
