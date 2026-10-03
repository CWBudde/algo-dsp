package time

import (
	"math"
	"testing"
)

func TestNearestZeroCrossing(t *testing.T) {
	maxRadius := int(^uint(0) >> 1)

	tests := []struct {
		name                 string
		signal               []float64
		target, radius, want int
		found                bool
	}{
		{"empty", nil, 0, 0, 0, false},
		{"negative_target", []float64{0}, -1, 1, 0, false},
		{"past_eof", []float64{0}, 2, 1, 0, false},
		{"negative_radius", []float64{0}, 0, -1, 0, false},
		{"all_zero", []float64{0, 0, 0}, 1, maxRadius, 1, true},
		{"signed_zero", []float64{1, math.Copysign(0, -1), 1}, 1, 0, 1, true},
		{"zero_first", []float64{0, 1}, 0, 0, 0, true},
		{"single_nonzero", []float64{-1}, 0, maxRadius, 0, false},
		{"strict_opposite", []float64{-1, 1}, 1, 0, 1, true},
		{"opposite_reverse", []float64{1, -1}, 1, 0, 1, true},
		{"subnormal_opposite", []float64{-math.SmallestNonzeroFloat32, math.SmallestNonzeroFloat32}, 1, 0, 1, true},
		{"extreme_opposite", []float64{-math.MaxFloat32, math.MaxFloat32}, 1, 0, 1, true},
		{"zero_to_nonzero", []float64{0, -1}, 1, 0, 0, false},
		{"tie_earlier", []float64{1, 0, 1, 0, 1}, 2, 1, 1, true},
		{"nearest_later", []float64{0, 1, 1, 0}, 2, 2, 3, true},
		{"left_inclusive", []float64{0, 1, 1, 1}, 2, 2, 0, true},
		{"right_inclusive", []float64{1, 1, 1, 0}, 1, 2, 3, true},
		{"just_outside", []float64{0, 1, 1, 1}, 2, 1, 0, false},
		{"predecessor_outside_window", []float64{-1, 1, 1}, 1, 0, 1, true},
		{"eof_zero_radius", []float64{0}, 1, 0, 0, false},
		{"eof_zero_sample", []float64{1, 0}, 2, 1, 1, true},
		{"eof_crossing", []float64{-1, 1}, 2, 1, 1, true},
		{"eof_no_crossing", []float64{1, 1}, 2, maxRadius, 0, false},
		{"overflow_radius", []float64{0, 1, 1, 0}, 2, maxRadius, 3, true},
		{"overflow_radius_eof", []float64{0, 1, 1, 0}, 4, maxRadius, 3, true},
		{"nan_gap", []float64{-1, math.NaN(), 1}, 2, maxRadius, 0, false},
		{"infinity_gap", []float64{-1, math.Inf(1), 1}, 2, maxRadius, 0, false},
		{"opposite_infinities", []float64{math.Inf(-1), math.Inf(1)}, 1, maxRadius, 0, false},
		{"infinite_previous", []float64{math.Inf(-1), 1}, 1, 0, 0, false},
		{"infinite_current", []float64{-1, math.Inf(1)}, 1, 0, 0, false},
		{"zero_after_gap", []float64{math.NaN(), 0}, 1, 0, 1, true},
		{"crossing_after_gap", []float64{math.Inf(1), -1, 1}, 2, 0, 2, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checkZeroCrossing(t, test.signal, test.target, test.radius, test.want, test.found)

			signal32 := make([]float32, len(test.signal))
			for i, value := range test.signal {
				signal32[i] = float32(value)
			}

			checkZeroCrossing(t, signal32, test.target, test.radius, test.want, test.found)
		})
	}

	checkZeroCrossing(t, []float64{-math.SmallestNonzeroFloat64, math.SmallestNonzeroFloat64}, 1, 0, 1, true)
	checkZeroCrossing(t, []float64{-math.MaxFloat64, math.MaxFloat64}, 1, 0, 1, true)

	type sample float32

	checkZeroCrossing(t, []sample{-1, 1}, 1, 0, 1, true)
}

func checkZeroCrossing[T ~float32 | ~float64](t *testing.T, signal []T, target, radius, want int, found bool) {
	t.Helper()

	if got, ok := NearestZeroCrossing(signal, target, radius); got != want || ok != found {
		t.Fatalf("target=%d radius=%d got=(%d,%v), want=(%d,%v)", target, radius, got, ok, want, found)
	}
}

func TestNearestZeroCrossingExhaustiveWindows(t *testing.T) {
	signal := []float64{1, 0, -1, math.NaN(), 1, -1, math.Inf(-1), 0, -1, 1, 1}
	for target := 0; target <= len(signal); target++ {
		for radius := 0; radius <= len(signal)+1; radius++ {
			want, found, best := 0, false, len(signal)+1
			for index, value := range signal {
				if math.IsNaN(value) || math.IsInf(value, 0) {
					continue
				}

				crossing := value == 0

				if index > 0 {
					previous := signal[index-1]
					crossing = crossing || (!math.IsNaN(previous) && !math.IsInf(previous, 0) &&
						((previous < 0 && value > 0) || (previous > 0 && value < 0)))
				}

				distance := int(math.Abs(float64(index - target)))
				if crossing && distance <= radius && distance < best {
					want, found, best = index, true, distance
				}
			}

			checkZeroCrossing(t, signal, target, radius, want, found)
		}
	}
}

func TestNearestZeroCrossingDoesNotMutateOrAllocate(t *testing.T) {
	signal32 := []float32{math.Float32frombits(0x7f812345), -1, 1, math.Float32frombits(0x80000000)}
	signal64 := []float64{math.Float64frombits(0x7ff0123456789abc), -1, 1, math.Copysign(0, -1)}

	before32, before64 := make([]uint32, len(signal32)), make([]uint64, len(signal64))
	for i := range signal32 {
		before32[i] = math.Float32bits(signal32[i])
		before64[i] = math.Float64bits(signal64[i])
	}

	for _, scan := range []func(){
		func() { NearestZeroCrossing(signal32, 2, 3) },
		func() { NearestZeroCrossing(signal64, 2, 3) },
	} {
		if allocs := testing.AllocsPerRun(100, scan); allocs != 0 {
			t.Fatalf("allocations = %g, want zero", allocs)
		}
	}

	for i := range signal32 {
		if math.Float32bits(signal32[i]) != before32[i] || math.Float64bits(signal64[i]) != before64[i] {
			t.Fatalf("mutated sample %d", i)
		}
	}
}
