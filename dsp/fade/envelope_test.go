package fade_test

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/fade"
)

func TestSharedEnvelopeExactParity(t *testing.T) {
	t.Parallel()

	for _, shape := range []fade.Shape{fade.Linear, fade.EqualPower, fade.Logarithmic, fade.SCurve} {
		for _, position := range [][2]int64{{0, 1}, {0, 257}, {1234567, 28800000}, {math.MaxInt64 - 1000, math.MaxInt64}} {
			start, total := position[0], position[1]
			length := int(min(257, total-start))
			env, falling := make([]float64, length), make([]float64, length)

			src, right, got, want := make([]float32, length), make([]float32, length), make([]float32, length), make([]float32, length)
			for i := range src {
				src[i], right[i] = float32(math.Sin(float64(i))*.7), float32(math.Cos(float64(i))*.3)
			}

			for _, rising := range []bool{true, false} {
				if err := fade.EnvelopeInto64(env, start, total, shape, rising); err != nil {
					t.Fatal(err)
				}

				if err := fade.ApplyEnvelopeInto32(got, src, env); err != nil {
					t.Fatal(err)
				}

				if err := fade.ApplyInto32(want, src, start, total, shape, rising); err != nil {
					t.Fatal(err)
				}

				if !slices.Equal(got, want) {
					t.Fatalf("%s direction%v position%v", shape, rising, position)
				}

				for i, gain := range env {
					x := float64(start+int64(i)) / float64(max(total-1, 1))
					if !rising {
						x = 1 - x
					}

					if total == 1 {
						x = 0
					}

					if math.Float64bits(gain) != math.Float64bits(referenceEnvelope(x, shape)) {
						t.Fatalf("float64 envelope %s index%d got%v", shape, i, gain)
					}
				}
			}

			if total == 1 {
				continue
			}

			if err := fade.EnvelopeInto64(env, start, total, shape, true); err != nil {
				t.Fatal(err)
			}

			if err := fade.EnvelopeInto64(falling, start, total, shape, false); err != nil {
				t.Fatal(err)
			}

			if err := fade.CrossfadeEnvelopeInto32(got, src, right, falling, env); err != nil {
				t.Fatal(err)
			}

			if err := fade.CrossfadeInto32(want, src, right, start, total, shape); err != nil {
				t.Fatal(err)
			}

			if !slices.Equal(got, want) {
				t.Fatalf("crossfade %s position%v", shape, position)
			}
		}
	}
}

func TestSharedEnvelopeValidationAliasingAndAllocations(t *testing.T) {
	out := []float64{99}
	if err := fade.EnvelopeInto64(out, -1, 3, fade.Linear, true); err == nil || out[0] != 99 {
		t.Fatal("invalid envelope changed output")
	}

	if err := fade.EnvelopeInto64(nil, 3, 3, fade.Linear, true); err != nil {
		t.Fatal(err)
	}

	dst := []float32{99}
	for _, operation := range []func() error{
		func() error { return fade.ApplyEnvelopeInto32(dst, nil, out) },
		func() error { return fade.ApplyEnvelopeInto32(dst, dst, nil) },
		func() error { return fade.CrossfadeEnvelopeInto32(dst, dst, nil, out, out) },
		func() error { return fade.CrossfadeEnvelopeInto32(dst, dst, dst, nil, out) },
	} {
		if err := operation(); err == nil || dst[0] != 99 {
			t.Fatal("invalid shape changed output")
		}
	}

	if err := fade.ApplyEnvelopeInto32(nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	if err := fade.CrossfadeEnvelopeInto32(nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	env, falling := make([]float64, 4096), make([]float64, 4096)
	left, right := make([]float32, 4096), make([]float32, 4096)

	allocations := testing.AllocsPerRun(10, func() {
		if err := fade.EnvelopeInto64(env, 100, 28800000, fade.Logarithmic, true); err != nil {
			t.Fatal(err)
		}

		if err := fade.EnvelopeInto64(falling, 100, 28800000, fade.Logarithmic, false); err != nil {
			t.Fatal(err)
		}

		if err := fade.ApplyEnvelopeInto32(left, left, env); err != nil {
			t.Fatal(err)
		}

		if err := fade.CrossfadeEnvelopeInto32(right, left, right, falling, env); err != nil {
			t.Fatal(err)
		}
	})
	if allocations != 0 {
		t.Fatalf("allocations=%v", allocations)
	}

	dst = []float32{2, 4, 8}
	if err := fade.ApplyEnvelopeInto32(dst, dst, []float64{0, .5, 1}); err != nil || !slices.Equal(dst, []float32{0, 2, 8}) {
		t.Fatal("alias", dst, err)
	}

	if err := fade.CrossfadeEnvelopeInto32(dst, dst, []float32{6, 6, 6}, []float64{1, .5, 0}, []float64{0, .5, 1}); err != nil || !slices.Equal(dst, []float32{0, 4, 6}) {
		t.Fatal("crossfade alias", dst, err)
	}
}

func BenchmarkSharedEnvelopeStereo(b *testing.B) {
	for _, shape := range []fade.Shape{fade.Linear, fade.EqualPower, fade.Logarithmic, fade.SCurve} {
		b.Run(string(shape), func(b *testing.B) {
			env := make([]float64, 4096)
			left, right := make([]float32, 4096), make([]float32, 4096)

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				if err := fade.EnvelopeInto64(env, 8192, 480000, shape, true); err != nil {
					b.Fatal(err)
				}

				if err := fade.ApplyEnvelopeInto32(left, left, env); err != nil {
					b.Fatal(err)
				}

				if err := fade.ApplyEnvelopeInto32(right, right, env); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func ExampleEnvelopeInto64() {
	envelope := make([]float64, 3)
	if err := fade.EnvelopeInto64(envelope, 0, 3, fade.Linear, true); err != nil {
		panic(err)
	}

	fmt.Println(envelope)
	// Output: [0 0.5 1]
}

func ExampleApplyEnvelopeInto32() {
	out := make([]float32, 3)
	if err := fade.ApplyEnvelopeInto32(out, []float32{2, 4, 8}, []float64{0, .5, 1}); err != nil {
		panic(err)
	}

	fmt.Println(out)
	// Output: [0 2 8]
}

func ExampleCrossfadeEnvelopeInto32() {
	out := make([]float32, 3)
	if err := fade.CrossfadeEnvelopeInto32(out, []float32{2, 2, 2}, []float32{6, 6, 6}, []float64{1, .5, 0}, []float64{0, .5, 1}); err != nil {
		panic(err)
	}

	fmt.Println(out)
	// Output: [2 4 6]
}
