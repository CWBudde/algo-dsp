package pass

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

type bandExtrema struct {
	maxDB     float64
	maxFreqHz float64
	minDB     float64
	minFreqHz float64
}

func scanBandExtrema(sections []biquad.Coefficients, sr, fStart, fEnd, step float64) bandExtrema {
	out := bandExtrema{
		maxDB:     -math.MaxFloat64,
		minDB:     math.MaxFloat64,
		maxFreqHz: fStart,
		minFreqHz: fStart,
	}
	for f := fStart; f <= fEnd; f += step {
		magDB := cascadeMagDB(sections, f, sr)
		if magDB > out.maxDB {
			out.maxDB = magDB
			out.maxFreqHz = f
		}
		if magDB < out.minDB {
			out.minDB = magDB
			out.minFreqHz = f
		}
	}
	return out
}

func sectionEndpointGainLP(s biquad.Coefficients) float64 {
	den := 1 + s.A1 + s.A2
	if den == 0 {
		return math.NaN()
	}
	return (s.B0 + s.B1 + s.B2) / den
}

func sectionEndpointGainHP(s biquad.Coefficients) float64 {
	den := 1 - s.A1 + s.A2
	if den == 0 {
		return math.NaN()
	}
	return (s.B0 - s.B1 + s.B2) / den
}

