package resample

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
)

// Keep the previous allocating implementation as the numerical reference,
// using int64 absolute counters so it also models long streams on WASM.
type legacyStream struct {
	phases              [][]float64
	up, down            int
	phase               int
	inputIndex, totalIn int64
	history             []float64
	keep                int
}

func newLegacyStream(r *Resampler) *legacyStream {
	return &legacyStream{phases: r.phases, up: r.up, down: r.down, keep: r.maxPhaseLn - 1}
}

func (r *legacyStream) process(input []float64) []float64 {
	work := append(append([]float64(nil), r.history...), input...)
	base := r.totalIn - int64(len(r.history))
	last := r.totalIn + int64(len(input)) - 1

	var out []float64

	for r.inputIndex <= last {
		var value float64

		for index, coefficient := range r.phases[r.phase] {
			position := r.inputIndex - int64(index)
			if position < base || position > last {
				continue
			}

			value += coefficient * work[position-base]
		}

		out = append(out, value)
		r.phase += r.down
		r.inputIndex += int64(r.phase / r.up)
		r.phase %= r.up
	}

	r.totalIn += int64(len(input))
	keep := min(r.keep, len(work))
	r.history = append(r.history[:0], work[len(work)-keep:]...)

	return out
}

func mustResampler(t *testing.T, up, down int, options ...Option) *Resampler {
	t.Helper()

	r, err := NewRational(up, down, options...)
	if err != nil {
		t.Fatal(err)
	}

	return r
}

func checkOutputBits(t *testing.T, got, want []float64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("output length %d, want %d", len(got), len(want))
	}

	for index := range got {
		if math.Float64bits(got[index]) != math.Float64bits(want[index]) {
			t.Fatalf("sample%d bits=%016x want=%016x", index, math.Float64bits(got[index]), math.Float64bits(want[index]))
		}
	}
}

func TestProcessIntoLegacyAndChunkParity(t *testing.T) {
	ratios := [][2]int{{1, 1}, {2, 1}, {1, 2}, {3, 2}, {2, 3}, {160, 147}, {147, 160}, {1, 48}, {48, 1}, {5, 7}}
	for _, ratio := range ratios {
		for _, quality := range []Quality{QualityFast, QualityBalanced, QualityBest} {
			t.Run(fmt.Sprintf("%d_%d/quality%d", ratio[0], ratio[1], quality), func(t *testing.T) {
				runProcessIntoParity(t, ratio, quality)
			})
		}
	}
}

func runProcessIntoParity(t *testing.T, ratio [2]int, quality Quality) {
	t.Helper()

	input := sine(1200, 48000, 2049)
	r := mustResampler(t, ratio[0], ratio[1], WithQuality(quality))
	want := newLegacyStream(r).process(input)
	checkOutputBits(t, r.Process(input), want)

	for _, chunk := range []int{1, 7, 127, 1024} {
		r.Reset()

		var got []float64

		for offset := 0; offset < len(input); offset += chunk {
			block := input[offset:min(offset+chunk, len(input))]
			count := r.PredictOutputLen(len(block))
			dst := make([]float64, count+1)
			dst[count] = 123

			written, err := r.ProcessInto(dst, block)
			if err != nil || written != count || dst[count] != 123 {
				t.Fatalf("ProcessInto count=%d predicted=%d sentinel=%g err=%v", written, count, dst[count], err)
			}

			got = append(got, dst[:written]...)
		}

		checkOutputBits(t, got, want)
	}
}

func TestProcessIntoRejectsShortDstAtomically(t *testing.T) {
	r := mustResampler(t, 3, 2)
	r.Process([]float64{1, -1, 0.5})
	state := *r
	history := append([]float64(nil), r.history...)
	input := []float64{1, 2, 3, 4}
	dst := []float64{99}

	count, err := r.ProcessInto(dst, input)
	if count != 0 || !errors.Is(err, ErrShortDst) || dst[0] != 99 {
		t.Fatalf("short dst count=%d dst=%v err=%v", count, dst, err)
	}

	if !reflect.DeepEqual(*r, state) || !reflect.DeepEqual(r.history, history) {
		t.Fatal("short destination changed streaming state")
	}

	control := newLegacyStream(r)
	control.phase, control.totalIn, control.inputIndex = r.phase, 3, int64(3+r.inputIndex)
	control.history = append([]float64(nil), r.history...)
	checkOutputBits(t, r.Process(input), control.process(input))
}

func TestProcessIntoEmptyAndZeroOutput(t *testing.T) {
	r := mustResampler(t, 1, 48)
	if count, err := r.ProcessInto(nil, nil); count != 0 || err != nil || r.PredictOutputLen(-1) != 0 {
		t.Fatalf("empty input count=%d err=%v", count, err)
	}

	if _, err := r.ProcessInto(make([]float64, 1), []float64{1}); err != nil {
		t.Fatal(err)
	}

	for range 47 {
		if count, err := r.ProcessInto(nil, []float64{2}); count != 0 || err != nil {
			t.Fatalf("zero output count=%d err=%v", count, err)
		}
	}

	if r.PredictOutputLen(1) != 1 {
		t.Fatal("small downsampled blocks lost the next output")
	}
}

