package band

import (
	"math"
	"math/cmplx"
	"testing"
)

func TestLanden_Convergence(t *testing.T) {
	v := landen(0.5, 1e-15)
	if len(v) == 0 {
		t.Fatal("landen returned empty sequence")
	}
	last := v[len(v)-1]
	if last > 1e-15 {
		t.Errorf("landen did not converge: last value = %e", last)
	}
	for i := 1; i < len(v); i++ {
		if v[i] >= v[i-1] {
			t.Errorf("landen not monotonically decreasing at index %d: %e >= %e", i, v[i], v[i-1])
		}
	}
}

func TestLanden_Limits(t *testing.T) {
	v0 := landen(0, 1e-15)
	if len(v0) != 1 || v0[0] != 0 {
		t.Errorf("landen(0) = %v, expected [0]", v0)
	}
	v1 := landen(1, 1e-15)
	if len(v1) != 1 || v1[0] != 1 {
		t.Errorf("landen(1) = %v, expected [1]", v1)
	}
}

func TestEllipk_KnownValues(t *testing.T) {
	K, Kp := ellipk(0, 1e-15)
	if !almostEqual(K, math.Pi/2, 1e-10) {
		t.Errorf("K(0) = %v, expected pi/2 = %v", K, math.Pi/2)
	}
	if !math.IsInf(Kp, 1) {
		t.Errorf("K'(0) = %v, expected +Inf", Kp)
	}

	K1, _ := ellipk(1, 1e-15)
	if !math.IsInf(K1, 1) {
		t.Errorf("K(1) = %v, expected +Inf", K1)
	}
}

func TestEllipk_SymmetryRelation(t *testing.T) {
	k := 0.6
	kp := math.Sqrt(1 - k*k)
	K, Kprime := ellipk(k, 1e-15)
	Kkp, Kpkp := ellipk(kp, 1e-15)
	ratio1 := K / Kprime
	ratio2 := Kpkp / Kkp
	if !almostEqual(ratio1, ratio2, 1e-8) {
		t.Errorf("symmetry: K/K' = %v, K'(k')/K(k') = %v", ratio1, ratio2)
	}
}

func TestCde_Sne_InverseRelation(t *testing.T) {
	k := 0.5
	for _, uVal := range []float64{0.1, 0.3, 0.5, 0.7, 0.9} {
		u := complex(uVal, 0)
		cd := cde(u, k, 1e-15)
		cdImag := imag(cd)
		if math.Abs(cdImag) > 1e-10 {
			t.Errorf("cde(%v, %v): imaginary part = %v, expected ~0", uVal, k, cdImag)
		}
		cdReal := real(cd)
		if cdReal < -0.01 || cdReal > 1.01 {
			t.Errorf("cde(%v, %v) = %v, outside expected range [0,1]", uVal, k, cdReal)
		}
	}
}

func TestCde_Endpoints(t *testing.T) {
	k := 0.7
	cd0 := cde(0, k, 1e-15)
	if !almostEqual(real(cd0), 1.0, 1e-10) {
		t.Errorf("cde(0, %v) = %v, expected 1", k, cd0)
	}
	cd1 := cde(1, k, 1e-15)
	if !almostEqual(real(cd1), 0.0, 1e-10) {
		t.Errorf("cde(1, %v) = %v, expected 0", k, cd1)
	}
}

func TestAcde_Asne_InverseOfCde(t *testing.T) {
	k := 0.5
	for _, uVal := range []float64{0.2, 0.5, 0.8} {
		u := complex(uVal, 0)
		w := cde(u, k, 1e-15)
		uRecovered := acde(w, k, 1e-15)
		if !almostEqual(real(uRecovered), uVal, 1e-8) {
			t.Errorf("acde(cde(%v)) = %v, expected %v", uVal, real(uRecovered), uVal)
		}
		if math.Abs(imag(uRecovered)) > 1e-8 {
			t.Errorf("acde(cde(%v)): imag = %v, expected ~0", uVal, imag(uRecovered))
		}
	}
}

func TestSne_Endpoints(t *testing.T) {
	k := 0.5
	s0 := sne([]float64{0}, k, 1e-15)
	if !almostEqual(s0[0], 0.0, 1e-10) {
		t.Errorf("sne(0) = %v, expected 0", s0[0])
	}
	s1 := sne([]float64{1}, k, 1e-15)
	if !almostEqual(s1[0], 1.0, 1e-10) {
		t.Errorf("sne(1) = %v, expected 1", s1[0])
	}
}

