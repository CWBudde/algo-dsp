package design

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

func TestBilinearTransform_NormalizesA0(t *testing.T) {
	got := BilinearTransform([3]float64{1, 1, 1}, 48000)
	if !almostEqual(got[0], 1, 1e-12) {
		t.Fatalf("got a0=%v, want 1", got[0])
	}

	for i := range got {
		if math.IsNaN(got[i]) || math.IsInf(got[i], 0) {
			t.Fatalf("coef[%d] invalid: %v", i, got[i])
		}
	}
}

func TestBiquadDesigners_BasicResponseShape(t *testing.T) {
	sr := 48000.0
	f := 1000.0
	q := 1 / math.Sqrt2

	lp := Lowpass(f, q, sr)
	if !(mag(lp, 100, sr) > mag(lp, 10000, sr)) {
		t.Fatal("lowpass shape check failed")
	}

	hp := Highpass(f, q, sr)
	if !(mag(hp, 10000, sr) > mag(hp, 100, sr)) {
		t.Fatal("highpass shape check failed")
	}

	bp := Bandpass(f, q, sr)
	if !(mag(bp, f, sr) > mag(bp, 100, sr) && mag(bp, f, sr) > mag(bp, 10000, sr)) {
		t.Fatal("bandpass shape check failed")
	}

	n := Notch(f, q, sr)
	if !(mag(n, f, sr) < mag(n, 100, sr) && mag(n, f, sr) < mag(n, 10000, sr)) {
		t.Fatal("notch shape check failed")
	}

	ap := Allpass(f, q, sr)
	for _, hz := range []float64{100, 500, 1000, 5000, 10000} {
		if !almostEqual(mag(ap, hz, sr), 1, 1e-6) {
			t.Fatalf("allpass magnitude at %v Hz = %v, want ~1", hz, mag(ap, hz, sr))
		}
	}
}

func TestEQDesigners_BasicBehavior(t *testing.T) {
	sampleRate := 48000.0
	freq := 1000.0
	q := 1.0

	peakUp := Peak(freq, 6, q, sampleRate)

	peakDown := Peak(freq, -6, q, sampleRate)
	if !(mag(peakUp, freq, sampleRate) > 1 && mag(peakDown, freq, sampleRate) < 1) {
		t.Fatal("peak filter gain check failed")
	}

	ls := LowShelf(500, 6, q, sampleRate)
	if !(mag(ls, 100, sampleRate) > mag(ls, 10000, sampleRate)) {
		t.Fatal("low shelf tilt check failed")
	}

	hs := HighShelf(4000, 6, q, sampleRate)
	if !(mag(hs, 10000, sampleRate) > mag(hs, 100, sampleRate)) {
		t.Fatal("high shelf tilt check failed")
	}
}

func TestDesigners_ValidateAcrossSampleRates(t *testing.T) {
	for _, sr := range []float64{44100, 48000, 96000, 192000} {
		for _, c := range []biquad.Coefficients{
			Lowpass(1000, 0.707, sr),
			Highpass(1000, 0.707, sr),
			Bandpass(1000, 1.2, sr),
			Notch(1000, 1.2, sr),
			Allpass(1000, 1.2, sr),
			Peak(1000, 3, 1.0, sr),
			LowShelf(300, 6, 1.0, sr),
			HighShelf(3000, -6, 1.0, sr),
		} {
			assertFiniteCoefficients(t, c)
			assertStableSection(t, c)
		}
	}
}

func TestButterworthLP_OrderAndShape(t *testing.T) {
	sr := 48000.0

	coeffs := ButterworthLP(1000, 5, sr)
	if len(coeffs) != 3 {
		t.Fatalf("len=%d, want 3", len(coeffs))
	}

	if coeffs[len(coeffs)-1].A2 != 0 || coeffs[len(coeffs)-1].B2 != 0 {
		t.Fatalf("expected final first-order section, got %#v", coeffs[len(coeffs)-1])
	}

	for _, c := range coeffs {
		assertStableSection(t, c)
	}

	chain := biquad.NewChain(coeffs)
	if !(magChain(chain, 100, sr) > magChain(chain, 10000, sr)) {
		t.Fatal("ButterworthLP response shape check failed")
	}
}

