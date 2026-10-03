package pitch

import (
	"math"
	"slices"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/interp"
)

// legacyOverlapSearch retains the scalar search before scratch-window caching.
// Its linear source history deliberately avoids the optimized ring/window path.
func legacyOverlapSearch(fx *PitchShifter, input []float64, count, previous, predicted int64) int64 {
	sample := func(position int64) float64 {
		if position < 0 || position >= count {
			return 0
		}

		return input[position]
	}
	best, score, refEnergy := predicted, math.Inf(-1), pitchShifterTiny

	for i := 0; i < fx.overlapLen; i++ {
		x := sample(previous + int64(fx.stepOut+i))
		refEnergy += x * x
	}

	for candidate := max(predicted-int64(fx.searchLen), 0); candidate <= predicted+int64(fx.searchLen); candidate++ {
		dot, energy := 0.0, pitchShifterTiny

		for i := 0; i < fx.overlapLen; i++ {
			ref := sample(previous + int64(fx.stepOut+i))
			x := sample(candidate + int64(i))
			dot += ref * x
			energy += x * x
		}

		correlation := dot / math.Sqrt(refEnergy*energy)
		if correlation > score {
			score, best = correlation, candidate
		}
	}

	return best
}

func overlapTestProcessor(t *testing.T, rate, ratio float64) *PitchShifter {
	t.Helper()

	fx, err := NewPitchShifter(rate)
	if err != nil {
		t.Fatal(err)
	}

	for _, configure := range []func() error{
		func() error { return fx.SetSequence(20) },
		func() error { return fx.SetOverlap(4) },
		func() error { return fx.SetSearch(3) },
		func() error { return fx.SetPitchRatio(ratio) },
	} {
		if err := configure(); err != nil {
			t.Fatal(err)
		}
	}

	return fx
}

func overlapVariedSignal(index int) float64 {
	// Deterministic tones, DC and impulses distinguish nearly tied candidates.
	x := float64(index)
	return 0.31*math.Sin(x*.137) + 0.19*math.Cos(x*.071) + float64((index*17)%29-14)/91
}

func TestStreamingOverlapLegacySearchParity(t *testing.T) {
	for _, test := range []struct {
		name               string
		count, previous    int64
		predicted          int64
		signal             func(int) float64
		firstTiedCandidate bool
	}{
		{"negative-clamped", 300, 80, -10, overlapVariedSignal, false},
		{"entirely-negative-search", 300, 80, -40, overlapVariedSignal, false},
		{"negative-reference", 300, -75, 120, overlapVariedSignal, false},
		{"future-zero-padding", 300, 80, 285, overlapVariedSignal, false},
		{"entirely-future-search", 300, 80, 360, overlapVariedSignal, true},
		{"candidate-window-wrap", 1300, 1100, 1020, overlapVariedSignal, false},
		{"reference-window-wrap", 1400, 880, 1150, overlapVariedSignal, false},
		{"long-history", 100000, 99700, 99800, overlapVariedSignal, false},
		{"silent-ties", 300, 80, 150, func(int) float64 { return 0 }, true},
		{"constant-ties", 300, 80, 150, func(int) float64 { return .25 }, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fx := overlapTestProcessor(t, 8000, 2)

			stream, err := NewStreamingPitchShifter(fx)
			if err != nil {
				t.Fatal(err)
			}

			input := make([]float64, test.count)
			for i := range input {
				input[i] = test.signal(i)
				stream.input[int64(i)&stream.inputMask] = input[i]
			}

			stream.count, stream.previousStart = test.count, test.previous

			want := legacyOverlapSearch(fx, input, test.count, test.previous, test.predicted)
			if got := stream.bestOverlap(test.predicted); got != want {
				t.Fatalf("overlap start %d want legacy %d", got, want)
			}

			if test.firstTiedCandidate && want != max(test.predicted-int64(fx.searchLen), 0) {
				t.Fatalf("tie chose %d, not first ascending candidate", want)
			}
		})
	}
}

