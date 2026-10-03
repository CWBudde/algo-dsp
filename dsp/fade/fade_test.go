package fade_test

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/fade"
)

func TestFadeGolden(t *testing.T) {
	t.Parallel()

	tests := []struct {
		shape fade.Shape
		want  []float64
	}{
		{fade.Linear, []float64{0, .25, .5, .75, 1}},
		{fade.EqualPower, []float64{0, .3826834323650898, .7071067811865476, .9238795325112867, 1}},
		{fade.Logarithmic, []float64{0, .5118833609788744, .7403626894942439, .8893017025063104, 1}},
		{fade.SCurve, []float64{0, .15625, .5, .84375, 1}},
	}
	for _, tt := range tests {
		t.Run(string(tt.shape), func(t *testing.T) {
			t.Parallel()

			for _, rising := range []bool{true, false} {
				got := []float32{1, 1, 1, 1, 1}
				if err := fade.ApplyInto32(got, got, 0, 5, tt.shape, rising); err != nil {
					t.Fatal(err)
				}

				for i, sample := range got {
					index := i
					if !rising {
						index = len(got) - 1 - i
					}

					if math.Abs(float64(sample)-tt.want[index]) > 5e-8 {
						t.Fatalf("rising=%v sample=%d got=%v want=%v", rising, i, sample, tt.want[index])
					}
				}
			}
		})
	}
}

func TestCrossfadeGoldenAndPower(t *testing.T) {
	t.Parallel()

	left := []float32{2, 2, 2, 2, 2}
	right := []float32{6, 6, 6, 6, 6}

	got := make([]float32, 5)
	if err := fade.CrossfadeInto32(got, left, right, 0, 5, fade.Linear); err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(got, []float32{2, 3, 4, 5, 6}) {
		t.Fatal(got)
	}

	for _, shape := range []fade.Shape{fade.Linear, fade.EqualPower, fade.Logarithmic, fade.SCurve} {
		if err := fade.CrossfadeInto32(got, left, right, 0, 5, shape); err != nil {
			t.Fatal(err)
		}

		if got[0] != 2 || got[4] != 6 {
			t.Fatalf("%s endpoints: %v", shape, got)
		}
	}

	if err := fade.CrossfadeInto32(left, left, right, 0, 5, fade.Linear); err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(left, []float32{2, 3, 4, 5, 6}) {
		t.Fatal("alias", left)
	}

	in, out, ones := make([]float32, 97), make([]float32, 97), make([]float32, 97)
	for i := range ones {
		ones[i] = 1
	}

	if err := fade.ApplyInto32(in, ones, 0, 97, fade.EqualPower, true); err != nil {
		t.Fatal(err)
	}

	if err := fade.ApplyInto32(out, ones, 0, 97, fade.EqualPower, false); err != nil {
		t.Fatal(err)
	}

	for i := range in {
		power := float64(in[i])*float64(in[i]) + float64(out[i])*float64(out[i])
		if math.Abs(power-1) > 1e-7 {
			t.Fatalf("power sample %d: %v", i, power)
		}
	}
}

func TestFadePartitionsAndBoundaries(t *testing.T) {
	t.Parallel()

	for _, shape := range []fade.Shape{fade.Linear, fade.EqualPower, fade.Logarithmic, fade.SCurve} {
		left, right := make([]float32, 103), make([]float32, 103)
		for i := range left {
			left[i], right[i] = float32(i)-50, float32(i)*.02
		}

		for _, cross := range []bool{false, true} {
			whole, split := make([]float32, 103), make([]float32, 103)

			process := func(dst, a, b []float32, start int64) error {
				if cross {
					return fade.CrossfadeInto32(dst, a, b, start, 103, shape)
				}

				return fade.ApplyInto32(dst, a, start, 103, shape, true)
			}
			if err := process(whole, left, right, 0); err != nil {
				t.Fatal(err)
			}

			for start := 0; start < 103; {
				end := min(103, start+1+start%13)
				if err := process(split[start:end], left[start:end], right[start:end], int64(start)); err != nil {
					t.Fatal(err)
				}

				start = end
			}

			if !slices.Equal(whole, split) {
				t.Fatalf("partition mismatch %s cross=%v", shape, cross)
			}
		}
	}

	for _, rising := range []bool{true, false} {
		one := []float32{1}
		if err := fade.ApplyInto32(one, one, 0, 1, fade.Linear, rising); err != nil || one[0] != 0 {
			t.Fatalf("single frame: %v %v", one, err)
		}
	}

	if err := fade.ApplyInto32(nil, nil, 5, 5, fade.Linear, true); err != nil {
		t.Fatal(err)
	}
}

