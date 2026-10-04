package signal_test

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/signal"
)

func TestMixFloat32NumericalParity(t *testing.T) {
	t.Parallel()

	values := []float32{
		0, math.Float32frombits(0x80000000), math.SmallestNonzeroFloat32, -math.SmallestNonzeroFloat32,
		math.Float32frombits(3), math.Float32frombits(0x007fffff), math.Float32frombits(0x00800000),
		1, -1, math.Nextafter32(1, 2), math.MaxFloat32, -math.MaxFloat32,
		float32(math.Inf(1)), float32(math.Inf(-1)), math.Float32frombits(0x7f812345),
	}

	a, b := make([]float32, 0, len(values)*len(values)), make([]float32, 0, len(values)*len(values))

	for _, left := range values {
		for _, right := range values {
			a, b = append(a, left), append(b, right)
		}
	}

	// Exercise unrelated exponent/sign combinations beyond the explicit edges.
	state := uint32(0x317cb809)
	for range 65536 {
		state ^= state << 13
		state ^= state >> 17
		state ^= state << 5
		left := math.Float32frombits(state)
		state ^= state << 13
		state ^= state >> 17
		state ^= state << 5

		a, b = append(a, left), append(b, math.Float32frombits(state))
	}

	for _, test := range []struct {
		name string
		op   func([]float32, []float32, []float32) error
		gain float64
	}{
		{"add", signal.AddInto32, 1}, {"average", signal.AverageInto32, 0.5},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for _, alias := range []string{"none", "left", "right", "both"} {
				t.Run(alias, func(t *testing.T) {
					t.Parallel()

					left, right, dst := slices.Clone(a), slices.Clone(b), make([]float32, len(a))

					switch alias {
					case "left":
						dst = left
					case "right":
						dst = right
					case "both":
						dst, right = left, left
					}

					want := make([]float32, len(a))
					for i := range want {
						want[i] = float32((float64(left[i]) + float64(right[i])) * test.gain)
					}

					if err := test.op(dst, left, right); err != nil {
						t.Fatal(err)
					}

					for i, sample := range dst {
						if math.IsNaN(float64(want[i])) && math.IsNaN(float64(sample)) {
							continue
						}

						if math.Float32bits(sample) != math.Float32bits(want[i]) {
							t.Fatalf("frame %d: got %08x want %08x", i, math.Float32bits(sample), math.Float32bits(want[i]))
						}
					}
				})
			}
		})
	}
}

func TestMixFloat32LengthValidation(t *testing.T) {
	t.Parallel()

	for _, op := range []func([]float32, []float32, []float32) error{signal.AddInto32, signal.AverageInto32} {
		for _, lengths := range [][3]int{{2, 1, 2}, {2, 2, 1}, {1, 2, 2}, {0, 1, 1}} {
			dst := []float32{99, 99}[:lengths[0]]
			if err := op(dst, make([]float32, lengths[1]), make([]float32, lengths[2])); err == nil {
				t.Fatal("mismatched lengths accepted")
			}

			for _, sample := range dst {
				if sample != 99 {
					t.Fatal("invalid arguments modified destination")
				}
			}
		}

		if err := op(nil, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMixFloat32Allocations(t *testing.T) {
	a, b, dst := make([]float32, 4096), make([]float32, 4096), make([]float32, 4096)

	for _, op := range []func([]float32, []float32, []float32) error{signal.AddInto32, signal.AverageInto32} {
		if allocations := testing.AllocsPerRun(10, func() {
			if err := op(dst, a, b); err != nil {
				t.Fatal(err)
			}
		}); allocations != 0 {
			t.Fatalf("allocations = %v", allocations)
		}
	}
}

func BenchmarkMixFloat32(b *testing.B) {
	for _, test := range []struct {
		name string
		op   func([]float32, []float32, []float32) error
	}{
		{"add", signal.AddInto32}, {"average", signal.AverageInto32},
	} {
		b.Run(test.name, func(b *testing.B) {
			a, right, dst := make([]float32, 4096), make([]float32, 4096), make([]float32, 4096)
			for i := range a {
				a[i], right[i] = 0.25, 0.5
			}

			b.ReportAllocs()
			b.SetBytes(int64(len(a) * 12))
			b.ResetTimer()

			for b.Loop() {
				if err := test.op(dst, a, right); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func ExampleAddInto32() {
	dst := make([]float32, 3)
	if err := signal.AddInto32(dst, []float32{0.75, -0.5, 2}, []float32{0.75, 0.25, -1}); err != nil {
		panic(err)
	}

	fmt.Println(dst)
	// Output: [1.5 -0.25 1]
}

func ExampleAverageInto32() {
	a := []float32{0.75, -0.5, math.MaxFloat32}
	if err := signal.AverageInto32(a, a, []float32{0.25, 0.5, math.MaxFloat32}); err != nil {
		panic(err)
	}

	fmt.Println(a[0], a[1], a[2] == math.MaxFloat32)
	// Output: 0.5 0 true
}
