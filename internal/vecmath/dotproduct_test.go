package vecmath

import (
	"math"
	"runtime"
	"testing"

	"github.com/cwbudde/algo-dsp/internal/cpu"
)

func dotProductRef(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return 0
	}

	sum := 0.0
	for i := 0; i < n; i++ {
		sum += a[i] * b[i]
	}
	return sum
}

func TestDotProduct(t *testing.T) {
	cases := []struct {
		name string
		a    []float64
		b    []float64
		want float64
	}{
		{name: "empty", a: nil, b: nil, want: 0},
		{name: "one empty", a: []float64{1, 2}, b: nil, want: 0},
		{name: "single", a: []float64{3.5}, b: []float64{2.0}, want: 7.0},
		{name: "two elements", a: []float64{1, 2}, b: []float64{3, 4}, want: 11},
		{name: "mixed signs", a: []float64{-1, 2, -3}, b: []float64{4, -5, 6}, want: -32},
		{name: "orthogonal", a: []float64{1, 0}, b: []float64{0, 1}, want: 0},
		{name: "different lengths", a: []float64{1, 2, 3, 4}, b: []float64{2, 3}, want: 8},
		{name: "includes inf", a: []float64{1, math.Inf(1), 2}, b: []float64{1, 1, 1}, want: math.Inf(1)},
		{name: "simple dot", a: []float64{1, 2, 3}, b: []float64{4, 5, 6}, want: 32},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DotProduct(tc.a, tc.b)
			if math.IsInf(tc.want, 1) {
				if !math.IsInf(got, 1) {
					t.Fatalf("DotProduct() = %v, want +Inf", got)
				}
				return
			}
			if math.IsInf(tc.want, -1) {
				if !math.IsInf(got, -1) {
					t.Fatalf("DotProduct() = %v, want -Inf", got)
				}
				return
			}
			if !closeEnough(got, tc.want) {
				t.Fatalf("DotProduct() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDotProductReferenceParity(t *testing.T) {
	sizes := []int{0, 1, 2, 3, 4, 5, 7, 8, 15, 16, 17, 31, 32, 33, 63, 64, 100, 1000, 1023, 1024, 1025}
	for _, n := range sizes {
		t.Run(sizeStr(n), func(t *testing.T) {
			a := make([]float64, n)
			b := make([]float64, n)
			for i := range a {
				a[i] = float64((i*37)%113) + 0.125
				b[i] = float64((i*53)%97) + 0.25
			}
			got := DotProduct(a, b)
			want := dotProductRef(a, b)
			if !closeEnough(got, want) {
				t.Fatalf("DotProduct() = %v, want %v", got, want)
			}
		})
	}
}

func TestDotProductDispatchParity(t *testing.T) {
	hw := cpu.DetectFeatures()
	if !hw.HasAVX2 {
		t.Skip("AVX2 not available on this host")
	}

	a := make([]float64, 1025)
	b := make([]float64, 1025)
	for i := range a {
		a[i] = math.Sin(float64(i) * 0.1)
		b[i] = math.Cos(float64(i) * 0.17)
	}

	defer cpu.ResetDetection()

	cpu.SetForcedFeatures(cpu.Features{Architecture: "amd64", HasAVX2: false})
	gotGeneric := DotProduct(a, b)

	cpu.SetForcedFeatures(cpu.Features{Architecture: "amd64", HasAVX2: true})
	gotAVX2 := DotProduct(a, b)

	if !closeEnough(gotGeneric, gotAVX2) {
		t.Fatalf("dispatch parity mismatch: generic=%v avx2=%v", gotGeneric, gotAVX2)
	}
}

func TestDotProductSSE2DispatchParity(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("SSE2 dispatch test is amd64-only")
	}

	a := make([]float64, 1025)
	b := make([]float64, 1025)
	for i := range a {
		a[i] = math.Cos(float64(i) * 0.19)
		b[i] = math.Sin(float64(i) * 0.23)
	}

	defer cpu.ResetDetection()

	cpu.SetForcedFeatures(cpu.Features{Architecture: "amd64"})
	gotGeneric := DotProduct(a, b)

	cpu.SetForcedFeatures(cpu.Features{Architecture: "amd64", HasSSE2: true})
	gotSSE2 := DotProduct(a, b)

	if !closeEnough(gotGeneric, gotSSE2) {
		t.Fatalf("dispatch parity mismatch: generic=%v sse2=%v", gotGeneric, gotSSE2)
	}
}
