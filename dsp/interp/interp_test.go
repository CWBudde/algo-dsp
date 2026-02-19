package interp

import (
	"math"
	"testing"
)

const tol = 1e-12

func approxEqual(a, b, eps float64) bool {
	return math.Abs(a-b) < eps
}

// --- Linear2 ---

func TestLinear2Endpoints(t *testing.T) {
	if got := Linear2(0, 3, 7); got != 3 {
		t.Fatalf("t=0: got %v want 3", got)
	}
	if got := Linear2(1, 3, 7); got != 7 {
		t.Fatalf("t=1: got %v want 7", got)
	}
}

func TestLinear2Midpoint(t *testing.T) {
	if got := Linear2(0.5, 2, 6); got != 4 {
		t.Fatalf("t=0.5: got %v want 4", got)
	}
}

func TestLinear2Quarter(t *testing.T) {
	if got := Linear2(0.25, 0, 8); got != 2 {
		t.Fatalf("t=0.25: got %v want 2", got)
	}
}

// --- Hermite4 ---

func TestHermite4IdentityOnLinearRamp(t *testing.T) {
	xm1, x0, x1, x2 := -1.0, 0.0, 1.0, 2.0
	for _, tc := range []struct {
		t float64
		w float64
	}{
		{t: 0.0, w: 0.0},
		{t: 0.25, w: 0.25},
		{t: 0.5, w: 0.5},
		{t: 1.0, w: 1.0},
	} {
		got := Hermite4(tc.t, xm1, x0, x1, x2)
		if !approxEqual(got, tc.w, tol) {
			t.Fatalf("t=%v: got %v want %v", tc.t, got, tc.w)
		}
	}
}

func TestHermite4EndpointsReturnBracketSamples(t *testing.T) {
	xm1, x0, x1, x2 := 1.0, 5.0, 9.0, 13.0
	if got := Hermite4(0, xm1, x0, x1, x2); !approxEqual(got, x0, tol) {
		t.Fatalf("t=0: got %v want %v", got, x0)
	}
	if got := Hermite4(1, xm1, x0, x1, x2); !approxEqual(got, x1, tol) {
		t.Fatalf("t=1: got %v want %v", got, x1)
	}
}

func TestHermite4QuadraticExact(t *testing.T) {
	// f(x) = x^2 sampled at -1,0,1,2
	f := func(x float64) float64 { return x * x }
	xm1, x0, x1, x2 := f(-1), f(0), f(1), f(2)
	for _, frac := range []float64{0.1, 0.3, 0.5, 0.7, 0.9} {
		want := f(frac)
		got := Hermite4(frac, xm1, x0, x1, x2)
		if !approxEqual(got, want, 1e-10) {
			t.Fatalf("t=%v: got %v want %v", frac, got, want)
		}
	}
}

// --- Lagrange4 ---

func TestLagrange4LinearRamp(t *testing.T) {
	xm1, x0, x1, x2 := -1.0, 0.0, 1.0, 2.0
	for _, frac := range []float64{0, 0.25, 0.5, 0.75, 1.0} {
		want := frac
		got := Lagrange4(frac, xm1, x0, x1, x2)
		if !approxEqual(got, want, tol) {
			t.Fatalf("t=%v: got %v want %v", frac, got, want)
		}
	}
}

func TestLagrange4CubicExact(t *testing.T) {
	// Lagrange order-3 should exactly reproduce a cubic polynomial.
	f := func(x float64) float64 { return x*x*x - 2*x*x + x - 3 }
	xm1, x0, x1, x2 := f(-1), f(0), f(1), f(2)
	for _, frac := range []float64{0.1, 0.3, 0.5, 0.7, 0.9} {
		want := f(frac)
		got := Lagrange4(frac, xm1, x0, x1, x2)
		if !approxEqual(got, want, 1e-10) {
			t.Fatalf("t=%v: got %v want %v", frac, got, want)
		}
	}
}

func TestLagrange4Endpoints(t *testing.T) {
	xm1, x0, x1, x2 := 1.0, 5.0, 9.0, 13.0
	if got := Lagrange4(0, xm1, x0, x1, x2); !approxEqual(got, x0, tol) {
		t.Fatalf("t=0: got %v want %v", got, x0)
	}
	if got := Lagrange4(1, xm1, x0, x1, x2); !approxEqual(got, x1, tol) {
		t.Fatalf("t=1: got %v want %v", got, x1)
	}
}

// --- LagrangeInterpolator (legacy) ---

