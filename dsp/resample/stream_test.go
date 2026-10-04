package resample

import (
	"errors"
	"math"
	"math/big"
	"slices"
	"testing"
)

func TestStreamReferenceAndChunks(t *testing.T) {
	for _, rates := range [][2]int{{44100, 48000}, {48000, 44100}, {8000, 384000}, {384000, 8000}, {44100, 44100}} {
		for _, length := range []int{0, 1, 17, 1037} {
			input := make([]float64, length)
			for i := range input {
				input[i] = math.Sin(float64(i)*.17) + .2*math.Cos(float64(i)*.49)
			}

			plan, err := NewStreamPlan(rates[0], rates[1], 64, QualityBalanced)
			if err != nil {
				t.Fatal(err)
			}

			want := streamReference(t, plan, input)
			for _, block := range []int{1, 7, 64} {
				stream, err := plan.NewStream(int64(length))
				if err != nil {
					t.Fatal(err)
				}

				got := collectStream(t, stream, input, block)
				if !slices.Equal(got, want) {
					t.Fatalf("rates%v length%d block%d: differs from raw trimmed reference", rates, length, block)
				}

				if stream.InputFrames() != int64(length) || stream.OutputFrames() != int64(len(want)) || !stream.Done() {
					t.Fatal("incorrect progress")
				}

				if n, done, err := stream.FlushInto(nil); err != nil || !done || n != 0 {
					t.Fatal("completed flush")
				}

				stream.Reset()

				if again := collectStream(t, stream, input, block); !slices.Equal(again, want) {
					t.Fatal("reset changed samples")
				}

				if clone := collectStream(t, stream.Clone(), input, block); !slices.Equal(clone, want) {
					t.Fatal("clone did not start fresh")
				}
			}
		}
	}
}

func TestStreamQualityReference(t *testing.T) {
	for _, quality := range []Quality{QualityFast, QualityBest} {
		for _, rates := range [][2]int{{44100, 48000}, {48000, 44100}} {
			input := make([]float64, 521)
			for i := range input {
				input[i] = math.Sin(float64(i) * .21)
			}

			plan, err := NewStreamPlan(rates[0], rates[1], 64, quality)
			if err != nil {
				t.Fatal(err)
			}

			want := streamReference(t, plan, input)
			for _, block := range []int{1, 7, 64} {
				stream, err := plan.NewStream(int64(len(input)))
				if err != nil {
					t.Fatal(err)
				}

				if got := collectStream(t, stream, input, block); !slices.Equal(got, want) {
					t.Fatalf("quality%d rates%v block%d", quality, rates, block)
				}
			}
		}
	}
}

func streamReference(t *testing.T, plan StreamPlan, input []float64) []float64 {
	t.Helper()

	if plan.up == plan.down {
		return slices.Clone(input)
	}

	count, err := FrameCount(int64(len(input)), plan.inRate, plan.outRate)
	if err != nil {
		t.Fatal(err)
	}

	if count == 0 {
		return nil
	}

	r, err := NewRational(plan.up, plan.down, WithQuality(plan.quality), WithTapsPerPhase(plan.taps))
	if err != nil {
		t.Fatal(err)
	}

	skip := int(math.Ceil(r.GroupDelayOutput()))
	zeros := (skip*plan.down+plan.up-1)/plan.up + 1
	raw := r.Process(append(slices.Clone(input), make([]float64, zeros)...))

	return raw[skip : skip+int(count)]
}

func collectStream(t *testing.T, s *Stream, input []float64, block int) []float64 {
	t.Helper()

	out := make([]float64, s.plan.outputBlock)
	got := make([]float64, 0)

	for len(input) > 0 {
		n := min(len(input), block)
		prediction := s.PredictOutputLen(n)

		written, err := s.ProcessInto(out[:prediction], input[:n])
		if err != nil || written != prediction {
			t.Fatalf("process%d prediction%d: %d %v", n, prediction, written, err)
		}

		got = append(got, out[:written]...)
		input = input[n:]
	}

	for steps := 0; !s.Done(); steps++ {
		if steps > 10000 {
			t.Fatal("unbounded flush")
		}

		n, _, err := s.FlushInto(out)
		if err != nil {
			t.Fatal(err)
		}

		got = append(got, out[:n]...)
	}

	return got
}

