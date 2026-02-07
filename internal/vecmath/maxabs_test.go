package vecmath

import (
	"math"
	"runtime"
	"testing"

	"github.com/cwbudde/algo-dsp/internal/cpu"
)

func maxAbsRef(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}
	max := math.Abs(x[0])
	for i := 1; i < len(x); i++ {
		v := math.Abs(x[i])
		if v > max {
			max = v
		}
	}
	return max
}

func TestMaxAbs(t *testing.T) {
	cases := []struct {
		name string
		x    []float64
		want float64
	}{
		{name: "empty", x: nil, want: 0},
		{name: "single positive", x: []float64{3.5}, want: 3.5},
		{name: "single negative", x: []float64{-7.25}, want: 7.25},
		{name: "mixed", x: []float64{-1, 2, -3, 0.5}, want: 3},
		{name: "includes inf", x: []float64{1, math.Inf(-1), 2}, want: math.Inf(1)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MaxAbs(tc.x)
			if math.IsInf(tc.want, 1) {
				if !math.IsInf(got, 1) {
					t.Fatalf("MaxAbs() = %v, want +Inf", got)
				}
				return
			}
			if !closeEnough(got, tc.want) {
				t.Fatalf("MaxAbs() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMaxAbsReferenceParity(t *testing.T) {
	sizes := []int{0, 1, 2, 3, 4, 5, 7, 8, 15, 16, 17, 31, 32, 33, 63, 64, 100, 1000, 1023, 1024, 1025}
	for _, n := range sizes {
		t.Run(sizeStr(n), func(t *testing.T) {
			x := make([]float64, n)
			for i := range x {
				sign := 1.0
				if i%2 == 0 {
					sign = -1.0
				}
				x[i] = sign * (float64((i*37)%113) + 0.125)
			}
			got := MaxAbs(x)
			want := maxAbsRef(x)
			if !closeEnough(got, want) {
				t.Fatalf("MaxAbs() = %v, want %v", got, want)
			}
		})
	}
}

func TestMaxAbsDispatchParity(t *testing.T) {
	hw := cpu.DetectFeatures()
	if !hw.HasAVX2 {
		t.Skip("AVX2 not available on this host")
	}

	x := make([]float64, 1025)
	for i := range x {
		x[i] = math.Sin(float64(i)*0.1) * float64(i-400)
	}

	defer cpu.ResetDetection()

	cpu.SetForcedFeatures(cpu.Features{Architecture: "amd64", HasAVX2: false})
	gotGeneric := MaxAbs(x)

	cpu.SetForcedFeatures(cpu.Features{Architecture: "amd64", HasAVX2: true})
	gotAVX2 := MaxAbs(x)

	if !closeEnough(gotGeneric, gotAVX2) {
		t.Fatalf("dispatch parity mismatch: generic=%v avx2=%v", gotGeneric, gotAVX2)
	}
}

func TestMaxAbsSSE2DispatchParity(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("SSE2 dispatch test is amd64-only")
	}

	x := make([]float64, 1025)
	for i := range x {
		x[i] = math.Cos(float64(i)*0.17) * float64(i-321)
	}

	defer cpu.ResetDetection()

	cpu.SetForcedFeatures(cpu.Features{Architecture: "amd64"})
	gotGeneric := MaxAbs(x)

	cpu.SetForcedFeatures(cpu.Features{Architecture: "amd64", HasSSE2: true})
	gotSSE2 := MaxAbs(x)

	if !closeEnough(gotGeneric, gotSSE2) {
		t.Fatalf("dispatch parity mismatch: generic=%v sse2=%v", gotGeneric, gotSSE2)
	}
}
