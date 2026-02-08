package pass

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

const tol = 1e-9

func almostEqual(a, b, eps float64) bool {
	return math.Abs(a-b) <= eps
}

func mag(c biquad.Coefficients, freq, sr float64) float64 {
	h := c.Response(freq, sr)
	return cmplx.Abs(h)
}

func magChain(c *biquad.Chain, freq, sr float64) float64 {
	h := c.Response(freq, sr)
	return cmplx.Abs(h)
}

func assertFiniteCoefficients(t *testing.T, c biquad.Coefficients) {
	t.Helper()
	v := []float64{c.B0, c.B1, c.B2, c.A1, c.A2}
	for i := range v {
		if math.IsNaN(v[i]) || math.IsInf(v[i], 0) {
			t.Fatalf("invalid coefficient[%d]=%v", i, v[i])
		}
	}
}

func assertStableSection(t *testing.T, c biquad.Coefficients) {
	t.Helper()
	r1, r2 := sectionRoots(c)
	if cmplx.Abs(r1) >= 1+tol || cmplx.Abs(r2) >= 1+tol {
		t.Fatalf("unstable poles: |r1|=%v |r2|=%v coeff=%#v", cmplx.Abs(r1), cmplx.Abs(r2), c)
	}
}

func sectionRoots(c biquad.Coefficients) (complex128, complex128) {
	disc := complex(c.A1*c.A1-4*c.A2, 0)
	sqrtDisc := cmplx.Sqrt(disc)
	r1 := (-complex(c.A1, 0) + sqrtDisc) / 2
	r2 := (-complex(c.A1, 0) - sqrtDisc) / 2
	return r1, r2
}

func coeffSliceEqual(a, b []biquad.Coefficients) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !almostEqual(a[i].B0, b[i].B0, 1e-12) ||
			!almostEqual(a[i].B1, b[i].B1, 1e-12) ||
			!almostEqual(a[i].B2, b[i].B2, 1e-12) ||
			!almostEqual(a[i].A1, b[i].A1, 1e-12) ||
			!almostEqual(a[i].A2, b[i].A2, 1e-12) {
			return false
		}
	}
	return true
}

func legacyCheby1LP(freq float64, order int, ripple float64, sampleRate float64) []biquad.Coefficients {
	k := math.Tan(math.Pi * freq / sampleRate)
	k2 := k * k
	t := math.Asinh(ripple) / float64(order)
	r1 := math.Sinh(t)
	r0 := math.Cosh(t)
	r0 = r0 * r0

	out := make([]biquad.Coefficients, 0, (order+1)/2)
	for i := (order / 2) - 1; i >= 0; i-- {
		x := math.Cos(float64(2*i+1) * math.Pi / (2 * float64(order)))
		b := 1 / (r0 - x*x)
		a := k * 2 * b * r1 * x
		n := 1 / (a + b + k2)
		out = append(out, biquad.Coefficients{
			B0: k2 * n, B1: 2 * k2 * n, B2: k2 * n,
			A1: 2 * (b - k2) * n, A2: (a - k2 - b) * n,
		})
	}
	if order%2 != 0 {
		out = append(out, butterworthFirstOrderLP(freq, sampleRate))
	}
	return out
}

func legacyCheby1HP(freq float64, order int, ripple float64, sampleRate float64) []biquad.Coefficients {
	k := math.Tan(math.Pi * freq / sampleRate)
	k2 := k * k
	t := math.Asinh(ripple) / float64(order)
	r1 := math.Sinh(t)
	r0 := math.Cosh(t)
	r0 = r0 * r0

	out := make([]biquad.Coefficients, 0, (order+1)/2)
	for i := (order / 2) - 1; i >= 0; i-- {
		s := math.Sin(float64(2*i+1) * math.Pi / (4 * float64(order)))
		x := s * s
		a := 1 / (r0 + 4*x - 4*x*x - 1)
		b := 2 * k * a * r1 * (1 - 2*x)
		n := 1 / (b + 1 + a*k2)
		out = append(out, biquad.Coefficients{
			B0: n, B1: -2 * n, B2: n,
			A1: 2 * (1 - a*k2) * n, A2: (b - 1 - a*k2) * n,
		})
	}
	if order%2 != 0 {
		out = append(out, butterworthFirstOrderHP(freq, sampleRate))
	}
	return out
}