func TestStreamErrorsAtomic(t *testing.T) {
	plan, _ := NewStreamPlan(44100, 48000, 64, QualityBalanced)
	stream, _ := plan.NewStream(128)
	input := make([]float64, 64)

	out := make([]float64, plan.OutputBlockFrames())
	if _, _, err := stream.FlushInto(out); !errors.Is(err, ErrInvalidStream) {
		t.Fatal("premature flush")
	}

	if _, err := stream.ProcessInto(out, make([]float64, 65)); !errors.Is(err, ErrInvalidStream) {
		t.Fatal("oversized input")
	}

	prediction := stream.PredictOutputLen(len(input))

	short := make([]float64, prediction-1)
	for i := range short {
		short[i] = .7
	}

	if _, err := stream.ProcessInto(short, input); !errors.Is(err, ErrShortDst) || stream.InputFrames() != 0 {
		t.Fatal("short destination changed state")
	}

	for _, v := range short {
		if v != .7 {
			t.Fatal("short destination modified")
		}
	}

	if _, err := stream.ProcessInto(out, input); err != nil {
		t.Fatal(err)
	}

	if _, err := stream.ProcessInto(out, input); err != nil {
		t.Fatal(err)
	}

	if _, err := stream.ProcessInto(out, input[:1]); !errors.Is(err, ErrInvalidStream) {
		t.Fatal("source overrun")
	}

	written := stream.OutputFrames()
	if _, _, err := stream.FlushInto(nil); !errors.Is(err, ErrShortDst) || stream.OutputFrames() != written {
		t.Fatal("short flush changed state")
	}

	if _, err := stream.ProcessInto(nil, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := plan.NewStream(-2); !errors.Is(err, ErrInvalidStream) {
		t.Fatal("invalid source length")
	}

	if _, err := (StreamPlan{}).NewStream(1); !errors.Is(err, ErrInvalidStream) {
		t.Fatal("zero plan")
	}
}

func TestStreamIndefinite(t *testing.T) {
	plan, _ := NewStreamPlan(48000, 44100, 64, QualityFast)
	stream, _ := plan.NewStream(-1)
	raw, _ := NewRational(plan.up, plan.down, WithQuality(plan.quality), WithTapsPerPhase(plan.taps))
	out := make([]float64, plan.OutputBlockFrames())
	expected := make([]float64, plan.OutputBlockFrames())
	input := make([]float64, 64)
	skip := int(math.Ceil(raw.GroupDelayOutput()))

	for step := 0; step < 100; step++ {
		for i := range input {
			input[i] = math.Sin(float64(step*64+i) * .14)
		}

		n, err := stream.ProcessInto(out, input)
		if err != nil {
			t.Fatal(err)
		}

		rawN, err := raw.ProcessInto(expected, input)
		if err != nil {
			t.Fatal(err)
		}

		start := min(skip, rawN)

		skip -= start
		if !slices.Equal(out[:n], expected[start:rawN]) {
			t.Fatal("indefinite continuity mismatch")
		}
	}

	if stream.Done() {
		t.Fatal("indefinite finished")
	}

	if _, _, err := stream.FlushInto(out); !errors.Is(err, ErrInvalidStream) {
		t.Fatal("indefinite flush")
	}
}

func TestStreamPlanValidationAndWorkspace(t *testing.T) {
	for _, test := range []struct {
		in, out, block int
		q              Quality
		want           error
	}{
		{0, 48000, 64, QualityFast, ErrInvalidRate},
		{48000, -1, 64, QualityFast, ErrInvalidRate},
		{48000, 44100, 0, QualityFast, ErrInvalidStream},
		{48000, 44100, 64, Quality(4), ErrInvalidStream},
		{1, math.MaxInt, 64, QualityBest, ErrOutputTooLarge},
		{math.MaxInt, 1, 64, QualityBest, ErrOutputTooLarge},
		{48000, 44100, math.MaxInt, QualityFast, ErrOutputTooLarge},
	} {
		if _, err := NewStreamPlan(test.in, test.out, test.block, test.q); !errors.Is(err, test.want) {
			t.Fatalf("%+v: %v", test, err)
		}
	}

	plan, err := NewStreamPlan(44100, 48000, 65536, QualityBest)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := plan.WorkspaceBytes(0); !errors.Is(err, ErrInvalidStream) {
		t.Fatal("invalid channels")
	}

	if _, err := plan.WorkspaceBytes(math.MaxInt); !errors.Is(err, ErrOutputTooLarge) {
		t.Fatal("workspace overflow")
	}

	bytes, err := plan.WorkspaceBytes(2)

	expected := int64(plan.taps*plan.up)*16 + int64(plan.up)*32 + 2*int64(plan.taps+plan.inputBlock+plan.outputBlock)*8 + 2*512
	if err != nil || bytes != expected {
		t.Fatalf("workspace %d want %d: %v", bytes, expected, err)
	}

	input, err := plan.InputFramesForOutputLimit(65536)
	if err != nil {
		t.Fatal(err)
	}

	smaller, err := NewStreamPlan(44100, 48000, input, QualityBest)
	if err != nil || smaller.OutputBlockFrames() > 65536 {
		t.Fatal("bounded input inverse")
	}

	if _, err := plan.InputFramesForOutputLimit(1); !errors.Is(err, ErrShortDst) {
		t.Fatal("impossible output limit")
	}

	if _, err := plan.InputFramesForOutputLimit(0); !errors.Is(err, ErrInvalidStream) {
		t.Fatal("invalid output limit")
	}
	// These huge plans must fail without allocating any coefficients.
	if _, err := plan.NewStream(math.MaxInt64); !errors.Is(err, ErrOutputTooLarge) {
		t.Fatal("duration overflow")
	}

	if _, err := NewStreamPlan(1, 2, math.MaxInt/4, QualityFast); !errors.Is(err, ErrOutputTooLarge) {
		t.Fatal("scratch bytes overflow")
	}
}

func TestFrameConversionAgainstBigInteger(t *testing.T) {
	for _, frames := range []int64{0, 1, 17, 1 << 53, math.MaxInt64 - 1, math.MaxInt64} {
		for _, rates := range [][2]int{{1, 1}, {44100, 48000}, {48000, 44100}, {math.MaxInt, 1}, {1, math.MaxInt}, {math.MaxInt, math.MaxInt - 1}, {2, 3}} {
			for _, ceil := range []bool{true, false} {
				numerator := new(big.Int).Mul(big.NewInt(frames), big.NewInt(int64(rates[1])))

				denominator := big.NewInt(int64(rates[0]))
				if ceil {
					numerator.Add(numerator, new(big.Int).Sub(denominator, big.NewInt(1)))
				} else {
					numerator.Add(numerator, new(big.Int).Rsh(new(big.Int).Set(denominator), 1))
				}

				want := new(big.Int).Quo(numerator, denominator)

				var (
					got int64
					err error
				)
				if ceil {
					got, err = FrameCount(frames, rates[0], rates[1])
				} else {
					got, err = FramePosition(frames, rates[0], rates[1])
				}

				if !want.IsInt64() {
					if !errors.Is(err, ErrOutputTooLarge) {
						t.Fatalf("overflow %d %v ceil%v: %d %v", frames, rates, ceil, got, err)
					}
				} else if err != nil || got != want.Int64() {
					t.Fatalf("frame%d rates%v ceil%v: %d want%s %v", frames, rates, ceil, got, want, err)
				}
			}
		}
	}

	if _, err := FrameCount(-1, 1, 1); !errors.Is(err, ErrInvalidStream) {
		t.Fatal("negative frame")
	}

	if _, err := FramePosition(1, 0, 1); !errors.Is(err, ErrInvalidRate) {
		t.Fatal("invalid rate")
	}
}

func TestStreamZeroAllocations(t *testing.T) {
	plan, _ := NewStreamPlan(44100, 48000, 64, QualityBalanced)
	stream, _ := plan.NewStream(64)
	input := make([]float64, 64)
	output := make([]float64, plan.OutputBlockFrames())

	allocations := testing.AllocsPerRun(100, func() {
		stream.Reset()

		if _, err := stream.ProcessInto(output, input); err != nil {
			panic(err)
		}

		for !stream.Done() {
			if _, _, err := stream.FlushInto(output); err != nil {
				panic(err)
			}
		}
	})
	if allocations != 0 {
		t.Fatalf("%g allocations", allocations)
	}

	if allocations := testing.AllocsPerRun(100, func() {
		if _, err := plan.WorkspaceBytes(2); err != nil {
			panic(err)
		}
	}); allocations != 0 {
		t.Fatalf("preflight allocations%g", allocations)
	}
}

func BenchmarkStreamProcessInto(b *testing.B) {
	plan, _ := NewStreamPlan(44100, 48000, 1024, QualityBalanced)
	stream, _ := plan.NewStream(-1)
	input := make([]float64, plan.InputBlockFrames())
	output := make([]float64, plan.OutputBlockFrames())

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, err := stream.ProcessInto(output, input); err != nil {
			b.Fatal(err)
		}
	}
}