func TestButterworthHP_OrderAndShape(t *testing.T) {
	sampleRate := 48000.0

	coeffs := ButterworthHP(1000, 5, sampleRate)
	if len(coeffs) != 3 {
		t.Fatalf("len=%d, want 3", len(coeffs))
	}

	if coeffs[len(coeffs)-1].A2 != 0 || coeffs[len(coeffs)-1].B2 != 0 {
		t.Fatalf("expected final first-order section, got %#v", coeffs[len(coeffs)-1])
	}

	for _, c := range coeffs {
		assertStableSection(t, c)
	}

	chain := biquad.NewChain(coeffs)
	if !(magChain(chain, 10000, sampleRate) > magChain(chain, 100, sampleRate)) {
		t.Fatal("ButterworthHP response shape check failed")
	}
}

func TestChebyshev1ParityWithRefFormulas(t *testing.T) {
	sampleRate := 48000.0
	freq := 1000.0
	order := 4
	ripple := 2.0

	c1lp := Chebyshev1LP(freq, order, ripple, sampleRate)
	c1hp := Chebyshev1HP(freq, order, ripple, sampleRate)
	ref1lp := refCheby1LP(freq, order, ripple, sampleRate)
	ref1hp := refCheby1HP(freq, order, ripple, sampleRate)

	if !coeffSliceEqual(c1lp, ref1lp) {
		t.Fatal("Chebyshev1LP parity mismatch")
	}

	if !coeffSliceEqual(c1hp, ref1hp) {
		t.Fatal("Chebyshev1HP parity mismatch")
	}
}

func TestChebyshevResponseShape(t *testing.T) {
	sr := 48000.0
	freq := 1000.0
	order := 4
	ripple := 2.0

	c1lp := biquad.NewChain(Chebyshev1LP(freq, order, ripple, sr))
	c1hp := biquad.NewChain(Chebyshev1HP(freq, order, ripple, sr))

	if !(magChain(c1lp, 100, sr) > magChain(c1lp, 10000, sr)) {
		t.Fatal("Chebyshev1LP shape check failed")
	}

	if !(magChain(c1hp, 10000, sr) > magChain(c1hp, 100, sr)) {
		t.Fatal("Chebyshev1HP shape check failed")
	}
}

func TestChebyshev2ParityWithRefFormulas(t *testing.T) {
	sr := 48000.0
	freq := 1000.0
	order := 4
	ripple := 2.0

	gotLP := Chebyshev2LP(freq, order, ripple, sr)
	gotHP := Chebyshev2HP(freq, order, ripple, sr)

	refLP := refCheby2LP(freq, order, ripple, sr)
	refHP := refCheby2HP(freq, order, ripple, sr)

	if !coeffSliceEqual(gotLP, refLP) {
		t.Fatal("Chebyshev2LP parity mismatch")
	}

	if !coeffSliceEqual(gotHP, refHP) {
		t.Fatal("Chebyshev2HP parity mismatch")
	}
}

func TestChebyshev2FiniteResponses(t *testing.T) {
	for _, sr := range []float64{44100, 48000, 96000, 192000} {
		for _, order := range []int{3, 4, 5} {
			lp := Chebyshev2LP(1000, order, 2, sr)

			hp := Chebyshev2HP(1000, order, 2, sr)
			if len(lp) == 0 || len(hp) == 0 {
				t.Fatalf("expected non-empty sections for sr=%v order=%d", sr, order)
			}

			for _, s := range lp {
				assertFiniteCoefficients(t, s)
			}

			for _, s := range hp {
				assertFiniteCoefficients(t, s)
			}

			chainLP := biquad.NewChain(lp)
			chainHP := biquad.NewChain(hp)

			for _, f := range []float64{10, 100, 1000, 5000, 10000, 20000} {
				if m := magChain(chainLP, f, sr); math.IsNaN(m) || math.IsInf(m, 0) {
					t.Fatalf("invalid Chebyshev2LP response at sr=%v order=%d f=%v", sr, order, f)
				}

				if m := magChain(chainHP, f, sr); math.IsNaN(m) || math.IsInf(m, 0) {
					t.Fatalf("invalid Chebyshev2HP response at sr=%v order=%d f=%v", sr, order, f)
				}
			}
		}
	}
}

