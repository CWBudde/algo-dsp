package conv

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/cwbudde/algo-vecmath"
)

// correlateStridedRef is the naive reference for CorrelateStridedInto: a
// double loop with an explicit bounds check per sample.
func correlateStridedRef(outLen int, x, h []float64, start, stride int) []float64 {
	out := make([]float64, outLen)

	for i := range out {
		var sum float64

		for k := range h {
			j := start + i*stride + k
			if j >= 0 && j < len(x) {
				sum += h[k] * x[j]
			}
		}

		out[i] = sum
	}

	return out
}

// decimateRef is a copy of the private decimate routine in dsp/cqt, which
// CorrelateStridedInto must reproduce bit for bit.
func decimateRef(dst, x, h []float64, factor int) {
	pad := (len(h) - 1) / 2

	for i := range dst {
		off := i*factor - pad
		kLo := max(0, -off)
		kHi := min(len(h), len(x)-off)
		dst[i] = vecmath.DotProduct(x[off+kLo:off+kHi], h[kLo:kHi])
	}
}

func randomSlice(rng *rand.Rand, n int) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = 2*rng.Float64() - 1
	}

	return s
}

func rampSlice(n int) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = float64(i + 1)
	}

	return s
}

// assertCloseSlices compares got with want using a tolerance relative to the
// magnitude of the terms (DotProduct may sum in a different order).
func assertCloseSlices(t *testing.T, got, want, x, h []float64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %d, want %d", len(got), len(want))
	}

	var scale float64
	for _, v := range x {
		scale = max(scale, math.Abs(v))
	}

	var hSum float64
	for _, v := range h {
		hSum += math.Abs(v)
	}

	tol := 1e-12 * max(1, scale*hSum)

	for i := range got {
		if math.Abs(got[i]-want[i]) > tol {
			t.Fatalf("index %d: got %.17g, want %.17g (tol %g)", i, got[i], want[i], tol)
		}
	}
}

func TestCorrelateStridedInto(t *testing.T) {
	x10 := rampSlice(10)

	tests := []struct {
		name   string
		x      []float64
		h      []float64
		start  int
		stride int
		outLen int
	}{
		{"valid correlation", x10, []float64{1, -2, 3}, 0, 1, 8},
		{"left zero padding", x10, []float64{1, 2, 3, 4}, -3, 1, 13},
		{"torch padding stride 2", x10, []float64{0.25, 0.5, 1, 0.5, 0.25}, -2, 2, (10+2*2-5)/2 + 1},
		{"torch padding even kernel", x10, []float64{1, 2, 3, 4}, -1, 2, (10+2*1-4)/2 + 1},
		{"stride larger than kernel", x10, []float64{1, 1}, 0, 4, 3},
		{"kernel longer than x", []float64{1, 2, 3}, []float64{1, 2, 3, 4, 5, 6, 7}, -6, 1, 9},
		{"start far left", x10, []float64{1, 2, 3}, -1000, 3, 5},
		{"start far right", x10, []float64{1, 2, 3}, 11, 1, 4},
		{"start at end", x10, []float64{1, 2, 3}, 10, 2, 3},
		{"runs past end", x10, []float64{1, 2, 3}, 4, 2, 6},
		{"single tap strided sampling", x10, []float64{1}, 1, 3, 4},
		{"single tap with padding", x10, []float64{2}, -2, 3, 6},
		{"odd kernel", x10, []float64{1, 2, 3, 4, 5}, -2, 1, 10},
		{"even kernel", x10, []float64{1, 2, 3, 4, 5, 6}, -3, 1, 10},
		{"left padding and huge stride", x10, []float64{1, 2}, -5, 100, 3},
		{"single output", x10, []float64{1, 2, 3}, 2, 7, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := make([]float64, tt.outLen)
			for i := range dst {
				dst[i] = math.NaN()
			}

			err := CorrelateStridedInto(dst, tt.x, tt.h, tt.start, tt.stride)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			want := correlateStridedRef(tt.outLen, tt.x, tt.h, tt.start, tt.stride)
			assertCloseSlices(t, dst, want, tt.x, tt.h)
		})
	}
}

func TestCorrelateStridedIntoExactValues(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5}
	h := []float64{1, 10, 100}
	dst := make([]float64, 4)

	// Windows start at -1, 1, 3, 5.
	err := CorrelateStridedInto(dst, x, h, -1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []float64{
		0*1 + 1*10 + 2*100,
		2*1 + 3*10 + 4*100,
		4*1 + 5*10 + 0*100,
		0,
	}

	for i := range want {
		if dst[i] != want[i] {
			t.Errorf("dst[%d] = %v, want %v", i, dst[i], want[i])
		}
	}
}

func TestCorrelateStridedIntoAllOutside(t *testing.T) {
	x := rampSlice(16)
	h := []float64{1, 2, 3}

	tests := []struct {
		name   string
		start  int
		stride int
	}{
		{"far left", -1000, 1},
		{"far left large stride", -1000, 7},
		{"just left", -3 - 4*5, 5},
		{"far right", 1000, 1},
		{"just right", 16, 1},
		{"min int start", math.MinInt, 1},
		{"max int start", math.MaxInt, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := []float64{7, 7, 7, 7}

			err := CorrelateStridedInto(dst, x, h, tt.start, tt.stride)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for i, v := range dst {
				if v != 0 {
					t.Errorf("dst[%d] = %v, want 0", i, v)
				}
			}
		})
	}
}