// legacyOverlapOutput uses unbounded linear histories and the pre-optimization
// brute-force search. It preserves the established WSOLA arithmetic and order.
func legacyOverlapOutput(fx *PitchShifter, source []float64, latency int) []float64 {
	input := make([]float64, len(source))
	stretched := make([]float64, int(math.Ceil(float64(len(source))*fx.pitchRatio))+2*fx.sequenceLen)
	output := make([]float64, len(source))

	var count, frames, previous, finalized int64

	sample := func(position int64) float64 {
		if position < 0 || position >= count {
			return 0
		}

		return input[position]
	}
	makeFrame := func(start int64) {
		out := frames * int64(fx.stepOut)
		for i := 0; i < fx.sequenceLen; i++ {
			x := sample(start + int64(i))
			if frames > 0 && i < fx.overlapLen {
				x = sample(previous+int64(fx.stepOut+i))*fx.fadeOut[i] + x*fx.fadeIn[i]
			}

			stretched[out+int64(i)] = x
		}

		previous = start
		frames++
		finalized = out + int64(fx.stepOut)
	}

	for position, x := range source {
		input[position] = x
		count++

		if frames == 0 && count >= int64(fx.sequenceLen) {
			makeFrame(0)
		}

		for frames > 0 {
			predicted := int64(math.Round(float64(frames) * float64(fx.stepOut) / fx.pitchRatio))
			if count < predicted+int64(fx.searchLen+fx.sequenceLen) {
				break
			}

			makeFrame(legacyOverlapSearch(fx, input, count, previous, predicted))
		}

		if position < latency {
			continue
		}

		read := float64(position-latency) * fx.pitchRatio

		index := int64(math.Floor(read))
		if index+2 >= finalized {
			panic("legacy overlap oracle has insufficient lookahead")
		}

		output[position] = interp.Hermite4(read-float64(index), stretched[max(index-1, 0)], stretched[index], stretched[index+1], stretched[index+2])
	}

	return output
}

func TestStreamingOverlapLegacyOutputAcrossPartitionsAndReset(t *testing.T) {
	for _, test := range []struct {
		rate, ratio float64
	}{{8000, .5}, {8000, .75}, {8000, 1.5}, {8000, 2}, {48000, 2}} {
		fx := overlapTestProcessor(t, test.rate, test.ratio)
		if test.rate == 48000 {
			var err error

			fx, err = NewPitchShifter(test.rate)
			if err != nil {
				t.Fatal(err)
			}

			if err := fx.SetPitchRatio(test.ratio); err != nil {
				t.Fatal(err)
			}
		}

		stream, err := NewStreamingPitchShifter(fx)
		if err != nil {
			t.Fatal(err)
		}

		// Multiple input/synthesis wraps and a late discontinuity exercise search
		// history, not merely startup frames. Drain with zeros after the source.
		source := make([]float64, 16000+stream.Latency()+fx.sequenceLen)
		for i := range 16000 {
			source[i] = overlapVariedSignal(i)
			if i > 12000 {
				source[i] *= -.7
			}
		}

		want := legacyOverlapOutput(fx, source, stream.Latency())
		for _, partition := range [][]int{{len(source)}, {128}, {1, 137, 31, 512, 3}} {
			// Reset during an active overlap, not only at an already-drained EOF.
			prefix := slices.Clone(source[:len(source)/3])
			if err := stream.ProcessInPlace(prefix); err != nil {
				t.Fatal(err)
			}

			stream.Reset()

			got := slices.Clone(source)

			for offset, block := 0, 0; offset < len(got); block++ {
				end := min(offset+partition[block%len(partition)], len(got))
				if err := stream.ProcessInPlace(got[offset:end]); err != nil {
					t.Fatal(err)
				}

				offset = end
			}

			for i := range got {
				if math.Float64bits(got[i]) != math.Float64bits(want[i]) {
					t.Fatalf("rate%g ratio%g partition%v frame%d got%.17g want%.17g", test.rate, test.ratio, partition, i, got[i], want[i])
				}
			}
		}
	}
}