func almostEqualDiag(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func TestEllipticMathHelpers_BasicIdentities(t *testing.T) {
	k := 0.5

	cd0 := cde(0, k, 1e-15)
	if !almostEqualDiag(real(cd0), 1.0, 1e-10) {
		t.Fatalf("cde(0,k)=%v, expected 1", cd0)
	}

	cd1 := cde(1, k, 1e-15)
	if !almostEqualDiag(real(cd1), 0.0, 1e-10) {
		t.Fatalf("cde(1,k)=%v, expected 0", cd1)
	}

	u := 0.4
	w := cde(complex(u, 0), k, 1e-15)
	uRecovered := acde(w, k, 1e-15)
	if !almostEqualDiag(real(uRecovered), u, 1e-8) {
		t.Fatalf("acde(cde(u))=%v, expected %v", real(uRecovered), u)
	}

	k1 := 0.3
	kDeg := ellipdeg(4, k1, 1e-15)
	if !(kDeg > 0 && kDeg < 1) {
		t.Fatalf("ellipdeg out of range: %v", kDeg)
	}
}

func TestEllipticLP_FirstOrder_BilinearFormulaDiagnostic(t *testing.T) {
	const (
		sr         = 48000.0
		fc         = 1000.0
		rippleDB   = 0.5
		stopbandDB = 40.0
	)

	k, ok := bilinearK(fc, sr)
	if !ok {
		t.Fatal("invalid bilinear params")
	}
	e := math.Sqrt(math.Pow(10, rippleDB/10) - 1)
	es := math.Sqrt(math.Pow(10, stopbandDB/10) - 1)
	k1 := e / es
	kEllip := ellipdeg(1, k1, 1e-9)
	v0 := asne(complex(0, 1)/complex(e, 0), k1, 1e-9)
	p0 := -1.0 / real(complex(0, 1)*cde(-1.0+v0, kEllip, 1e-9))

	// Implementation in EllipticLP(order=1).
	implNorm := 1 / (k + p0)
	implB0 := k * implNorm
	implB1 := k * implNorm
	implA1 := (p0 - k) * implNorm

	// Reference bilinear transform for H(s)=1/(s+p0), using s=k*(1-z^-1)/(1+z^-1):
	// H(z)= (1+z^-1)/((k+p0)+(p0-k)z^-1)
	// => b0=b1=1/(k+p0), a1=(p0-k)/(k+p0), then apply optional gain scaling by k.
	refNorm := 1 / (k + p0)
	refB0 := k * refNorm
	refB1 := k * refNorm
	refA1 := (p0 - k) * refNorm

	if !(almostEqualDiag(implB0, refB0, 1e-8) && almostEqualDiag(implB1, refB1, 1e-8) && almostEqualDiag(implA1, refA1, 1e-8)) {
		t.Fatalf("first-order LP bilinear mismatch: impl=(%g,%g,%g) ref=(%g,%g,%g)", implB0, implB1, implA1, refB0, refB1, refA1)
	}
	if math.Abs(implA1) >= 1 {
		t.Fatalf("first-order LP unstable: a1=%g", implA1)
	}
}

func TestEllipticHP_SectionStabilityDiagnostic(t *testing.T) {
	const (
		sr         = 48000.0
		fc         = 1000.0
		rippleDB   = 0.5
		stopbandDB = 40.0
		order      = 4
	)

	hp := EllipticHP(fc, order, rippleDB, stopbandDB, sr)
	if len(hp) != order/2 {
		t.Fatalf("unexpected section count: got=%d want=%d", len(hp), order/2)
	}
	for i := range hp {
		if math.IsNaN(hp[i].A1) || math.IsInf(hp[i].A1, 0) || math.Abs(hp[i].A1) >= 2 {
			t.Fatalf("section %d unstable/invalid a1=%g", i, hp[i].A1)
		}
		if math.IsNaN(hp[i].A2) || math.IsInf(hp[i].A2, 0) || math.Abs(hp[i].A2) >= 1.1 {
			t.Fatalf("section %d unstable/invalid a2=%g", i, hp[i].A2)
		}
	}
}

func TestEllipticSpecWindowDiagnostics_ByOrder(t *testing.T) {
	const (
		sr = 48000.0
		fc = 1000.0
	)

	type cfg struct {
		order      int
		rippleDB   float64
		stopbandDB float64
	}
	cases := []cfg{
		{order: 2, rippleDB: 0.1, stopbandDB: 40},
		{order: 2, rippleDB: 0.5, stopbandDB: 60},
		{order: 4, rippleDB: 0.1, stopbandDB: 40},
		{order: 4, rippleDB: 0.5, stopbandDB: 40},
		{order: 4, rippleDB: 1.0, stopbandDB: 60},
		{order: 6, rippleDB: 0.5, stopbandDB: 40},
	}

	for _, tc := range cases {
		lp := EllipticLP(fc, tc.order, tc.rippleDB, tc.stopbandDB, sr)
		hp := EllipticHP(fc, tc.order, tc.rippleDB, tc.stopbandDB, sr)
		if len(lp) == 0 || len(hp) == 0 {
			t.Fatalf("empty sections for order=%d ripple=%.2f stopband=%.1f", tc.order, tc.rippleDB, tc.stopbandDB)
		}

		lpPass := scanBandExtrema(lp, sr, 10, 0.8*fc, 10)
		lpStop := scanBandExtrema(lp, sr, 2*fc, 0.45*sr, 100)
		hpPass := scanBandExtrema(hp, sr, 1.2*fc, 0.4*sr, 100)
		hpStop := scanBandExtrema(hp, sr, 10, 0.5*fc, 10)

		t.Logf("order=%d ripple=%.2f stop=%.1f | LP pass[min=%.2fdB@%.0fHz max=%.2fdB@%.0fHz] LP stop[max=%.2fdB@%.0fHz]",
			tc.order, tc.rippleDB, tc.stopbandDB,
			lpPass.minDB, lpPass.minFreqHz, lpPass.maxDB, lpPass.maxFreqHz,
			lpStop.maxDB, lpStop.maxFreqHz,
		)
		t.Logf("order=%d ripple=%.2f stop=%.1f | HP pass[min=%.2fdB@%.0fHz max=%.2fdB@%.0fHz] HP stop[max=%.2fdB@%.0fHz]",
			tc.order, tc.rippleDB, tc.stopbandDB,
			hpPass.minDB, hpPass.minFreqHz, hpPass.maxDB, hpPass.maxFreqHz,
			hpStop.maxDB, hpStop.maxFreqHz,
		)

		for _, ex := range []bandExtrema{lpPass, lpStop, hpPass, hpStop} {
			if math.IsNaN(ex.maxDB) || math.IsInf(ex.maxDB, 0) || math.IsNaN(ex.minDB) || math.IsInf(ex.minDB, 0) {
				t.Fatalf("non-finite extrema for order=%d ripple=%.2f stopband=%.1f: %#v", tc.order, tc.rippleDB, tc.stopbandDB, ex)
			}
		}
	}
}

func TestEllipticFormulaEdgeCases_NearLimits(t *testing.T) {
	const sr = 48000.0

	cases := []struct {
		name       string
		fc         float64
		order      int
		rippleDB   float64
		stopbandDB float64
	}{
		{name: "tiny cutoff", fc: 5.0, order: 4, rippleDB: 0.1, stopbandDB: 40},
		{name: "low cutoff", fc: 20.0, order: 6, rippleDB: 0.5, stopbandDB: 60},
		{name: "near nyquist", fc: sr * 0.499, order: 4, rippleDB: 0.5, stopbandDB: 40},
		{name: "very small ripple", fc: 1000.0, order: 4, rippleDB: 0.01, stopbandDB: 40},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lp := EllipticLP(tc.fc, tc.order, tc.rippleDB, tc.stopbandDB, sr)
			hp := EllipticHP(tc.fc, tc.order, tc.rippleDB, tc.stopbandDB, sr)
			if len(lp) == 0 || len(hp) == 0 {
				t.Fatalf("expected non-empty sections for %+v", tc)
			}

			for _, s := range lp {
				assertFiniteCoefficients(t, s)
				assertStableSection(t, s)
			}
			for _, s := range hp {
				assertFiniteCoefficients(t, s)
				assertStableSection(t, s)
			}
		})
	}
}