func TestEllipdeg_Order2(t *testing.T) {
	k1 := 0.5
	k := ellipdeg(2, k1, 1e-15)
	if k <= 0 || k >= 1 {
		t.Errorf("ellipdeg(2, 0.5) = %v, expected in (0,1)", k)
	}
	K, Kp := ellipk(k, 1e-15)
	K1, K1p := ellipk(k1, 1e-15)
	lhs := float64(2) * Kp / K
	rhs := K1p / K1
	if !almostEqual(lhs, rhs, 1e-6) {
		t.Errorf("degree equation: N*K'/K=%v, K1'/K1=%v", lhs, rhs)
	}
}

func TestEllipdeg_Order4(t *testing.T) {
	k1 := 0.3
	k := ellipdeg(4, k1, 1e-15)
	if k <= 0 || k >= 1 {
		t.Errorf("ellipdeg(4, 0.3) = %v, expected in (0,1)", k)
	}
	K, Kp := ellipk(k, 1e-15)
	K1, K1p := ellipk(k1, 1e-15)
	lhs := float64(4) * Kp / K
	rhs := K1p / K1
	if !almostEqual(lhs, rhs, 1e-5) {
		t.Errorf("degree equation: N*K'/K=%v, K1'/K1=%v", lhs, rhs)
	}
}

func TestSrem(t *testing.T) {
	tests := []struct {
		x, y, want float64
	}{
		{0.5, 4, 0.5},
		{-0.5, 4, -0.5},
		{5.0, 4, 1.0},
		{-5.0, 4, -1.0},
	}
	for _, tt := range tests {
		got := srem(tt.x, tt.y)
		if !almostEqual(got, tt.want, 1e-12) {
			t.Errorf("srem(%v, %v) = %v, want %v", tt.x, tt.y, got, tt.want)
		}
	}
}

func TestIsZero(t *testing.T) {
	if !isZero(0) {
		t.Error("isZero(0) should be true")
	}
	if !isZero(1e-13) {
		t.Error("isZero(1e-13) should be true")
	}
	if isZero(1e-11) {
		t.Error("isZero(1e-11) should be false")
	}
}

func TestBlt_GainOnlySection(t *testing.T) {
	sections := []soSection{{b0: 2.5, a0: 1, b1: 0, b2: 0, a1: 0, a2: 0}}
	w0 := 2 * math.Pi * 1000 / testSR
	fo := blt(sections, w0)
	if len(fo) != 1 {
		t.Fatalf("expected 1 section, got %d", len(fo))
	}
	if !almostEqual(fo[0].b[0], 2.5, 1e-12) {
		t.Errorf("gain section b[0] = %v, expected 2.5", fo[0].b[0])
	}
	if !almostEqual(fo[0].a[0], 1.0, 1e-12) {
		t.Errorf("gain section a[0] = %v, expected 1.0", fo[0].a[0])
	}
}

func TestBlt_AllSectionsProcessed(t *testing.T) {
	sections := make([]soSection, 5)
	for i := range sections {
		v := float64(i + 1)
		sections[i] = soSection{
			b0: v, b1: v * 0.1, b2: v * 0.01,
			a0: 1, a1: 0.2 * v, a2: 0.03 * v,
		}
	}
	w0 := 2 * math.Pi * 1000 / testSR
	fo := blt(sections, w0)
	if len(fo) != 5 {
		t.Fatalf("expected 5 output sections, got %d", len(fo))
	}
	for i, s := range fo {
		allZero := true
		for j := 0; j < 5; j++ {
			if !isZero(s.b[j]) || !isZero(s.a[j]) {
				allZero = false
				break
			}
		}
		if allZero {
			t.Errorf("section %d has all-zero coefficients; blt did not process it", i)
		}
	}
}

// ============================================================
// Elliptic band designer diagnostics
// ============================================================

