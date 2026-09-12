package conv

import (
	"math"
	"testing"
)

// TestAddBlockInPlaceParity checks that the float64 instantiation, which
// dispatches to the vecmath kernel, and the float32 one, which keeps the scalar
// loop, both match a plain element-wise add. The float64 case is compared with
// == -- an element-wise add has nothing to reassociate or fuse, so the vector
// kernel is bit-identical to the scalar loop on every architecture.
func TestAddBlockInPlaceParity(t *testing.T) {
	for _, n := range []int{0, 1, 2, 3, 4, 7, 8, 15, 16, 17, 31, 33, 1024} {
		dst64, src64 := make([]float64, n), make([]float64, n)
		dst32, src32 := make([]float32, n), make([]float32, n)
		want64, want32 := make([]float64, n), make([]float32, n)

		for i := range n {
			d := math.Sin(float64(i) * 0.31)
			s := math.Cos(float64(i) * 0.17)

			dst64[i], src64[i] = d, s
			dst32[i], src32[i] = float32(d), float32(s)
			want64[i] = d + s
			want32[i] = float32(d) + float32(s)
		}

		addBlockInPlace(dst64, src64)
		addBlockInPlace(dst32, src32)

		for i := range n {
			if dst64[i] != want64[i] {
				t.Errorf("float64 n=%d index %d: got %v, want %v", n, i, dst64[i], want64[i])
			}

			if dst32[i] != want32[i] {
				t.Errorf("float32 n=%d index %d: got %v, want %v", n, i, dst32[i], want32[i])
			}
		}
	}
}

// TestAddBlockInPlaceAllocs pins the helper as allocation-free, which is the
// point of dispatching on unsafe.Sizeof rather than boxing through any().
func TestAddBlockInPlaceAllocs(t *testing.T) {
	dst64, src64 := make([]float64, 512), make([]float64, 512)
	dst32, src32 := make([]float32, 512), make([]float32, 512)

	if got := testing.AllocsPerRun(100, func() { addBlockInPlace(dst64, src64) }); got != 0 {
		t.Errorf("float64: allocated %v times per run, want 0", got)
	}

	if got := testing.AllocsPerRun(100, func() { addBlockInPlace(dst32, src32) }); got != 0 {
		t.Errorf("float32: allocated %v times per run, want 0", got)
	}
}
