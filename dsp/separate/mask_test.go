package separate_test

import (
	"errors"
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/separate"
)

func matrix(rows ...[]float64) [][]float64 { return rows }

func zerosLike(m [][]float64) [][]float64 {
	out := make([][]float64, len(m))
	for i := range m {
		out[i] = make([]float64, len(m[i]))
	}

	return out
}

func TestSoftMasks(t *testing.T) {
	t.Parallel()

	inf := math.Inf(1)

	tests := []struct {
		name  string
		est   [][][]float64
		power float64
		want  [][][]float64
	}{
		{
			name:  "wiener two sources",
			est:   [][][]float64{matrix([]float64{3, 0, 1}), matrix([]float64{4, 2, 1})},
			power: 2,
			want:  [][][]float64{matrix([]float64{9.0 / 25, 0, 0.5}), matrix([]float64{16.0 / 25, 1, 0.5})},
		},
		{
			name:  "power one",
			est:   [][][]float64{matrix([]float64{1, 3}), matrix([]float64{3, 1})},
			power: 1,
			want:  [][][]float64{matrix([]float64{0.25, 0.75}), matrix([]float64{0.75, 0.25})},
		},
		{
			name:  "zero bins split evenly over three sources",
			est:   [][][]float64{matrix([]float64{0}), matrix([]float64{0}), matrix([]float64{0})},
			power: 2,
			want:  [][][]float64{matrix([]float64{1.0 / 3}), matrix([]float64{1.0 / 3}), matrix([]float64{1.0 / 3})},
		},
		{
			name:  "binary with tie",
			est:   [][][]float64{matrix([]float64{2, 1, 5}), matrix([]float64{1, 2, 5})},
			power: inf,
			want:  [][][]float64{matrix([]float64{1, 0, 0.5}), matrix([]float64{0, 1, 0.5})},
		},
		{
			name:  "single source",
			est:   [][][]float64{matrix([]float64{0, 7})},
			power: 2,
			want:  [][][]float64{matrix([]float64{1, 1})},
		},
		{
			name:  "no overflow for large values and powers",
			est:   [][][]float64{matrix([]float64{1e300}), matrix([]float64{1e300})},
			power: 7,
			want:  [][][]float64{matrix([]float64{0.5}), matrix([]float64{0.5})},
		},
		{
			name:  "general power",
			est:   [][][]float64{matrix([]float64{1}), matrix([]float64{2})},
			power: 3,
			want:  [][][]float64{matrix([]float64{1.0 / 9}), matrix([]float64{8.0 / 9})},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dst := make([][][]float64, len(tc.est))
			for s := range dst {
				dst[s] = zerosLike(tc.est[s])
			}

			err := separate.SoftMasks(dst, tc.est, tc.power)
			if err != nil {
				t.Fatal(err)
			}

			for s := range dst {
				for i := range dst[s] {
					for k := range dst[s][i] {
						if math.Abs(dst[s][i][k]-tc.want[s][i][k]) > 1e-15 {
							t.Fatalf("mask %d [%d][%d] = %v, want %v", s, i, k, dst[s][i][k], tc.want[s][i][k])
						}
					}
				}
			}
		})
	}
}

func TestSoftMasksSumToOneInPlace(t *testing.T) {
	t.Parallel()

	const frames, bins, sources = 7, 33, 4

	est := make([][][]float64, sources)
	for s := range est {
		est[s] = make([][]float64, frames)
		for i := range est[s] {
			est[s][i] = make([]float64, bins)
			for k := range est[s][i] {
				est[s][i][k] = math.Abs(math.Sin(float64(1+s*frames*bins+i*bins+k))) * float64(k%3)
			}
		}
	}

	// In place: dst[s] == est[s].
	err := separate.SoftMasks(est, est, 2)
	if err != nil {
		t.Fatal(err)
	}

	for i := range frames {
		for k := range bins {
			sum := 0.0
			for s := range sources {
				sum += est[s][i][k]
			}

			if math.Abs(sum-1) > 1e-15 {
				t.Fatalf("[%d][%d] sum %v", i, k, sum)
			}
		}
	}
}