func legacyCheby2LP(freq float64, order int, ripple float64, sampleRate float64) []biquad.Coefficients {
	k := math.Tan(math.Pi * freq / sampleRate)
	k2 := k * k
	t := math.Asinh(1/ripple) / float64(order)
	r1 := math.Sinh(t)
	r0 := math.Cosh(t)
	r0 = r0 * r0

	out := make([]biquad.Coefficients, 0, (order+1)/2)
	for i := (order / 2) - 1; i >= 0; i-- {
		x := math.Cos(float64(2*i+1) / (2 * float64(order)))
		c0 := 1 - x*x
		c1 := 2 * x * r1 * k
		n := 1 / (c1 + k2 + r0 + c0)
		out = append(out, biquad.Coefficients{
			B0: (k2 + c0) * n, B1: 2 * (k2 - c0) * n, B2: (k2 + c0) * n,
			A1: 2 * (-k2 + r0 + c0) * n, A2: (c1 - k2 - r0 - c0) * n,
		})
	}
	if order%2 != 0 {
		out = append(out, butterworthFirstOrderLP(freq, sampleRate))
	}
	return out
}

func correctedCheby2LP(freq float64, order int, ripple float64, sampleRate float64) []biquad.Coefficients {
	k := math.Tan(math.Pi * freq / sampleRate)
	k2 := k * k
	t := math.Asinh(1/ripple) / float64(order)
	r1 := math.Sinh(t)
	r0 := math.Cosh(t)
	r0 = r0 * r0

	out := make([]biquad.Coefficients, 0, (order+1)/2)
	for i := (order / 2) - 1; i >= 0; i-- {
		x := math.Cos(float64(2*i+1) * math.Pi / (2 * float64(order)))
		c0 := 1 - x*x
		c1 := 2 * x * r1 * k
		n := 1 / (c1 + k2 + r0 + c0)
		out = append(out, biquad.Coefficients{
			B0: (k2 + c0) * n, B1: 2 * (k2 - c0) * n, B2: (k2 + c0) * n,
			A1: 2 * (-k2 + r0 + c0) * n, A2: (c1 - k2 - r0 - c0) * n,
		})
	}
	if order%2 != 0 {
		out = append(out, butterworthFirstOrderLP(freq, sampleRate))
	}
	return out
}

func correctedCheby2HP(freq float64, order int, ripple float64, sampleRate float64) []biquad.Coefficients {
	k := 1 / math.Tan(math.Pi*freq/sampleRate)
	k2 := k * k
	t := math.Asinh(1/ripple) / float64(order)
	r1 := math.Sinh(t)
	r0 := math.Cosh(t)
	r0 = r0 * r0

	out := make([]biquad.Coefficients, 0, (order+1)/2)
	for i := 0; i < order/2; i++ {
		x := math.Cos(float64(2*i+1) * math.Pi / (2 * float64(order)))
		c0 := 1 - x*x
		c1 := 2 * x * r1 * k
		n := 1 / (c1 + k2 + r0 + c0)
		out = append(out, biquad.Coefficients{
			B0: (c0 + k2) * n, B1: 2 * (c0 - k2) * n, B2: (c0 + k2) * n,
			A1: 2 * (k2 - r0 - c0) * n, A2: (c1 - k2 - r0 - c0) * n,
		})
	}
	if order%2 != 0 {
		out = append(out, butterworthFirstOrderHP(freq, sampleRate))
	}
	return out
}