func TestEllipticBandRad_Diagnose(t *testing.T) {
	w0 := 2 * math.Pi * 1000 / testSR
	wb := 2 * math.Pi * 500 / testSR
	gainDB := 12.0
	gbDB := ellipticBWGainDB(gainDB)
	order := 4

	t.Logf("w0=%.6f wb=%.6f gainDB=%.1f gbDB=%.6f order=%d", w0, wb, gainDB, gbDB, order)

	G0 := db2Lin(0)
	G := db2Lin(gainDB)
	Gb := db2Lin(gbDB)
	Gs := db2Lin(gainDB - gbDB)

	t.Logf("G0=%.6f G=%.6f Gb=%.6f Gs=%.6f", G0, G, Gb, Gs)

	WB := math.Tan(wb / 2.0)
	eVal := math.Sqrt((G*G - Gb*Gb) / (Gb*Gb - G0*G0))
	es := math.Sqrt((G*G - Gs*Gs) / (Gs*Gs - G0*G0))
	k1 := eVal / es

	t.Logf("WB=%.10f e=%.10f es=%.10f k1=%.10f", WB, eVal, es, k1)

	if k1 <= 0 || k1 >= 1 {
		t.Logf("k1=%.10f is outside (0,1) - this will cause problems!", k1)
	}

	k := ellipdeg(order, k1, 2.2e-16)
	t.Logf("k (from ellipdeg) = %.10f", k)

	// Try computing ju0 and jv0
	ju0 := asne(complex(0, 1)*complex(G/(eVal*G0), 0), k1, 2.2e-16) / complex(float64(order), 0)
	jv0 := asne(complex(0, 1)/complex(eVal, 0), k1, 2.2e-16) / complex(float64(order), 0)
	t.Logf("ju0 = (%.10f, %.10f)", real(ju0), imag(ju0))
	t.Logf("jv0 = (%.10f, %.10f)", real(jv0), imag(jv0))

	// Try computing zeros/poles for first section
	L := order / 2
	for i := 1; i <= L; i++ {
		ui := (2.0*float64(i) - 1.0) / float64(order)
		zeros := complex(0, 1) * cde(complex(ui, 0)-ju0, k, 2.2e-16)
		poles := complex(0, 1) * cde(complex(ui, 0)-jv0, k, 2.2e-16)
		t.Logf("Section %d (ui=%.4f): zeros=(%.10f, %.10f) poles=(%.10f, %.10f)",
			i, ui, real(zeros), imag(zeros), real(poles), imag(poles))

		invZero := 1.0 / zeros
		invPole := 1.0 / poles
		t.Logf("  1/zeros=(%.10f, %.10f) 1/poles=(%.10f, %.10f)",
			real(invZero), imag(invZero), real(invPole), imag(invPole))
	}

	// Reproduce the internal pipeline to trace the failure point.
	_ = WB // reuse WB from above
	rr := order % 2
	LL := (order - rr) / 2

	var aSections []soSection
	if rr == 0 {
		aSections = append(aSections, soSection{b0: Gb, b1: 0, b2: 0, a0: 1, a1: 0, a2: 0})
	}
	for i := 1; i <= LL; i++ {
		ui := (2.0*float64(i) - 1.0) / float64(order)
		zeros := complex(0, 1) * cde(complex(ui, 0)-ju0, k, 2.2e-16)
		poles := complex(0, 1) * cde(complex(ui, 0)-jv0, k, 2.2e-16)
		invZero := 1.0 / zeros
		invPole := 1.0 / poles
		zre := real(invZero)
		pre := real(invPole)
		zabs := cmplx.Abs(invZero)
		pabs := cmplx.Abs(invPole)
		sa := soSection{
			b0: WB * WB, b1: -2 * WB * zre, b2: zabs * zabs,
			a0: WB * WB, a1: -2 * WB * pre, a2: pabs * pabs,
		}
		aSections = append(aSections, sa)
		t.Logf("Analog section %d: b=[%.6e, %.6e, %.6e] a=[%.6e, %.6e, %.6e]",
			i, sa.b0, sa.b1, sa.b2, sa.a0, sa.a1, sa.a2)
	}

	t.Logf("Total analog sections: %d (rr=%d, LL=%d)", len(aSections), rr, LL)

	foSections := blt(aSections, w0)
	for i, s := range foSections {
		t.Logf("FO section %d: b=[%.6e, %.6e, %.6e, %.6e, %.6e]  a=[%.6e, %.6e, %.6e, %.6e, %.6e]",
			i, s.b[0], s.b[1], s.b[2], s.b[3], s.b[4], s.a[0], s.a[1], s.a[2], s.a[3], s.a[4])

		biquads, err := splitFOSection(s.b, s.a)
		if err != nil {
			t.Logf("  splitFOSection[%d] FAILED: %v", i, err)
		} else {
			t.Logf("  splitFOSection[%d] OK: %d biquads", i, len(biquads))
		}
	}

	// Try the full pipeline
	sections, err := ellipticBandRad(w0, wb, gainDB, gbDB, order)
	if err != nil {
		t.Logf("ellipticBandRad FAILED: %v", err)
	} else {
		t.Logf("ellipticBandRad SUCCESS: %d sections", len(sections))
	}
}
