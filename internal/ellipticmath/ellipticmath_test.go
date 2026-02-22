package ellipticmath

import (
	"math"
	"testing"
)

func almostEqual(a, b, tol float64) bool {
	if a == b {
		return true
	}

	diff := math.Abs(a - b)
	if tol > 0 && tol < 1 {
		mag := math.Max(math.Abs(a), math.Abs(b))
		if mag > 1 {
			return diff/mag < tol
		}
	}

	return diff < tol
}

func TestLanden_Convergence(t *testing.T) {
	v := Landen(0.5, 1e-15)
	if len(v) == 0 {
		t.Fatal("Landen returned empty sequence")
	}

	last := v[len(v)-1]
	if last > 1e-15 {
		t.Fatalf("Landen did not converge: last value = %e", last)
	}

	for i := 1; i < len(v); i++ {
		if v[i] >= v[i-1] {
			t.Fatalf("Landen not monotonically decreasing at index %d: %e >= %e", i, v[i], v[i-1])
		}
	}
}

func TestLanden_Limits(t *testing.T) {
	v0 := Landen(0, 1e-15)
	if len(v0) != 1 || v0[0] != 0 {
		t.Fatalf("Landen(0) = %v, expected [0]", v0)
	}

	v1 := Landen(1, 1e-15)
	if len(v1) != 1 || v1[0] != 1 {
		t.Fatalf("Landen(1) = %v, expected [1]", v1)
	}
}

func TestLanden_FixedIterations(t *testing.T) {
	const iter = 6

	v := Landen(0.5, iter)
	if len(v) != iter {
		t.Fatalf("Landen fixed-iteration length = %d, want %d", len(v), iter)
	}

	for i := 1; i < len(v); i++ {
		if v[i] >= v[i-1] {
			t.Fatalf("fixed-iteration Landen not monotonically decreasing at index %d", i)
		}
	}
}

func TestLandenK_MatchesEllipK(t *testing.T) {
	k := 0.6
	v := Landen(k, 1e-15)
	got := LandenK(v)

	want, _ := EllipK(k, 1e-15)
	if !almostEqual(got, want, 1e-12) {
		t.Fatalf("LandenK mismatch: got=%g want=%g", got, want)
	}
}

func TestEllipK_KnownValues(t *testing.T) {
	K, Kp := EllipK(0, 1e-15)
	if !almostEqual(K, math.Pi/2, 1e-10) {
		t.Fatalf("K(0) = %v, expected pi/2 = %v", K, math.Pi/2)
	}

	if !math.IsInf(Kp, 1) {
		t.Fatalf("K'(0) = %v, expected +Inf", Kp)
	}

	K1, _ := EllipK(1, 1e-15)
	if !math.IsInf(K1, 1) {
		t.Fatalf("K(1) = %v, expected +Inf", K1)
	}
}

func TestEllipK_SymmetryRelation(t *testing.T) {
	k := 0.6
	kp := math.Sqrt(1 - k*k)
	K, Kprime := EllipK(k, 1e-15)
	Kkp, Kpkp := EllipK(kp, 1e-15)
	ratio1 := K / Kprime

	ratio2 := Kpkp / Kkp
	if !almostEqual(ratio1, ratio2, 1e-8) {
		t.Fatalf("symmetry: K/K' = %v, K'(k')/K(k') = %v", ratio1, ratio2)
	}
}

func TestEllipKReuse_MatchesEllipK(t *testing.T) {
	k := 0.7
	v := Landen(k, 1e-15)
	K1, Kp1 := EllipK(k, 1e-15)

	K2, Kp2 := EllipKReuse(k, 1e-15, v)
	if !almostEqual(K1, K2, 1e-12) || !almostEqual(Kp1, Kp2, 1e-12) {
		t.Fatalf("EllipKReuse mismatch: direct=(%g,%g) reuse=(%g,%g)", K1, Kp1, K2, Kp2)
	}
}

func TestCDE_RealInputRange(t *testing.T) {
	k := 0.5

	for _, uVal := range []float64{0.1, 0.3, 0.5, 0.7, 0.9} {
		u := complex(uVal, 0)

		cd := CDE(u, k, 1e-15)
		if math.Abs(imag(cd)) > 1e-10 {
			t.Fatalf("CDE(%v, %v): imaginary part = %v, expected ~0", uVal, k, imag(cd))
		}

		cdReal := real(cd)
		if cdReal < -0.01 || cdReal > 1.01 {
			t.Fatalf("CDE(%v, %v) = %v, outside expected range [0,1]", uVal, k, cdReal)
		}
	}
}