func TestLagrangeInterpolator(t *testing.T) {
	l1 := NewLagrangeInterpolator(1)
	if got := l1.Interpolate([]float64{2, 4}, 0.25); got != 2.5 {
		t.Fatalf("order1 got %v want 2.5", got)
	}

	l3 := NewLagrangeInterpolator(3)
	got := l3.Interpolate([]float64{0, 1, 2, 3}, 0.5)
	if !approxEqual(got, 1.5, tol) {
		t.Fatalf("order3 got %v want 1.5", got)
	}
}

func TestLagrangeInterpolatorEmptySlice(t *testing.T) {
	l := NewLagrangeInterpolator(1)
	if got := l.Interpolate(nil, 0.5); got != 0 {
		t.Fatalf("empty: got %v want 0", got)
	}
}

func TestLagrangeInterpolatorSingleSample(t *testing.T) {
	l := NewLagrangeInterpolator(1)
	if got := l.Interpolate([]float64{42}, 0.5); got != 42 {
		t.Fatalf("single: got %v want 42", got)
	}
}

// --- Lanczos ---

func TestLanczos6DC(t *testing.T) {
	// DC signal must be reproduced exactly after normalization.
	samples := []float64{4, 4, 4, 4, 4, 4}
	for _, frac := range []float64{0, 0.25, 0.5, 0.75} {
		got := Lanczos6(frac, samples)
		if !approxEqual(got, 4.0, 1e-10) {
			t.Fatalf("t=%v: got %v want 4", frac, got)
		}
	}
}

func TestLanczos6LinearRamp(t *testing.T) {
	// Lanczos reproduces a linear ramp approximately; verify it stays
	// close (within ~10%) rather than requiring machine precision.
	samples := []float64{-2, -1, 0, 1, 2, 3}
	for _, frac := range []float64{0, 0.25, 0.5, 0.75} {
		want := frac
		got := Lanczos6(frac, samples)
		if !approxEqual(got, want, 0.1) {
			t.Fatalf("t=%v: got %v want ~%v", frac, got, want)
		}
	}
}

func TestLanczosNVariousWidths(t *testing.T) {
	// DC signal: all interpolation should return the constant.
	for _, a := range []int{2, 3, 4} {
		samples := make([]float64, 2*a)
		for i := range samples {
			samples[i] = 5.0
		}
		got := LanczosN(0.3, samples, a)
		if !approxEqual(got, 5.0, 1e-10) {
			t.Fatalf("a=%d DC: got %v want 5", a, got)
		}
	}
}

func TestLanczos6SineWave(t *testing.T) {
	// A low-frequency sine should be well-interpolated by Lanczos3.
	freq := 0.05
	frac := 0.4
	samples := make([]float64, 6)
	for i := range samples {
		samples[i] = math.Sin(2 * math.Pi * freq * float64(i-2))
	}
	got := Lanczos6(frac, samples)
	exact := math.Sin(2 * math.Pi * freq * frac)
	if !approxEqual(got, exact, 5e-3) {
		t.Fatalf("sine: got %v want %v (err=%e)", got, exact, math.Abs(got-exact))
	}
}

// --- SincInterp ---

func TestSincInterpDC(t *testing.T) {
	// A constant signal should be reproduced exactly.
	for _, n := range []int{4, 8, 16} {
		samples := make([]float64, 2*n)
		for i := range samples {
			samples[i] = 7.0
		}
		got := SincInterp(0.3, samples, n)
		if !approxEqual(got, 7.0, 1e-6) {
			t.Fatalf("n=%d DC: got %v want 7", n, got)
		}
	}
}

func TestSincInterpLinearRamp(t *testing.T) {
	n := 8
	samples := make([]float64, 2*n)
	for i := range samples {
		samples[i] = float64(i - (n - 1)) // centered ramp
	}
	for _, frac := range []float64{0, 0.25, 0.5, 0.75} {
		want := frac
		got := SincInterp(frac, samples, n)
		if !approxEqual(got, want, 1e-4) {
			t.Fatalf("t=%v: got %v want %v", frac, got, want)
		}
	}
}

func TestSincInterpVsLanczosQuality(t *testing.T) {
	// For a pure sine wave, higher-tap sinc should have lower error
	// than a 6-tap Lanczos at mid-band.
	freq := 0.1 // well below Nyquist
	frac := 0.37

	// Generate Lanczos samples (6 taps, a=3).
	lanSamples := make([]float64, 6)
	for i := range lanSamples {
		lanSamples[i] = math.Sin(2 * math.Pi * freq * float64(i-2))
	}
	lanResult := Lanczos6(frac, lanSamples)

	// Generate sinc samples (16 taps, n=8).
	n := 8
	sincSamples := make([]float64, 2*n)
	for i := range sincSamples {
		sincSamples[i] = math.Sin(2 * math.Pi * freq * float64(i-(n-1)))
	}
	sincResult := SincInterp(frac, sincSamples, n)

	exact := math.Sin(2 * math.Pi * freq * frac)
	lanErr := math.Abs(lanResult - exact)
	sincErr := math.Abs(sincResult - exact)

	if sincErr > lanErr {
		t.Fatalf("sinc16 error (%e) should be <= lanczos6 error (%e)", sincErr, lanErr)
	}
}

