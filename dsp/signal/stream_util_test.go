package signal_test

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/signal"
)

func TestMeanAccumulator(t *testing.T) {
	t.Parallel()

	for _, samples := range [][]float32{{1, 2, 3, 4}, {math.MaxFloat32, 1, -math.MaxFloat32}, {-7, -7, -7}} {
		var full, split signal.MeanAccumulator
		if err := full.AddFloat32(samples); err != nil {
			t.Fatal(err)
		}

		for _, sample := range samples {
			if err := split.AddFloat32([]float32{sample}); err != nil {
				t.Fatal(err)
			}
		}

		mean, err := full.Mean()
		if err != nil {
			t.Fatal(err)
		}

		other, err := split.Mean()
		if err != nil || mean != other || full.Count() != int64(len(samples)) {
			t.Fatalf("partition mismatch %v %v %v", mean, other, err)
		}

		var want float64

		switch len(samples) {
		case 4:
			want = 2.5
		case 3:
			want = -7
			if samples[0] == math.MaxFloat32 {
				want = 1.0 / 3
			}
		}

		if mean != want {
			t.Fatalf("mean golden got=%v want=%v", mean, want)
		}
	}

	var mean signal.MeanAccumulator
	if _, err := mean.Mean(); err == nil {
		t.Fatal("accepted empty mean")
	}

	if err := mean.AddFloat32(nil); err != nil || mean.Count() != 0 {
		t.Fatal("empty add", err)
	}

	if err := mean.AddFloat32([]float32{1, 3}); err != nil {
		t.Fatal(err)
	}

	for _, invalid := range []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))} {
		if err := mean.AddFloat32([]float32{9, invalid}); err == nil {
			t.Fatal("accepted nonfinite input")
		}

		got, err := mean.Mean()
		if err != nil || got != 2 || mean.Count() != 2 {
			t.Fatal("failed add changed accumulator")
		}
	}
}

func TestFloat32Utilities(t *testing.T) {
	t.Parallel()

	src := []float32{1, 2, 3, 4}

	dst := make([]float32, len(src))
	if err := signal.SubtractMeanInto32(dst, src, 2.5); err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(dst, []float32{-1.5, -.5, .5, 1.5}) {
		t.Fatal(dst)
	}

	if err := signal.SubtractMeanInto32(dst, dst, .5); err != nil || !slices.Equal(dst, []float32{-2, -1, 0, 1}) {
		t.Fatal("alias mean", dst, err)
	}

	for _, gain := range []float64{-1, 0, 1, 1.00000005, .12933483434} {
		input := []float32{math.SmallestNonzeroFloat32, -.23456789, 13.7578125, 0}
		if err := signal.ScaleInto32(input, input, gain); err != nil {
			t.Fatal(err)
		}

		for i, sample := range []float32{math.SmallestNonzeroFloat32, -.23456789, 13.7578125, 0} {
			if math.Float32bits(input[i]) != math.Float32bits(float32(float64(sample)*gain)) {
				t.Fatalf("scale gain=%v sample=%v got=%v", gain, sample, input[i])
			}
		}
	}

	negativeZero := math.Float32frombits(1 << 31)
	if err := signal.ScaleInto32(dst[:1], []float32{negativeZero}, 1); err != nil || math.Float32bits(dst[0]) != 1<<31 {
		t.Fatal("identity did not preserve signed zero")
	}

	for _, operation := range []func([]float32, []float32, float64) error{signal.SubtractMeanInto32, signal.ScaleInto32} {
		for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
			dst := []float32{99}
			if err := operation(dst, []float32{1}, value); err == nil || dst[0] != 99 {
				t.Fatal("nonfinite parameter changed output")
			}
		}

		if err := operation(dst, src[:1], 1); err == nil {
			t.Fatal("mismatched length accepted")
		}

		if err := operation(nil, nil, 1); err != nil {
			t.Fatal("empty input rejected")
		}
	}
}

func TestFloat32UtilityAllocations(t *testing.T) {
	input, output := make([]float32, 4096), make([]float32, 4096)

	var accumulator signal.MeanAccumulator

	allocations := testing.AllocsPerRun(10, func() {
		accumulator = signal.MeanAccumulator{}
		if err := accumulator.AddFloat32(input); err != nil {
			t.Fatal(err)
		}

		if err := signal.SubtractMeanInto32(output, input, .5); err != nil {
			t.Fatal(err)
		}

		if err := signal.ScaleInto32(output, input, .5); err != nil {
			t.Fatal(err)
		}
	})
	if allocations != 0 {
		t.Fatalf("allocations=%v", allocations)
	}
}

func BenchmarkMeanFloat32(b *testing.B) {
	input := make([]float32, 4096)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		var accumulator signal.MeanAccumulator
		if err := accumulator.AddFloat32(input); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkScaleInto32(b *testing.B) {
	input, output := make([]float32, 4096), make([]float32, 4096)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if err := signal.ScaleInto32(output, input, .12345678); err != nil {
			b.Fatal(err)
		}
	}
}

func ExampleMeanAccumulator() {
	var mean signal.MeanAccumulator
	if err := mean.AddFloat32([]float32{1, 2}); err != nil {
		panic(err)
	}

	if err := mean.AddFloat32([]float32{3, 4}); err != nil {
		panic(err)
	}

	value, err := mean.Mean()
	if err != nil {
		panic(err)
	}

	fmt.Println(value, mean.Count())
	// Output: 2.5 4
}

func ExampleSubtractMeanInto32() {
	out := make([]float32, 3)
	if err := signal.SubtractMeanInto32(out, []float32{1, 2, 3}, 2); err != nil {
		panic(err)
	}

	fmt.Println(out)
	// Output: [-1 0 1]
}

func ExampleScaleInto32() {
	out := make([]float32, 3)
	if err := signal.ScaleInto32(out, []float32{1, -2, 3}, -.5); err != nil {
		panic(err)
	}

	fmt.Println(out)
	// Output: [-0.5 1 -1.5]
}