func TestCDE_Endpoints(t *testing.T) {
	k := 0.7

	cd0 := CDE(0, k, 1e-15)
	if !almostEqual(real(cd0), 1.0, 1e-10) {
		t.Fatalf("CDE(0, %v) = %v, expected 1", k, cd0)
	}

	cd1 := CDE(1, k, 1e-15)
	if !almostEqual(real(cd1), 0.0, 1e-10) {
		t.Fatalf("CDE(1, %v) = %v, expected 0", k, cd1)
	}
}

func TestACDE_InverseOfCDE(t *testing.T) {
	k := 0.5

	for _, uVal := range []float64{0.2, 0.5, 0.8} {
		u := complex(uVal, 0)
		w := CDE(u, k, 1e-15)

		uRecovered := ACDE(w, k, 1e-15)
		if !almostEqual(real(uRecovered), uVal, 1e-8) {
			t.Fatalf("ACDE(CDE(%v)) = %v, expected %v", uVal, real(uRecovered), uVal)
		}

		if math.Abs(imag(uRecovered)) > 1e-8 {
			t.Fatalf("ACDE(CDE(%v)): imag = %v, expected ~0", uVal, imag(uRecovered))
		}
	}
}

func TestSNE_Endpoints(t *testing.T) {
	k := 0.5

	s0 := SNE([]float64{0}, k, 1e-15)
	if !almostEqual(s0[0], 0.0, 1e-10) {
		t.Fatalf("SNE(0) = %v, expected 0", s0[0])
	}

	s1 := SNE([]float64{1}, k, 1e-15)
	if !almostEqual(s1[0], 1.0, 1e-10) {
		t.Fatalf("SNE(1) = %v, expected 1", s1[0])
	}
}

func TestEllipDeg_Order2(t *testing.T) {
	k1 := 0.5

	k := EllipDeg(2, k1, 1e-15)
	if k <= 0 || k >= 1 {
		t.Fatalf("EllipDeg(2, 0.5) = %v, expected in (0,1)", k)
	}

	K, Kp := EllipK(k, 1e-15)
	K1, K1p := EllipK(k1, 1e-15)
	lhs := float64(2) * Kp / K

	rhs := K1p / K1
	if !almostEqual(lhs, rhs, 1e-6) {
		t.Fatalf("degree equation: N*K'/K=%v, K1'/K1=%v", lhs, rhs)
	}
}

func TestEllipDeg_Order4(t *testing.T) {
	k1 := 0.3

	k := EllipDeg(4, k1, 1e-15)
	if k <= 0 || k >= 1 {
		t.Fatalf("EllipDeg(4, 0.3) = %v, expected in (0,1)", k)
	}

	K, Kp := EllipK(k, 1e-15)
	K1, K1p := EllipK(k1, 1e-15)
	lhs := float64(4) * Kp / K

	rhs := K1p / K1
	if !almostEqual(lhs, rhs, 1e-5) {
		t.Fatalf("degree equation: N*K'/K=%v, K1'/K1=%v", lhs, rhs)
	}
}

func TestEllipDeg_SmallK1MatchesSeries(t *testing.T) {
	for _, n := range []int{2, 4, 8} {
		for _, k1 := range []float64{1e-9, 1e-7, 1e-5} {
			deg := EllipDeg(n, k1, 1e-12)

			series := EllipDeg2(1.0/float64(n), k1, 1e-12)
			if deg <= 0 || deg >= 1 || series <= 0 || series >= 1 {
				t.Fatalf("out-of-range degree: n=%d k1=%g deg=%g series=%g", n, k1, deg, series)
			}

			rel := math.Abs(deg-series) / series
			if rel > 0.15 {
				t.Fatalf("EllipDeg mismatch too large: n=%d k1=%g deg=%g series=%g rel=%.3f", n, k1, deg, series, rel)
			}
		}
	}
}

func TestSymmetricRemainder(t *testing.T) {
	tests := []struct {
		x, y, want float64
	}{
		{0.5, 4, 0.5},
		{-0.5, 4, -0.5},
		{5.0, 4, 1.0},
		{-5.0, 4, -1.0},
	}
	for _, tt := range tests {
		got := SymmetricRemainder(tt.x, tt.y)
		if !almostEqual(got, tt.want, 1e-12) {
			t.Fatalf("SymmetricRemainder(%v, %v) = %v, want %v", tt.x, tt.y, got, tt.want)
		}
	}
}
