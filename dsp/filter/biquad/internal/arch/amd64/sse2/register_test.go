//go:build amd64 && !purego

package sse2

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad/internal/arch/registry"
)

func TestProcessBlock_MatchesReference(t *testing.T) {
	c := registry.Coefficients{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04}
	in := []float64{1, 0.5, -0.3, 0.7, 0, -1, 0.2, 0.8, -0.1}
	got := append([]float64(nil), in...)
	want := append([]float64(nil), in...)

	d0g, d1g := processBlock(c, 0, 0, got)
	d0w, d1w := refProcess(c, 0, 0, want)

	if !almostEq(d0g, d0w, 1e-12) || !almostEq(d1g, d1w, 1e-12) {
		t.Fatalf("state mismatch: got (%g,%g), want (%g,%g)", d0g, d1g, d0w, d1w)
	}
	for i := range got {
		if !almostEq(got[i], want[i], 1e-12) {
			t.Fatalf("sample %d mismatch: got %.15f, want %.15f", i, got[i], want[i])
		}
	}
}

func BenchmarkProcessBlock_SSE2Kernel(b *testing.B) {
	c := registry.Coefficients{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04}
	for _, n := range []int{256, 1024, 4096} {
		b.Run("n="+itoa(n), func(b *testing.B) {
			buf := make([]float64, n)
			for i := range buf {
				buf[i] = float64(i) * 0.001
			}
			b.SetBytes(int64(n * 8))
			b.ReportAllocs()
			var d0, d1 float64
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				d0, d1 = processBlock(c, d0, d1, buf)
			}
		})
	}
}

func refProcess(c registry.Coefficients, d0, d1 float64, buf []float64) (float64, float64) {
	for i, x := range buf {
		y := c.B0*x + d0
		d0 = c.B1*x - c.A1*y + d1
		d1 = c.B2*x - c.A2*y
		buf[i] = y
	}
	return d0, d1
}

func almostEq(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func itoa(v int) string {
	if v == 256 {
		return "256"
	}
	if v == 1024 {
		return "1024"
	}
	if v == 4096 {
		return "4096"
	}
	return "x"
}