// --- Allpass ---

func TestAllpassCoeff(t *testing.T) {
	// t=0 => eta=1, t=1 => eta=0, t=0.5 => eta=1/3
	if got := AllpassCoeff(0); !approxEqual(got, 1, tol) {
		t.Fatalf("t=0: got %v want 1", got)
	}
	if got := AllpassCoeff(1); !approxEqual(got, 0, tol) {
		t.Fatalf("t=1: got %v want 0", got)
	}
	if got := AllpassCoeff(0.5); !approxEqual(got, 1.0/3.0, tol) {
		t.Fatalf("t=0.5: got %v want 1/3", got)
	}
}

func TestAllpassTickDCPassthrough(t *testing.T) {
	// DC signal should pass through an allpass filter with unity gain
	// once the state settles.
	state := 0.0
	dc := 5.0
	var last float64
	for i := 0; i < 100; i++ {
		last = AllpassTick(0.3, dc, dc, &state)
	}
	if !approxEqual(last, dc, 1e-6) {
		t.Fatalf("DC settled: got %v want %v", last, dc)
	}
}

func TestAllpassTickUnityMagnitude(t *testing.T) {
	// For a sine wave, the allpass filter should have approximately
	// unity magnitude response (power in ~= power out) once settled.
	state := 0.0
	freq := 0.1
	frac := 0.4
	n := 1000
	var sumSqIn, sumSqOut float64
	for i := 0; i < n; i++ {
		x0 := math.Sin(2 * math.Pi * freq * float64(i))
		x1 := math.Sin(2 * math.Pi * freq * float64(i+1))
		out := AllpassTick(frac, x0, x1, &state)
		if i >= 100 { // skip transient
			sumSqIn += x0 * x0
			sumSqOut += out * out
		}
	}
	ratio := sumSqOut / sumSqIn
	if !approxEqual(ratio, 1.0, 0.05) {
		t.Fatalf("magnitude ratio: got %v want ~1.0", ratio)
	}
}

// --- sincNormalized (internal, tested via exported functions) ---

func TestSincNormalized(t *testing.T) {
	if got := sincNormalized(0); got != 1 {
		t.Fatalf("sinc(0): got %v want 1", got)
	}
	// sinc(1) = sin(pi)/pi = 0
	if got := sincNormalized(1); !approxEqual(got, 0, tol) {
		t.Fatalf("sinc(1): got %v want 0", got)
	}
	// sinc(0.5) = sin(pi/2)/(pi/2) = 2/pi
	want := 2.0 / math.Pi
	if got := sincNormalized(0.5); !approxEqual(got, want, tol) {
		t.Fatalf("sinc(0.5): got %v want %v", got, want)
	}
}

// --- Mode enum ---

func TestModeValues(t *testing.T) {
	// Verify enum ordering is stable (useful for serialization).
	modes := []Mode{Linear, Hermite, Lagrange3, Lanczos3, Sinc, Allpass}
	for i, m := range modes {
		if int(m) != i {
			t.Fatalf("Mode %d has value %d, want %d", i, int(m), i)
		}
	}
}

// --- Benchmarks ---

func BenchmarkLinear2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Linear2(0.3, 1.0, 2.0)
	}
}

func BenchmarkHermite4(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Hermite4(0.3, -1, 0, 1, 2)
	}
}

func BenchmarkLagrange4(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Lagrange4(0.3, -1, 0, 1, 2)
	}
}

func BenchmarkLanczos6(b *testing.B) {
	s := []float64{-2, -1, 0, 1, 2, 3}
	for i := 0; i < b.N; i++ {
		Lanczos6(0.3, s)
	}
}

func BenchmarkSincInterp8(b *testing.B) {
	n := 8
	s := make([]float64, 2*n)
	for i := range s {
		s[i] = float64(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SincInterp(0.3, s, n)
	}
}

func BenchmarkAllpassTick(b *testing.B) {
	state := 0.0
	for i := 0; i < b.N; i++ {
		AllpassTick(0.3, 1.0, 2.0, &state)
	}
}