func TestProcessIntoBoundedAcrossInt32StreamBoundary(t *testing.T) {
	for _, ratio := range [][2]int{{160, 147}, {147, 160}, {1, 48}, {48, 1}} {
		r := mustResampler(t, ratio[0], ratio[1])
		r.Process(sine(1000, 48000, 97))
		legacy := newLegacyStream(r)
		legacy.phase = r.phase
		legacy.totalIn = int64(math.MaxInt32) - 17
		legacy.inputIndex = legacy.totalIn + int64(r.inputIndex)
		legacy.history = append([]float64(nil), r.history...)

		for blockIndex := range 64 {
			input := sine(float64(1000+blockIndex), 48000, 53)
			checkOutputBits(t, r.Process(input), legacy.process(input))

			if r.inputIndex < 0 || r.inputIndex > (r.down+r.up-1)/r.up || r.phase < 0 || r.phase >= r.up {
				t.Fatalf("unbounded state offset=%d phase=%d", r.inputIndex, r.phase)
			}
		}

		if legacy.totalIn <= int64(math.MaxInt32) {
			t.Fatal("regression did not cross int32 streaming boundary")
		}
	}
}

func TestPredictOutputLenPlatformLimit(t *testing.T) {
	maxLength := int(^uint(0) >> 1)
	r := mustResampler(t, 147, 160)

	expected := int((uint64(maxLength)/160)*147 + ((uint64(maxLength)%160)*147+159)/160)
	if count := r.PredictOutputLen(maxLength); count != expected {
		t.Fatalf("large prediction=%d want=%d", count, expected)
	}

	r = mustResampler(t, 2, 1)
	if count, fits := r.outputLen(maxLength); count != maxLength || fits {
		t.Fatalf("overflow prediction count=%d fits=%v", count, fits)
	}
}

func TestProcessIntoOutputLimitIsAtomic(t *testing.T) {
	r := mustResampler(t, 2, 1)
	// Exercise the guard without allocating an impossible input or filter.
	r.up = int(^uint(0) >> 1)
	state := *r
	dst := []float64{99}

	count, err := r.ProcessInto(dst, []float64{1, 2})
	if count != 0 || !errors.Is(err, ErrOutputTooLarge) || dst[0] != 99 || !reflect.DeepEqual(*r, state) {
		t.Fatalf("overflow changed output/state: count=%d dst=%v err=%v", count, dst, err)
	}
}

func TestProcessIntoVeryLargeDownFactor(t *testing.T) {
	maxLength := int(^uint(0) >> 1)

	r := mustResampler(t, 2, maxLength)
	if count, err := r.ProcessInto(make([]float64, 1), []float64{1, 2}); count != 1 || err != nil {
		t.Fatalf("large factor count=%d err=%v", count, err)
	}

	for range 3 {
		if count, err := r.ProcessInto(nil, []float64{1, 2}); count != 0 || err != nil {
			t.Fatalf("large-factor state overflow count=%d err=%v", count, err)
		}
	}
}

func TestResamplerCloneIsIndependentResetStream(t *testing.T) {
	r := mustResampler(t, 160, 147)
	input := sine(1000, 44100, 257)
	want := r.Process(input)

	clone := r.Clone()
	if &r.taps[0] != &clone.taps[0] || &r.phases[0][0] != &clone.phases[0][0] {
		t.Fatal("clone redesigned immutable coefficients")
	}

	checkOutputBits(t, clone.Process(input), want)

	if &r.history[0] == &clone.history[0] {
		t.Fatal("clone shares mutable history")
	}

	state := append([]float64(nil), r.history...)

	clone.Reset()
	clone.Process([]float64{99, 98, 97})

	if !reflect.DeepEqual(r.history, state) {
		t.Fatal("clone modified its source's history")
	}
}

func TestProcessIntoZeroAllocations(t *testing.T) {
	for _, ratio := range [][2]int{{160, 147}, {147, 160}, {1, 48}, {48, 1}} {
		r := mustResampler(t, ratio[0], ratio[1])
		input := sine(1000, 48000, 128)
		dst := make([]float64, r.PredictOutputLen(len(input)))

		allocs := testing.AllocsPerRun(100, func() {
			r.Reset()

			if _, err := r.ProcessInto(dst, input); err != nil {
				t.Fatal(err)
			}
		})
		if allocs != 0 {
			t.Fatalf("ratio %v allocs=%g", ratio, allocs)
		}
	}
}