func TestFadeValidationAtomic(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		start, total int64
		shape        fade.Shape
	}{
		{-1, 5, fade.Linear},
		{0, 0, fade.Linear},
		{5, 5, fade.Linear},
		{math.MaxInt64, 5, fade.Linear},
		{0, 5, "unknown"},
	} {
		dst := []float32{9, 9}
		if err := fade.ApplyInto32(dst, []float32{1, 1}, tc.start, tc.total, tc.shape, true); err == nil {
			t.Fatalf("accepted %+v", tc)
		}

		if !slices.Equal(dst, []float32{9, 9}) {
			t.Fatal("modified invalid output")
		}

		if err := fade.CrossfadeInto32(dst, dst, dst, tc.start, tc.total, tc.shape); err == nil {
			t.Fatalf("crossfade accepted %+v", tc)
		}
	}

	dst := []float32{9}
	if err := fade.ApplyInto32(dst, nil, 0, 5, fade.Linear, true); err == nil {
		t.Fatal("accepted mismatched lengths")
	}

	if err := fade.CrossfadeInto32(dst, nil, dst, 0, 5, fade.Linear); err == nil {
		t.Fatal("accepted mismatched crossfade lengths")
	}

	if err := fade.CrossfadeInto32(dst, dst, dst, 0, 1, fade.Linear); err == nil {
		t.Fatal("accepted single-frame crossfade")
	}

	if dst[0] != 9 {
		t.Fatal("modified invalid output")
	}
}

func BenchmarkFadeInto32(b *testing.B) {
	for _, shape := range []fade.Shape{fade.Linear, fade.EqualPower, fade.Logarithmic, fade.SCurve} {
		b.Run(string(shape), func(b *testing.B) {
			dst, src := make([]float32, 4096), make([]float32, 4096)

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				if err := fade.ApplyInto32(dst, src, 8192, 480000, shape, true); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestFadeAllocations(t *testing.T) {
	input, output := make([]float32, 4096), make([]float32, 4096)

	for _, shape := range []fade.Shape{fade.Linear, fade.EqualPower, fade.Logarithmic, fade.SCurve} {
		allocations := testing.AllocsPerRun(10, func() {
			if err := fade.ApplyInto32(output, input, 8192, 480000, shape, true); err != nil {
				t.Fatal(err)
			}

			if err := fade.CrossfadeInto32(output, input, input, 8192, 480000, shape); err != nil {
				t.Fatal(err)
			}
		})
		if allocations != 0 {
			t.Fatalf("%s allocates %v", shape, allocations)
		}
	}
}

func ExampleApplyInto32() {
	out := make([]float32, 3)
	if err := fade.ApplyInto32(out, []float32{1, 1, 1}, 0, 3, fade.Linear, true); err != nil {
		panic(err)
	}

	fmt.Println(out)
	// Output: [0 0.5 1]
}

func ExampleCrossfadeInto32() {
	out := make([]float32, 3)
	if err := fade.CrossfadeInto32(out, []float32{1, 1, 1}, []float32{3, 3, 3}, 0, 3, fade.Linear); err != nil {
		panic(err)
	}

	fmt.Println(out)
	// Output: [1 2 3]
}