func TestStreamShortFiniteFlushWithLargePlan(t *testing.T) {
	for _, rates := range [][2]int{{44100, 48000}, {48000, 44100}, {8000, 384000}, {384000, 8000}} {
		plan, err := NewStreamPlan(rates[0], rates[1], 65536, QualityBalanced)
		if err != nil {
			t.Fatal(err)
		}

		for _, length := range []int{1, 7, 17} {
			input := make([]float64, length)
			input[length-1] = 1

			stream, err := plan.NewStream(int64(length))
			if err != nil {
				t.Fatal(err)
			}

			out := make([]float64, plan.OutputBlockFrames())

			n, err := stream.ProcessInto(out, input)
			if err != nil {
				t.Fatal(err)
			}

			got := slices.Clone(out[:n])

			if !stream.Done() {
				count := stream.flushInputFrames()
				if count >= 2048 {
					t.Fatalf("rates%v length%d: wasteful flush %d", rates, length, count)
				}

				remaining := stream.targetFrames - stream.written
				if int64(stream.PredictOutputLen(count)) != remaining {
					t.Fatal("tail count incomplete")
				}

				if count > 1 && int64(stream.PredictOutputLen(count-1)) >= remaining {
					t.Fatal("tail count not minimal")
				}

				n, done, err := stream.FlushInto(out)
				if err != nil || !done {
					t.Fatalf("tail %v %v", done, err)
				}

				got = append(got, out[:n]...)
			}

			if want := streamReference(t, plan, input); !slices.Equal(got, want) {
				t.Fatal("short finite reference changed")
			}
		}
	}
}

func BenchmarkStreamShortFinite(b *testing.B) {
	plan, _ := NewStreamPlan(44100, 48000, 65536, QualityBalanced)
	stream, _ := plan.NewStream(1)
	input := []float64{1}
	output := make([]float64, plan.OutputBlockFrames())

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		stream.Reset()

		if _, err := stream.ProcessInto(output, input); err != nil {
			b.Fatal(err)
		}

		for !stream.Done() {
			if _, _, err := stream.FlushInto(output); err != nil {
				b.Fatal(err)
			}
		}
	}
}