func TestSoftMasksErrors(t *testing.T) {
	t.Parallel()

	one := func(v float64) [][]float64 { return matrix([]float64{v}) }

	tests := []struct {
		name  string
		dst   [][][]float64
		est   [][][]float64
		power float64
		want  error
	}{
		{"no sources", nil, nil, 2, separate.ErrShapeMismatch},
		{"dst count", [][][]float64{one(0)}, [][][]float64{one(1), one(1)}, 2, separate.ErrShapeMismatch},
		{"estimate shape", [][][]float64{one(0), one(0)}, [][][]float64{one(1), matrix([]float64{1, 2})}, 2, separate.ErrShapeMismatch},
		{"dst shape", [][][]float64{one(0), matrix()}, [][][]float64{one(1), one(1)}, 2, separate.ErrShapeMismatch},
		{"negative", [][][]float64{one(0), one(0)}, [][][]float64{one(1), one(-1)}, 2, separate.ErrInvalidValue},
		{"nan", [][][]float64{one(0), one(0)}, [][][]float64{one(math.NaN()), one(1)}, 2, separate.ErrInvalidValue},
		{"inf", [][][]float64{one(0), one(0)}, [][][]float64{one(math.Inf(1)), one(1)}, 2, separate.ErrInvalidValue},
		{"zero power", [][][]float64{one(0)}, [][][]float64{one(1)}, 0, separate.ErrInvalidPower},
		{"nan power", [][][]float64{one(0)}, [][][]float64{one(1)}, math.NaN(), separate.ErrInvalidPower},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := separate.SoftMasks(tc.dst, tc.est, tc.power)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestApplyMaskAndMagnitudes(t *testing.T) {
	t.Parallel()

	spec := [][]complex128{{3 + 4i, -1}, {2i, 0}}
	mask := [][]float64{{0.5, 2}, {0.25, 1}}
	dst := [][]complex128{make([]complex128, 2), make([]complex128, 2)}

	err := separate.ApplyMask(dst, spec, mask)
	if err != nil {
		t.Fatal(err)
	}

	want := [][]complex128{{1.5 + 2i, -2}, {0.5i, 0}}
	for i := range want {
		for k := range want[i] {
			if dst[i][k] != want[i][k] {
				t.Fatalf("[%d][%d] = %v, want %v", i, k, dst[i][k], want[i][k])
			}
		}
	}

	// In place.
	err = separate.ApplyMask(spec, spec, mask)
	if err != nil {
		t.Fatal(err)
	}

	if spec[0][0] != want[0][0] || spec[1][0] != want[1][0] {
		t.Fatalf("in-place result %v", spec)
	}

	mag := [][]float64{make([]float64, 2), make([]float64, 2)}

	err = separate.Magnitudes(mag, [][]complex128{{3 + 4i, -1}, {2i, 0}})
	if err != nil {
		t.Fatal(err)
	}

	if mag[0][0] != 5 || mag[0][1] != 1 || mag[1][0] != 2 || mag[1][1] != 0 {
		t.Fatalf("magnitudes %v", mag)
	}

	short := [][]complex128{make([]complex128, 1)}

	err = separate.ApplyMask(short, spec, mask)
	if !errors.Is(err, separate.ErrShapeMismatch) {
		t.Fatalf("dst shape: got %v", err)
	}

	err = separate.ApplyMask(dst, spec, [][]float64{{1, 1}, {1}})
	if !errors.Is(err, separate.ErrShapeMismatch) {
		t.Fatalf("mask shape: got %v", err)
	}

	err = separate.Magnitudes([][]float64{{0}}, spec)
	if !errors.Is(err, separate.ErrShapeMismatch) {
		t.Fatalf("magnitudes shape: got %v", err)
	}
}

func TestMaskHelpersNoAllocs(t *testing.T) {
	const frames, bins = 10, 513

	est := [][][]float64{make([][]float64, frames), make([][]float64, frames)}
	dst := [][][]float64{make([][]float64, frames), make([][]float64, frames)}
	spec := make([][]complex128, frames)

	for i := range frames {
		spec[i] = make([]complex128, bins)

		for s := range est {
			est[s][i] = make([]float64, bins)
			dst[s][i] = make([]float64, bins)

			for k := range bins {
				est[s][i][k] = float64((s+1)*k%7) + 0.5
			}
		}
	}

	allocs := testing.AllocsPerRun(10, func() {
		_ = separate.SoftMasks(dst, est, 2)
		_ = separate.ApplyMask(spec, spec, dst[0])
		_ = separate.Magnitudes(est[0], spec)
	})
	if allocs != 0 {
		t.Fatalf("mask helpers allocate %v times per run", allocs)
	}
}