func TestCorrelateStridedIntoExtremeStride(t *testing.T) {
	x := rampSlice(8)
	h := []float64{1, 2}
	dst := []float64{7, 7, 7}

	err := CorrelateStridedInto(dst, x, h, -1, math.MaxInt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only the first window [-1, 1) overlaps x.
	want := []float64{2 * x[0], 0, 0}
	for i := range want {
		if dst[i] != want[i] {
			t.Errorf("dst[%d] = %v, want %v", i, dst[i], want[i])
		}
	}
}

func TestCorrelateStridedIntoEmptyX(t *testing.T) {
	dst := []float64{3, math.NaN(), math.Inf(1), -4}

	err := CorrelateStridedInto(dst, nil, []float64{1, 2, 3}, -1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, v := range dst {
		if v != 0 {
			t.Errorf("dst[%d] = %v, want 0", i, v)
		}
	}
}

func TestCorrelateStridedIntoEmptyDst(t *testing.T) {
	x := rampSlice(8)

	err := CorrelateStridedInto(nil, x, []float64{1, 2}, 0, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = CorrelateStridedInto(x[:0], x, []float64{1, 2}, 0, 1)
	if err != nil {
		t.Fatalf("empty dst sharing x's array: unexpected error: %v", err)
	}
}

func TestCorrelateStridedIntoRandom(t *testing.T) {
	rng := rand.New(rand.NewPCG(46, 2))

	for _, n := range []int{0, 1, 5, 17, 64} {
		for _, m := range []int{1, 2, 3, 8, 21} {
			x := randomSlice(rng, n)
			h := randomSlice(rng, m)

			for _, start := range []int{-30, -m, -(m - 1) / 2, -1, 0, 3, n - 1, n + 2} {
				for _, stride := range []int{1, 2, 3, 7} {
					outLen := (n+m)/stride + 3

					t.Run(fmt.Sprintf("n=%d_m=%d_start=%d_stride=%d", n, m, start, stride), func(t *testing.T) {
						dst := make([]float64, outLen)
						for i := range dst {
							dst[i] = math.NaN()
						}

						err := CorrelateStridedInto(dst, x, h, start, stride)
						if err != nil {
							t.Fatalf("unexpected error: %v", err)
						}

						want := correlateStridedRef(outLen, x, h, start, stride)
						assertCloseSlices(t, dst, want, x, h)
					})
				}
			}
		}
	}
}

func TestCorrelateStridedIntoMatchesDecimate(t *testing.T) {
	rng := rand.New(rand.NewPCG(46, 3))

	tests := []struct {
		n, m, factor int
	}{
		{43844, 256, 2},
		{1000, 256, 2},
		{100, 255, 2},
		{777, 64, 3},
		{513, 33, 4},
		{10, 5, 2},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("n=%d_m=%d_factor=%d", tt.n, tt.m, tt.factor), func(t *testing.T) {
			x := randomSlice(rng, tt.n)
			h := randomSlice(rng, tt.m)
			pad := (tt.m - 1) / 2
			outLen := (tt.n+2*pad-tt.m)/tt.factor + 1

			want := make([]float64, outLen)
			decimateRef(want, x, h, tt.factor)

			got := make([]float64, outLen)

			err := CorrelateStridedInto(got, x, h, -pad, tt.factor)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for i := range want {
				if math.Float64bits(got[i]) != math.Float64bits(want[i]) {
					t.Fatalf("index %d: got %.17g, want %.17g", i, got[i], want[i])
				}
			}
		})
	}
}

func TestCorrelateStridedIntoErrors(t *testing.T) {
	x := rampSlice(8)
	h := []float64{1, 2, 3}

	tests := []struct {
		name   string
		dst    []float64
		x      []float64
		h      []float64
		stride int
		want   error
	}{
		{"zero stride", make([]float64, 2), x, h, 0, ErrInvalidStride},
		{"negative stride", make([]float64, 2), x, h, -1, ErrInvalidStride},
		{"empty kernel", make([]float64, 2), x, nil, 1, ErrEmptyKernel},
		{"dst aliases x", x[1:3], x, h, 1, ErrAliasing},
		{"dst identical to x", x, x, h, 1, ErrAliasing},
		{"dst aliases h", h[2:], x, h, 1, ErrAliasing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CorrelateStridedInto(tt.dst, tt.x, tt.h, 0, tt.stride)
			if !errors.Is(err, tt.want) {
				t.Fatalf("got error %v, want %v", err, tt.want)
			}
		})
	}
}

func TestCorrelateStridedIntoZeroAlloc(t *testing.T) {
	x := makeTestSignal(4096)
	h := makeTestKernel(64)
	dst := make([]float64, (len(x)+62-len(h))/2+1)

	allocs := testing.AllocsPerRun(100, func() {
		_ = CorrelateStridedInto(dst, x, h, -31, 2)
	})
	if allocs != 0 {
		t.Fatalf("CorrelateStridedInto allocated %v times per run, want 0", allocs)
	}
}