func TestInvalidInputs(t *testing.T) {
	if got := Lowpass(1000, 0.707, 0); got != (biquad.Coefficients{}) {
		t.Fatalf("expected zero coefficients for invalid sample rate, got %#v", got)
	}

	if got := Highpass(0, 0.707, 48000); got != (biquad.Coefficients{}) {
		t.Fatalf("expected zero coefficients for invalid frequency, got %#v", got)
	}

	_ = Bandpass(1000, 0, 48000) // q<=0 path uses defaultQ
	_ = Notch(1000, -1, 48000)   // q<=0 path uses defaultQ
	_ = Allpass(1000, 0, 48000)  // q<=0 path uses defaultQ
	_ = Peak(1000, 3, 0, 48000)  // q<=0 path uses defaultQ
	_ = LowShelf(1000, 3, 0, 48000)
	_ = HighShelf(1000, 3, 0, 48000)

	if got := BilinearTransform([3]float64{1, 1, 1}, 0); got != ([3]float64{1, 0, 0}) {
		t.Fatalf("unexpected bilinear fallback: %#v", got)
	}

	if got := BilinearTransform([3]float64{0, 0, 0}, 48000); got != ([3]float64{1, 0, 0}) {
		t.Fatalf("unexpected bilinear zero-poly fallback: %#v", got)
	}

	if got := ButterworthLP(1000, 0, 48000); got != nil {
		t.Fatalf("expected nil for order <= 0, got %#v", got)
	}

	if got := ButterworthHP(1000, 0, 48000); got != nil {
		t.Fatalf("expected nil for order <= 0, got %#v", got)
	}
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

// refCheby1LP is a reference Chebyshev Type I lowpass implementation with
// corrected sign convention (A1/A2 negated for subtraction-based biquad engine).
func refCheby1LP(freq float64, order int, ripple float64, sampleRate float64) []biquad.Coefficients {
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
			A1: -2 * (b - k2) * n, A2: -(a - k2 - b) * n,
		})
	}

	if order%2 != 0 {
		out = append(out, butterworthFirstOrderLP(freq, sampleRate))
	}

	return out
}

// refCheby1HP is a reference Chebyshev Type I highpass implementation with
// corrected sign convention.
func refCheby1HP(freq float64, order int, ripple float64, sampleRate float64) []biquad.Coefficients {
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
			A1: -2 * (1 - a*k2) * n, A2: -(b - 1 - a*k2) * n,
		})
	}

	if order%2 != 0 {
		out = append(out, butterworthFirstOrderHP(freq, sampleRate))
	}

	return out
}