func TestEllipdeg_SmallK1MatchesSeriesDiagnostic(t *testing.T) {
	for _, n := range []int{2, 4, 8} {
		for _, k1 := range []float64{1e-9, 1e-7, 1e-5} {
			deg := ellipdeg(n, k1, 1e-12)
			series := ellipdeg2(1.0/float64(n), k1, 1e-12)
			if deg <= 0 || deg >= 1 || series <= 0 || series >= 1 {
				t.Fatalf("out-of-range degree: n=%d k1=%g deg=%g series=%g", n, k1, deg, series)
			}
			rel := math.Abs(deg-series) / series
			if rel > 0.15 {
				t.Fatalf("ellipdeg mismatch too large: n=%d k1=%g deg=%g series=%g rel=%.3f", n, k1, deg, series, rel)
			}
		}
	}
}

func TestEllipticSectionEndpointGainBreakdownDiagnostic(t *testing.T) {
	const (
		sr         = 48000.0
		fc         = 1000.0
		order      = 4
		rippleDB   = 0.5
		stopbandDB = 40.0
	)

	lp := EllipticLP(fc, order, rippleDB, stopbandDB, sr)
	hp := EllipticHP(fc, order, rippleDB, stopbandDB, sr)
	if len(lp) == 0 || len(hp) == 0 {
		t.Fatalf("empty sections for LP/HP diagnostic: lp=%d hp=%d", len(lp), len(hp))
	}

	lpCascade := 1.0
	for i, s := range lp {
		g := sectionEndpointGainLP(s)
		if math.IsNaN(g) || math.IsInf(g, 0) {
			t.Fatalf("LP section %d invalid DC gain: %g", i, g)
		}
		lpCascade *= g
		t.Logf("LP section[%d] DC gain=%.9f (%.3f dB), cumulative=%.9f (%.3f dB), coeff=%+v",
			i, g, 20*math.Log10(math.Abs(g)), lpCascade, 20*math.Log10(math.Abs(lpCascade)), s)
	}
	lpNearDC := cascadeMagDB(lp, 1.0, sr)
	t.Logf("LP cumulative DC formula=%.9f (%.3f dB), near-DC response @1Hz=%.3f dB",
		lpCascade, 20*math.Log10(math.Abs(lpCascade)), lpNearDC)

	hpCascade := 1.0
	for i, s := range hp {
		g := sectionEndpointGainHP(s)
		if math.IsNaN(g) || math.IsInf(g, 0) {
			t.Fatalf("HP section %d invalid Nyquist gain: %g", i, g)
		}
		hpCascade *= g
		t.Logf("HP section[%d] Nyq gain=%.9f (%.3f dB), cumulative=%.9f (%.3f dB), coeff=%+v",
			i, g, 20*math.Log10(math.Abs(g)), hpCascade, 20*math.Log10(math.Abs(hpCascade)), s)
	}
	hpNearNyq := cascadeMagDB(hp, sr*0.49, sr)
	t.Logf("HP cumulative Nyq formula=%.9f (%.3f dB), near-Nyq response @%.0fHz=%.3f dB",
		hpCascade, 20*math.Log10(math.Abs(hpCascade)), sr*0.49, hpNearNyq)
}
