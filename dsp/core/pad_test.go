package core

import (
	"errors"
	"math"
	"testing"
)

// naivePadReflect computes reflect padding by mirroring each virtual index
// about the first and last sample (edge not repeated).
func naivePadReflect(x []float64, left, right int) []float64 {
	n := len(x)
	out := make([]float64, 0, left+n+right)

	for i := -left; i < n+right; i++ {
		j := i
		if j < 0 {
			j = -j
		}

		if j >= n {
			j = 2*(n-1) - j
		}

		out = append(out, x[j])
	}

	return out
}

func TestPadReflect(t *testing.T) {
	tests := []struct {
		name        string
		x           []float64
		left, right int
		want        []float64
	}{
		{name: "both", x: []float64{1, 2, 3, 4}, left: 2, right: 2, want: []float64{3, 2, 1, 2, 3, 4, 3, 2}},
		{name: "left only", x: []float64{1, 2, 3, 4}, left: 3, right: 0},
		{name: "right only", x: []float64{1, 2, 3, 4}, left: 0, right: 3},
		{name: "asymmetric", x: []float64{1, 2, 3, 4, 5}, left: 1, right: 4},
		{name: "maximal", x: []float64{1, 2, 3, 4, 5}, left: 4, right: 4},
		{name: "n=2 pad 1", x: []float64{1, 2}, left: 1, right: 1, want: []float64{2, 1, 2, 1}},
		{name: "zero pads", x: []float64{1, 2, 3}, left: 0, right: 0, want: []float64{1, 2, 3}},
		{name: "single sample zero pads", x: []float64{7}, left: 0, right: 0, want: []float64{7}},
		{name: "empty zero pads", x: nil, left: 0, right: 0, want: []float64{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			want := naivePadReflect(tc.x, tc.left, tc.right)
			if tc.want != nil {
				assertEqual(t, "reference", want, tc.want)
			}

			// dst longer than needed: the tail must stay untouched.
			const extra = 3

			sentinel := math.Inf(-1)

			dst := make([]float64, len(want)+extra)
			for i := range dst {
				dst[i] = sentinel
			}

			if err := PadReflect(dst, tc.x, tc.left, tc.right); err != nil {
				t.Fatalf("PadReflect: %v", err)
			}

			assertEqual(t, "output", dst[:len(want)], want)

			for i, v := range dst[len(want):] {
				if v != sentinel {
					t.Fatalf("tail[%d] = %v, want untouched sentinel", i, v)
				}
			}
		})
	}
}

func TestPadReflectErrors(t *testing.T) {
	x4 := []float64{1, 2, 3, 4}
	buf := make([]float64, 16)

	tests := []struct {
		name        string
		dst, x      []float64
		left, right int
		want        error
	}{
		{name: "negative left", dst: make([]float64, 8), x: x4, left: -1, right: 0, want: ErrNegativePad},
		{name: "negative right", dst: make([]float64, 8), x: x4, left: 0, right: -1, want: ErrNegativePad},
		{name: "left equals len", dst: make([]float64, 16), x: x4, left: 4, right: 1, want: ErrPadTooLong},
		{name: "right equals len", dst: make([]float64, 16), x: x4, left: 1, right: 4, want: ErrPadTooLong},
		{name: "left exceeds len", dst: make([]float64, 16), x: x4, left: 9, right: 0, want: ErrPadTooLong},
		{name: "empty x left pad", dst: make([]float64, 4), x: nil, left: 1, right: 0, want: ErrPadTooLong},
		{name: "empty x right pad", dst: make([]float64, 4), x: []float64{}, left: 0, right: 1, want: ErrPadTooLong},
		{name: "single sample pad 1", dst: make([]float64, 4), x: []float64{1}, left: 1, right: 1, want: ErrPadTooLong},
		{name: "short dst", dst: make([]float64, 7), x: x4, left: 2, right: 2, want: ErrShortBuffer},
		{name: "short dst zero pads", dst: make([]float64, 3), x: x4, left: 0, right: 0, want: ErrShortBuffer},
		{name: "dst aliases x", dst: buf[0:8], x: buf[2:6], left: 2, right: 2, want: ErrOverlap},
		{name: "dst identical to x", dst: buf[0:4], x: buf[0:4], left: 0, right: 0, want: ErrOverlap},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := PadReflect(tc.dst, tc.x, tc.left, tc.right)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestPadReflectErrorMessage(t *testing.T) {
	err := PadReflect(make([]float64, 16), []float64{1, 2, 3, 4}, 4, 1)

	const want = "core: reflect pad must be shorter than the input: left 4, right 1 for 4 samples"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestPadReflectDstAfterXInSameArray(t *testing.T) {
	// x and the output range live in the same array but do not overlap.
	buf := []float64{1, 2, 3, 0, 0, 0, 0, 0}

	if err := PadReflect(buf[3:], buf[:3], 1, 1); err != nil {
		t.Fatalf("PadReflect: %v", err)
	}

	assertEqual(t, "output", buf, []float64{1, 2, 3, 2, 1, 2, 3, 2})
}

func TestPadReflectZeroAlloc(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5, 6, 7, 8}
	dst := make([]float64, 20)

	allocs := testing.AllocsPerRun(100, func() {
		_ = PadReflect(dst, x, 6, 6)
	})
	if allocs != 0 {
		t.Fatalf("allocs = %v, want 0", allocs)
	}
}

func BenchmarkPadReflect(b *testing.B) {
	const (
		n   = 4096
		pad = 1024
	)

	x := make([]float64, n)
	for i := range x {
		x[i] = float64(i)
	}

	dst := make([]float64, n+2*pad)

	b.ReportAllocs()
	b.SetBytes(int64(len(dst) * 8))

	for b.Loop() {
		if err := PadReflect(dst, x, pad, pad); err != nil {
			b.Fatal(err)
		}
	}
}

func assertEqual(t *testing.T, what string, got, want []float64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s: len = %d, want %d (%v vs %v)", what, len(got), len(want), got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: [%d] = %v, want %v (%v vs %v)", what, i, got[i], want[i], got, want)
		}
	}
}