// refCheby2LP is a reference Chebyshev Type II lowpass using analog prototype
// (inverted Chebyshev I poles + imaginary-axis zeros) with bilinear transform.
func refCheby2LP(freq float64, order int, ripple float64, sampleRate float64) []biquad.Coefficients {
	wc := math.Tan(math.Pi * freq / sampleRate)

	if ripple <= 0 {
		ripple = 1
	}

	mu := math.Asinh(ripple) / float64(order)
	out := make([]biquad.Coefficients, 0, (order+1)/2)

	for i := 0; i < order/2; i++ {
		phi := math.Pi * float64(2*i+1) / float64(2*order)
		sigma1 := math.Sinh(mu) * math.Sin(phi)
		omega1 := math.Cosh(mu) * math.Cos(phi)
		pmag2 := sigma1*sigma1 + omega1*omega1
		sigP := sigma1 / pmag2
		omP := omega1 / pmag2
		omZ := 1.0 / math.Cos(phi)

		wpr := wc * sigP
		wz := wc * omZ
		wp2 := wpr*wpr + (wc*omP)*(wc*omP)
		wz2 := wz * wz

		bn0, bn1, bn2 := 1+wz2, -2+2*wz2, 1+wz2
		ad0 := 1 + 2*wpr + wp2
		ad1 := -2 + 2*wp2
		ad2 := 1 - 2*wpr + wp2

		b0, b1, b2 := bn0/ad0, bn1/ad0, bn2/ad0
		a1, a2 := ad1/ad0, ad2/ad0
		dc := (b0 + b1 + b2) / (1 + a1 + a2)
		b0 /= dc
		b1 /= dc
		b2 /= dc
		out = append(out, biquad.Coefficients{B0: b0, B1: b1, B2: b2, A1: a1, A2: a2})
	}

	if order%2 != 0 {
		sp := wc / math.Sinh(mu)
		g := sp / (1 + sp)
		out = append(out, biquad.Coefficients{B0: g, B1: g, A1: (sp - 1) / (1 + sp)})
	}

	return out
}

// refCheby2HP is a reference Chebyshev Type II highpass using LP-to-HP analog
// transform followed by bilinear transform.
func refCheby2HP(freq float64, order int, ripple float64, sampleRate float64) []biquad.Coefficients {
	wc := math.Tan(math.Pi * freq / sampleRate)

	if ripple <= 0 {
		ripple = 1
	}

	mu := math.Asinh(ripple) / float64(order)
	out := make([]biquad.Coefficients, 0, (order+1)/2)

	for i := 0; i < order/2; i++ {
		phi := math.Pi * float64(2*i+1) / float64(2*order)
		sigma1 := math.Sinh(mu) * math.Sin(phi)
		omega1 := math.Cosh(mu) * math.Cos(phi)

		hpSig := wc * sigma1
		hpOm := wc * omega1
		hpWz := wc * math.Cos(phi)

		hp2 := hpSig*hpSig + hpOm*hpOm
		wz2 := hpWz * hpWz

		bn0, bn1, bn2 := 1+wz2, -2+2*wz2, 1+wz2
		ad0 := 1 + 2*hpSig + hp2
		ad1 := -2 + 2*hp2
		ad2 := 1 - 2*hpSig + hp2

		b0, b1, b2 := bn0/ad0, bn1/ad0, bn2/ad0
		a1, a2 := ad1/ad0, ad2/ad0
		nyq := (b0 - b1 + b2) / (1 - a1 + a2)
		b0 /= nyq
		b1 /= nyq
		b2 /= nyq
		out = append(out, biquad.Coefficients{B0: b0, B1: b1, B2: b2, A1: a1, A2: a2})
	}

	if order%2 != 0 {
		sp := wc * math.Sinh(mu)
		g := 1.0 / (1 + sp)
		out = append(out, biquad.Coefficients{B0: g, B1: -g, A1: (sp - 1) / (1 + sp)})
	}

	return out
}

// butterworthFirstOrderLP and butterworthFirstOrderHP are test helpers
// used by legacy test reference implementations
func butterworthFirstOrderLP(freq, sampleRate float64) biquad.Coefficients {
	if sampleRate <= 0 || freq <= 0 || freq >= sampleRate/2 {
		return biquad.Coefficients{}
	}

	k := math.Tan(math.Pi * freq / sampleRate)
	norm := 1 / (1 + k)

	return biquad.Coefficients{
		B0: k * norm,
		B1: k * norm,
		B2: 0,
		A1: (k - 1) * norm,
		A2: 0,
	}
}

func butterworthFirstOrderHP(freq, sampleRate float64) biquad.Coefficients {
	if sampleRate <= 0 || freq <= 0 || freq >= sampleRate/2 {
		return biquad.Coefficients{}
	}

	k := math.Tan(math.Pi * freq / sampleRate)
	norm := 1 / (1 + k)

	return biquad.Coefficients{
		B0: norm,
		B1: -norm,
		B2: 0,
		A1: (k - 1) * norm,
		A2: 0,
	}
}
